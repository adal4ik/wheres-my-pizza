package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"

	"wheres-my-pizza/internal/connections/database"
	"wheres-my-pizza/internal/connections/rabbitmq"
)

type Config struct {
	Database database.Config
	Rabbit   rabbitmq.Config
}

func main() {
	var cfgPath string
	flag.StringVar(&cfgPath, "config", "config.yml", "path to YAML config")
	flag.Parse()

	cfg, err := loadConfig(cfgPath)
	if err != nil {
		log.Fatalf("config load error: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// DB connect
	dbPool, err := database.New(ctx, cfg.Database)
	if err != nil {
		log.Fatalf("db connect error: %v", err)
	}
	defer dbPool.Close()
	if err := database.Ping(ctx, dbPool); err != nil {
		log.Fatalf("db ping error: %v", err)
	}
	log.Printf("Postgres connected: %s:%d/%s", cfg.Database.Host, cfg.Database.Port, cfg.Database.Database)

	// Rabbit connect
	rmq, err := rabbitmq.Dial(cfg.Rabbit)
	if err != nil {
		log.Fatalf("rabbitmq connect error: %v", err)
	}
	defer rmq.Close()
	if err := rmq.Ping(); err != nil {
		log.Fatalf("rabbitmq ping error: %v", err)
	}
	log.Printf("RabbitMQ connected: %s:%d vhost=%q", cfg.Rabbit.Host, cfg.Rabbit.Port, cfg.Rabbit.VHost)

	// graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	log.Println("server is up. press Ctrl+C to stop.")
	<-sigCh
	log.Println("shutdown signal received, exiting...")
}

// --- very small, purpose-built YAML reader for our simple config format ---
// Supports top-level sections `database:` and `rabbitmq:` and their k:v pairs.
func loadConfig(path string) (Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return Config{}, err
	}
	defer f.Close()

	var (
		cfg     Config
		section string
	)
	// Defaults (optional)
	cfg.Database.Port = 5432
	cfg.Rabbit.Port = 5672
	cfg.Rabbit.VHost = "/"
	cfg.Database.SSLMode = "disable"
	cfg.Database.MaxConns = 10

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// section?
		if strings.HasSuffix(line, ":") && !strings.Contains(line, " ") {
			section = strings.TrimSuffix(line, ":")
			continue
		}
		// key: value
		kv := strings.SplitN(line, ":", 2)
		if len(kv) != 2 {
			continue
		}
		key := strings.TrimSpace(kv[0])
		val := strings.TrimSpace(kv[1])
		val = strings.Trim(val, `"'`)

		switch section {
		case "database":
			switch key {
			case "host":
				cfg.Database.Host = val
			case "port":
				cfg.Database.Port = atoi(val, 5432)
			case "user":
				cfg.Database.User = val
			case "password":
				cfg.Database.Password = val
			case "database":
				cfg.Database.Database = val
			case "sslmode":
				if val != "" {
					cfg.Database.SSLMode = val
				}
			case "max_conns":
				cfg.Database.MaxConns = atoi(val, 10)
			}
		case "rabbitmq":
			switch key {
			case "host":
				cfg.Rabbit.Host = val
			case "port":
				cfg.Rabbit.Port = atoi(val, 5672)
			case "user":
				cfg.Rabbit.User = val
			case "password":
				cfg.Rabbit.Password = val
			case "vhost":
				if val != "" {
					cfg.Rabbit.VHost = val
				}
			}
		}
	}
	if err := sc.Err(); err != nil {
		return Config{}, err
	}

	// Basic validation
	if cfg.Database.Host == "" || cfg.Database.User == "" || cfg.Database.Database == "" {
		return Config{}, fmt.Errorf("database config incomplete")
	}
	if cfg.Rabbit.Host == "" || cfg.Rabbit.User == "" {
		return Config{}, fmt.Errorf("rabbitmq config incomplete")
	}
	return cfg, nil
}

func atoi(s string, def int) int {
	if s == "" {
		return def
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return n
}
