package config

import (
	"fmt"
	"os"
	"strings"
)

// Config 是服务运行所需的全部配置，统一来自环境变量。
type Config struct {
	Addr          string
	DBPath        string
	AdminUsername string
	AdminPassword string
	JWTSecret     string
}

// Load 读取环境变量并校验必填项。
func Load() (Config, error) {
	cfg := Config{
		Addr:          ":" + strings.TrimPrefix(env("PORT", "8000"), ":"),
		DBPath:        env("DB_PATH", "./data/netx.db"),
		AdminUsername: os.Getenv("ADMIN_USERNAME"),
		AdminPassword: os.Getenv("ADMIN_PASSWORD"),
		JWTSecret:     os.Getenv("JWT_SECRET"),
	}

	missing := make([]string, 0, 3)
	if cfg.AdminUsername == "" {
		missing = append(missing, "ADMIN_USERNAME")
	}
	if cfg.AdminPassword == "" {
		missing = append(missing, "ADMIN_PASSWORD")
	}
	if cfg.JWTSecret == "" {
		missing = append(missing, "JWT_SECRET")
	}
	if len(missing) > 0 {
		return Config{}, fmt.Errorf("缺少必需的环境变量: %s", strings.Join(missing, ", "))
	}

	return cfg, nil
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
