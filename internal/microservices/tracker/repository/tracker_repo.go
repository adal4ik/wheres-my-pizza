package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"wheres-my-pizza/internal/microservices/tracker/models"
)

type TrackerRepoInterface interface {
	GetOrderView(ctx context.Context, orderNumber string) (models.OrderView, bool, error)
	GetOrderTimeline(ctx context.Context, orderNumber string) ([]models.OrderEvent, error)
	ListWorkersStatus(ctx context.Context) ([]models.WorkerStatus, error)
}

type TrackerRepo struct{ db *sql.DB }

func NewTrackerRepo(db *sql.DB) *TrackerRepo { return &TrackerRepo{db: db} }

func mapDBStatus(s string) models.OrderStatus {
	switch s {
	case "received", "created":
		return models.StatusCreated
	case "accepted":
		return models.StatusAccepted
	case "ready":
		return models.StatusReady
	case "picked_up":
		return models.StatusPickedUp
	case "delivered", "completed":
		return models.StatusDelivered
	case "canceled", "cancelled":
		return models.StatusCanceled
	default:
		return models.StatusCreated
	}
}

func (r *TrackerRepo) getOrderID(ctx context.Context, orderNumber string) (int64, error) {
	var id int64
	err := r.db.QueryRowContext(ctx, `SELECT id FROM orders WHERE order_number = $1`, orderNumber).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, sql.ErrNoRows
	}
	return id, err
}

func (r *TrackerRepo) GetOrderView(ctx context.Context, orderNumber string) (models.OrderView, bool, error) {
	var (
		statusDB          string
		updatedAt         time.Time
		createdAt         time.Time
		customerName      sql.NullString
		estimatedComplete time.Time
	)

	q := `
SELECT
  o.status,
  o.updated_at,
  o.created_at,
  o.customer_name,
  (o.created_at + (20 * interval '1 minute')) AS eta
FROM orders o
WHERE o.order_number = $1
`
	err := r.db.QueryRowContext(ctx, q, orderNumber).Scan(
		&statusDB, &updatedAt, &createdAt, &customerName, &estimatedComplete,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return models.OrderView{}, false, nil
	}
	if err != nil {
		return models.OrderView{}, false, err
	}

	view := models.OrderView{
		OrderID:             orderNumber,
		Status:              mapDBStatus(statusDB),
		UpdatedAt:           updatedAt,
		CustomerName:        valueOrEmpty(customerName),
		EstimatedCompletion: estimatedComplete,
	}
	return view, true, nil
}
func (r *TrackerRepo) GetOrderTimeline(ctx context.Context, orderNumber string) ([]models.OrderEvent, error) {
	internalID, err := r.getOrderID(ctx, orderNumber)
	if errors.Is(err, sql.ErrNoRows) {
		return []models.OrderEvent{}, nil
	}
	if err != nil {
		return nil, err
	}

	rows, err := r.db.QueryContext(ctx, `
SELECT status, changed_by, changed_at, COALESCE(notes,'')
FROM order_status_log
WHERE order_id = $1
ORDER BY changed_at ASC, id ASC
`, internalID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []models.OrderEvent
	for rows.Next() {
		var (
			status sql.NullString
			by     sql.NullString
			at     time.Time
			notes  string
		)
		if err := rows.Scan(&status, &by, &at, &notes); err != nil {
			return nil, err
		}
		payload := map[string]any{
			"changed_by": valueOrEmpty(by),
			"notes":      notes,
		}
		evType := fmt.Sprintf("status.%s", mapDBStatus(status.String))
		out = append(out, models.OrderEvent{
			OrderID:    orderNumber,
			EventType:  evType,
			Payload:    payload,
			OccurredAt: at,
		})
	}
	return out, rows.Err()
}

func (r *TrackerRepo) ListWorkersStatus(ctx context.Context) ([]models.WorkerStatus, error) {
	const heartbeatSeconds = 30                                  // базовый heartbeat
	threshold := time.Duration(2*heartbeatSeconds) * time.Second // оффлайн если last_seen старше 2*heartbeat

	rows, err := r.db.QueryContext(ctx, `
SELECT name, COALESCE(orders_processed,0) AS orders_processed, last_seen
FROM workers
ORDER BY name ASC
`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	now := time.Now().UTC()
	var out []models.WorkerStatus

	for rows.Next() {
		var (
			name      string
			processed int
			lastSeen  time.Time
		)
		if err := rows.Scan(&name, &processed, &lastSeen); err != nil {
			return nil, err
		}
		status := "online"
		if lastSeen.IsZero() || now.Sub(lastSeen) > threshold {
			status = "offline"
		}
		out = append(out, models.WorkerStatus{
			WorkerName:      name,
			Status:          status,
			OrdersProcessed: processed,
			LastSeen:        lastSeen,
		})
	}
	return out, rows.Err()
}

func valueOrEmpty(s sql.NullString) string {
	if s.Valid {
		return s.String
	}
	return ""
}
