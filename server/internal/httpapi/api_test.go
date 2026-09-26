package httpapi_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"netx/server/internal/auth"
	"netx/server/internal/config"
	"netx/server/internal/httpapi"
	"netx/server/internal/store"
)

const (
	testUsername = "admin"
	testPassword = "secret"
	testSecret   = "test-jwt-secret"
)

type testEnv struct {
	handler http.Handler
	db      *store.DB
	token   string
}

func newTestEnv(t *testing.T) *testEnv {
	t.Helper()

	cfg := config.Config{
		Addr:          ":0",
		DBPath:        filepath.Join(t.TempDir(), "netx.db"),
		AdminUsername: testUsername,
		AdminPassword: testPassword,
		JWTSecret:     testSecret,
	}
	db, err := store.Open(cfg.DBPath)
	if err != nil {
		t.Fatalf("打开测试数据库失败: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	if err := store.Migrate(db.DB, os.DirFS("../..")); err != nil {
		t.Fatalf("执行迁移失败: %v", err)
	}

	webFS := fstest.MapFS{
		"index.html":                  &fstest.MapFile{Data: []byte("<html>netx landing</html>")},
		"admin/index.html":            &fstest.MapFile{Data: []byte("<html>netx admin</html>")},
		"dashboard/index.html":        &fstest.MapFile{Data: []byte("<html>netx dashboard</html>")},
		"dashboard/access/index.html": &fstest.MapFile{Data: []byte("<html>netx access</html>")},
	}
	return &testEnv{
		handler: httpapi.New(cfg, db, webFS),
		db:      db,
		token:   issueToken(t, cfg),
	}
}

func issueToken(t *testing.T, cfg config.Config) string {
	t.Helper()
	access, _, _, err := auth.NewManager(cfg.JWTSecret).Issue(cfg.AdminUsername)
	if err != nil {
		t.Fatalf("签发测试凭证失败: %v", err)
	}
	return access
}

func (e *testEnv) do(t *testing.T, method, path string, body any, token string) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()

	var reader *bytes.Reader
	if body == nil {
		reader = bytes.NewReader(nil)
	} else {
		payload, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("序列化请求体失败: %v", err)
		}
		reader = bytes.NewReader(payload)
	}

	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	recorder := httptest.NewRecorder()
	e.handler.ServeHTTP(recorder, req)

	payload := map[string]any{}
	if recorder.Body.Len() > 0 {
		_ = json.Unmarshal(recorder.Body.Bytes(), &payload)
	}
	return recorder, payload
}

func itemsOf(t *testing.T, payload map[string]any) []map[string]any {
	t.Helper()
	data, ok := payload["data"].(map[string]any)
	if !ok {
		t.Fatalf("响应缺少 data: %v", payload)
	}
	raw, ok := data["items"].([]any)
	if !ok {
		t.Fatalf("响应缺少 items: %v", data)
	}
	items := make([]map[string]any, 0, len(raw))
	for _, entry := range raw {
		item, ok := entry.(map[string]any)
		if !ok {
			t.Fatalf("兑换码数据结构不正确: %v", entry)
		}
		items = append(items, item)
	}
	return items
}

func totalOf(t *testing.T, payload map[string]any) int {
	t.Helper()
	data, ok := payload["data"].(map[string]any)
	if !ok {
		t.Fatalf("响应缺少 data: %v", payload)
	}
	total, ok := data["total"].(float64)
	if !ok {
		t.Fatalf("响应缺少 total: %v", data)
	}
	return int(total)
}

