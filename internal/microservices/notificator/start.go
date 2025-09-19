package notificator

import (
	"context"

	"wheres-my-pizza/internal/connections/rabbitmq"
	"wheres-my-pizza/internal/microservices/notificator/service"
)

func Start(ctx context.Context, rmqClient *rabbitmq.Client) {
	service := service.NewNotificatorService(*rmqClient)
	service.Notify()
}
