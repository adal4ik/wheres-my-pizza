package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	"wheres-my-pizza/internal/config"
	"wheres-my-pizza/internal/connections/database"
	"wheres-my-pizza/internal/connections/rabbitmq"
	"wheres-my-pizza/internal/microservices/kitchen"
	"wheres-my-pizza/internal/microservices/notificator"
	"wheres-my-pizza/internal/microservices/order"
	"wheres-my-pizza/internal/microservices/tracker"
)

func main() {
	// Global flags
	mode := flag.String("mode", "", "Service mode: order-service | kitchen-worker | tracking-service | notification-subscriber")
	orderPort := flag.Int("port", 3000, "HTTP port for order-service")
	orderMaxConcurrent := flag.Int("max-concurrent", 50, "Max concurrent orders")

	// Kitchen Worker
	workerName := flag.String("worker-name", "", "Unique worker name (required for kitchen-worker)")
	orderTypes := flag.String("order-types", "general", "Comma-separated order types (optional)")
	heartbeat := flag.Int("heartbeat-interval", 30, "Heartbeat interval in seconds")
	prefetch := flag.Int("prefetch", 1, "RabbitMQ prefetch count")

	// Tracking Service
	trackingPort := flag.Int("tracking-port", 3002, "HTTP port for tracking-service")

	flag.Parse()

	// Load config
	cfg, err := config.LoadConfig("config.yml")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}
	// Validate required flag
	if *mode == "" {
		fmt.Println("Error: --mode flag is required")
		flag.Usage()
		os.Exit(1)
	}

	// Start selected service
	switch *mode {
	case "order-service":
		ctx := context.Background()
		fmt.Printf("Starting Order Service on port %d (max concurrent = %d)\n", *orderPort, *orderMaxConcurrent)
		db, err := database.ConnectDB(ctx, cfg.Database)
		if err != nil {
			log.Fatalf("Database connection failed: %v", err)
		}
		defer db.Close()
		rmqClient, err := rabbitmq.Dial(cfg.RabbitMQ)
		if err != nil {
			log.Fatalf("RabbitMQ connection failed: %v", err)
		}
		defer rmqClient.Close()
		order.Run(fmt.Sprintf("%d", *orderPort), *orderMaxConcurrent, db, rmqClient)

	case "kitchen-worker":
		if *workerName == "" {
			log.Fatal("Error: --worker-name flag is required for kitchen-worker")
		}
		fmt.Printf("Starting Kitchen Worker: %s (types = %s, prefetch = %d, heartbeat = %d)\n",
			*workerName, *orderTypes, *prefetch, *heartbeat)
		// TODO: init DB + RabbitMQ consumer
		ctx := context.Background()
		fmt.Printf("Starting Order Service on port %d (max concurrent = %d)\n", *orderPort, *orderMaxConcurrent)
		db, err := database.ConnectDB(ctx, cfg.Database)
		if err != nil {
			log.Fatalf("Database connection failed: %v", err)
		}
		defer db.Close()
		rmqClient, err := rabbitmq.Dial(cfg.RabbitMQ)
		if err != nil {
			log.Fatalf("RabbitMQ connection failed: %v", err)
		}
		defer rmqClient.Close()
		kitchen.Run(context.Background(), db, rmqClient, *workerName, *orderTypes, *prefetch, *heartbeat)

	case "tracking-service":
		fmt.Printf("Starting Tracking Service on port %d\n", *trackingPort)

		ctx := context.Background()
		db, err := database.ConnectDB(ctx, cfg.Database) // уже есть в твоём коде
		if err != nil {
			log.Fatalf("Database connection failed: %v", err)
		}
		defer db.Close()

		addr := fmt.Sprintf(":%d", *trackingPort)
		if err := tracker.Start(addr, db); err != nil {
			log.Fatalf("Tracking service failed: %v", err)
		}
	case "notification-subscriber":
		fmt.Println("Starting Notification Subscriber")
		ctx := context.Background()
		rmqClient, err := rabbitmq.Dial(cfg.RabbitMQ)
		if err != nil {
			log.Fatalf("RabbitMQ connection failed: %v", err)
		}
		defer rmqClient.Close()
		notificator.Start(ctx, rmqClient)

	default:
		fmt.Printf("Unknown mode: %s\n", *mode)
		flag.Usage()
		os.Exit(1)
	}
}
