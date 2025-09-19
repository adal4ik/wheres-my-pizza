package service

import "wheres-my-pizza/internal/connections/rabbitmq"

type Service struct {
	NotificatorService *NotificatorService
}

func New(rmqClient rabbitmq.Client) *Service {
	return &Service{NotificatorService: NewNotificatorService(rmqClient)}
}
