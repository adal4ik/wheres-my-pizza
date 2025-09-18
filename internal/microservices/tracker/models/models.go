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
	OrderID    string         // "abc-123"
	EventType  string         // "order.created", "kitchen.ready", ...
	Payload    map[string]any // детали
	OccurredAt time.Time      // из сообщения
}

type OrderView struct {
	OrderID      string
	Status       OrderStatus
	UpdatedAt    time.Time
	CustomerName string
	LastEventAt  time.Time
}
