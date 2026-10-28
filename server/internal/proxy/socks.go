package proxy

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode"

	xproxy "golang.org/x/net/proxy"
)

const ProbeURL = "https://www.gstatic.com/generate_204"

// Endpoint 是一组 SOCKS5 连接参数。
type Endpoint struct {
	Host     string
	Port     int
	Username string
	Password string
}

// ParseURL 解析 socks5://用户名:密码@地址:端口。用户名和密码可以不写。
func ParseURL(raw string) (Endpoint, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return Endpoint{}, errors.New("请输入 SOCKS5 链接")
	}
	parsed, err := url.Parse(raw)
	if err != nil || !strings.EqualFold(parsed.Scheme, "socks5") || parsed.Host == "" {
		return Endpoint{}, errors.New("只支持 socks5:// 链接")
	}
	if (parsed.Path != "" && parsed.Path != "/") || parsed.RawQuery != "" || parsed.Fragment != "" {
		return Endpoint{}, errors.New("SOCKS5 链接不能带路径或参数")
	}
	host, portText, err := net.SplitHostPort(parsed.Host)
	if err != nil || host == "" || portText == "" {
		return Endpoint{}, errors.New("SOCKS5 链接缺少地址或端口")
	}
	port, err := strconv.Atoi(portText)
	if err != nil {
		return Endpoint{}, errors.New("SOCKS5 链接缺少地址或端口")
	}
	username := ""
	password := ""
	if parsed.User != nil {
		username = parsed.User.Username()
		password, _ = parsed.User.Password()
	}
	return Normalize(Endpoint{Host: host, Port: port, Username: username, Password: password})
}

// Normalize 校验并整理代理参数。用户名为空时不保存密码。
func Normalize(in Endpoint) (Endpoint, error) {
	host, err := normalizeHost(in.Host)
	if err != nil {
		return Endpoint{}, err
	}
	if in.Port < 1 || in.Port > 65535 {
		return Endpoint{}, errors.New("端口必须在 1 到 65535 之间")
	}
	username := strings.TrimSpace(in.Username)
	password := in.Password
	if username == "" && password != "" {
		return Endpoint{}, errors.New("填写密码时必须填写用户名")
	}
	if len(username) > 255 || len(password) > 255 {
		return Endpoint{}, errors.New("用户名或密码过长")
	}
	return Endpoint{Host: host, Port: in.Port, Username: username, Password: password}, nil
}

func normalizeHost(host string) (string, error) {
	host = strings.TrimSpace(host)
	if strings.HasPrefix(host, "[") && strings.HasSuffix(host, "]") && len(host) >= 2 {
		host = host[1 : len(host)-1]
	}
	if host == "" {
		return "", errors.New("请填写代理地址")
	}
	if strings.Contains(host, "://") || strings.ContainsAny(host, " /\t?#") {
		return "", errors.New("代理地址不正确")
	}
	if ip := net.ParseIP(host); ip != nil {
		return ip.String(), nil
	}
	if len(host) > 253 {
		return "", errors.New("代理地址不正确")
	}
	for _, label := range strings.Split(host, ".") {
		if label == "" || len(label) > 63 || strings.HasPrefix(label, "-") || strings.HasSuffix(label, "-") {
			return "", errors.New("代理地址不正确")
		}
		for _, r := range label {
			if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' {
				continue
			}
			return "", errors.New("代理地址不正确")
		}
	}
	return host, nil
}

// HTTPClient 返回经 SOCKS5 出站的 HTTP 客户端。域名会交给代理解析。
func HTTPClient(cfg Endpoint, timeout time.Duration) (*http.Client, error) {
	dial, err := dialContext(cfg)
	if err != nil {
		return nil, err
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	transport.DialContext = dial
	return &http.Client{Timeout: timeout, Transport: transport}, nil
}

// Probe 通过代理访问目标地址，用来确认这组 SOCKS5 参数可用。
func Probe(ctx context.Context, cfg Endpoint, target string) (time.Duration, int, error) {
	client, err := HTTPClient(cfg, 20*time.Second)
	if err != nil {
		return 0, 0, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return 0, 0, err
	}
	started := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		return 0, 0, err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<20))
	return time.Since(started), resp.StatusCode, nil
}

func dialContext(cfg Endpoint) (func(context.Context, string, string) (net.Conn, error), error) {
	normalized, err := Normalize(cfg)
	if err != nil {
		return nil, err
	}
	var auth *xproxy.Auth
	if normalized.Username != "" {
		auth = &xproxy.Auth{User: normalized.Username, Password: normalized.Password}
	}
	raw, err := xproxy.SOCKS5("tcp", net.JoinHostPort(normalized.Host, strconv.Itoa(normalized.Port)), auth, nil)
	if err != nil {
		return nil, err
	}
	dialer, ok := raw.(xproxy.ContextDialer)
	if !ok {
		return nil, errors.New("SOCKS5 拨号器不可用")
	}
	return dialer.DialContext, nil
}
