package store

import "time"

// Settings 读取全部系统配置。
func (d *DB) Settings() (map[string]string, error) {
	rows, err := d.Query("SELECT key, value FROM settings")
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	values := make(map[string]string)
	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			return nil, err
		}
		values[key] = value
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return values, nil
}

// SaveSettings 逐项写入系统配置。
func (d *DB) SaveSettings(values map[string]string) error {
	tx, err := d.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	now := time.Now().UTC().Truncate(time.Second).Unix()
	for key, value := range values {
		_, err := tx.Exec(
			`INSERT INTO settings (key, value, updated_at) VALUES (?, ?, ?)
			 ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at`,
			key, value, now,
		)
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}
