package domain

type OrderItemMsg struct {
	Name     string  `json:"name"`
	Quantity int     `json:"quantity"`
	Price    float64 `json:"price"`
}

type OrderMessage struct {
	OrderNumber   string         `json:"order_number"`
	CustomerName  string         `json:"customer_name"`
	OrderType     string         `json:"order_type"`
	TableNumber   *int           `json:"table_number,omitempty"`
	DeliveryAddr  *string        `json:"delivery_address,omitempty"`
	Items         []OrderItemMsg `json:"items"`
	TotalAmount   float64        `json:"total_amount"`
	Priority      int            `json:"priority"`
}
