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

	adminFS := fstest.MapFS{
		"index.html": &fstest.MapFile{Data: []byte("<html>netx admin</html>")},
	}
	return &testEnv{
		handler: httpapi.New(cfg, db, adminFS),
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
	pattern := regexp.MustCompile(`^NETX-[0-9A-Z]{4}-[0-9A-Z]{4}-[0-9A-Z]{4}$`)

	recorder, payload := env.do(t, http.MethodPost, "/api/admin/codes", map[string]any{
		"count": 5,
		"note":  "首批",
	}, env.token)
	if recorder.Code != http.StatusOK {
		t.Fatalf("生成兑换码应返回 200，实际 %d: %s", recorder.Code, recorder.Body.String())
	}
	generated := itemsOf(t, payload)
	if len(generated) != 5 {
		t.Fatalf("应生成 5 个兑换码，实际 %d", len(generated))
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

	if recorder, _ := env.do(t, http.MethodPost, "/api/admin/codes", map[string]any{"count": 0}, env.token); recorder.Code != http.StatusBadRequest {
		t.Fatalf("数量为 0 应返回 400，实际 %d", recorder.Code)
	}
	if recorder, _ := env.do(t, http.MethodPost, "/api/admin/codes", map[string]any{"count": 201}, env.token); recorder.Code != http.StatusBadRequest {
		t.Fatalf("数量为 201 应返回 400，实际 %d", recorder.Code)
	}
	if recorder, _ := env.do(t, http.MethodPost, "/api/admin/codes", map[string]any{
		"count": 1,
		"note":  strings.Repeat("长", 101),
	}, env.token); recorder.Code != http.StatusBadRequest {
		t.Fatalf("备注超长应返回 400，实际 %d", recorder.Code)
	}

	_, payload = env.do(t, http.MethodPost, "/api/admin/codes", map[string]any{
		"count": 3,
		"note":  "第二批",
	}, env.token)

	_, list := env.do(t, http.MethodGet, "/api/admin/codes?page=1&size=2", nil, env.token)
	if total := totalOf(t, list); total != 8 {
		t.Fatalf("总数应为 8，实际 %d", total)
	}
	if len(itemsOf(t, list)) != 2 {
		t.Fatalf("分页 size=2 应返回 2 条，实际 %d", len(itemsOf(t, list)))
	}

	_, filtered := env.do(t, http.MethodGet, "/api/admin/codes?keyword=第二批", nil, env.token)
	if total := totalOf(t, filtered); total != 3 {
		t.Fatalf("按备注筛选应为 3，实际 %d", total)
	}

	if recorder, _ := env.do(t, http.MethodGet, "/api/admin/codes?status=unknown", nil, env.token); recorder.Code != http.StatusBadRequest {
		t.Fatalf("非法状态应返回 400，实际 %d", recorder.Code)
	}
}

func TestVoidAndRestoreCode(t *testing.T) {
	env := newTestEnv(t)

	_, payload := env.do(t, http.MethodPost, "/api/admin/codes", map[string]any{
		"count": 2,
		"note":  "",
	}, env.token)
	items := itemsOf(t, payload)
	firstID := int64(items[0]["id"].(float64))
	secondID := int64(items[1]["id"].(float64))

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
	if len(groups) != 1 {
		t.Fatalf("应返回 1 组配置，实际 %d", len(groups))
	}
	group := groups[0].(map[string]any)
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

	recorder, _ := env.do(t, http.MethodPut, "/api/admin/settings", map[string]any{
		"values": map[string]string{
			"alipay.app_id":      "2021000000000000",
			"alipay.private_key": "MIIEvQIBADANBgkqhkiG9w0BAQEFAA0C",
			"alipay.sandbox":     "true",
		},
	}, env.token)
	if recorder.Code != http.StatusOK {
		t.Fatalf("保存配置应返回 200，实际 %d: %s", recorder.Code, recorder.Body.String())
	}

	_, payload = env.do(t, http.MethodGet, "/api/admin/settings", nil, env.token)
	fields = map[string]map[string]any{}
	for _, raw := range payload["data"].(map[string]any)["groups"].([]any)[0].(map[string]any)["fields"].([]any) {
		field := raw.(map[string]any)
		fields[field["key"].(string)] = field
	}
	if fields["alipay.app_id"]["value"] != "2021000000000000" {
		t.Fatalf("APPID 未保存成功: %v", fields["alipay.app_id"]["value"])
	}
	if fields["alipay.sandbox"]["value"] != "true" {
		t.Fatalf("沙箱开关未保存成功: %v", fields["alipay.sandbox"]["value"])
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
	for _, raw := range payload["data"].(map[string]any)["groups"].([]any)[0].(map[string]any)["fields"].([]any) {
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
	if recorder, _ := env.do(t, http.MethodPut, "/api/admin/settings", map[string]any{
		"values": map[string]string{"alipay.sandbox": "yes"},
	}, env.token); recorder.Code != http.StatusBadRequest {
		t.Fatalf("非法布尔值应返回 400，实际 %d", recorder.Code)
	}
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
