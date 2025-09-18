##### Config #####

PROJECT        ?= where-is-my-pizza
BIN_DIR        ?= bin
BIN            ?= $(BIN_DIR)/$(PROJECT)
CMD            ?= ./cmd/server

GO             ?= go
COMPOSE        ?= docker compose
PWD            := $(shell pwd)

# DSN для migrate (совпадает с docker-compose.yml из нашей переписки)
POSTGRES_DSN ?= postgres://restaurant_user:restaurant_pass@localhost:5431/pizza?sslmode=disable
AMQP_URL       ?= amqp://pizza:pizza@localhost:5672/
MIGRATIONS_DIR ?= ./migrations
MIGRATE_IMG    ?= migrate/migrate

# Запуск бинаря
MODE           ?= all           # order|kitchen|tracking|notification|all
CONFIG         ?= ./configs/$(MODE).yaml

##### Phony #####
.PHONY: help compose-up compose-down up down logs \
        build run clean fmt vet test \
        migrate-up migrate-down migrate-down-all migrate-force migrate-create \
        db-wait rmq-wait psql

help:
	@echo "Targets:"
	@echo "  compose-up          — запустить Postgres и RabbitMQ (docker compose up -d)"
	@echo "  compose-down        — остановить и удалить контейнеры (+ тома)"
	@echo "  up                  — compose-up + ожидание портов + migrate-up"
	@echo "  down                — compose-down"
	@echo "  logs                — tail логов docker compose"
	@echo "  build               — собрать бинарь ($(BIN))"
	@echo "  run                 — запустить бинарь (--mode=$(MODE) --config=$(CONFIG))"
	@echo "  fmt                 — gofumpt форматирование"
	@echo "  vet                 — go vet"
	@echo "  test                — go test ./..."
	@echo "  migrate-up          — выполнить все up-миграции"
	@echo "  migrate-down        — откатить одну миграцию"
	@echo "  migrate-down-all    — откатить все миграции"
	@echo "  migrate-force VER=  — выставить версию миграций (FORCE)"
	@echo "  migrate-create NAME=short_desc — создать пару файлов .up/.down"
	@echo "  psql                — открыть psql внутри контейнера postgres"

##### Docker #####

compose-up:
	$(COMPOSE) up -d

compose-down:
	$(COMPOSE) down -v

logs:
	$(COMPOSE) logs -f

##### Waiters (для локалки на Linux) #####

db-wait:
	@echo "⌛ Waiting for Postgres on 5432..."
	@bash -c 'for i in {1..60}; do nc -z 127.0.0.1 5432 && exit 0; sleep 1; done; echo "Postgres not up"; exit 1'

rmq-wait:
	@echo "⌛ Waiting for RabbitMQ on 5672..."
	@bash -c 'for i in {1..60}; do nc -z 127.0.0.1 5672 && exit 0; sleep 1; done; echo "RabbitMQ not up"; exit 1'

up: compose-up db-wait rmq-wait migrate-up
down: compose-down

##### Build / Run #####

$(BIN):
	@mkdir -p $(BIN_DIR)
	$(GO) build -o $(BIN) $(CMD)

build: $(BIN)

run: build
	$(BIN) --mode=$(MODE) --config=$(CONFIG)

clean:
	rm -rf $(BIN_DIR)

fmt:
	@command -v gofumpt >/dev/null || { echo "Installing gofumpt..."; $(GO) install mvdan.cc/gofumpt@latest; }
	gofumpt -l -w .

vet:
	$(GO) vet ./...

test:
	$(GO) test ./...

##### Migrations (golang-migrate в docker) #####

migrate-up:
	docker run --rm -v "$(PWD)/$(MIGRATIONS_DIR):/migrations:ro" \
		--network host $(MIGRATE_IMG) \
		-path=/migrations -database "$(POSTGRES_DSN)" up

migrate-down:
	docker run --rm -v "$(PWD)/$(MIGRATIONS_DIR):/migrations:ro" \
		--network host $(MIGRATE_IMG) \
		-path=/migrations -database "$(POSTGRES_DSN)" down 1

migrate-down-all:
	docker run --rm -v "$(PWD)/$(MIGRATIONS_DIR):/migrations:ro" \
		--network host $(MIGRATE_IMG) \
		-path=/migrations -database "$(POSTGRES_DSN)" down -all

migrate-force:
	@if [ -z "$(VER)" ]; then echo "Usage: make migrate-force VER=<version|0>"; exit 1; fi
	docker run --rm -v "$(PWD)/$(MIGRATIONS_DIR):/migrations:ro" \
		--network host $(MIGRATE_IMG) \
		-path=/migrations -database "$(POSTGRES_DSN)" force $(VER)

migrate-create:
	@if [ -z "$(NAME)" ]; then echo "Usage: make migrate-create NAME=<short_desc>"; exit 1; fi
	@mkdir -p $(MIGRATIONS_DIR)
	@ts=$$(date +%Y%m%d%H%M%S); \
	up="$(MIGRATIONS_DIR)/$${ts}_$(NAME).up.sql"; \
	down="$(MIGRATIONS_DIR)/$${ts}_$(NAME).down.sql"; \
	touch $$up $$down; \
	echo "created: $$up"; echo "created: $$down"

##### psql helper #####

psql:
	@docker exec -it pizza-postgres psql -U postgres -d pizza
