package httpapi

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"

	"netx/server/internal/proxy"
	"netx/server/internal/store"
)

var errFetchProxyRead = errors.New("read fetch proxy")

type fetchProxyRequest struct {
	Enabled        bool   `json:"enabled"`
	Host           string `json:"host"`
	Port           int    `json:"port"`
	Username       string `json:"username"`
	Password       string `json:"password"`
	UpdatePassword bool   `json:"updatePassword"`
}

type fetchProxyView struct {
	Configured         bool   `json:"configured"`
	Enabled            bool   `json:"enabled"`
	Host               string `json:"host"`
	Port               int    `json:"port"`
	Username           string `json:"username"`
	PasswordConfigured bool   `json:"passwordConfigured"`
	PasswordHint       string `json:"passwordHint"`
}

func (s *Server) handleGetFetchProxy(w http.ResponseWriter, r *http.Request) {
	item, found, err := s.store.GetFetchProxy()
	if err != nil {
		fail(w, http.StatusInternalServerError, "读取订阅代理失败")
		return
	}
	if !found {
		ok(w, fetchProxyView{})
		return
	}
	ok(w, fetchProxyView{
		Configured:         true,
		Enabled:            item.Enabled,
		Host:               item.Host,
		Port:               item.Port,
		Username:           item.Username,
		PasswordConfigured: item.Password != "",
		PasswordHint:       maskHint(item.Password),
	})
}

func (s *Server) handleSaveFetchProxy(w http.ResponseWriter, r *http.Request) {
	var req fetchProxyRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	endpoint, err := s.endpointFromRequest(req, !req.UpdatePassword)
	if err != nil {
		s.failFetchProxyRequest(w, err)
		return
	}
	if err := s.store.SaveFetchProxy(store.FetchProxy{
		Enabled:  req.Enabled,
		Host:     endpoint.Host,
		Port:     endpoint.Port,
		Username: endpoint.Username,
		Password: endpoint.Password,
	}); err != nil {
		fail(w, http.StatusInternalServerError, "保存订阅代理失败")
		return
	}
	s.markDirectHealthy()
	ok(w, nil)
}

func (s *Server) handleDeleteFetchProxy(w http.ResponseWriter, r *http.Request) {
	if err := s.store.DeleteFetchProxy(); err != nil {
		fail(w, http.StatusInternalServerError, "删除订阅代理失败")
		return
	}
	s.markDirectHealthy()
	ok(w, nil)
}

func (s *Server) handleParseFetchProxy(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Link string `json:"link"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	endpoint, err := proxy.ParseURL(req.Link)
	if err != nil {
		fail(w, http.StatusBadRequest, err.Error())
		return
	}
	ok(w, map[string]any{
		"host":     endpoint.Host,
		"port":     endpoint.Port,
		"username": endpoint.Username,
		"password": endpoint.Password,
	})
}

func (s *Server) handleTestFetchProxy(w http.ResponseWriter, r *http.Request) {
	var req fetchProxyRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	endpoint, err := s.endpointFromRequest(req, true)
	if err != nil {
		s.failFetchProxyRequest(w, err)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), proxyRequestTimeout)
	defer cancel()
	elapsed, status, err := proxy.Probe(ctx, endpoint, proxy.ProbeURL)
	if err != nil {
		logProxyFailure(err, endpoint.Password)
		fail(w, http.StatusBadGateway, "连接 SOCKS5 失败")
		return
	}
	if status != http.StatusNoContent {
		fail(w, http.StatusBadGateway, fmt.Sprintf("测试地址返回 HTTP %d", status))
		return
	}
	ok(w, map[string]any{"elapsedMs": elapsed.Milliseconds()})
}

func (s *Server) endpointFromRequest(req fetchProxyRequest, keepStoredPassword bool) (proxy.Endpoint, error) {
	password := req.Password
	if keepStoredPassword && strings.TrimSpace(req.Username) != "" && password == "" {
		existing, found, err := s.store.GetFetchProxy()
		if err != nil {
			return proxy.Endpoint{}, fmt.Errorf("%w: %v", errFetchProxyRead, err)
		}
		if found {
			password = existing.Password
		}
	}
	return proxy.Normalize(proxy.Endpoint{
		Host:     req.Host,
		Port:     req.Port,
		Username: req.Username,
		Password: password,
	})
}

func (s *Server) failFetchProxyRequest(w http.ResponseWriter, err error) {
	if errors.Is(err, errFetchProxyRead) {
		fail(w, http.StatusInternalServerError, "读取订阅代理失败")
		return
	}
	fail(w, http.StatusBadRequest, err.Error())
}

func (s *Server) enabledFetchProxy() (proxy.Endpoint, bool, error) {
	item, found, err := s.store.GetFetchProxy()
	if err != nil || !found || !item.Enabled {
		return proxy.Endpoint{}, false, err
	}
	return proxy.Endpoint{
		Host:     item.Host,
		Port:     item.Port,
		Username: item.Username,
		Password: item.Password,
	}, true, nil
}

func logProxyFailure(err error, password string) {
	message := err.Error()
	if password != "" {
		message = strings.ReplaceAll(message, password, "****")
	}
	log.Printf("SOCKS5 测试失败: %s", message)
}