func TestLoginAndAuthGuard(t *testing.T) {
	env := newTestEnv(t)

	recorder, payload := env.do(t, http.MethodPost, "/api/admin/login", map[string]string{
		"username": testUsername,
		"password": "wrong",
	}, "")
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("密码错误应返回 401，实际 %d", recorder.Code)
	}
	if payload["success"] != false {
		t.Fatalf("密码错误响应应有 success=false，实际 %v", payload)
	}

	recorder, payload = env.do(t, http.MethodPost, "/api/admin/login", map[string]string{
		"username": testUsername,
		"password": testPassword,
	}, "")
	if recorder.Code != http.StatusOK {
		t.Fatalf("登录应返回 200，实际 %d", recorder.Code)
	}
	data, ok := payload["data"].(map[string]any)
	if !ok {
		t.Fatalf("登录响应缺少 data: %v", payload)
	}
	access, _ := data["accessToken"].(string)
	if access == "" {
		t.Fatalf("登录响应缺少 accessToken: %v", data)
	}
	if roles, _ := data["roles"].([]any); len(roles) != 1 || roles[0] != "admin" {
		t.Fatalf("登录响应角色不正确: %v", data["roles"])
	}

	if recorder, _ := env.do(t, http.MethodGet, "/api/admin/codes", nil, ""); recorder.Code != http.StatusUnauthorized {
		t.Fatalf("缺少凭证应返回 401，实际 %d", recorder.Code)
	}

	expired, _, _, err := auth.NewManagerForTesting(testSecret, -time.Minute).Issue(testUsername)
	if err != nil {
		t.Fatalf("签发过期凭证失败: %v", err)
	}
	if recorder, _ := env.do(t, http.MethodGet, "/api/admin/codes", nil, expired); recorder.Code != http.StatusUnauthorized {
		t.Fatalf("过期凭证应返回 401，实际 %d", recorder.Code)
	}

	if recorder, _ := env.do(t, http.MethodGet, "/api/admin/codes", nil, access); recorder.Code != http.StatusOK {
		t.Fatalf("有效凭证应返回 200，实际 %d", recorder.Code)
	}
}

func TestGenerateAndListCodes(t *testing.T) {
	env := newTestEnv(t)
	pattern := regexp.MustCompile(`^[0-9a-z]{16}$`)

	recorder, payload := env.do(t, http.MethodPost, "/api/admin/codes", map[string]any{
		"note":            "首批",
		"subscriptionUrl": "https://upstream.example/profile.yaml",
	}, env.token)
	if recorder.Code != http.StatusOK {
		t.Fatalf("生成兑换码应返回 200，实际 %d: %s", recorder.Code, recorder.Body.String())
	}
	generated := itemsOf(t, payload)
	if len(generated) != 1 {
		t.Fatalf("一次只能生成 1 个兑换码，实际 %d", len(generated))
	}
	seen := map[string]bool{}
	for _, item := range generated {
		code, _ := item["code"].(string)
		if !pattern.MatchString(code) {
			t.Fatalf("兑换码格式不正确: %s", code)
		}
		if seen[code] {
			t.Fatalf("出现重复兑换码: %s", code)
		}
		seen[code] = true
		if item["status"] != store.StatusUnused {
			t.Fatalf("新码状态应为 unused，实际 %v", item["status"])
		}
	}

	if recorder, _ := env.do(t, http.MethodPost, "/api/admin/codes", map[string]any{}, env.token); recorder.Code != http.StatusBadRequest {
		t.Fatalf("缺少 3x-ui 订阅链接应返回 400，实际 %d", recorder.Code)
	}
	if recorder, _ := env.do(t, http.MethodPost, "/api/admin/codes", map[string]any{
		"note": strings.Repeat("长", 101),
	}, env.token); recorder.Code != http.StatusBadRequest {
		t.Fatalf("备注超长应返回 400，实际 %d", recorder.Code)
	}

	_, payload = env.do(t, http.MethodPost, "/api/admin/codes", map[string]any{
		"note":            "第二批",
		"subscriptionUrl": "https://upstream.example/profile.yaml",
	}, env.token)

	_, list := env.do(t, http.MethodGet, "/api/admin/codes?page=1&size=2", nil, env.token)
	if total := totalOf(t, list); total != 2 {
		t.Fatalf("总数应为 2，实际 %d", total)
	}
	if len(itemsOf(t, list)) != 2 {
		t.Fatalf("分页 size=2 应返回 2 条，实际 %d", len(itemsOf(t, list)))
	}

	_, filtered := env.do(t, http.MethodGet, "/api/admin/codes?keyword=第二批", nil, env.token)
	if total := totalOf(t, filtered); total != 1 {
		t.Fatalf("按备注筛选应为 1，实际 %d", total)
	}

	if recorder, _ := env.do(t, http.MethodGet, "/api/admin/codes?status=unknown", nil, env.token); recorder.Code != http.StatusBadRequest {
		t.Fatalf("非法状态应返回 400，实际 %d", recorder.Code)
	}
}

