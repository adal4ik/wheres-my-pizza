package db

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Conn struct{ *pgxpool.Pool }

func Connect(ctx context.Context, host string, port int, user, pass, name string) (*Conn, error) {
	dsn := fmt.Sprintf("postgres://%s:%s@%s:%d/%s", user, pass, host, port, name)
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return &Conn{Pool: pool}, nil
}

func (c *Conn) Close() { if c != nil && c.Pool != nil { c.Pool.Close() } }
