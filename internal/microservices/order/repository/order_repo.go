package repository

import (
	"database/sql"
	"fmt"

	"wheres-my-pizza/internal/microservices/order/domain/dao"
)

type OrderRepositoryInterface interface {
	AddOrder(order dao.Order) error
	GetOrderCount() (int, error)
}

type OrderRepository struct {
	db *sql.DB
}

func NewOrderRepository(db *sql.DB) OrderRepositoryInterface {
	return &OrderRepository{db: db}
}
func (or *OrderRepository) GetOrderCount() (int, error) {
	var count int
	err := or.db.QueryRow("SELECT COUNT(*) FROM orders").Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to get order count: %w", err)
	}
	return count, nil
}

func (or *OrderRepository) AddOrder(order dao.Order) error {
	tx, err := or.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	// 1. Insert order
	var orderID int
	err = tx.QueryRow(`
		INSERT INTO orders 
		    (order_number, customer_name, order_type, table_number, delivery_address, total_amount, priority, status, processed_by, completed_at, created_at, updated_at) 
		VALUES 
		    ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, NOW(), NOW())
		RETURNING id
	`,
		order.OrderNumber,
		order.CustomerName,
		order.OrderType,
		order.TableNumber,
		order.DeliveryAddr,
		order.TotalAmount,
		order.Priority,
		order.Status,
		order.ProcessedBy,
		order.CompletedAt,
	).Scan(&orderID)
	if err != nil {
		return fmt.Errorf("failed to insert order: %w", err)
	}

	// 2. Insert order items
	for _, item := range order.Items {
		_, err = tx.Exec(`
			INSERT INTO order_items (order_id, name, quantity, price, created_at)
			VALUES ($1, $2, $3, $4, NOW())
		`, orderID, item.Name, item.Quantity, item.Price)
		if err != nil {
			return fmt.Errorf("failed to insert order item %s: %w", item.Name, err)
		}
	}

	// 3. Insert into order_status_log
	_, err = tx.Exec(`
	INSERT INTO order_status_log (order_id, status, changed_by, changed_at)
	VALUES ($1, $2, 'order-service', NOW())
`, orderID, order.Status)
	if err != nil {
		return fmt.Errorf("failed to insert order status log: %w", err)
	}

	// Commit
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}
