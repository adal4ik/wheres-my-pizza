SHELL := /bin/bash

build:
	go fmt ./...
	go build -o restaurant-system ./cmd/restaurant-system

run-order:
	./restaurant-system --mode=order-service --port=3000

run-tracking:
	./restaurant-system --mode=tracking-service --port=3002

run-kitchen:
	./restaurant-system --mode=kitchen-worker --worker-name=chef_anna --prefetch=1

run-notify:
	./restaurant-system --mode=notification-subscriber

compose-up:
	docker compose -f deploy/docker-compose.yml up -d --build

compose-down:
	docker compose -f deploy/docker-compose.yml down -v
