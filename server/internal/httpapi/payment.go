package httpapi

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"netx/server/internal/store"
)

const defaultAppBaseURL = "https://netx.beisi.tech"

var paymentPlans = map[string]int{
	"极速版": 50000,
	"至尊版": 80000,
}

func (s *Server) handleCreateAlipayPayment(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Plan string `json:"plan"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	amountFen, ok := paymentPlans[strings.TrimSpace(req.Plan)]
	if !ok {
		fail(w, http.StatusBadRequest, "套餐类型不合法")
		return
	}

	settings, err := s.store.Settings()
	if err != nil {
		fail(w, http.StatusInternalServerError, "读取支付配置失败")
		return
	}
	config, err := alipayConfigFromSettings(settings)
	if err != nil {
		fail(w, http.StatusServiceUnavailable, err.Error())
		return
	}
	if strings.TrimSpace(config.SubscriptionURL) == "" {
		fail(w, http.StatusServiceUnavailable, "请先在管理端配置默认订阅源地址")
		return
	}

	order, err := s.store.CreatePaymentOrder(req.Plan, config.SubscriptionURL, amountFen)
	if err != nil {
		fail(w, http.StatusInternalServerError, "创建支付订单失败")
		return
	}

	form, err := config.pagePayForm(order)
	if err != nil {
		fail(w, http.StatusBadGateway, "生成支付宝支付请求失败")
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(form))
}

func (s *Server) handleAlipayNotify(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	if err := r.ParseForm(); err != nil {
		writeAlipayResult(w, false)
		return
	}
	params := formValues(r.Form)

	settings, err := s.store.Settings()
	if err != nil {
		writeAlipayResult(w, false)
		return
	}
	config, err := alipayConfigFromSettings(settings)
	if err != nil || !config.verifyNotification(params) {
		writeAlipayResult(w, false)
		return
	}
	if params["app_id"] != config.AppID {
		writeAlipayResult(w, false)
		return
	}
	status := params["trade_status"]
	if status != "TRADE_SUCCESS" && status != "TRADE_FINISHED" {
		writeAlipayResult(w, true)
		return
	}
	amountFen, err := parseAmountFen(params["total_amount"])
	if err != nil {
		writeAlipayResult(w, false)
		return
	}
	if _, err := s.store.MarkPaymentPaid(params["out_trade_no"], params["trade_no"], amountFen); err != nil {
		writeAlipayResult(w, false)
		return
	}
	writeAlipayResult(w, true)
}

type alipayConfig struct {
	AppID           string
	Gateway         string
	PrivateKey      *rsa.PrivateKey
	PublicKey       *rsa.PublicKey
	RootURL         string
	SubscriptionURL string
}

func alipayConfigFromSettings(settings map[string]string) (alipayConfig, error) {
	privateKey, err := parsePrivateKey(settings["alipay.private_key"])
	if err != nil {
		return alipayConfig{}, errors.New("支付宝应用私钥未配置或格式不正确")
	}
	publicKey, err := parsePublicKey(settings["alipay.public_key"])
	if err != nil {
		return alipayConfig{}, errors.New("支付宝公钥未配置或格式不正确")
	}
	appID := strings.TrimSpace(settings["alipay.app_id"])
	if appID == "" {
		return alipayConfig{}, errors.New("支付宝 APPID 未配置")
	}
	config := alipayConfig{
		AppID: appID, Gateway: strings.TrimSpace(settings["alipay.gateway"]),
		PrivateKey: privateKey, PublicKey: publicKey,
		RootURL:         strings.TrimRight(strings.TrimSpace(settings["app.base_url"]), "/"),
		SubscriptionURL: strings.TrimSpace(settings["alipay.subscription_url"]),
	}
	if config.RootURL == "" {
		config.RootURL = defaultAppBaseURL
	}
	base, err := url.Parse(config.RootURL)
	if err != nil || (base.Scheme != "http" && base.Scheme != "https") || base.Host == "" || base.Path != "" {
		return alipayConfig{}, errors.New("项目运行根地址必须是完整的 http(s) 地址")
	}
	if config.Gateway == "" {
		config.Gateway = "https://openapi.alipay.com/gateway.do"
	}
	return config, nil
}

func (c alipayConfig) pagePayForm(order store.PaymentOrder) (string, error) {
	bizContent, err := json.Marshal(map[string]string{
		"body":         "NetX " + order.Plan,
		"subject":      "NetX " + order.Plan,
		"out_trade_no": order.OutTradeNo,
		"total_amount": formatAmount(order.AmountFen),
		"product_code": "FAST_INSTANT_TRADE_PAY",
	})
	if err != nil {
		return "", err
	}
	params := map[string]string{
		"app_id":      c.AppID,
		"method":      "alipay.trade.page.pay",
		"format":      "JSON",
		"charset":     "utf-8",
		"sign_type":   "RSA2",
		"timestamp":   time.Now().Format("2006-01-02 15:04:05"),
		"version":     "1.0",
		"notify_url":  c.RootURL + "/api/payment/alipay/notify",
		"return_url":  c.RootURL + "/access/" + order.Code,
		"biz_content": string(bizContent),
	}
	sign, err := c.sign(params)
	if err != nil {
		return "", err
	}
	params["sign"] = sign

	request, err := http.NewRequest(http.MethodPost, c.Gateway, strings.NewReader(formEncode(params)))
	if err != nil {
		return "", err
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded;charset=utf-8")
	response, err := (&http.Client{Timeout: 20 * time.Second}).Do(request)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 2<<20))
	if err != nil {
		return "", err
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices || !strings.Contains(strings.ToLower(string(body)), "<form") {
		return "", fmt.Errorf("支付宝网关返回异常 HTTP %d", response.StatusCode)
	}
	return string(body), nil
}

func (c alipayConfig) sign(params map[string]string) (string, error) {
	digest := sha256.Sum256([]byte(signContent(params)))
	signature, err := rsa.SignPKCS1v15(rand.Reader, c.PrivateKey, crypto.SHA256, digest[:])
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(signature), nil
}

func (c alipayConfig) verifyNotification(params map[string]string) bool {
	signature, err := base64.StdEncoding.DecodeString(params["sign"])
	if err != nil || params["sign_type"] != "RSA2" {
		return false
	}
	digest := sha256.Sum256([]byte(signContent(params)))
	return rsa.VerifyPKCS1v15(c.PublicKey, crypto.SHA256, digest[:], signature) == nil
}

func signContent(params map[string]string) string {
	keys := make([]string, 0, len(params))
	for key, value := range params {
		if key == "sign" || key == "sign_type" || value == "" {
			continue
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, key+"="+params[key])
	}
	return strings.Join(parts, "&")
}

func formValues(values url.Values) map[string]string {
	params := make(map[string]string, len(values))
	for key, values := range values {
		if len(values) > 0 {
			params[key] = values[0]
		}
	}
	return params
}

func formEncode(params map[string]string) string {
	values := make(url.Values, len(params))
	for key, value := range params {
		values.Set(key, value)
	}
	return values.Encode()
}

func parsePrivateKey(value string) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode([]byte(strings.TrimSpace(value)))
	if block == nil {
		return nil, errors.New("私钥 PEM 无效")
	}
	if key, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return key, nil
	}
	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	rsaKey, ok := key.(*rsa.PrivateKey)
	if !ok {
		return nil, errors.New("私钥不是 RSA 类型")
	}
	return rsaKey, nil
}

func parsePublicKey(value string) (*rsa.PublicKey, error) {
	block, _ := pem.Decode([]byte(strings.TrimSpace(value)))
	if block == nil {
		return nil, errors.New("公钥 PEM 无效")
	}
	if key, err := x509.ParsePKIXPublicKey(block.Bytes); err == nil {
		if rsaKey, ok := key.(*rsa.PublicKey); ok {
			return rsaKey, nil
		}
	}
	key, err := x509.ParsePKCS1PublicKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	return key, nil
}

func parseAmountFen(value string) (int, error) {
	parts := strings.Split(strings.TrimSpace(value), ".")
	if len(parts) > 2 || len(parts) == 0 || parts[0] == "" {
		return 0, errors.New("金额格式不正确")
	}
	whole, err := strconv.Atoi(parts[0])
	if err != nil || whole < 0 {
		return 0, errors.New("金额格式不正确")
	}
	fraction := ""
	if len(parts) == 2 {
		fraction = parts[1]
	}
	if len(fraction) > 2 || (fraction != "" && strings.Trim(fraction, "0123456789") != "") {
		return 0, errors.New("金额格式不正确")
	}
	if len(fraction) == 0 {
		fraction = "00"
	} else if len(fraction) == 1 {
		fraction += "0"
	}
	cents, err := strconv.Atoi(fraction)
	if err != nil {
		return 0, errors.New("金额格式不正确")
	}
	return whole*100 + cents, nil
}

func formatAmount(amountFen int) string {
	return fmt.Sprintf("%d.%02d", amountFen/100, amountFen%100)
}

func writeAlipayResult(w http.ResponseWriter, success bool) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	if !success {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("failure"))
		return
	}
	_, _ = w.Write([]byte("success"))
}
