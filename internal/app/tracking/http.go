package tracking

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	"restaurant-system/internal/common/httpx"
)

func Run(ctx context.Context, port int) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/orders/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"message": "tracking stub"})
	})
	srv := httpx.New(":"+strconv.Itoa(port), mux)
	return srv.Run(ctx)
}
