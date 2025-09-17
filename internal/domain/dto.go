package domain

type CreateOrderItem struct {
	Name     string  `json:"name"`
	Quantity int     `json:"quantity"`
	Price    float64 `json:"price"`
}

type CreateOrderRequest struct {
	CustomerName string            `json:"customer_name"`
	OrderType    string            `json:"order_type"`
	TableNumber  *int              `json:"table_number,omitempty"`
	DeliveryAddr *string           `json:"delivery_address,omitempty"`
	Items        []CreateOrderItem `json:"items"`
}

type CreateOrderResponse struct {
	OrderNumber string  `json:"order_number"`
	Status      string  `json:"status"`
	TotalAmount float64 `json:"total_amount"`
}
