package repository

import "context"

type Orders interface {
	CreateOrderTx(ctx context.Context, req any) (orderNumber string, total float64, priority int, err error)
}

type ordersPG struct{}

func NewOrdersPG() Orders { return &ordersPG{} }

func (o *ordersPG) CreateOrderTx(ctx context.Context, req any) (string, float64, int, error) {
	return "ORD_19700101_001", 0, 1, nil
}
