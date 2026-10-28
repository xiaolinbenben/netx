package httpapi

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"netx/server/internal/proxy"
	"netx/server/internal/store"
)

const (
	maxSubscriptionSize   = 10 << 20
	directRequestTimeout  = 20 * time.Second
	directFallbackTimeout = 8 * time.Second
	proxyRequestTimeout   = 20 * time.Second
	directFailureCooldown = 5 * time.Minute
)

type upstreamDecision int

const (
	upstreamOK upstreamDecision = iota
	upstreamStatus
	upstreamFallback
)

var errInvalidSubscriptionURL = errors.New("invalid subscription URL")

type clashSubscription struct {
	Proxies       []map[string]any        `yaml:"proxies"`
	ProxyGroups   []proxyGroup            `yaml:"proxy-groups"`
	RuleProviders map[string]ruleProvider `yaml:"rule-providers"`
	Rules         []string                `yaml:"rules"`
}

type proxyGroup struct {
	Name    string   `yaml:"name"`
	Type    string   `yaml:"type"`
	Proxies []string `yaml:"proxies"`
}

type ruleProvider struct {
	Type     string `yaml:"type"`
	Behavior string `yaml:"behavior"`
	URL      string `yaml:"url"`
	Path     string `yaml:"path"`
	Interval int    `yaml:"interval"`
}

func buildSubscription(body []byte) ([]byte, error) {
	var upstream clashSubscription
	if err := yaml.Unmarshal(body, &upstream); err != nil {
		return nil, err
	}
	if len(upstream.Proxies) == 0 {
		return nil, fmt.Errorf("订阅不包含代理节点")
	}

	for _, proxy := range upstream.Proxies {
		name, ok := proxy["name"].(string)
		if !ok || strings.TrimSpace(name) == "" {
			return nil, fmt.Errorf("代理节点缺少名称")
		}
	}
	sort.SliceStable(upstream.Proxies, func(i, j int) bool {
		return lessProxy(upstream.Proxies[i], upstream.Proxies[j])
	})

	proxyNames := make([]string, 0, len(upstream.Proxies))
	multiple := len(upstream.Proxies) > 1
	for i, proxy := range upstream.Proxies {
		name := "Netx"
		if multiple {
			name = fmt.Sprintf("Netx-%d", i+1)
		}
		proxy["name"] = name
		proxyNames = append(proxyNames, name)
	}

	result := clashSubscription{
		Proxies: upstream.Proxies,
		ProxyGroups: []proxyGroup{{
			Name:    "PROXY",
			Type:    "select",
			Proxies: append(proxyNames, "DIRECT"),
		}},
		RuleProviders: map[string]ruleProvider{
			"private": {Type: "http", Behavior: "domain", URL: "https://raw.githubusercontent.com/Loyalsoldier/clash-rules/release/private.txt", Path: "./ruleset/private.yaml", Interval: 86400},
			"reject":  {Type: "http", Behavior: "domain", URL: "https://raw.githubusercontent.com/Loyalsoldier/clash-rules/release/reject.txt", Path: "./ruleset/reject.yaml", Interval: 86400},
			"direct":  {Type: "http", Behavior: "domain", URL: "https://raw.githubusercontent.com/Loyalsoldier/clash-rules/release/direct.txt", Path: "./ruleset/direct.yaml", Interval: 86400},
			"proxy":   {Type: "http", Behavior: "domain", URL: "https://raw.githubusercontent.com/Loyalsoldier/clash-rules/release/proxy.txt", Path: "./ruleset/proxy.yaml", Interval: 86400},
			"apple":   {Type: "http", Behavior: "domain", URL: "https://raw.githubusercontent.com/Loyalsoldier/clash-rules/release/apple.txt", Path: "./ruleset/apple.yaml", Interval: 86400},
			"google":  {Type: "http", Behavior: "domain", URL: "https://raw.githubusercontent.com/Loyalsoldier/clash-rules/release/google.txt", Path: "./ruleset/google.yaml", Interval: 86400},
		},
		Rules: []string{
			"DOMAIN-SUFFIX,openai.com,PROXY",
			"DOMAIN-SUFFIX,chatgpt.com,PROXY",
			"DOMAIN-SUFFIX,oaiusercontent.com,PROXY",
			"IP-CIDR,127.0.0.0/8,DIRECT,no-resolve",
			"IP-CIDR,10.0.0.0/8,DIRECT,no-resolve",
			"IP-CIDR,172.16.0.0/12,DIRECT,no-resolve",
			"IP-CIDR,192.168.0.0/16,DIRECT,no-resolve",
			"IP-CIDR,169.254.0.0/16,DIRECT,no-resolve",
			"IP-CIDR,100.64.0.0/10,DIRECT,no-resolve",
			"IP-CIDR6,::1/128,DIRECT,no-resolve",
			"IP-CIDR6,fc00::/7,DIRECT,no-resolve",
			"IP-CIDR6,fe80::/10,DIRECT,no-resolve",
			"RULE-SET,private,DIRECT",
			"RULE-SET,reject,REJECT",
			"RULE-SET,apple,DIRECT",
			"RULE-SET,google,PROXY",
			"RULE-SET,proxy,PROXY",
			"RULE-SET,direct,DIRECT",
			"GEOIP,CN,DIRECT",
			"MATCH,PROXY",
		},
	}
	return yaml.Marshal(result)
}

