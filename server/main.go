package main

import (
	"context"
	"embed"
	"errors"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"netx/server/internal/config"
	"netx/server/internal/httpapi"
	"netx/server/internal/store"
)

//go:embed all:web/dist
var adminAssets embed.FS

//go:embed migrations/*.sql
var migrations embed.FS

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("配置错误: %v", err)
	}

	db, err := store.Open(cfg.DBPath)
	if err != nil {
		log.Fatalf("初始化数据库失败: %v", err)
	}
	defer func() { _ = db.Close() }()

	if err := store.Migrate(db.DB, migrations); err != nil {
		log.Fatalf("执行数据库迁移失败: %v", err)
	}

	adminFS, err := fs.Sub(adminAssets, "web/dist")
	if err != nil {
		log.Fatalf("读取管理端静态资源失败: %v", err)
	}
	if _, err := fs.Stat(adminFS, "index.html"); err != nil {
		log.Print("提示：未检测到管理端构建产物，/admin 将不可用，请先执行 pnpm build")
	}

	server := &http.Server{
		Addr:              cfg.Addr,
		Handler:           httpapi.New(cfg, db, adminFS),
		ReadHeaderTimeout: 5 * time.Second,
	}
	if err := db.ReleaseExpiredPaymentOrders(time.Now()); err != nil {
		log.Printf("启动时释放过期支付预占失败: %v", err)
	}
	cleanupCtx, cancelCleanup := context.WithCancel(context.Background())
	defer cancelCleanup()
	go func() {
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				if err := db.ReleaseExpiredPaymentOrders(time.Now()); err != nil {
					log.Printf("释放过期支付预占失败: %v", err)
				}
			case <-cleanupCtx.Done():
				return
			}
		}
	}()

	go func() {
		log.Printf("netx-server 已启动，监听 %s", cfg.Addr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("服务异常退出: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		log.Printf("关闭服务失败: %v", err)
	}
}
