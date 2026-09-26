package httpapi

import (
	"crypto/rand"
	"crypto/rsa"
	"net/url"
	"testing"

	"netx/server/internal/store"
)

func TestPagePayURLIncludesUsageNoticeParameter(t *testing.T) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		t.Fatalf("生成测试私钥失败: %v", err)
	}

	config := alipayConfig{
		AppID:      "test-app",
		Gateway:    "https://gateway.example/pay",
		PrivateKey: privateKey,
		RootURL:    "https://netx.example",
	}
	order := store.PaymentOrder{
		Code:       "mdz5h0yvsug4wapb",
		Plan:       "极速版",
		AmountFen:  50000,
		OutTradeNo: "NX2026092600000000001",
	}

	paymentURL, err := config.pagePayURL(order)
	if err != nil {
		t.Fatalf("生成支付链接失败: %v", err)
	}
	parsed, err := url.Parse(paymentURL)
	if err != nil {
		t.Fatalf("解析支付链接失败: %v", err)
	}
	if got := parsed.Query().Get("return_url"); got != "https://netx.example/access/mdz5h0yvsug4wapb?showNotice=1" {
		t.Fatalf("支付回跳地址不正确: %s", got)
	}
}