func lessProxy(left, right map[string]any) bool {
	leftServer, _ := left["server"].(string)
	rightServer, _ := right["server"].(string)
	leftIP := net.ParseIP(leftServer)
	rightIP := net.ParseIP(rightServer)
	leftV6 := leftIP != nil && leftIP.To4() == nil
	rightV6 := rightIP != nil && rightIP.To4() == nil
	if leftV6 != rightV6 {
		return !leftV6
	}
	return protocolOrder(left["type"]) < protocolOrder(right["type"])
}

func protocolOrder(kind any) int {
	switch kind {
	case "vless":
		return 0
	case "hysteria2":
		return 1
	default:
		return 2
	}
}

type subscriptionUsage struct {
	UsedBytes      int64 `json:"usedBytes"`
	TotalBytes     int64 `json:"totalBytes"`
	ExpireAt       int64 `json:"expireAt"`
	RemainingDays  int64 `json:"remainingDays"`
	UnlimitedTotal bool  `json:"unlimitedTotal"`
	UnlimitedTime  bool  `json:"unlimitedTime"`
}

func (s *Server) fetchSubscription(code string) (*http.Response, error) {
	item, err := s.store.FindCode(code)
	if err != nil {
		return nil, err
	}
	if item.Status != store.StatusUsed || strings.TrimSpace(item.SubscriptionURL) == "" {
		return nil, store.ErrCodeNotFound
	}

	source, err := url.Parse(strings.TrimSpace(item.SubscriptionURL))
	if err != nil || (source.Scheme != "http" && source.Scheme != "https") || source.Host == "" {
		return nil, errInvalidSubscriptionURL
	}

	endpoint, proxyOn, err := s.enabledFetchProxy()
	if err != nil {
		return nil, err
	}
	rawURL := source.String()
	if !proxyOn {
		return readUpstream(requestUpstream(rawURL, directRequestTimeout, nil))
	}
	if s.directCoolingDown() {
		return readUpstream(requestUpstream(rawURL, proxyRequestTimeout, &endpoint))
	}

	resp, err := requestUpstream(rawURL, directFallbackTimeout, nil)
	if err != nil {
		s.failDirect(endpoint)
		return readUpstream(requestUpstream(rawURL, proxyRequestTimeout, &endpoint))
	}
	decision, restored, peekErr := classifyUpstream(resp)
	if peekErr != nil || decision == upstreamFallback {
		if restored != nil {
			restored.Body.Close()
		}
		s.failDirect(endpoint)
		return readUpstream(requestUpstream(rawURL, proxyRequestTimeout, &endpoint))
	}
	s.markDirectHealthy()
	if decision == upstreamStatus {
		status := restored.StatusCode
		restored.Body.Close()
		return nil, fmt.Errorf("upstream status %d", status)
	}
	return restored, nil
}

func readUpstream(resp *http.Response, err error) (*http.Response, error) {
	if err != nil {
		return nil, err
	}
	decision, restored, peekErr := classifyUpstream(resp)
	if peekErr != nil {
		return nil, peekErr
	}
	if decision == upstreamStatus {
		status := restored.StatusCode
		restored.Body.Close()
		return nil, fmt.Errorf("upstream status %d", status)
	}
	return restored, nil
}

func requestUpstream(rawURL string, timeout time.Duration, endpoint *proxy.Endpoint) (*http.Response, error) {
	client := &http.Client{Timeout: timeout}
	if endpoint != nil {
		proxied, err := proxy.HTTPClient(*endpoint, timeout)
		if err != nil {
			return nil, err
		}
		client = proxied
	}
	return client.Get(rawURL)
}

func classifyUpstream(resp *http.Response) (upstreamDecision, *http.Response, error) {
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxSubscriptionSize+1))
	closeErr := resp.Body.Close()
	if err != nil {
		return upstreamFallback, nil, err
	}
	if closeErr != nil {
		return upstreamFallback, nil, closeErr
	}
	resp.Body = io.NopCloser(bytes.NewReader(body))
	if isHTMLBlock(resp.Header.Get("Content-Type"), body) {
		return upstreamFallback, resp, nil
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return upstreamStatus, resp, nil
	}
	return upstreamOK, resp, nil
}

func isHTMLBlock(contentType string, body []byte) bool {
	if strings.Contains(strings.ToLower(contentType), "text/html") {
		return true
	}
	trimmed := strings.ToLower(strings.TrimSpace(string(body)))
	return strings.HasPrefix(trimmed, "<!doctype html") || strings.HasPrefix(trimmed, "<html")
}

func (s *Server) directCoolingDown() bool {
	s.proxyMu.Lock()
	defer s.proxyMu.Unlock()
	return time.Now().Before(s.proxyDownUntil)
}

