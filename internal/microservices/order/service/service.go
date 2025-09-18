package service

import (
	"wheres-my-pizza/internal/connections/rabbitmq"
	"wheres-my-pizza/internal/microservices/order/repository"
)

type Service struct {
	OrderService OrderServiceInterface
}

func New(db repository.Repository, rmqClient rabbitmq.Client) *Service {
	return &Service{
		OrderService: NewOrderService(db.OrderRepo, rmqClient),
	}
}
