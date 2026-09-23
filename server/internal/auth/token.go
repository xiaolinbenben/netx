package auth

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	kindAccess  = "access"
	kindRefresh = "refresh"

	// accessTTL 固定 2 小时，避免通过部署环境意外改变登录凭证有效期。
	accessTTL = 2 * time.Hour

	// refreshTTL 固定 7 天，用于无感刷新访问令牌
	refreshTTL = 7 * 24 * time.Hour
)

// ErrInvalidToken 表示凭证缺失、过期或类型不符。
var ErrInvalidToken = errors.New("无效的登录凭证")

// Manager 负责签发与校验登录凭证。
type Manager struct {
	secret    []byte
	accessTTL time.Duration
}

func NewManager(secret string) *Manager {
	return &Manager{secret: []byte(secret), accessTTL: accessTTL}
}

// NewManagerForTesting 创建自定义有效期的凭证管理器，仅用于测试过期逻辑。
func NewManagerForTesting(secret string, ttl time.Duration) *Manager {
	return &Manager{secret: []byte(secret), accessTTL: ttl}
}

type claims struct {
	Kind string `json:"kind"`
	jwt.RegisteredClaims
}

// Issue 同时签发访问令牌与刷新令牌。
func (m *Manager) Issue(username string) (string, string, time.Time, error) {
	access, expires, err := m.sign(username, kindAccess, m.accessTTL)
	if err != nil {
		return "", "", time.Time{}, err
	}
	refresh, _, err := m.sign(username, kindRefresh, refreshTTL)
	if err != nil {
		return "", "", time.Time{}, err
	}
	return access, refresh, expires, nil
}

// Refresh 用刷新令牌换新的访问令牌。
func (m *Manager) Refresh(refreshToken string) (string, time.Time, error) {
	parsed, err := m.parse(refreshToken, kindRefresh)
	if err != nil {
		return "", time.Time{}, err
	}
	return m.sign(parsed.Subject, kindAccess, m.accessTTL)
}

// Verify 校验访问令牌并返回用户名。
func (m *Manager) Verify(accessToken string) (string, error) {
	parsed, err := m.parse(accessToken, kindAccess)
	if err != nil {
		return "", err
	}
	return parsed.Subject, nil
}

func (m *Manager) sign(username, kind string, ttl time.Duration) (string, time.Time, error) {
	now := time.Now()
	expires := now.Add(ttl)
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims{
		Kind: kind,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   username,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expires),
		},
	})
	signed, err := token.SignedString(m.secret)
	if err != nil {
		return "", time.Time{}, err
	}
	return signed, expires, nil
}

func (m *Manager) parse(token, kind string) (*claims, error) {
	parsed, err := jwt.ParseWithClaims(
		token,
		&claims{},
		func(*jwt.Token) (any, error) { return m.secret, nil },
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
	)
	if err != nil || !parsed.Valid {
		return nil, ErrInvalidToken
	}
	result, ok := parsed.Claims.(*claims)
	if !ok || result.Kind != kind {
		return nil, ErrInvalidToken
	}
	return result, nil
}
