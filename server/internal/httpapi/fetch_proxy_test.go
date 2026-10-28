package httpapi_test

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"netx/server/internal/socks5test"
)

func TestFetchProxyAdminRequiresAuth(t *testing.T) {
	env := newTestEnv(t)
	if recorder, _ := env.do(t, http.MethodGet, "/api/admin/fetch-proxy", nil, ""); recorder.Code != http.StatusUnauthorized {
		t.Fatalf("未登录读取代理应返回 401，实际 %d", recorder.Code)
	}
}

func TestFetchProxySaveMasksPasswordAndParsesLink(t *testing.T) {
	env := newTestEnv(t)
	recorder, payload := env.do(t, http.MethodPost, "/api/admin/fetch-proxy/parse", map[string]any{
		"link": "socks5://alice:p%40ss@[2001:db8::1]:1080",
	}, env.token)
	if recorder.Code != http.StatusOK {
		t.Fatalf("解析链接应成功，实际 %d: %s", recorder.Code, recorder.Body.String())
	}
	data := payload["data"].(map[string]any)
	if data["host"] != "2001:db8::1" || data["port"] != float64(1080) || data["username"] != "alice" || data["password"] != "p@ss" {
		t.Fatalf("解析结果不正确: %v", data)
	}
	if recorder, _ := env.do(t, http.MethodPost, "/api/admin/fetch-proxy/parse", map[string]any{"link": "https://example.com"}, env.token); recorder.Code != http.StatusBadRequest {
		t.Fatalf("非 SOCKS5 链接应返回 400，实际 %d", recorder.Code)
	}

	recorder, _ = env.do(t, http.MethodPut, "/api/admin/fetch-proxy", map[string]any{
		"enabled": true, "host": "203.0.113.10", "port": 1080, "username": "alice", "password": "super-secret-pass", "updatePassword": true,
	}, env.token)
	if recorder.Code != http.StatusOK {
		t.Fatalf("保存代理应成功，实际 %d: %s", recorder.Code, recorder.Body.String())
	}
	recorder, payload = env.do(t, http.MethodGet, "/api/admin/fetch-proxy", nil, env.token)
	if recorder.Code != http.StatusOK {
		t.Fatalf("读取代理应成功，实际 %d", recorder.Code)
	}
	body := recorder.Body.String()
	if strings.Contains(body, "super-secret-pass") {
		t.Fatal("读取代理不应返回明文密码")
	}
	view := payload["data"].(map[string]any)
	if view["configured"] != true || view["enabled"] != true || view["host"] != "203.0.113.10" || view["username"] != "alice" || view["passwordConfigured"] != true || view["passwordHint"] != "****pass" {
		t.Fatalf("代理配置不正确: %v", view)
	}

	recorder, _ = env.do(t, http.MethodPut, "/api/admin/fetch-proxy", map[string]any{
		"enabled": false, "host": "203.0.113.11", "port": 1081, "username": "alice", "password": "", "updatePassword": false,
	}, env.token)
	if recorder.Code != http.StatusOK {
		t.Fatalf("留空密码再次保存应成功，实际 %d: %s", recorder.Code, recorder.Body.String())
	}
	if got := storedPassword(t, env); got != "super-secret-pass" {
		t.Fatalf("留空密码不应覆盖原密码，实际 %q", got)
	}

	recorder, _ = env.do(t, http.MethodPut, "/api/admin/fetch-proxy", map[string]any{
		"enabled": true, "host": "bad host", "port": 1080, "username": "alice", "password": "x", "updatePassword": true,
	}, env.token)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("无效地址应返回 400，实际 %d", recorder.Code)
	}

	recorder, payload = env.do(t, http.MethodPost, "/api/admin/fetch-proxy/test", map[string]any{
		"host": "127.0.0.1", "port": 1, "username": "alice", "password": "",
	}, env.token)
	if recorder.Code != http.StatusBadGateway {
		t.Fatalf("连不上的代理测试应返回 502，实际 %d: %s", recorder.Code, recorder.Body.String())
	}
	if strings.Contains(recorder.Body.String(), "super-secret-pass") {
		t.Fatal("测试失败响应不应包含密码")
	}
	if payload["message"] != "连接 SOCKS5 失败" {
		t.Fatalf("测试失败文案不正确: %v", payload["message"])
	}

	recorder, _ = env.do(t, http.MethodDelete, "/api/admin/fetch-proxy", nil, env.token)
	if recorder.Code != http.StatusOK {
		t.Fatalf("删除代理应成功，实际 %d", recorder.Code)
	}
	recorder, payload = env.do(t, http.MethodGet, "/api/admin/fetch-proxy", nil, env.token)
	if payload["data"].(map[string]any)["configured"] != false {
		t.Fatalf("删除后不应再显示已配置: %v", payload)
	}
}

