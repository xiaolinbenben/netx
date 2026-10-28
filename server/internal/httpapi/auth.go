package httpapi

import (
	"crypto/subtle"
	"net/http"
)

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type refreshRequest struct {
	RefreshToken string `json:"refreshToken"`
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if !secureEqual(req.Username, s.cfg.AdminUsername) || !secureEqual(req.Password, s.cfg.AdminPassword) {
		fail(w, http.StatusUnauthorized, "账号或密码错误")
		return
	}

	access, refresh, _, err := s.auth.Issue(req.Username)
	if err != nil {
		fail(w, http.StatusInternalServerError, "签发登录凭证失败")
		return
	}

	ok(w, map[string]any{
		"username":     req.Username,
		"accessToken":  access,
		"refreshToken": refresh,
	})
}

func (s *Server) handleRefreshToken(w http.ResponseWriter, r *http.Request) {
	var req refreshRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	access, _, err := s.auth.Refresh(req.RefreshToken)
	if err != nil {
		fail(w, http.StatusUnauthorized, "登录已过期，请重新登录")
		return
	}

	ok(w, map[string]any{
		"accessToken":  access,
		"refreshToken": req.RefreshToken,
	})
}

func secureEqual(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}
