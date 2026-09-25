package store

import (
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// 兑换码状态
const (
	StatusUnused   = "unused"
	StatusReserved = "reserved"
	StatusUsed     = "used"
	StatusVoid     = "void"
)

var (
	ErrCodeNotFound = errors.New("兑换码不存在")
	ErrCodeUsed     = errors.New("已使用的兑换码不能修改")
	ErrCodeReserved = errors.New("待支付兑换码不能修改")
)

// 兑换码只使用小写数字和字母，便于直接放入 URL 和文件名。
const (
	codeAlphabet = "0123456789abcdefghijklmnopqrstuvwxyz"
	codeLength   = 16
)

type Code struct {
	ID              int64
	Code            string
	Note            string
	Status          string
	CreatedAt       time.Time
	UsedAt          *time.Time
	Plan            string
	SubscriptionURL string
}

// FindCode 查询兑换码，不改变兑换码状态。
func (d *DB) FindCode(code string) (Code, error) {
	var item Code
	var createdAt, usedAt sql.NullInt64
	err := d.QueryRow("SELECT id, code, note, status, created_at, used_at, plan, subscription_url FROM codes WHERE code = ?", code).Scan(&item.ID, &item.Code, &item.Note, &item.Status, &createdAt, &usedAt, &item.Plan, &item.SubscriptionURL)
	if errors.Is(err, sql.ErrNoRows) {
		return Code{}, ErrCodeNotFound
	}
	if err != nil {
		return Code{}, err
	}
	item.CreatedAt = time.Unix(createdAt.Int64, 0).UTC()
	if usedAt.Valid {
		used := time.Unix(usedAt.Int64, 0).UTC()
		item.UsedAt = &used
	}
	return item, nil
}

// RedeemCode 原子地消费一个兑换码，并返回其订阅配置。
func (d *DB) RedeemCode(code string) (Code, error) {
	var item Code
	var createdAt, usedAt sql.NullInt64
	err := d.QueryRow("SELECT id, code, note, status, created_at, used_at, plan, subscription_url FROM codes WHERE code = ?", code).Scan(&item.ID, &item.Code, &item.Note, &item.Status, &createdAt, &usedAt, &item.Plan, &item.SubscriptionURL)
	if errors.Is(err, sql.ErrNoRows) {
		return Code{}, ErrCodeNotFound
	}
	if err != nil {
		return Code{}, err
	}
	if item.Status != StatusUnused {
		return Code{}, ErrCodeUsed
	}
	now := time.Now().UTC().Truncate(time.Second)
	if _, err := d.Exec("UPDATE codes SET status = ?, used_at = ? WHERE id = ? AND status = ?", StatusUsed, now.Unix(), item.ID, StatusUnused); err != nil {
		return Code{}, err
	}
	item.Status, item.UsedAt, item.CreatedAt = StatusUsed, &now, time.Unix(createdAt.Int64, 0).UTC()
	return item, nil
}

type CodeFilter struct {
	Page    int
	Size    int
	Status  string
	Keyword string
}

// NewCode 生成一个 16 位小写字母数字随机兑换码。
func NewCode() (string, error) {
	const maxByte = 256 - (256 % len(codeAlphabet))
	code := make([]byte, codeLength)
	buffer := make([]byte, codeLength)
	for written := 0; written < codeLength; {
		if _, err := rand.Read(buffer); err != nil {
			return "", fmt.Errorf("生成随机兑换码失败: %w", err)
		}
		for _, b := range buffer {
			if int(b) >= maxByte {
				continue
			}
			code[written] = codeAlphabet[int(b)%len(codeAlphabet)]
			written++
			if written == codeLength {
				break
			}
		}
	}
	return string(code), nil
}

func (d *DB) CreateCodeWithDetails(plan, subscriptionURL, note string) (Code, error) {
	tx, err := d.Begin()
	if err != nil {
		return Code{}, err
	}
	defer func() { _ = tx.Rollback() }()

	created, err := insertCode(tx, plan, subscriptionURL, note)
	if err != nil {
		return Code{}, err
	}
	if err := tx.Commit(); err != nil {
		return Code{}, err
	}
	return created, nil
}

func insertCode(tx *sql.Tx, plan, subscriptionURL, note string) (Code, error) {
	now := time.Now().UTC().Truncate(time.Second)
	for attempt := 0; attempt < 5; attempt++ {
		code, err := NewCode()
		if err != nil {
			return Code{}, err
		}
		result, err := tx.Exec(
			"INSERT INTO codes (code, note, status, created_at, plan, subscription_url) VALUES (?, ?, ?, ?, ?, ?)",
			code, note, StatusUnused, now.Unix(), plan, subscriptionURL,
		)
		if err != nil {
			if strings.Contains(err.Error(), "UNIQUE constraint failed") {
				continue
			}
			return Code{}, err
		}
		id, err := result.LastInsertId()
		if err != nil {
			return Code{}, err
		}
		return Code{
			ID:              id,
			Code:            code,
			Note:            note,
			Status:          StatusUnused,
			CreatedAt:       now,
			Plan:            plan,
			SubscriptionURL: subscriptionURL,
		}, nil
	}
	return Code{}, errors.New("生成兑换码失败，请重试")
}

// ListCodes 分页查询兑换码，同时返回总数。
func (d *DB) ListCodes(filter CodeFilter) ([]Code, int, error) {
	conditions := make([]string, 0, 2)
	args := make([]any, 0, 4)
	if filter.Status != "" {
		conditions = append(conditions, "status = ?")
		args = append(args, filter.Status)
	}
	if filter.Keyword != "" {
		conditions = append(conditions, "(code LIKE ? OR note LIKE ?)")
		like := "%" + filter.Keyword + "%"
		args = append(args, like, like)
	}
	where := ""
	if len(conditions) > 0 {
		where = " WHERE " + strings.Join(conditions, " AND ")
	}

	var total int
	if err := d.QueryRow("SELECT COUNT(*) FROM codes"+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	query := "SELECT id, code, note, status, created_at, used_at, plan, subscription_url FROM codes" + where +
		" ORDER BY id DESC LIMIT ? OFFSET ?"
	rows, err := d.Query(query, append(args, filter.Size, (filter.Page-1)*filter.Size)...)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = rows.Close() }()

	items := make([]Code, 0, filter.Size)
	for rows.Next() {
		var (
			item      Code
			createdAt int64
			usedAt    sql.NullInt64
		)
		if err := rows.Scan(&item.ID, &item.Code, &item.Note, &item.Status, &createdAt, &usedAt, &item.Plan, &item.SubscriptionURL); err != nil {
			return nil, 0, err
		}
		item.CreatedAt = time.Unix(createdAt, 0).UTC()
		if usedAt.Valid {
			used := time.Unix(usedAt.Int64, 0).UTC()
			item.UsedAt = &used
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// UpdateCodeStatus 作废或恢复兑换码，已使用的兑换码不允许再变更。
func (d *DB) UpdateCodeStatus(id int64, status string) error {
	var current string
	err := d.QueryRow("SELECT status FROM codes WHERE id = ?", id).Scan(&current)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrCodeNotFound
	}
	if err != nil {
		return err
	}
	if current == StatusUsed {
		return ErrCodeUsed
	}
	if current == StatusReserved {
		return ErrCodeReserved
	}
	if current == status {
		return nil
	}
	_, err = d.Exec("UPDATE codes SET status = ? WHERE id = ?", status, id)
	return err
}
