package httpapi

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestBuildSubscriptionNamesSingleProxy(t *testing.T) {
	body, err := buildSubscription([]byte("proxies:\n  - name: only\n    type: hysteria2\n    server: 2605:52c0::1\n"))
	if err != nil {
		t.Fatalf("构建订阅失败: %v", err)
	}
	var result clashSubscription
	if err := yaml.Unmarshal(body, &result); err != nil {
		t.Fatalf("解析结果失败: %v\n%s", err, body)
	}
	if len(result.Proxies) != 1 || result.Proxies[0]["name"] != "Netx" {
		t.Fatalf("单节点应命名为 Netx，实际 %v", result.Proxies)
	}
	if strings.Join(result.ProxyGroups[0].Proxies, ",") != "Netx,DIRECT" {
		t.Fatalf("代理组顺序不正确: %v", result.ProxyGroups[0].Proxies)
	}
}

func TestBuildSubscriptionOrdersByAddressAndProtocol(t *testing.T) {
	upstream := `
proxies:
  - name: Los-D1-ipv6_test
    type: vless
    server: 179.255.113.219
  - name: Los-D1
    type: vless
    server: 2605:52c0:2:5be3:be24:11ff:fed7:839f
  - name: Los-D1-HY2
    type: hysteria2
    server: 179.255.113.219
`
	body, err := buildSubscription([]byte(upstream))
	if err != nil {
		t.Fatalf("构建订阅失败: %v", err)
	}
	var result clashSubscription
	if err := yaml.Unmarshal(body, &result); err != nil {
		t.Fatalf("解析结果失败: %v\n%s", err, body)
	}
	got := make([]string, 0, len(result.Proxies))
	for _, proxy := range result.Proxies {
		got = append(got, proxy["name"].(string)+" "+proxy["type"].(string)+" "+proxy["server"].(string))
	}
	want := []string{
		"Netx-1 vless 179.255.113.219",
		"Netx-2 hysteria2 179.255.113.219",
		"Netx-3 vless 2605:52c0:2:5be3:be24:11ff:fed7:839f",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("排序结果不正确:\n%s", strings.Join(got, "\n"))
	}
	group := result.ProxyGroups[0].Proxies
	if strings.Join(group, ",") != "Netx-1,Netx-2,Netx-3,DIRECT" {
		t.Fatalf("代理组顺序不正确: %v", group)
	}
	if strings.Contains(string(body), "Los-D1") {
		t.Fatalf("结果不应保留上游节点名称: %s", body)
	}
}

func TestBuildSubscriptionKeepsSameRankOrder(t *testing.T) {
	upstream := `
proxies:
  - name: first
    type: vless
    server: 1.1.1.1
    uuid: first
  - name: second
    type: vless
    server: example.com
    uuid: second
`
	body, err := buildSubscription([]byte(upstream))
	if err != nil {
		t.Fatalf("构建订阅失败: %v", err)
	}
	var result clashSubscription
	if err := yaml.Unmarshal(body, &result); err != nil {
		t.Fatalf("解析结果失败: %v\n%s", err, body)
	}
	if result.Proxies[0]["name"] != "Netx-1" || result.Proxies[0]["uuid"] != "first" {
		t.Fatalf("同级第一个节点应保持原序: %v", result.Proxies[0])
	}
	if result.Proxies[1]["name"] != "Netx-2" || result.Proxies[1]["uuid"] != "second" {
		t.Fatalf("同级第二个节点应保持原序: %v", result.Proxies[1])
	}
}
