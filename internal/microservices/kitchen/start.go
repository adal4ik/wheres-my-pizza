package kitchen

import (
	"context"
	"database/sql"
	"log"

	"wheres-my-pizza/internal/connections/rabbitmq"
	"wheres-my-pizza/internal/microservices/kitchen/repository"
	"wheres-my-pizza/internal/microservices/kitchen/service"
)

func Run(ctx context.Context, db *sql.DB, rmqClient *rabbitmq.Client, workerName string, orderTypes string, prefetch int, heartbeatInterval int) {
	repo := repository.NewKitchenRepository(db)
	service := service.NewKitchenService(repo, *rmqClient, workerName, orderTypes, prefetch, heartbeatInterval)

	err := service.Run(ctx)
	if err != nil {
		log.Printf("kitchen service stopped with error: %v", err)
	}
}
