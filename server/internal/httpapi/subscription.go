package httpapi

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"netx/server/internal/store"
)

const maxSubscriptionSize = 10 << 20

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

	proxyNames := make([]string, 0, len(upstream.Proxies))
	for _, proxy := range upstream.Proxies {
		name, ok := proxy["name"].(string)
		if !ok || strings.TrimSpace(name) == "" {
			return nil, fmt.Errorf("代理节点缺少名称")
		}
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

	client := &http.Client{Timeout: 20 * time.Second}
	upstream, err := client.Get(source.String())
	if err != nil {
		return nil, err
	}
	if upstream.StatusCode < http.StatusOK || upstream.StatusCode >= http.StatusMultipleChoices {
		upstream.Body.Close()
		return nil, fmt.Errorf("upstream status %d", upstream.StatusCode)
	}
	return upstream, nil
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
	if upstream.StatusCode < http.StatusOK || upstream.StatusCode >= http.StatusMultipleChoices {
		fail(w, http.StatusBadGateway, fmt.Sprintf("上游订阅返回 HTTP %d", upstream.StatusCode))
		return
	}
	body, err := io.ReadAll(io.LimitReader(upstream.Body, maxSubscriptionSize+1))
	if err != nil {
		fail(w, http.StatusBadGateway, "读取订阅失败")
		return
	}
	trimmedBody := strings.ToLower(strings.TrimSpace(string(body)))
	if len(body) > maxSubscriptionSize || strings.Contains(strings.ToLower(upstream.Header.Get("Content-Type")), "text/html") || strings.HasPrefix(trimmedBody, "<!doctype html") || strings.HasPrefix(trimmedBody, "<html") {
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