func TestVoidAndRestoreCode(t *testing.T) {
	env := newTestEnv(t)

	_, payload := env.do(t, http.MethodPost, "/api/admin/codes", map[string]any{
		"note":            "",
		"subscriptionUrl": "https://upstream.example/profile.yaml",
	}, env.token)
	items := itemsOf(t, payload)
	firstID := int64(items[0]["id"].(float64))
	_, secondPayload := env.do(t, http.MethodPost, "/api/admin/codes", map[string]any{
		"note":            "",
		"subscriptionUrl": "https://upstream.example/profile.yaml",
	}, env.token)
	secondID := int64(itemsOf(t, secondPayload)[0]["id"].(float64))

	path := "/api/admin/codes/" + strconv.FormatInt(firstID, 10)
	if recorder, _ := env.do(t, http.MethodPatch, path, map[string]string{"status": "void"}, env.token); recorder.Code != http.StatusOK {
		t.Fatalf("作废应返回 200，实际 %d", recorder.Code)
	}

	_, voided := env.do(t, http.MethodGet, "/api/admin/codes?status=void", nil, env.token)
	if total := totalOf(t, voided); total != 1 {
		t.Fatalf("作废后应有 1 条作废记录，实际 %d", total)
	}

	if recorder, _ := env.do(t, http.MethodPatch, path, map[string]string{"status": "unused"}, env.token); recorder.Code != http.StatusOK {
		t.Fatalf("恢复应返回 200，实际 %d", recorder.Code)
	}
	_, restored := env.do(t, http.MethodGet, "/api/admin/codes?status=unused", nil, env.token)
	if total := totalOf(t, restored); total != 2 {
		t.Fatalf("恢复后应有 2 条未使用记录，实际 %d", total)
	}

	if _, err := env.db.Exec(
		"UPDATE codes SET status = ?, used_at = ? WHERE id = ?",
		store.StatusUsed, time.Now().Unix(), secondID,
	); err != nil {
		t.Fatalf("准备已使用兑换码失败: %v", err)
	}
	usedPath := "/api/admin/codes/" + strconv.FormatInt(secondID, 10)
	if recorder, _ := env.do(t, http.MethodPatch, usedPath, map[string]string{"status": "void"}, env.token); recorder.Code != http.StatusBadRequest {
		t.Fatalf("已使用的兑换码作废应返回 400，实际 %d", recorder.Code)
	}
	if recorder, _ := env.do(t, http.MethodPatch, "/api/admin/codes/999999", map[string]string{"status": "void"}, env.token); recorder.Code != http.StatusNotFound {
		t.Fatalf("不存在的兑换码应返回 404，实际 %d", recorder.Code)
	}
	if recorder, _ := env.do(t, http.MethodPatch, path, map[string]string{"status": "used"}, env.token); recorder.Code != http.StatusBadRequest {
		t.Fatalf("非法目标状态应返回 400，实际 %d", recorder.Code)
	}
}