func TestSubscriptionUsesDirectWhenProxyConfigured(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/yaml")
		_, _ = w.Write([]byte("proxies:\n  - name: origin\n"))
	}))
	defer upstream.Close()
	socks := rejectSOCKS(t)

	env := newTestEnv(t)
	saveFetchProxy(t, env, socks, true)
	code := activateSubscription(t, env, upstream.URL+"/profile.yaml")
	recorder, _ := env.do(t, http.MethodGet, "/sub/"+code, nil, "")
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), "name: Netx") {
		t.Fatalf("直连成功时应返回订阅，实际 %d: %s", recorder.Code, recorder.Body.String())
	}
	if got := socks.Requests(); len(got) != 0 {
		t.Fatalf("直连成功不应使用 SOCKS5，实际 %v", got)
	}
}

func TestSubscriptionFallsBackWhenDirectReturnsHTML(t *testing.T) {
	var directHits atomic.Int32
	blocked := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		directHits.Add(1)
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte("<!doctype html><html><body>blocked</body></html>"))
	}))
	defer blocked.Close()
	upstream := yamlServer(t, "upload=1; download=2; total=10; expire=0")
	socks := rewriteSOCKS(t, upstream)

	env := newTestEnv(t)
	saveFetchProxy(t, env, socks, true)
	code := activateSubscription(t, env, blocked.URL+"/profile.yaml")

	recorder, _ := env.do(t, http.MethodGet, "/sub/"+code, nil, "")
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), "name: Netx") {
		t.Fatalf("拦截页后应改走代理，实际 %d: %s", recorder.Code, recorder.Body.String())
	}
	if recorder.Header().Get("Subscription-Userinfo") == "" {
		t.Fatal("代理返回的用量头应透传")
	}
	if directHits.Load() != 1 {
		t.Fatalf("第一次应访问直连，实际 %d", directHits.Load())
	}

	recorder, _ = env.do(t, http.MethodGet, "/sub/"+code, nil, "")
	if recorder.Code != http.StatusOK {
		t.Fatalf("冷却期内第二次订阅应成功，实际 %d: %s", recorder.Code, recorder.Body.String())
	}
	if directHits.Load() != 1 {
		t.Fatalf("冷却期内不应再直连，实际 %d", directHits.Load())
	}
}

func TestSubscriptionFallsBackWhenDirectUnreachable(t *testing.T) {
	upstream := yamlServer(t, "upload=1; download=2; total=10; expire=0")
	socks := rewriteSOCKS(t, upstream)
	env := newTestEnv(t)
	saveFetchProxy(t, env, socks, true)
	code := activateSubscription(t, env, "http://127.0.0.1:1/profile.yaml")

	recorder, _ := env.do(t, http.MethodGet, "/sub/"+code, nil, "")
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), "name: Netx") {
		t.Fatalf("直连失败后应改走代理，实际 %d: %s", recorder.Code, recorder.Body.String())
	}
	recorder, payload := env.do(t, http.MethodGet, "/api/subscription/"+code+"/usage", nil, "")
	if recorder.Code != http.StatusOK {
		t.Fatalf("用量接口也应走代理，实际 %d: %s", recorder.Code, recorder.Body.String())
	}
	if payload["data"].(map[string]any)["usedBytes"] != float64(3) {
		t.Fatalf("代理返回的用量不正确: %v", payload)
	}
}

