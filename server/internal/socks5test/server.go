// Package socks5test 提供测试用的小型 SOCKS5 服务。
package socks5test

import (
	"context"
	"io"
	"net"
	"strconv"
	"sync"
	"time"
)

// Server 是只接受 CONNECT 的 SOCKS5 监听器。
type Server struct {
	Addr string

	ln       net.Listener
	user     string
	password string
	dial     func(context.Context, string) (net.Conn, error)

	mu       sync.Mutex
	requests []string
}

// Listen 在 127.0.0.1 上启动 SOCKS5。user 为空表示不认证。
// dial 为空时按客户端请求的地址建立 TCP 连接。
func Listen(user, password string, dial func(context.Context, string) (net.Conn, error)) (*Server, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	server := &Server{Addr: ln.Addr().String(), ln: ln, user: user, password: password, dial: dial}
	if server.dial == nil {
		server.dial = func(ctx context.Context, addr string) (net.Conn, error) {
			var dialer net.Dialer
			return dialer.DialContext(ctx, "tcp", addr)
		}
	}
	go server.serve()
	return server, nil
}

// Close 停止监听。
func (s *Server) Close() error {
	return s.ln.Close()
}

// Host 返回监听地址。
func (s *Server) Host() string {
	host, _, _ := net.SplitHostPort(s.Addr)
	return host
}

// Port 返回监听端口。
func (s *Server) Port() int {
	_, port, _ := net.SplitHostPort(s.Addr)
	number, _ := strconv.Atoi(port)
	return number
}

// Requests 返回客户端要求连接的地址，顺序与请求一致。
func (s *Server) Requests() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]string, len(s.requests))
	copy(out, s.requests)
	return out
}

func (s *Server) serve() {
	for {
		conn, err := s.ln.Accept()
		if err != nil {
			return
		}
		go s.handle(conn)
	}
}

func (s *Server) handle(conn net.Conn) {
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(10 * time.Second))
	if !s.handshake(conn) {
		return
	}
	addr, ok := readConnect(conn)
	if !ok {
		_, _ = conn.Write([]byte{5, 7, 0, 1, 0, 0, 0, 0, 0, 0})
		return
	}
	s.mu.Lock()
	s.requests = append(s.requests, addr)
	s.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	target, err := s.dial(ctx, addr)
	cancel()
	if err != nil {
		_, _ = conn.Write([]byte{5, 5, 0, 1, 0, 0, 0, 0, 0, 0})
		return
	}
	defer target.Close()
	if _, err := conn.Write([]byte{5, 0, 0, 1, 0, 0, 0, 0, 0, 0}); err != nil {
		return
	}
	_ = conn.SetDeadline(time.Time{})
	relay(conn, target)
}

func (s *Server) handshake(conn net.Conn) bool {
	header := make([]byte, 2)
	if _, err := io.ReadFull(conn, header); err != nil || header[0] != 5 {
		return false
	}
	methods := make([]byte, int(header[1]))
	if len(methods) > 0 {
		if _, err := io.ReadFull(conn, methods); err != nil {
			return false
		}
	}
	if s.user != "" {
		if !containsByte(methods, 2) {
			_, _ = conn.Write([]byte{5, 0xff})
			return false
		}
		if _, err := conn.Write([]byte{5, 2}); err != nil || !s.authenticate(conn) {
			return false
		}
		return true
	}
	if !containsByte(methods, 0) {
		_, _ = conn.Write([]byte{5, 0xff})
		return false
	}
	_, err := conn.Write([]byte{5, 0})
	return err == nil
}

func (s *Server) authenticate(conn net.Conn) bool {
	header := make([]byte, 2)
	if _, err := io.ReadFull(conn, header); err != nil || header[0] != 1 {
		return false
	}
	username := make([]byte, int(header[1]))
	if len(username) > 0 {
		if _, err := io.ReadFull(conn, username); err != nil {
			return false
		}
	}
	var plen [1]byte
	if _, err := io.ReadFull(conn, plen[:]); err != nil {
		return false
	}
	password := make([]byte, int(plen[0]))
	if len(password) > 0 {
		if _, err := io.ReadFull(conn, password); err != nil {
			return false
		}
	}
	if string(username) != s.user || string(password) != s.password {
		_, _ = conn.Write([]byte{1, 1})
		return false
	}
	_, err := conn.Write([]byte{1, 0})
	return err == nil
}

func readConnect(conn net.Conn) (string, bool) {
	header := make([]byte, 4)
	if _, err := io.ReadFull(conn, header); err != nil || header[0] != 5 || header[1] != 1 {
		return "", false
	}
	var host string
	switch header[3] {
	case 1:
		buf := make([]byte, 4)
		if _, err := io.ReadFull(conn, buf); err != nil {
			return "", false
		}
		host = net.IP(buf).String()
	case 3:
		var n [1]byte
		if _, err := io.ReadFull(conn, n[:]); err != nil {
			return "", false
		}
		buf := make([]byte, int(n[0]))
		if _, err := io.ReadFull(conn, buf); err != nil {
			return "", false
		}
		host = string(buf)
	case 4:
		buf := make([]byte, 16)
		if _, err := io.ReadFull(conn, buf); err != nil {
			return "", false
		}
		host = net.IP(buf).String()
	default:
		return "", false
	}
	var portBuf [2]byte
	if _, err := io.ReadFull(conn, portBuf[:]); err != nil {
		return "", false
	}
	port := int(portBuf[0])<<8 | int(portBuf[1])
	return net.JoinHostPort(host, strconv.Itoa(port)), true
}

func relay(left, right net.Conn) {
	done := make(chan struct{}, 2)
	copyOne := func(dst, src net.Conn) {
		_, _ = io.Copy(dst, src)
		_ = dst.Close()
		done <- struct{}{}
	}
	go copyOne(right, left)
	go copyOne(left, right)
	<-done
	<-done
}

func containsByte(items []byte, target byte) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}