func TestSettingsReadWriteAndMask(t *testing.T) {
	env := newTestEnv(t)

	_, payload := env.do(t, http.MethodGet, "/api/admin/settings", nil, env.token)
	groups, _ := payload["data"].(map[string]any)["groups"].([]any)
	if len(groups) != 2 {
		t.Fatalf("应返回 2 组配置，实际 %d", len(groups))
	}
	var group map[string]any
	for _, raw := range groups {
		candidate := raw.(map[string]any)
		if candidate["key"] == "alipay" {
			group = candidate
		}
	}
	if group == nil {
		t.Fatal("缺少支付宝配置分组")
	}
	var appGroup map[string]any
	for _, raw := range groups {
		candidate := raw.(map[string]any)
		if candidate["key"] == "app" {
			appGroup = candidate
		}
	}
	if appGroup == nil {
		t.Fatal("缺少系统配置分组")
	}
	appFields := appGroup["fields"].([]any)
	if len(appFields) != 1 || appFields[0].(map[string]any)["value"] != "https://netx.beisi.tech" {
		t.Fatalf("项目根地址默认值不正确: %v", appFields)
	}
	if group["title"] != "支付宝" {
		t.Fatalf("配置分组标题不正确: %v", group["title"])
	}
	fields := map[string]map[string]any{}
	for _, raw := range group["fields"].([]any) {
		field := raw.(map[string]any)
		fields[field["key"].(string)] = field
	}
	if fields["alipay.gateway"]["value"] != "https://openapi.alipay.com/gateway.do" {
		t.Fatalf("网关地址应有默认值，实际 %v", fields["alipay.gateway"]["value"])
	}
	if fields["alipay.private_key"]["configured"] != false {
		t.Fatalf("未配置的私钥 configured 应为 false，实际 %v", fields["alipay.private_key"]["configured"])
	}
	if _, exists := fields["alipay.subscription_url"]; exists {
		t.Fatal("支付宝配置不应再包含全局订阅地址")
	}

	recorder, _ := env.do(t, http.MethodPut, "/api/admin/settings", map[string]any{
		"values": map[string]string{
			"alipay.app_id":      "2021000000000000",
			"alipay.private_key": "MIIEvQIBADANBgkqhkiG9w0BAQEFAA0C",
		},
	}, env.token)
	if recorder.Code != http.StatusOK {
		t.Fatalf("保存配置应返回 200，实际 %d: %s", recorder.Code, recorder.Body.String())
	}

	_, payload = env.do(t, http.MethodGet, "/api/admin/settings", nil, env.token)
	fields = map[string]map[string]any{}
	for _, raw := range alipayGroupFromPayload(t, payload)["fields"].([]any) {
		field := raw.(map[string]any)
		fields[field["key"].(string)] = field
	}
	if fields["alipay.app_id"]["value"] != "2021000000000000" {
		t.Fatalf("APPID 未保存成功: %v", fields["alipay.app_id"]["value"])
	}
	key := fields["alipay.private_key"]
	if key["configured"] != true {
		t.Fatalf("私钥应标记为已配置: %v", key)
	}
	if key["value"] != "" {
		t.Fatalf("私钥不应回显明文: %v", key["value"])
	}
	if hint, _ := key["hint"].(string); !strings.HasPrefix(hint, "****") || !strings.HasSuffix(hint, "AA0C") {
		t.Fatalf("私钥掩码不正确: %v", key["hint"])
	}

	// 留空表示不修改密钥
	if recorder, _ := env.do(t, http.MethodPut, "/api/admin/settings", map[string]any{
		"values": map[string]string{"alipay.app_id": "2021000000000001"},
	}, env.token); recorder.Code != http.StatusOK {
		t.Fatalf("保存配置应返回 200，实际 %d", recorder.Code)
	}
	_, payload = env.do(t, http.MethodGet, "/api/admin/settings", nil, env.token)
	for _, raw := range alipayGroupFromPayload(t, payload)["fields"].([]any) {
		field := raw.(map[string]any)
		if field["key"] == "alipay.private_key" && field["configured"] != true {
			t.Fatalf("未提交的密钥不应被清空: %v", field)
		}
	}

	if recorder, _ := env.do(t, http.MethodPut, "/api/admin/settings", map[string]any{
		"values": map[string]string{"alipay.unknown": "x"},
	}, env.token); recorder.Code != http.StatusBadRequest {
		t.Fatalf("未知配置项应返回 400，实际 %d", recorder.Code)
	}
}

func alipayGroupFromPayload(t *testing.T, payload map[string]any) map[string]any {
	t.Helper()
	groups := payload["data"].(map[string]any)["groups"].([]any)
	for _, raw := range groups {
		group := raw.(map[string]any)
		if group["key"] == "alipay" {
			return group
		}
	}
	t.Fatal("缺少支付宝配置分组")
	return nil
}

