package httpapi

import (
	"net/http"
	"strings"
)

// requireAuth 校验 Authorization: Bearer <token>。
func (s *Server) requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		parts := strings.Fields(r.Header.Get("Authorization"))
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			fail(w, http.StatusUnauthorized, "请先登录")
			return
		}
		if _, err := s.auth.Verify(parts[1]); err != nil {
			fail(w, http.StatusUnauthorized, "登录已过期，请重新登录")
			return
		}
		next(w, r)
	}
}
