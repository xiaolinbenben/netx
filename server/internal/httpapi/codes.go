package httpapi

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	"netx/server/internal/store"
)

const (
	defaultPageSize = 20
	maxPageSize     = 200
	maxNoteLength   = 100
)

type codeView struct {
	ID              int64  `json:"id"`
	Code            string `json:"code"`
	Note            string `json:"note"`
	Status          string `json:"status"`
	CreatedAt       string `json:"createdAt"`
	UsedAt          string `json:"usedAt"`
	Plan            string `json:"plan"`
	SubscriptionURL string `json:"subscriptionUrl"`
}

func (s *Server) handleListCodes(w http.ResponseWriter, r *http.Request) {
	params := r.URL.Query()
	page := positiveInt(params.Get("page"), 1)
	size := positiveInt(params.Get("size"), defaultPageSize)
	if size > maxPageSize {
		size = defaultPageSize
	}
	status := params.Get("status")
	if status != "" && !validStatus(status) {
		fail(w, http.StatusBadRequest, "状态参数不合法")
		return
	}

	items, total, err := s.store.ListCodes(store.CodeFilter{
		Page:    page,
		Size:    size,
		Status:  status,
		Keyword: strings.TrimSpace(params.Get("keyword")),
	})
	if err != nil {
		fail(w, http.StatusInternalServerError, "查询兑换码失败")
		return
	}

	ok(w, map[string]any{
		"items": codeViews(items),
		"total": total,
	})
}

func (s *Server) handleGenerateCodes(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Count           *int   `json:"count"`
		Note            string `json:"note"`
		Plan            string `json:"plan"`
		SubscriptionURL string `json:"subscriptionUrl"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	count := 1
	if req.Count != nil {
		count = *req.Count
	}
	if count < 1 || count > maxPageSize {
		fail(w, http.StatusBadRequest, "单次生成数量需在 1-200 之间")
		return
	}
	note := strings.TrimSpace(req.Note)
	plan := strings.TrimSpace(req.Plan)
	subscriptionURL := strings.TrimSpace(req.SubscriptionURL)
	if plan == "" {
		plan = "极速版"
	}
	if plan != "极速版" && plan != "至尊版" {
		fail(w, http.StatusBadRequest, "套餐类型不合法")
		return
	}
	if subscriptionURL == "" {
		fail(w, http.StatusBadRequest, "请填写 3x-ui 订阅链接")
		return
	}
	if utf8.RuneCountInString(note) > maxNoteLength {
		fail(w, http.StatusBadRequest, "备注不能超过 100 字")
		return
	}

	items, err := s.store.CreateCodesWithDetails(count, plan, subscriptionURL, note)
	if err != nil {
		fail(w, http.StatusInternalServerError, "生成兑换码失败")
		return
	}
	ok(w, map[string]any{"items": codeViews(items)})
}

func (s *Server) handleUpdateCodeStatus(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		fail(w, http.StatusBadRequest, "兑换码 ID 不合法")
		return
	}

	var req struct {
		Status string `json:"status"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.Status != store.StatusVoid && req.Status != store.StatusUnused {
		fail(w, http.StatusBadRequest, "只支持作废或恢复兑换码")
		return
	}

	switch err := s.store.UpdateCodeStatus(id, req.Status); {
	case errors.Is(err, store.ErrCodeNotFound):
		fail(w, http.StatusNotFound, "兑换码不存在")
	case errors.Is(err, store.ErrCodeUsed):
		fail(w, http.StatusBadRequest, "已使用的兑换码不能作废")
	case errors.Is(err, store.ErrCodeReserved):
		fail(w, http.StatusBadRequest, "待支付兑换码不能修改")
	case err != nil:
		fail(w, http.StatusInternalServerError, "更新兑换码失败")
	default:
		ok(w, nil)
	}
}

func codeViews(items []store.Code) []codeView {
	views := make([]codeView, 0, len(items))
	for _, item := range items {
		view := codeView{
			ID:              item.ID,
			Code:            item.Code,
			Note:            item.Note,
			Status:          item.Status,
			CreatedAt:       item.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
			Plan:            item.Plan,
			SubscriptionURL: item.SubscriptionURL,
		}
		if item.UsedAt != nil {
			view.UsedAt = item.UsedAt.Format("2006-01-02T15:04:05Z07:00")
		}
		views = append(views, view)
	}
	return views
}

func validStatus(status string) bool {
	switch status {
	case store.StatusUnused, store.StatusReserved, store.StatusUsed, store.StatusVoid:
		return true
	default:
		return false
	}
}

func positiveInt(raw string, fallback int) int {
	value, err := strconv.Atoi(raw)
	if err != nil || value < 1 {
		return fallback
	}
	return value
}
