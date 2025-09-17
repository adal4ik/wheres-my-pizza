package kitchen

import (
	"context"
	"time"
)

type Config struct {
	WorkerName string
	OrderTypes string
	Heartbeat  time.Duration
	Prefetch   int
}

func Run(ctx context.Context, cfg Config) error {
	// TODO: регистрация воркера + consume очереди + обработка статусов
	<-ctx.Done()
	return nil
}