func TestSubscriptionDoesNotFallBackOnUpstreamStatus(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "missing", http.StatusNotFound)
	}))
	defer upstream.Close()
	socks := rejectSOCKS(t)
	env := newTestEnv(t)
	saveFetchProxy(t, env, socks, true)
	code := activateSubscription(t, env, upstream.URL)

	recorder, payload := env.do(t, http.MethodGet, "/sub/"+code, nil, "")
	if recorder.Code != http.StatusBadGateway || payload["message"] != "获取订阅失败" {
		t.Fatalf("上游 404 应直接失败，实际 %d: %s", recorder.Code, recorder.Body.String())
	}
	if got := socks.Requests(); len(got) != 0 {
		t.Fatalf("上游 404 不应使用 SOCKS5，实际 %v", got)
	}
}

func TestSubscriptionDoesNotUseDisabledProxy(t *testing.T) {
	blocked := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte("<html>blocked</html>"))
	}))
	defer blocked.Close()
	socks := rejectSOCKS(t)
	env := newTestEnv(t)
	saveFetchProxy(t, env, socks, false)
	code := activateSubscription(t, env, blocked.URL)

	recorder, payload := env.do(t, http.MethodGet, "/sub/"+code, nil, "")
	if recorder.Code != http.StatusBadGateway || payload["message"] != "上游返回的不是 YAML 订阅" {
		t.Fatalf("关闭兜底时应保留原来的失败结果，实际 %d: %s", recorder.Code, recorder.Body.String())
	}
	if got := socks.Requests(); len(got) != 0 {
		t.Fatalf("关闭兜底时不应连接 SOCKS5，实际 %v", got)
	}
}

func saveFetchProxy(t *testing.T, env *testEnv, socks *socks5test.Server, enabled bool) {
	t.Helper()
	recorder, _ := env.do(t, http.MethodPut, "/api/admin/fetch-proxy", map[string]any{
		"enabled": enabled, "host": socks.Host(), "port": socks.Port(), "username": "netx", "password": "secret-pass", "updatePassword": true,
	}, env.token)
	if recorder.Code != http.StatusOK {
		t.Fatalf("保存测试代理失败: %d %s", recorder.Code, recorder.Body.String())
	}
}

func activateSubscription(t *testing.T, env *testEnv, subscriptionURL string) string {
	t.Helper()
	_, payload := env.do(t, http.MethodPost, "/api/admin/codes", map[string]any{"subscriptionUrl": subscriptionURL}, env.token)
	code := itemsOf(t, payload)[0]["code"].(string)
	if recorder, _ := env.do(t, http.MethodPost, "/api/redeem", map[string]string{"code": code}, ""); recorder.Code != http.StatusOK {
		t.Fatalf("兑换应成功，实际 %d: %s", recorder.Code, recorder.Body.String())
	}
	return code
}

func yamlServer(t *testing.T, userinfo string) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/yaml")
		w.Header().Set("Subscription-Userinfo", userinfo)
		_, _ = w.Write([]byte("proxies:\n  - name: origin\n"))
	}))
	t.Cleanup(server.Close)
	return server
}

func rewriteSOCKS(t *testing.T, upstream *httptest.Server) *socks5test.Server {
	t.Helper()
	target := upstream.Listener.Addr().String()
	socks, err := socks5test.Listen("netx", "secret-pass", func(ctx context.Context, addr string) (net.Conn, error) {
		var dialer net.Dialer
		return dialer.DialContext(ctx, "tcp", target)
	})
	if err != nil {
		t.Fatalf("启动 SOCKS5 失败: %v", err)
	}
	t.Cleanup(func() { _ = socks.Close() })
	return socks
}

func rejectSOCKS(t *testing.T) *socks5test.Server {
	t.Helper()
	socks, err := socks5test.Listen("netx", "secret-pass", func(context.Context, string) (net.Conn, error) {
		return nil, net.ErrClosed
	})
	if err != nil {
		t.Fatalf("启动 SOCKS5 失败: %v", err)
	}
	t.Cleanup(func() { _ = socks.Close() })
	return socks
}

func storedPassword(t *testing.T, env *testEnv) string {
	t.Helper()
	var password string
	if err := env.db.QueryRow("SELECT password FROM fetch_proxy WHERE id = 1").Scan(&password); err != nil {
		t.Fatalf("读取已保存密码失败: %v", err)
	}
	return password
}