func (s *Server) markDirectDown() {
	s.proxyMu.Lock()
	s.proxyDownUntil = time.Now().Add(directFailureCooldown)
	s.proxyMu.Unlock()
}

func (s *Server) markDirectHealthy() {
	s.proxyMu.Lock()
	s.proxyDownUntil = time.Time{}
	s.proxyMu.Unlock()
}

func (s *Server) failDirect(endpoint proxy.Endpoint) {
	s.markDirectDown()
	log.Printf("订阅直连失败，改用 SOCKS5 %s", net.JoinHostPort(endpoint.Host, strconv.Itoa(endpoint.Port)))
}

func parseSubscriptionUsage(value string, now time.Time) (subscriptionUsage, error) {
	values := map[string]int64{}
	for _, part := range strings.Split(value, ";") {
		keyValue := strings.SplitN(strings.TrimSpace(part), "=", 2)
		if len(keyValue) != 2 {
			continue
		}
		key := strings.TrimSpace(keyValue[0])
		number, err := strconv.ParseInt(strings.TrimSpace(keyValue[1]), 10, 64)
		if err != nil || number < 0 {
			return subscriptionUsage{}, fmt.Errorf("invalid Subscription-Userinfo")
		}
		values[key] = number
	}
	if _, ok := values["upload"]; !ok {
		return subscriptionUsage{}, fmt.Errorf("missing upload")
	}
	if _, ok := values["download"]; !ok {
		return subscriptionUsage{}, fmt.Errorf("missing download")
	}
	if _, ok := values["total"]; !ok {
		return subscriptionUsage{}, fmt.Errorf("missing total")
	}
	if _, ok := values["expire"]; !ok {
		return subscriptionUsage{}, fmt.Errorf("missing expire")
	}

	usage := subscriptionUsage{
		UsedBytes:      values["upload"] + values["download"],
		TotalBytes:     values["total"],
		ExpireAt:       values["expire"],
		UnlimitedTotal: values["total"] == 0,
		UnlimitedTime:  values["expire"] == 0,
	}
	if usage.ExpireAt > 0 {
		remaining := time.Unix(usage.ExpireAt, 0).Sub(now)
		if remaining > 0 {
			usage.RemainingDays = int64((remaining + 24*time.Hour - time.Nanosecond).Hours() / 24)
		}
	}
	return usage, nil
}

func (s *Server) handleSubscriptionUsage(w http.ResponseWriter, r *http.Request) {
	code := strings.TrimSpace(r.PathValue("code"))
	if code == "" {
		fail(w, http.StatusNotFound, "订阅不存在")
		return
	}
	upstream, err := s.fetchSubscription(code)
	if errors.Is(err, store.ErrCodeNotFound) {
		fail(w, http.StatusNotFound, "订阅不存在")
		return
	}
	if errors.Is(err, errInvalidSubscriptionURL) {
		fail(w, http.StatusBadGateway, "订阅地址无效")
		return
	}
	if err != nil {
		fail(w, http.StatusBadGateway, "获取订阅失败")
		return
	}
	defer upstream.Body.Close()
	usage, err := parseSubscriptionUsage(upstream.Header.Get("Subscription-Userinfo"), time.Now())
	if err != nil {
		fail(w, http.StatusBadGateway, "上游未提供有效用量信息")
		return
	}
	ok(w, usage)
}

func (s *Server) handleSubscription(w http.ResponseWriter, r *http.Request) {
	code := strings.TrimSpace(r.PathValue("code"))
	if code == "" {
		fail(w, http.StatusNotFound, "订阅不存在")
		return
	}
	upstream, err := s.fetchSubscription(code)
	if errors.Is(err, store.ErrCodeNotFound) {
		fail(w, http.StatusNotFound, "订阅不存在")
		return
	}
	if errors.Is(err, errInvalidSubscriptionURL) {
		fail(w, http.StatusBadGateway, "订阅地址无效")
		return
	}
	if err != nil {
		fail(w, http.StatusBadGateway, "获取订阅失败")
		return
	}
	defer upstream.Body.Close()
	body, err := io.ReadAll(io.LimitReader(upstream.Body, maxSubscriptionSize+1))
	if err != nil {
		fail(w, http.StatusBadGateway, "读取订阅失败")
		return
	}
	if len(body) > maxSubscriptionSize || isHTMLBlock(upstream.Header.Get("Content-Type"), body) {
		fail(w, http.StatusBadGateway, "上游返回的不是 YAML 订阅")
		return
	}
	body, err = buildSubscription(body)
	if err != nil {
		fail(w, http.StatusBadGateway, "上游订阅格式无效")
		return
	}

	w.Header().Set("Content-Type", "application/yaml; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename=`+code+`.yaml`)
	if userinfo := upstream.Header.Get("Subscription-Userinfo"); userinfo != "" {
		w.Header().Set("Subscription-Userinfo", userinfo)
	}
	_, _ = w.Write(body)
}
