package kitchen

import (
	"context"
	"database/sql"
	"log"
	"time"
	"wheres-my-pizza/internal/connections/rabbitmq"
	"wheres-my-pizza/internal/microservices/kitchen/repository"
	"wheres-my-pizza/internal/microservices/kitchen/service"
)

func Run(ctx context.Context, db *sql.DB, rmqClient *rabbitmq.Client) {
	repo := repository.NewKitchenRepository(db)
	service := service.NewKitchenService(repo, *rmqClient)
	_ = service
	// heartbeat отдельной горутиной (не критично, но полезно)
	go func() {
		t := time.NewTicker(30 * time.Second)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				if err := service.Heartbeat(ctx); err != nil {
					log.Printf("kitchen heartbeat error: %v", err)
				}
			}
		}
	}()

	err := service.Run(ctx)
	if err != nil {
		log.Printf("kitchen service stopped with error: %v", err)
	}
}
