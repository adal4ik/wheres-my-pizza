package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"wheres-my-pizza/internal/config"
	"wheres-my-pizza/internal/connections/database"
	"wheres-my-pizza/internal/connections/rabbitmq"
	"wheres-my-pizza/internal/microservices/kitchen"
	"wheres-my-pizza/internal/microservices/order"
	"wheres-my-pizza/internal/microservices/tracker"
)

func main() {
	// Global flags
	mode := flag.String("mode", "", "Service mode: order-service | kitchen-worker | tracking-service | notification-subscriber")
	orderPort := flag.String("port", "3000", "HTTP port for order-service")
	orderMaxConcurrent := flag.Int("max-concurrent", 50, "Max concurrent orders")

	// Kitchen Worker
	workerName := flag.String("worker-name", "", "Unique worker name (required for kitchen-worker)")
	orderTypes := flag.String("order-types", "", "Comma-separated order types: dine_in,takeout,delivery (optional; empty=all)")
	heartbeat := flag.Int("heartbeat-interval", 30, "Heartbeat interval in seconds")
	prefetch := flag.Int("prefetch", 1, "RabbitMQ prefetch count (ignored in GET mode)")

	// Tracking Service
	trackingPort := flag.Int("tracking-port", 3002, "HTTP port for tracking-service")

	flag.Parse()

	// Load config
	cfg, err := config.LoadConfig("config.yml")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}
	if *mode == "" {
		log.Fatalf("Error: --mode flag is required")
	}

	// common ctx with signals
	rootCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	switch *mode {
	case "order-service":
		log.Printf("Starting Order Service on port %d (max concurrent = %d)", *orderPort, *orderMaxConcurrent)

		db, err := database.ConnectDB(rootCtx, cfg.Database)
		if err != nil {
			log.Fatalf("Database connection failed: %v", err)
		}
		defer db.Close()

		rmqClient, err := rabbitmq.Dial(cfg.RabbitMQ) // *rabbitmq.Client
		if err != nil {
			log.Fatalf("RabbitMQ connection failed: %v", err)
		}
		defer rmqClient.Close()

		order.Run(*orderPort, *orderMaxConcurrent, db, rmqClient)

	case "kitchen-worker":
		if *workerName == "" {
			log.Fatal("Error: --worker-name flag is required for kitchen-worker")
		}
		log.Printf("Starting Kitchen Worker: %s (types=%q, prefetch=%d, heartbeat=%d)", *workerName, *orderTypes, *prefetch, *heartbeat)

		db, err := database.ConnectDB(rootCtx, cfg.Database)
		if err != nil {
			log.Fatalf("Database connection failed: %v", err)
		}
		defer db.Close()

		rmqClient, err := rabbitmq.Dial(cfg.RabbitMQ) // *rabbitmq.Client
		if err != nil {
			log.Fatalf("RabbitMQ connection failed: %v", err)
		}
		defer rmqClient.Close()

		// kitchen.Run должен принимать *rabbitmq.Client (указатель!), не значение
		kitchen.Run(rootCtx, db, rmqClient, *workerName, *orderTypes, *prefetch, *heartbeat)

	case "tracking-service":
		log.Printf("Starting Tracking Service on port %d", *trackingPort)

		db, err := database.ConnectDB(rootCtx, cfg.Database)
		if err != nil {
			log.Fatalf("Database connection failed: %v", err)
		}
		defer db.Close()

		addr := fmt.Sprintf(":%d", *trackingPort)
		if err := tracker.Start(addr, db); err != nil {
			log.Fatalf("Tracking service failed: %v", err)
		}

	case "notification-subscriber":
		log.Println("Starting Notification Subscriber (TODO)")
		// инициализация подписчика на notifications_fanout при необходимости

	default:
		log.Fatalf("Unknown mode: %s", *mode)
	}
}
