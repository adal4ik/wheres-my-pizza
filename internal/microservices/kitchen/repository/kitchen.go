package repository

import (
	"context"
	"database/sql"
)

type KitchenRepositoryInterface interface {
	UpsertWorker(ctx context.Context, name, wtype string) error
	Heartbeat(ctx context.Context, name string) error
	TryStartCookingTx(ctx context.Context, orderID int64, workerName string) (bool, error)
	MarkReadyTx(ctx context.Context, orderID int64, workerName string) error
	MarkCompletedTx(ctx context.Context, orderID int64, workerName string) error
}

type KitchenRepository struct {
	db *sql.DB
}

func NewKitchenRepository(db *sql.DB) KitchenRepositoryInterface {
	return &KitchenRepository{db: db}
}

func (r *KitchenRepository) UpsertWorker(ctx context.Context, name, wtype string) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO workers(name, type, status, last_seen)
		VALUES ($1,$2,'online',now())
		ON CONFLICT(name) DO UPDATE SET status='online', last_seen=now(), type=EXCLUDED.type
	`, name, wtype)
	return err
}

func (r *KitchenRepository) Heartbeat(ctx context.Context, name string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE workers SET last_seen=now() WHERE name=$1`, name)
	return err
}

func (r *KitchenRepository) TryStartCookingTx(ctx context.Context, orderID int64, workerName string) (bool, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback() }()

	res, err := tx.ExecContext(ctx, `
		UPDATE orders
		SET status='cooking', processed_by=$2, updated_at=now()
		WHERE id=$1 AND status='received'
	`, orderID, workerName)
	if err != nil {
		return false, err
	}

	aff, _ := res.RowsAffected()
	if aff == 0 {
		return false, tx.Commit() // никто не обновлён — просто коммитим пустую транзу
	}

	if _, err = tx.ExecContext(ctx, `
		INSERT INTO order_status_log(order_id,status,changed_by,changed_at)
		VALUES ($1,'cooking',$2,now())
	`, orderID, workerName); err != nil {
		return false, err
	}

	return true, tx.Commit()
}

func (r *KitchenRepository) MarkReadyTx(ctx context.Context, orderID int64, workerName string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err = tx.ExecContext(ctx, `
		UPDATE orders SET status='ready', updated_at=now()
		WHERE id=$1 AND status='cooking'
	`, orderID); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `
		INSERT INTO order_status_log(order_id,status,changed_by,changed_at)
		VALUES ($1,'ready',$2,now())
	`, orderID, workerName); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *KitchenRepository) MarkCompletedTx(ctx context.Context, orderID int64, workerName string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err = tx.ExecContext(ctx, `
		UPDATE orders SET status='completed', completed_at=now(), updated_at=now()
		WHERE id=$1 AND status IN ('ready','cooking')
	`, orderID); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `
		INSERT INTO order_status_log(order_id,status,changed_by,changed_at)
		VALUES ($1,'completed',$2,now())
	`, orderID, workerName); err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `UPDATE workers SET orders_processed = orders_processed + 1 WHERE name=$1`, workerName)
	if err != nil {
		return err
	}

	return tx.Commit()
}