func TestAdminStaticFilesAndFallback(t *testing.T) {
	env := newTestEnv(t)

	recorder, _ := env.do(t, http.MethodGet, "/admin/", nil, "")
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), "netx admin") {
		t.Fatalf("访问 /admin/ 应返回入口页，实际 %d: %s", recorder.Code, recorder.Body.String())
	}

	recorder, _ = env.do(t, http.MethodGet, "/admin/codes", nil, "")
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), "netx admin") {
		t.Fatalf("前端路由应回落到入口页，实际 %d: %s", recorder.Code, recorder.Body.String())
	}
}

func TestSubscriptionProxyReturnsYAML(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/yaml")
		_, _ = w.Write([]byte("proxies:\n  - name: Los-netx|963.32GB\n"))
	}))
	defer upstream.Close()

	env := newTestEnv(t)
	_, payload := env.do(t, http.MethodPost, "/api/admin/codes", map[string]any{
		"subscriptionUrl": upstream.URL + "/profile.yaml",
	}, env.token)
	code := itemsOf(t, payload)[0]["code"].(string)
	if recorder, _ := env.do(t, http.MethodGet, "/sub/"+code, nil, ""); recorder.Code != http.StatusNotFound {
		t.Fatalf("未兑换码不应提供订阅，实际 %d", recorder.Code)
	}

	redeemRecorder, redeemPayload := env.do(t, http.MethodPost, "/api/redeem", map[string]string{"code": code}, "")
	if redeemRecorder.Code != http.StatusOK {
		t.Fatalf("兑换应成功，实际 %d: %s", redeemRecorder.Code, redeemRecorder.Body.String())
	}
	if data, ok := redeemPayload["data"].(map[string]any); !ok || len(data) != 1 || data["accessPath"] == nil {
		t.Fatalf("兑换响应只应返回访问路径，实际 %v", redeemPayload)
	}
	recorder, _ := env.do(t, http.MethodGet, "/sub/"+code, nil, "")
	if recorder.Code != http.StatusOK {
		t.Fatalf("订阅代理应返回 200，实际 %d: %s", recorder.Code, recorder.Body.String())
	}
	if got := recorder.Header().Get("Content-Type"); got != "application/yaml; charset=utf-8" {
		t.Fatalf("订阅 Content-Type 不正确: %s", got)
	}
	if got := recorder.Header().Get("Content-Disposition"); got != "attachment; filename="+code+".yaml" {
		t.Fatalf("订阅文件名不正确: %s", got)
	}
	body := recorder.Body.String()
	for _, expected := range []string{
		"proxies:",
		"name: Netx",
		"proxy-groups:",
		"name: PROXY",
		"- Netx",
		"- DIRECT",
		"rule-providers:",
		"private:",
		"rules:",
		"DOMAIN-SUFFIX,openai.com,PROXY",
		"MATCH,PROXY",
	} {
		if !strings.Contains(body, expected) {
			t.Fatalf("订阅内容缺少 %q: %s", expected, body)
		}
	}
	if strings.Contains(body, "Los-netx|963.32GB") {
		t.Fatalf("订阅内容不应保留上游节点名称: %s", body)
	}
}

