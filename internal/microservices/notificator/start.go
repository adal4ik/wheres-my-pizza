package notificator

import (
	"context"
	"log"

	"wheres-my-pizza/internal/connections/rabbitmq"
	"wheres-my-pizza/internal/microservices/notificator/service"
)

func Start(ctx context.Context, rmqClient *rabbitmq.Client) {
	ns := service.NewNotificatorService(*rmqClient)
	if err := ns.Notify(ctx); err != nil {
		log.Printf("notificator stopped with error: %v", err)
	}
}
