package httpapi

import (
	"encoding/json"
	"log"
	"net/http"
)

// envelope 是统一的响应结构：HTTP 状态码表达错误，body 里带 success 与 message。
type envelope struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
	Data    any    `json:"data,omitempty"`
}

func writeJSON(w http.ResponseWriter, status int, body envelope) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		log.Printf("写入响应失败: %v", err)
	}
}

func ok(w http.ResponseWriter, data any) {
	writeJSON(w, http.StatusOK, envelope{Success: true, Data: data})
}

func fail(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, envelope{Success: false, Message: message})
}

func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	if err := decoder.Decode(dst); err != nil {
		fail(w, http.StatusBadRequest, "请求参数格式错误")
		return false
	}
	return true
}
