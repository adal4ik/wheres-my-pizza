package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"wheres-my-pizza/internal/microservices/tracker/models"
)

type TrackerRepoInterface interface {
	UpsertOrderView(ctx context.Context, v models.OrderView) error
	AppendEvent(ctx context.Context, e models.OrderEvent) error
	GetOrderView(ctx context.Context, id string) (models.OrderView, bool, error)
	GetOrderTimeline(ctx context.Context, id string, limit, offset int) ([]models.OrderEvent, error)
	ListOrders(ctx context.Context, filter map[string]string, limit int, cursor string) ([]models.OrderView, string, error)
}

type TrackerRepo struct {
	db *sql.DB
}

func NewTrackerRepo(db *sql.DB) *TrackerRepo { return &TrackerRepo{db: db} }

func (r *TrackerRepo) UpsertOrderView(ctx context.Context, v models.OrderView) error {
	_, err := r.db.ExecContext(ctx, `
INSERT INTO orders_view (order_id, status, updated_at, customer_name, last_event_at)
VALUES ($1,$2,$3,$4,$5)
ON CONFLICT (order_id) DO UPDATE SET
  status = EXCLUDED.status,
  updated_at = EXCLUDED.updated_at,
  customer_name = COALESCE(EXCLUDED.customer_name, orders_view.customer_name),
  last_event_at = EXCLUDED.last_event_at
`, v.OrderID, string(v.Status), v.UpdatedAt, nullIfEmpty(v.CustomerName), v.LastEventAt)
	return err
}

func (r *TrackerRepo) AppendEvent(ctx context.Context, e models.OrderEvent) error {
	b, _ := json.Marshal(e.Payload)
	_, err := r.db.ExecContext(ctx, `
INSERT INTO order_events (order_id, event_type, payload, occurred_at)
VALUES ($1,$2,$3,$4)
`, e.OrderID, e.EventType, b, e.OccurredAt)
	return err
}

func (r *TrackerRepo) GetOrderView(ctx context.Context, id string) (models.OrderView, bool, error) {
	var v models.OrderView
	var status string
	err := r.db.QueryRowContext(ctx, `
SELECT order_id, status, updated_at, COALESCE(customer_name,''), last_event_at
FROM orders_view WHERE order_id=$1
`, id).Scan(&v.OrderID, &status, &v.UpdatedAt, &v.CustomerName, &v.LastEventAt)
	if errors.Is(err, sql.ErrNoRows) {
		return models.OrderView{}, false, nil
	}
	if err != nil {
		return models.OrderView{}, false, err
	}
	v.Status = models.OrderStatus(status)
	return v, true, nil
}

func (r *TrackerRepo) GetOrderTimeline(ctx context.Context, id string, limit, offset int) ([]models.OrderEvent, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT event_type, payload, occurred_at
FROM order_events WHERE order_id=$1
ORDER BY occurred_at ASC
LIMIT $2 OFFSET $3
`, id, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []models.OrderEvent
	for rows.Next() {
		var t string
		var payloadRaw []byte
		var at time.Time
		if err := rows.Scan(&t, &payloadRaw, &at); err != nil {
			return nil, err
		}
		var pl map[string]any
		_ = json.Unmarshal(payloadRaw, &pl)
		out = append(out, models.OrderEvent{OrderID: id, EventType: t, Payload: pl, OccurredAt: at})
	}
	return out, rows.Err()
}

func (r *TrackerRepo) ListOrders(ctx context.Context, filter map[string]string, limit int, cursor string) ([]models.OrderView, string, error) {
	return nil, "", nil // TODO
}

func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}
