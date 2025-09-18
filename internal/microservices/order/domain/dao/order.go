package dao

import "time"

type Order struct {
	ID           int         `json:"id"`
	OrderNumber  string      `json:"order_number"`
	CustomerName string      `json:"customer_name"`
	OrderType    string      `json:"order_type"` // dine_in | takeout | delivery
	TableNumber  int         `json:"table_number"`
	DeliveryAddr string      `json:"delivery_address"`
	TotalAmount  float64     `json:"total_amount"`
	Priority     int         `json:"priority"`
	Status       string      `json:"status"`
	ProcessedBy  string      `json:"processed_by"`
	CompletedAt  time.Time   `json:"completed_at"`
	CreatedAt    time.Time   `json:"created_at"`
	UpdatedAt    time.Time   `json:"updated_at"`
	Items        []OrderItem `json:"items,omitempty"`
}

type OrderItem struct {
	ID        int       `json:"id"`
	OrderID   int       `json:"order_id"`
	Name      string    `json:"name"`
	Quantity  int       `json:"quantity"`
	Price     float64   `json:"price"`
	CreatedAt time.Time `json:"created_at"`
}

// FOR RABBITMQ MESSAGE

type OrderMessage struct {
	OrderNumber     string      `json:"order_number"`
	CustomerName    string      `json:"customer_name"`
	OrderType       string      `json:"order_type"`
	TableNumber     int         `json:"table_number"`
	DeliveryAddress string      `json:"delivery_address"`
	Items           []OrderItem `json:"items"`
	TotalAmount     float64     `json:"total_amount"`
	Priority        int         `json:"priority"`
}
