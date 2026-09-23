package httpapi

import (
	"io/fs"
	"log"
	"mime"
	"net/http"
	"path"
	"strings"
	"time"

	"netx/server/internal/auth"
	"netx/server/internal/config"
	"netx/server/internal/store"
)

type Server struct {
	cfg     config.Config
	store   *store.DB
	auth    *auth.Manager
	adminFS fs.FS
}

// New 组装全部路由：/api 提供接口，/admin 提供内嵌的管理端静态资源。
func New(cfg config.Config, db *store.DB, adminFS fs.FS) http.Handler {
	server := &Server{
		cfg:     cfg,
		store:   db,
		auth:    auth.NewManager(cfg.JWTSecret),
		adminFS: adminFS,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/admin/login", server.handleLogin)
	mux.HandleFunc("POST /api/admin/refresh-token", server.requireAuth(server.handleRefreshToken))
	mux.HandleFunc("GET /api/admin/codes", server.requireAuth(server.handleListCodes))
	mux.HandleFunc("POST /api/admin/codes", server.requireAuth(server.handleGenerateCodes))
	mux.HandleFunc("PATCH /api/admin/codes/{id}", server.requireAuth(server.handleUpdateCodeStatus))
	mux.HandleFunc("GET /api/admin/settings", server.requireAuth(server.handleGetSettings))
	mux.HandleFunc("PUT /api/admin/settings", server.requireAuth(server.handleSaveSettings))
	mux.HandleFunc("POST /api/redeem", server.handleRedeem)
	mux.HandleFunc("GET /sub/{code}", server.handleSubscription)
	mux.Handle("GET /admin/", server.adminHandler())

	return logAPIRequests(mux)
}

// adminHandler 提供管理端静态资源，未命中文件时回落到入口页，交给前端路由处理。
func (s *Server) adminHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(r.URL.Path, "/admin/")
		if name == "" {
			name = "index.html"
		}
		if strings.HasPrefix(name, ".") {
			name = "index.html"
		}
		if info, err := fs.Stat(s.adminFS, name); err != nil || info.IsDir() {
			name = "index.html"
		}
		content, err := fs.ReadFile(s.adminFS, name)
		if err != nil {
			fail(w, http.StatusNotFound, "管理端静态资源不存在，请先执行 pnpm build 重新构建后端")
			return
		}
		if name == "index.html" {
			// 入口页不能缓存，避免发版后浏览器继续用旧资源
			w.Header().Set("Cache-Control", "no-cache")
		} else if strings.HasPrefix(name, "static/") {
			// 带 hash 的静态资源可以长期缓存
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		}
		if contentType := mime.TypeByExtension(path.Ext(name)); contentType != "" {
			w.Header().Set("Content-Type", contentType)
		}
		_, _ = w.Write(content)
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func logAPIRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/api") {
			next.ServeHTTP(w, r)
			return
		}
		start := time.Now()
		recorder := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(recorder, r)
		log.Printf("%s %s %d %s", r.Method, r.URL.Path, recorder.status, time.Since(start).Round(time.Millisecond))
	})
}
