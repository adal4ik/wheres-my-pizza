package httpx

import (
	"context"
	"net/http"
	"time"
)

type Server struct{ *http.Server }

func New(addr string, h http.Handler) *Server { return &Server{Server: &http.Server{Addr: addr, Handler: h}} }

func (s *Server) Run(ctx context.Context) error {
	errCh := make(chan error, 1)
	go func() { errCh <- s.ListenAndServe() }()
	select {
	case <-ctx.Done():
		ctx2, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = s.Shutdown(ctx2)
		return nil
	case err := <-errCh:
		if err == http.ErrServerClosed {
			return nil
		}
		return err
	}
}
