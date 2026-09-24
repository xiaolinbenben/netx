package httpapi

import (
	"fmt"
	"net/http"
	"strings"
)

// settingField 描述一个可配置项，前端按这些定义渲染表单。
type settingField struct {
	Key         string
	Label       string
	Type        string
	Secret      bool
	Default     string
	Placeholder string
}

type settingGroup struct {
	Key    string
	Title  string
	Fields []settingField
}

// settingsSchema 是系统配置的白名单，新增配置项只需要在这里加一行。
var settingsSchema = []settingGroup{
	{
		Key:   "app",
		Title: "系统",
		Fields: []settingField{
			{
				Key:         "app.base_url",
				Label:       "项目运行根地址",
				Type:        "text",
				Default:     "https://netx.beisi.tech",
				Placeholder: "https://netx.beisi.tech",
			},
		},
	},
	{
		Key:   "alipay",
		Title: "支付宝",
		Fields: []settingField{
			{Key: "alipay.app_id", Label: "应用 APPID", Type: "text"},
			{
				Key:         "alipay.gateway",
				Label:       "网关地址",
				Type:        "text",
				Default:     "https://openapi.alipay.com/gateway.do",
				Placeholder: "https://openapi.alipay.com/gateway.do",
			},
			{Key: "alipay.private_key", Label: "应用私钥", Type: "textarea", Secret: true},
			{Key: "alipay.public_key", Label: "支付宝公钥", Type: "textarea", Secret: true},
			{
				Key:         "alipay.subscription_url",
				Label:       "默认订阅源地址",
				Type:        "text",
				Placeholder: "https://你的订阅服务.example/profile.yaml",
			},
		},
	},
}

func (s *Server) handleGetSettings(w http.ResponseWriter, r *http.Request) {
	stored, err := s.store.Settings()
	if err != nil {
		fail(w, http.StatusInternalServerError, "读取系统配置失败")
		return
	}

	groups := make([]map[string]any, 0, len(settingsSchema))
	for _, group := range settingsSchema {
		fields := make([]map[string]any, 0, len(group.Fields))
		for _, field := range group.Fields {
			raw := stored[field.Key]
			value := raw
			if value == "" {
				value = field.Default
			}
			// 密钥不返回明文，只告诉前端是否已配置
			hint := ""
			if field.Secret {
				hint = maskHint(raw)
				value = ""
			}
			fields = append(fields, map[string]any{
				"key":         field.Key,
				"label":       field.Label,
				"type":        field.Type,
				"secret":      field.Secret,
				"value":       value,
				"configured":  raw != "",
				"hint":        hint,
				"placeholder": field.Placeholder,
			})
		}
		groups = append(groups, map[string]any{
			"key":    group.Key,
			"title":  group.Title,
			"fields": fields,
		})
	}

	ok(w, map[string]any{"groups": groups})
}

func (s *Server) handleSaveSettings(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Values map[string]string `json:"values"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}

	updates := make(map[string]string, len(req.Values))
	for key, value := range req.Values {
		field, exists := settingsFieldByKey(key)
		if !exists {
			fail(w, http.StatusBadRequest, fmt.Sprintf("不支持的配置项: %s", key))
			return
		}
		value = strings.TrimSpace(value)
		// 密钥类字段留空表示保持原值
		if field.Secret && value == "" {
			continue
		}
		updates[key] = value
	}
	if len(updates) == 0 {
		fail(w, http.StatusBadRequest, "没有需要保存的配置")
		return
	}

	if err := s.store.SaveSettings(updates); err != nil {
		fail(w, http.StatusInternalServerError, "保存系统配置失败")
		return
	}
	ok(w, nil)
}

func settingsFieldByKey(key string) (settingField, bool) {
	for _, group := range settingsSchema {
		for _, field := range group.Fields {
			if field.Key == key {
				return field, true
			}
		}
	}
	return settingField{}, false
}

func maskHint(value string) string {
	if value == "" {
		return ""
	}
	if len(value) <= 4 {
		return "****"
	}
	return "****" + value[len(value)-4:]
}
