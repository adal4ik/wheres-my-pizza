package tracker

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"wheres-my-pizza/internal/microservices/tracker/handler"
	"wheres-my-pizza/internal/microservices/tracker/repository"
	"wheres-my-pizza/internal/microservices/tracker/service"
)

// Start запускает HTTP-сервер трекинга на addr, используя уже открытый db.
// Блокирует горутину до shutdown. Возвращает ошибку только при фатальном старте/стопе.
func Start(addr string, db *sql.DB) error {
	fmt.Println("asd")
	repo := repository.NewTrackerRepo(db)
	svc := service.NewTrackerService(repo)
	h := &handler.Handler{TrackerHandler: handler.NewTrackerHandler(svc)}

	mux := handler.Router(h)

	srv := &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// run
	errCh := make(chan error, 1)
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	// graceful shutdown по сигналам
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-stop:
		_ = sig // логируй при желании
	case err := <-errCh:
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)

	return nil
}
