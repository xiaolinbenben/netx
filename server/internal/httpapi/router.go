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
	webFS   fs.FS
}

// New 组装 API、订阅和由 Go 托管的三个前端入口。
func New(cfg config.Config, db *store.DB, webFS fs.FS) http.Handler {
	server := &Server{
		cfg:     cfg,
		store:   db,
		auth:    auth.NewManager(cfg.JWTSecret),
		webFS:   webFS,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/admin/login", server.handleLogin)
	mux.HandleFunc("POST /api/admin/refresh-token", server.requireAuth(server.handleRefreshToken))
	mux.HandleFunc("GET /api/admin/codes", server.requireAuth(server.handleListCodes))
	mux.HandleFunc("POST /api/admin/codes", server.requireAuth(server.handleGenerateCodes))
	mux.HandleFunc("PATCH /api/admin/codes/{id}", server.requireAuth(server.handleUpdateCodeStatus))
	mux.HandleFunc("GET /api/admin/settings", server.requireAuth(server.handleGetSettings))
	mux.HandleFunc("PUT /api/admin/settings", server.requireAuth(server.handleSaveSettings))
	mux.HandleFunc("POST /api/payment/alipay/create", server.handleCreateAlipayPayment)
	mux.HandleFunc("POST /api/payment/alipay/notify", server.handleAlipayNotify)
	mux.HandleFunc("POST /api/redeem", server.handleRedeem)
	mux.HandleFunc("GET /api/subscription/{code}/usage", server.handleSubscriptionUsage)
	mux.HandleFunc("GET /sub/{code}", server.handleSubscription)
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	mux.Handle("GET /", server.staticHandler())

	return logAPIRequests(mux)
}

// staticHandler 按公开路径选择静态前端，并把前端路由回落到对应入口页。
func (s *Server) staticHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		urlPrefix, fsPrefix, entry := staticTarget(r.URL.Path)
		name := strings.TrimPrefix(r.URL.Path, urlPrefix)
		if name == "" || name == "/" || strings.HasPrefix(name, ".") {
			name = entry
		} else {
			name = strings.TrimPrefix(name, "/")
		}
		if info, err := fs.Stat(s.webFS, path.Join(fsPrefix, name)); err != nil || info.IsDir() {
			name = entry
		}
		content, err := fs.ReadFile(s.webFS, path.Join(fsPrefix, name))
		if err != nil {
			fail(w, http.StatusNotFound, "前端静态资源不存在，请先执行前端构建")
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

func staticTarget(requestPath string) (string, string, string) {
	switch {
	case strings.HasPrefix(requestPath, "/dashboard/access/"):
		return "/dashboard/access", "dashboard/access", "index.html"
	case strings.HasPrefix(requestPath, "/admin/") || requestPath == "/admin":
		return "/admin", "admin", "index.html"
	case strings.HasPrefix(requestPath, "/dashboard/") || requestPath == "/dashboard":
		return "/dashboard", "dashboard", "index.html"
	case strings.HasPrefix(requestPath, "/access/") || requestPath == "/access":
		return "/access", "dashboard/access", "index.html"
	default:
		return "", ".", "index.html"
	}
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
