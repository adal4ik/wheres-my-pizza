package config

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Config хранит все параметры приложения
type Config struct {
	Database DatabaseConfig
	RabbitMQ RabbitMQConfig
}

type DatabaseConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	Database string
}

type RabbitMQConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	VHost    string
}

func LoadConfig(path string) (*Config, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("Couldnt open the file for the configuration: %w", err)
	}
	defer file.Close()

	cfg := &Config{}
	scanner := bufio.NewScanner(file)

	var section string
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		if strings.HasSuffix(line, ":") {
			section = strings.TrimSuffix(line, ":")
			continue
		}

		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		value = strings.Trim(value, `"'`)

		switch section {
		case "database":
			switch key {
			case "host":
				cfg.Database.Host = value
			case "port":
				cfg.Database.Port, _ = strconv.Atoi(value)
			case "user":
				cfg.Database.User = value
			case "password":
				cfg.Database.Password = value
			case "database":
				cfg.Database.Database = value
			}
		case "rabbitmq":
			switch key {
			case "host":
				cfg.RabbitMQ.Host = value
			case "port":
				cfg.RabbitMQ.Port, _ = strconv.Atoi(value)
			case "user":
				cfg.RabbitMQ.User = value
			case "password":
				cfg.RabbitMQ.Password = value
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("Error reading config.yaml: %w", err)
	}

	return cfg, nil
}
