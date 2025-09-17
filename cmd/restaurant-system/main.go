package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"restaurant-system/internal/app/kitchen"
	"restaurant-system/internal/app/notify"
	"restaurant-system/internal/app/order"
	"restaurant-system/internal/app/tracking"
	"restaurant-system/internal/common/logger"
)

func main() {
	mode := flag.String("mode", "", "order-service | kitchen-worker | tracking-service | notification-subscriber")
	port := flag.Int("port", 0, "http port for services that expose HTTP")
	maxConc := flag.Int("max-concurrent", 50, "order-service: max concurrent requests")
	workerName := flag.String("worker-name", "", "kitchen-worker: unique worker name")
	orderTypes := flag.String("order-types", "", "kitchen-worker: comma-separated order types to handle")
	heartbeat := flag.Int("heartbeat-interval", 30, "kitchen-worker: heartbeat interval seconds")
	prefetch := flag.Int("prefetch", 1, "kitchen-worker: RabbitMQ prefetch")
	flag.Parse()

	lg := logger.New("bootstrap")
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	switch *mode {
	case "order-service":
		if *port == 0 {
			*port = 3000
		}
		lg.Info("service_started", map[string]any{"service": "order-service", "port": *port, "max_concurrent": *maxConc})
		if err := order.Run(ctx, *port, *maxConc); err != nil {
			lg.Error("fatal", err, nil)
			os.Exit(1)
		}
	case "kitchen-worker":
		if *workerName == "" {
			fmt.Fprintln(os.Stderr, "--worker-name is required for kitchen-worker")
			os.Exit(2)
		}
		lg.Info("service_started", map[string]any{"service": "kitchen-worker", "worker": *workerName})
		if err := kitchen.Run(ctx, kitchen.Config{
			WorkerName: *workerName, OrderTypes: *orderTypes,
			Heartbeat: time.Duration(*heartbeat) * time.Second, Prefetch: *prefetch,
		}); err != nil {
			lg.Error("fatal", err, nil)
			os.Exit(1)
		}
	case "tracking-service":
		if *port == 0 {
			*port = 3002
		}
		lg.Info("service_started", map[string]any{"service": "tracking-service", "port": *port})
		if err := tracking.Run(ctx, *port); err != nil {
			lg.Error("fatal", err, nil)
			os.Exit(1)
		}
	case "notification-subscriber":
		lg.Info("service_started", map[string]any{"service": "notification-subscriber"})
		if err := notify.Run(ctx); err != nil {
			lg.Error("fatal", err, nil)
			os.Exit(1)
		}
	default:
		fmt.Fprintln(os.Stderr, "--mode is required: order-service | kitchen-worker | tracking-service | notification-subscriber")
		os.Exit(2)
	}
}