func TestSubscriptionUsageReadsUserinfoHeader(t *testing.T) {
	expire := time.Now().Add(49 * time.Hour).Unix()
	userinfo := "upload=1073741824; download=2147483648; total=10737418240; expire=" + strconv.FormatInt(expire, 10)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/yaml")
		w.Header().Set("Subscription-Userinfo", userinfo)
		_, _ = w.Write([]byte("proxies:\n  - name: test\n"))
	}))
	defer upstream.Close()

	env := newTestEnv(t)
	_, payload := env.do(t, http.MethodPost, "/api/admin/codes", map[string]any{
		"subscriptionUrl": upstream.URL + "/profile.yaml",
	}, env.token)
	code := itemsOf(t, payload)[0]["code"].(string)
	if recorder, _ := env.do(t, http.MethodPost, "/api/redeem", map[string]string{"code": code}, ""); recorder.Code != http.StatusOK {
		t.Fatalf("兑换应成功，实际 %d: %s", recorder.Code, recorder.Body.String())
	}

	recorder, response := env.do(t, http.MethodGet, "/api/subscription/"+code+"/usage", nil, "")
	if recorder.Code != http.StatusOK {
		t.Fatalf("用量接口应返回 200，实际 %d: %s", recorder.Code, recorder.Body.String())
	}
	data := response["data"].(map[string]any)
	if data["usedBytes"] != float64(3*1024*1024*1024) || data["totalBytes"] != float64(10*1024*1024*1024) {
		t.Fatalf("用量计算不正确: %v", data)
	}
	if data["unlimitedTotal"] != false || data["unlimitedTime"] != false {
		t.Fatalf("有限额订阅不应标记为无限制: %v", data)
	}
	remainingDays := data["remainingDays"].(float64)
	if remainingDays < 2 || remainingDays > 3 {
		t.Fatalf("剩余天数应约为 2-3 天，实际 %v", remainingDays)
	}

	if recorder, _ := env.do(t, http.MethodGet, "/sub/"+code, nil, ""); recorder.Header().Get("Subscription-Userinfo") != userinfo {
		t.Fatalf("订阅代理应透传用量头，实际 %q", recorder.Header().Get("Subscription-Userinfo"))
	}
}

func TestSubscriptionUsageSupportsUnlimitedValues(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Subscription-Userinfo", "upload=9671737; download=1145873032; total=0; expire=0")
		_, _ = w.Write([]byte("proxies: []\n"))
	}))
	defer upstream.Close()

	env := newTestEnv(t)
	_, payload := env.do(t, http.MethodPost, "/api/admin/codes", map[string]any{
		"subscriptionUrl": upstream.URL,
	}, env.token)
	code := itemsOf(t, payload)[0]["code"].(string)
	if recorder, _ := env.do(t, http.MethodPost, "/api/redeem", map[string]string{"code": code}, ""); recorder.Code != http.StatusOK {
		t.Fatalf("兑换应成功，实际 %d: %s", recorder.Code, recorder.Body.String())
	}

	recorder, response := env.do(t, http.MethodGet, "/api/subscription/"+code+"/usage", nil, "")
	if recorder.Code != http.StatusOK {
		t.Fatalf("无限制用量接口应返回 200，实际 %d: %s", recorder.Code, recorder.Body.String())
	}
	data := response["data"].(map[string]any)
	if data["unlimitedTotal"] != true || data["unlimitedTime"] != true || data["remainingDays"] != float64(0) {
		t.Fatalf("无限制数据不正确: %v", data)
	}
}

func TestPaymentOrderLifecycle(t *testing.T) {
	env := newTestEnv(t)
	created, err := env.db.CreateCodeWithDetails("极速版", "https://upstream.example/profile.yaml", "库存")
	if err != nil {
		t.Fatalf("预生成兑换码失败: %v", err)
	}
	codeCountBefore, err := countCodes(env.db)
	if err != nil {
		t.Fatalf("统计兑换码失败: %v", err)
	}
	order, err := env.db.CreatePaymentOrder("极速版", 50000)
	if err != nil {
		t.Fatalf("创建支付订单失败: %v", err)
	}
	if order.Status != store.PaymentPending || order.Code != created.Code {
		t.Fatalf("新订单状态或兑换码不正确: %+v", order)
	}
	codeCountAfter, err := countCodes(env.db)
	if err != nil || codeCountAfter != codeCountBefore {
		t.Fatalf("支付下单不应创建新兑换码: before=%d after=%d err=%v", codeCountBefore, codeCountAfter, err)
	}
	reserved, err := env.db.FindCode(order.Code)
	if err != nil || reserved.Status != store.StatusReserved {
		t.Fatalf("下单后兑换码应为 reserved: code=%+v err=%v", reserved, err)
	}
	paid, err := env.db.MarkPaymentPaid(order.OutTradeNo, "2026092400000000001", 50000)
	if err != nil {
		t.Fatalf("标记支付成功失败: %v", err)
	}
	if paid.Status != store.PaymentPaid {
		t.Fatalf("支付成功后订单状态不正确: %+v", paid)
	}
	code, err := env.db.FindCode(order.Code)
	if err != nil || code.Status != store.StatusUsed {
		t.Fatalf("支付成功后兑换码应自动启用: code=%+v err=%v", code, err)
	}
	if _, err := env.db.MarkPaymentPaid(order.OutTradeNo, "2026092400000000001", 50000); err != nil {
		t.Fatalf("重复通知应幂等成功: %v", err)
	}
	if _, err := env.db.MarkPaymentPaid(order.OutTradeNo, "2026092400000000001", 1); err != store.ErrPaymentAmount {
		t.Fatalf("金额不匹配应拒绝，实际 %v", err)
	}
}

