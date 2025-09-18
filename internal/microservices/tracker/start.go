package tracker

import (
	"context"
	"database/sql"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"wheres-my-pizza/internal/microservices/tracker/handler"
	"wheres-my-pizza/internal/microservices/tracker/repository"
	"wheres-my-pizza/internal/microservices/tracker/service"

	_ "github.com/jackc/pgx/v5/stdlib" // ВАЖНО: stdlib-адаптер
)

func Start() {
	httpAddr := ":8082"
	dsn := "postgres://restaurant_user:restaurant_pass@localhost:5431/restaurant_db?sslmode=disable"

	// ВАЖНО: драйвер "pgx", не "postgres"
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		panic(err)
	}
	if err := db.Ping(); err != nil {
		panic(err)
	}

	repo := repository.NewTrackerRepo(db)
	svc := service.NewTrackerService(repo)
	h := &handler.Handler{TrackerHandler: handler.NewTrackerHandler(svc)}

	mux := handler.Router(h)

	srv := &http.Server{
		Addr:         httpAddr,
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			panic(err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
	_ = db.Close()
}
