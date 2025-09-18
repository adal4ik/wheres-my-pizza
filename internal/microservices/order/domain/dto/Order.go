package domain

import (
	"time"

	"wheres-my-pizza/internal/microservices/order/domain/dao"
)

type CreateOrderRequest struct {
	CustomerName string           `json:"customer_name"`
	OrderType    string           `json:"order_type"`
	Table_number int              `json:"table_number"`
	DeliveryAddr string           `json:"delivery_address"`
	Items        []OrderItemInput `json:"items"`
}

type CreateOrderResponse struct {
	OrderNumber string  `json:"order_number"`
	Status      string  `json:"status"`
	TotalAmount float64 `json:"total_amount"`
}
type OrderItemInput struct {
	Name     string  `json:"name"`
	Quantity int     `json:"quantity"`
	Price    float64 `json:"price"`
}

// ConvertItems maps input items to domain OrderItem slice
func ConvertItems(inputs []OrderItemInput) []dao.OrderItem {
	items := make([]dao.OrderItem, 0, len(inputs))
	now := time.Now().UTC()

	for _, in := range inputs {
		items = append(items, dao.OrderItem{
			Name:      in.Name,
			Quantity:  in.Quantity,
			Price:     in.Price,
			CreatedAt: now,
		})
	}

	return items
}
