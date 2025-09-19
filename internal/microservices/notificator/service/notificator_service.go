package service

import "wheres-my-pizza/internal/connections/rabbitmq"

type NotificatorService struct {
	rmqClient rabbitmq.Client
}

func NewNotificatorService(rqmClient rabbitmq.Client) *NotificatorService {
	return &NotificatorService{rmqClient: rqmClient}
}

func 