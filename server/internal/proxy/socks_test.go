package proxy

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"netx/server/internal/socks5test"
)

func TestParseURL(t *testing.T) {
	parsed, err := ParseURL("socks5://alice:p%40ss@[2001:db8::1]:1080")
	if err != nil {
		t.Fatalf("解析链接失败: %v", err)
	}
	if parsed.Host != "2001:db8::1" || parsed.Port != 1080 || parsed.Username != "alice" || parsed.Password != "p@ss" {
		t.Fatalf("解析结果不正确: %+v", parsed)
	}

	withoutAuth, err := ParseURL("  SOCKS5://proxy.example:1080/  ")
	if err != nil || withoutAuth.Host != "proxy.example" || withoutAuth.Port != 1080 || withoutAuth.Username != "" || withoutAuth.Password != "" {
		t.Fatalf("无认证链接解析不正确: %+v %v", withoutAuth, err)
	}

	for _, raw := range []string{"", "https://example.com:1080", "socks5://example.com", "socks5://user:pass@example.com:1080/extra", "socks5://:secret@example.com:1080"} {
		if _, err := ParseURL(raw); err == nil {
			t.Fatalf("应拒绝无效链接 %q", raw)
		}
	}
}

func TestNormalizeRejectsBadEndpoint(t *testing.T) {
	if _, err := Normalize(Endpoint{Host: " ", Port: 1080}); err == nil {
		t.Fatal("空地址应被拒绝")
	}
	if _, err := Normalize(Endpoint{Host: "example.com", Port: 0}); err == nil {
		t.Fatal("端口 0 应被拒绝")
	}
	if _, err := Normalize(Endpoint{Host: "bad host", Port: 1080}); err == nil {
		t.Fatal("带空格的地址应被拒绝")
	}
	got, err := Normalize(Endpoint{Host: "[::1]", Port: 1080, Username: "user"})
	if err != nil || got.Host != "::1" || got.Password != "" {
		t.Fatalf("IPv6 地址整理不正确: %+v %v", got, err)
	}
}

func TestHTTPClientUsesSOCKS5AndKeepsHostname(t *testing.T) {
	socks, err := socks5test.Listen("", "", nil)
	if err != nil {
		t.Fatalf("启动 SOCKS5 失败: %v", err)
	}
	t.Cleanup(func() { _ = socks.Close() })

	client, err := HTTPClient(Endpoint{Host: socks.Host(), Port: socks.Port()}, 5*time.Second)
	if err != nil {
		t.Fatalf("创建客户端失败: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://subscription.invalid/profile", nil)
	if err != nil {
		t.Fatalf("创建请求失败: %v", err)
	}
	_, _ = client.Do(req)
	requests := socks.Requests()
	if len(requests) != 1 || requests[0] != "subscription.invalid:80" {
		t.Fatalf("应把域名原样交给 SOCKS5，实际 %v", requests)
	}
}

func TestProbeThroughSOCKS5(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(upstream.Close)

	socks, err := socks5test.Listen("netx", "secret-pass", nil)
	if err != nil {
		t.Fatalf("启动 SOCKS5 失败: %v", err)
	}
	t.Cleanup(func() { _ = socks.Close() })

	elapsed, status, err := Probe(context.Background(), Endpoint{
		Host: socks.Host(), Port: socks.Port(), Username: "netx", Password: "secret-pass",
	}, upstream.URL)
	if err != nil || status != http.StatusNoContent || elapsed < 0 {
		t.Fatalf("经 SOCKS5 探测失败: status=%d elapsed=%s err=%v", status, elapsed, err)
	}

	_, _, err = Probe(context.Background(), Endpoint{
		Host: socks.Host(), Port: socks.Port(), Username: "netx", Password: "wrong",
	}, upstream.URL)
	if err == nil {
		t.Fatal("错误密码应探测失败")
	}
}
