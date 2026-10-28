package store

import (
	"database/sql"
	"errors"
)

// FetchProxy 是订阅拉取失败时使用的单个 SOCKS5 代理。
type FetchProxy struct {
	Enabled  bool
	Host     string
	Port     int
	Username string
	Password string
}

// GetFetchProxy 读取兜底代理。没有配置时 ok 为 false。
func (d *DB) GetFetchProxy() (FetchProxy, bool, error) {
	var item FetchProxy
	var enabled int
	err := d.QueryRow(
		"SELECT enabled, host, port, username, password FROM fetch_proxy WHERE id = 1",
	).Scan(&enabled, &item.Host, &item.Port, &item.Username, &item.Password)
	if errors.Is(err, sql.ErrNoRows) {
		return FetchProxy{}, false, nil
	}
	if err != nil {
		return FetchProxy{}, false, err
	}
	item.Enabled = enabled != 0
	return item, true, nil
}

// SaveFetchProxy 保存唯一的兜底代理，再次保存会覆盖上一组参数。
func (d *DB) SaveFetchProxy(item FetchProxy) error {
	enabled := 0
	if item.Enabled {
		enabled = 1
	}
	_, err := d.Exec(
		`INSERT INTO fetch_proxy (id, enabled, host, port, username, password)
		 VALUES (1, ?, ?, ?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET
		    enabled = excluded.enabled,
		    host = excluded.host,
		    port = excluded.port,
		    username = excluded.username,
		    password = excluded.password`,
		enabled, item.Host, item.Port, item.Username, item.Password,
	)
	return err
}

// DeleteFetchProxy 删除兜底代理。
func (d *DB) DeleteFetchProxy() error {
	_, err := d.Exec("DELETE FROM fetch_proxy WHERE id = 1")
	return err
}
