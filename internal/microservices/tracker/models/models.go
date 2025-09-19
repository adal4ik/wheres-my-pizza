package models

import "time"

type OrderStatus string

const (
	StatusCreated   OrderStatus = "CREATED"
	StatusAccepted  OrderStatus = "ACCEPTED"
	StatusReady     OrderStatus = "READY"
	StatusPickedUp  OrderStatus = "PICKED_UP"
	StatusDelivered OrderStatus = "DELIVERED"
	StatusCanceled  OrderStatus = "CANCELED"
)

type OrderEvent struct {
	OrderID    string         // order_number
	EventType  string         // e.g. "status.ACCEPTED"
	Payload    map[string]any // arbitrary details
	OccurredAt time.Time
}

type OrderView struct {
	OrderID             string      // order_number
	Status              OrderStatus // mapped from orders.status
	UpdatedAt           time.Time   // orders.updated_at
	CustomerName        string      // orders.customer_name
	EstimatedCompletion time.Time   // created_at + 20m
}

type WorkerStatus struct {
	WorkerName      string    `json:"worker_name"`
	Status          string    `json:"status"` // "online" | "offline"
	OrdersProcessed int       `json:"orders_processed"`
	LastSeen        time.Time `json:"last_seen"`
}
