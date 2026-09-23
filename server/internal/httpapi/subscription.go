package httpapi

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"netx/server/internal/store"
)

const maxSubscriptionSize = 10 << 20

func (s *Server) handleSubscription(w http.ResponseWriter, r *http.Request) {
	code := strings.ToUpper(strings.TrimSpace(r.PathValue("code")))
	if code == "" {
		fail(w, http.StatusNotFound, "订阅不存在")
		return
	}
	item, err := s.store.FindCode(code)
	if err == store.ErrCodeNotFound {
		fail(w, http.StatusNotFound, "订阅不存在")
		return
	}
	if err != nil {
		fail(w, http.StatusInternalServerError, "读取订阅失败")
		return
	}
	if item.Status != store.StatusUsed || strings.TrimSpace(item.SubscriptionURL) == "" {
		fail(w, http.StatusNotFound, "订阅不存在")
		return
	}

	source, err := url.Parse(strings.TrimSpace(item.SubscriptionURL))
	if err != nil || (source.Scheme != "http" && source.Scheme != "https") || source.Host == "" {
		fail(w, http.StatusBadGateway, "订阅地址无效")
		return
	}

	client := &http.Client{Timeout: 20 * time.Second}
	upstream, err := client.Get(source.String())
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

	w.Header().Set("Content-Type", "application/yaml; charset=utf-8")
	w.Header().Set("Content-Disposition", `inline; filename="`+code+`.yaml"`)
	_, _ = w.Write(body)
}
