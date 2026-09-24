package httpapi

import (
	"net/http"
	"strings"

	"netx/server/internal/store"
)

func (s *Server) handleRedeem(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Code string `json:"code"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	code := strings.TrimSpace(req.Code)
	if code == "" {
		fail(w, http.StatusBadRequest, "请输入兑换码")
		return
	}
	item, err := s.store.RedeemCode(code)
	if err == store.ErrCodeNotFound {
		fail(w, http.StatusNotFound, "兑换码不存在")
		return
	}
	if err == store.ErrCodeUsed {
		fail(w, http.StatusBadRequest, "兑换码已使用或已作废")
		return
	}
	if err != nil {
		fail(w, http.StatusInternalServerError, "兑换码兑换失败")
		return
	}
	ok(w, map[string]any{"code": item.Code, "plan": item.Plan, "subscriptionUrl": item.SubscriptionURL, "accessPath": "/access/" + item.Code})
}
