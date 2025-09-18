package handlers

import "wheres-my-pizza/internal/microservices/order/service"

type Handler struct {
	OrderHandler *OrderHandler
}

func New(s *service.Service) *Handler {
	return &Handler{
		OrderHandler: NewOrderHandler(s.OrderService),
	}
}
