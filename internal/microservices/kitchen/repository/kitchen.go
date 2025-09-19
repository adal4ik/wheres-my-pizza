package repository

import (
	"context"
	"database/sql"
	"fmt"
)

type KitchenRepositoryInterface interface {
	RegisterOrFail(ctx context.Context, name, wtype string) (bool, error)
	SetOffline(ctx context.Context, name string) error
	Heartbeat(ctx context.Context, name string) error

	// Переходим по order_number, т.к. он уникален и нужен для уведомлений
	TryStartCookingTx(ctx context.Context, orderNumber, workerName string) (bool, string, error)
	MarkReadyTx(ctx context.Context, orderNumber, workerName string) error

	// Если входящие события содержат только id — вытянуть order_number
	GetOrderNumber(ctx context.Context, id int64) (string, error)
}
type KitchenRepository struct {
	db *sql.DB
}

func NewKitchenRepository(db *sql.DB) KitchenRepositoryInterface {
	return &KitchenRepository{db: db}
}
func (r *KitchenRepository) Heartbeat(ctx context.Context, name string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE workers SET last_seen=now() WHERE name=$1`, name)
	return err
}
func (r *KitchenRepository) RegisterOrFail(ctx context.Context, name, wtype string) (bool, error) {
	// смотрим текущее состояние
	var status string
	err := r.db.QueryRowContext(ctx, `SELECT status FROM workers WHERE name=$1`, name).Scan(&status)
	switch {
	case err == sql.ErrNoRows:
		_, err = r.db.ExecContext(ctx, `
			INSERT INTO workers(name,type,status,last_seen) VALUES ($1,$2,'online',now())
		`, name, wtype)
		return false, err
	case err != nil:
		return false, err
	default:
		if status == "online" {
			return true, fmt.Errorf("worker %s already online", name)
		}
		_, err = r.db.ExecContext(ctx, `
			UPDATE workers SET type=$2, status='online', last_seen=now() WHERE name=$1
		`, name, wtype)
		return false, err
	}
}

func (r *KitchenRepository) SetOffline(ctx context.Context, name string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE workers SET status='offline', last_seen=now() WHERE name=$1`, name)
	return err
}

func (r *KitchenRepository) GetOrderNumber(ctx context.Context, id int64) (string, error) {
	var n string
	err := r.db.QueryRowContext(ctx, `SELECT order_number FROM orders WHERE id=$1`, id).Scan(&n)
	return n, err
}

func (r *KitchenRepository) TryStartCookingTx(ctx context.Context, orderNumber, workerName string) (bool, string, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return false, "", err
	}
	defer func() { _ = tx.Rollback() }()

	var oldStatus string
	if err := tx.QueryRowContext(ctx, `SELECT status FROM orders WHERE order_number=$1 FOR UPDATE`, orderNumber).Scan(&oldStatus); err != nil {
		return false, "", err
	}
	if oldStatus != "received" {
		return false, oldStatus, tx.Commit() // ничего не меняем
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE orders SET status='cooking', processed_by=$2, updated_at=now()
		WHERE order_number=$1
	`, orderNumber, workerName); err != nil {
		return false, "", err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO order_status_log(order_id,status,changed_by,changed_at,notes)
		SELECT id,'cooking',$2,now(),'' FROM orders WHERE order_number=$1
	`, orderNumber, workerName); err != nil {
		return false, "", err
	}
	return true, "received", tx.Commit()
}

func (r *KitchenRepository) MarkReadyTx(ctx context.Context, orderNumber, workerName string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	// готово: статус ready + completed_at + счётчик повара
	if _, err := tx.ExecContext(ctx, `
		UPDATE orders SET status='ready', completed_at=now(), updated_at=now()
		WHERE order_number=$1 AND status IN ('cooking','ready')
	`, orderNumber); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO order_status_log(order_id,status,changed_by,changed_at,notes)
		SELECT id,'ready',$2,now(),'' FROM orders WHERE order_number=$1
	`, orderNumber, workerName); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE workers SET orders_processed = orders_processed + 1, last_seen=now()
		WHERE name=$1
	`, workerName); err != nil {
		return err
	}
	return tx.Commit()
}