func TestPaymentInventoryByPlanAndExpiry(t *testing.T) {
	env := newTestEnv(t)
	if _, err := env.db.CreateCodeWithDetails("至尊版", "https://upstream.example/profile.yaml", "库存"); err != nil {
		t.Fatalf("预生成兑换码失败: %v", err)
	}
	var ordersBefore int
	if err := env.db.QueryRow("SELECT COUNT(*) FROM payment_orders").Scan(&ordersBefore); err != nil {
		t.Fatalf("统计支付订单失败: %v", err)
	}
	if _, err := env.db.CreatePaymentOrder("极速版", 50000); err != store.ErrPaymentOutOfStock {
		t.Fatalf("无对应套餐库存应返回库存不足，实际 %v", err)
	}
	var ordersAfter int
	if err := env.db.QueryRow("SELECT COUNT(*) FROM payment_orders").Scan(&ordersAfter); err != nil || ordersAfter != ordersBefore {
		t.Fatalf("库存不足不应创建支付订单: before=%d after=%d err=%v", ordersBefore, ordersAfter, err)
	}

	created, err := env.db.CreateCodeWithDetails("极速版", "https://upstream.example/profile.yaml", "库存")
	if err != nil {
		t.Fatalf("预生成兑换码失败: %v", err)
	}
	codes := []store.Code{created}
	order, err := env.db.CreatePaymentOrder("极速版", 50000)
	if err != nil {
		t.Fatalf("创建支付订单失败: %v", err)
	}
	if recorder, _ := env.do(t, http.MethodPost, "/api/redeem", map[string]string{"code": order.Code}, ""); recorder.Code != http.StatusBadRequest {
		t.Fatalf("reserved 兑换码不应被手动兑换，实际 %d", recorder.Code)
	}
	reservedRow, err := env.db.FindCode(order.Code)
	if err != nil {
		t.Fatalf("读取预占兑换码失败: %v", err)
	}
	reservedPath := "/api/admin/codes/" + strconv.FormatInt(reservedRow.ID, 10)
	if recorder, _ := env.do(t, http.MethodPatch, reservedPath, map[string]string{"status": "void"}, env.token); recorder.Code != http.StatusBadRequest {
		t.Fatalf("reserved 兑换码不应被后台修改，实际 %d", recorder.Code)
	}
	old := time.Now().Add(-store.PaymentReservationTTL - time.Minute).Unix()
	if _, err := env.db.Exec("UPDATE payment_orders SET created_at = ? WHERE out_trade_no = ?", old, order.OutTradeNo); err != nil {
		t.Fatalf("准备过期订单失败: %v", err)
	}
	if err := env.db.ReleaseExpiredPaymentOrders(time.Now()); err != nil {
		t.Fatalf("释放过期订单失败: %v", err)
	}
	released, err := env.db.FindCode(codes[0].Code)
	if err != nil || released.Status != store.StatusUnused {
		t.Fatalf("过期订单应释放兑换码: code=%+v err=%v", released, err)
	}
	if _, err := env.db.MarkPaymentPaid(order.OutTradeNo, "2026092400000000002", 50000); err != store.ErrPaymentState {
		t.Fatalf("过期订单不应再支付成功，实际 %v", err)
	}
}

func countCodes(db *store.DB) (int, error) {
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM codes").Scan(&count)
	return count, err
}
