package service

import (
	"wheres-my-pizza/internal/connections/rabbitmq"
	"wheres-my-pizza/internal/microservices/kitchen/repository"
)

type Service struct {
	OrderService KitchenServiceInterface
}

func New(db repository.Repository, rmqClient rabbitmq.Client) *Service {
	return &Service{
		OrderService: NewKitchenService(db.KitchenRepo, rmqClient),
	}
}
