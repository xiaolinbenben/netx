package store

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
)

const (
	PaymentPending        = "pending"
	PaymentPaid           = "paid"
	PaymentExpired        = "expired"
	PaymentReservationTTL = 30 * time.Minute
)

var (
	ErrPaymentNotFound   = errors.New("支付订单不存在")
	ErrPaymentAmount     = errors.New("支付金额不匹配")
	ErrPaymentState      = errors.New("支付订单状态不允许更新")
	ErrPaymentOutOfStock = errors.New("该套餐暂无可用库存")
)

type PaymentOrder struct {
	OutTradeNo string
	Plan       string
	AmountFen  int
	CodeID     int64
	Code       string
	Status     string
}

// CreatePaymentOrder reserves an existing code and creates its payment order atomically.
func (d *DB) CreatePaymentOrder(plan string, amountFen int) (PaymentOrder, error) {
	tx, err := d.Begin()
	if err != nil {
		return PaymentOrder{}, err
	}
	defer func() { _ = tx.Rollback() }()

	now := time.Now().UTC().Truncate(time.Second)
	if err := releaseExpiredPaymentOrdersTx(tx, now); err != nil {
		return PaymentOrder{}, err
	}
	var codeID int64
	var code string
	err = tx.QueryRow(
		"SELECT id, code FROM codes WHERE status = ? AND plan = ? ORDER BY id LIMIT 1",
		StatusUnused, plan,
	).Scan(&codeID, &code)
	if errors.Is(err, sql.ErrNoRows) {
		return PaymentOrder{}, ErrPaymentOutOfStock
	}
	if err != nil {
		return PaymentOrder{}, err
	}
	result, err := tx.Exec("UPDATE codes SET status = ? WHERE id = ? AND status = ?", StatusReserved, codeID, StatusUnused)
	if err != nil {
		return PaymentOrder{}, err
	}
	if affected, err := result.RowsAffected(); err != nil || affected != 1 {
		if err != nil {
			return PaymentOrder{}, err
		}
		return PaymentOrder{}, ErrPaymentOutOfStock
	}

	for attempt := 0; attempt < 5; attempt++ {
		outTradeNo, err := newOutTradeNo()
		if err != nil {
			return PaymentOrder{}, err
		}
		_, err = tx.Exec(
			`INSERT INTO payment_orders (out_trade_no, plan, amount_fen, code_id, status, created_at)
			 VALUES (?, ?, ?, ?, ?, ?)`,
			outTradeNo, plan, amountFen, codeID, PaymentPending, now.Unix(),
		)
		if err != nil {
			if isUniqueConstraint(err) {
				continue
			}
			return PaymentOrder{}, err
		}
		if err := tx.Commit(); err != nil {
			return PaymentOrder{}, err
		}
		return PaymentOrder{
			OutTradeNo: outTradeNo, Plan: plan, AmountFen: amountFen,
			CodeID: codeID, Code: code, Status: PaymentPending,
		}, nil
	}
	return PaymentOrder{}, errors.New("创建支付订单失败，请重试")
}

// MarkPaymentPaid is idempotent and only accepts the exact order amount.
func (d *DB) MarkPaymentPaid(outTradeNo, tradeNo string, amountFen int) (PaymentOrder, error) {
	tx, err := d.Begin()
	if err != nil {
		return PaymentOrder{}, err
	}
	defer func() { _ = tx.Rollback() }()
	if err := releaseExpiredPaymentOrdersTx(tx, time.Now().UTC().Truncate(time.Second)); err != nil {
		return PaymentOrder{}, err
	}

	var order PaymentOrder
	err = tx.QueryRow(
		`SELECT p.out_trade_no, p.plan, p.amount_fen, p.code_id,
		        c.code, p.status
		 FROM payment_orders p JOIN codes c ON c.id = p.code_id
		 WHERE p.out_trade_no = ?`, outTradeNo,
	).Scan(&order.OutTradeNo, &order.Plan, &order.AmountFen, &order.CodeID, &order.Code, &order.Status)
	if errors.Is(err, sql.ErrNoRows) {
		return PaymentOrder{}, ErrPaymentNotFound
	}
	if err != nil {
		return PaymentOrder{}, err
	}
	if order.AmountFen != amountFen {
		return PaymentOrder{}, ErrPaymentAmount
	}
	if order.Status == PaymentPaid {
		if err := tx.Commit(); err != nil {
			return PaymentOrder{}, err
		}
		return order, nil
	}
	if order.Status != PaymentPending {
		return PaymentOrder{}, ErrPaymentState
	}

	now := time.Now().UTC().Truncate(time.Second)
	result, err := tx.Exec(
		"UPDATE codes SET status = ?, used_at = ? WHERE id = ? AND status = ?",
		StatusUsed, now.Unix(), order.CodeID, StatusReserved,
	)
	if err != nil {
		return PaymentOrder{}, err
	}
	if affected, err := result.RowsAffected(); err != nil || affected != 1 {
		if err != nil {
			return PaymentOrder{}, err
		}
		return PaymentOrder{}, ErrPaymentState
	}
	if _, err := tx.Exec(
		`UPDATE payment_orders SET status = ?, trade_no = ?, paid_at = ?
		 WHERE out_trade_no = ? AND status = ?`,
		PaymentPaid, tradeNo, now.Unix(), outTradeNo, PaymentPending,
	); err != nil {
		return PaymentOrder{}, err
	}
	order.Status = PaymentPaid
	if err := tx.Commit(); err != nil {
		return PaymentOrder{}, err
	}
	return order, nil
}

// ReleaseExpiredPaymentOrders releases reservations left by unpaid orders.
func (d *DB) ReleaseExpiredPaymentOrders(now time.Time) error {
	tx, err := d.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if err := releaseExpiredPaymentOrdersTx(tx, now.UTC().Truncate(time.Second)); err != nil {
		return err
	}
	return tx.Commit()
}

func releaseExpiredPaymentOrdersTx(tx *sql.Tx, now time.Time) error {
	cutoff := now.Add(-PaymentReservationTTL).Unix()
	if _, err := tx.Exec(
		`UPDATE codes SET status = ? WHERE status = ? AND id IN (
			SELECT code_id FROM payment_orders WHERE status = ? AND created_at < ?
		)`,
		StatusUnused, StatusReserved, PaymentPending, cutoff,
	); err != nil {
		return err
	}
	_, err := tx.Exec(
		"UPDATE payment_orders SET status = ? WHERE status = ? AND created_at < ?",
		PaymentExpired, PaymentPending, cutoff,
	)
	return err
}

func newOutTradeNo() (string, error) {
	raw := make([]byte, 8)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("生成订单号失败: %w", err)
	}
	return "NX" + time.Now().UTC().Format("20060102150405") + hex.EncodeToString(raw), nil
}

func isUniqueConstraint(err error) bool {
	return err != nil && (strings.Contains(err.Error(), "UNIQUE constraint failed") || strings.Contains(err.Error(), "constraint failed"))
}
