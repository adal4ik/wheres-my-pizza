package order

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"

	"wheres-my-pizza/internal/connections/rabbitmq"
	"wheres-my-pizza/internal/microservices/order/handlers"
	"wheres-my-pizza/internal/microservices/order/repository"
	"wheres-my-pizza/internal/microservices/order/service"
)

func Run(port string, maxConcurrency int, db *sql.DB, rmqClient *rabbitmq.Client) {
	// Initialize repository
	repo := repository.New(db)
	// Initialize service
	svc := service.New(*repo, *rmqClient)
	handler := handlers.New(svc)
	_ = maxConcurrency
	// Start listener
	Listener(port, handler)
	_ = rmqClient
}

func Listener(port string, handler *handlers.Handler) {
	mux := http.NewServeMux()

	mux.HandleFunc("POST /orders", handler.OrderHandler.AddOrder)

	addr := ":" + port
	fmt.Printf("Order Service listening on %s\n", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("Order Service failed: %v", err)
	}
}
