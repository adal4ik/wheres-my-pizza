package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"wheres-my-pizza/internal/config"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func ConnectDB(ctx context.Context, cfg config.DatabaseConfig) (*sql.DB, error) {
	dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.Database)

	const (
		maxRetries = 10
		retryDelay = 2 * time.Second
		pingTTL    = 5 * time.Second
	)

	var db *sql.DB
	var err error

	for i := 1; i <= maxRetries; i++ {
		// открываем соединение
		db, err = sql.Open("pgx", dsn)
		if err != nil {
			// ждём и пробуем снова
			select {
			case <-time.After(retryDelay):
				continue
			case <-ctx.Done():
				return nil, fmt.Errorf("db open canceled: %w", ctx.Err())
			}
		}

		pctx, cancel := context.WithTimeout(ctx, pingTTL)
		err = db.PingContext(pctx)
		cancel()
		if err == nil {
			return db, nil
		}

		_ = db.Close()

		select {
		case <-time.After(retryDelay):
			continue
		case <-ctx.Done():
			return nil, fmt.Errorf("db ping canceled: %w", ctx.Err())
		}
	}

	return nil, fmt.Errorf("database unreachable after %d attempts: %w", maxRetries, err)
}
