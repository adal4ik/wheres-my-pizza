package order

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	"restaurant-system/internal/common/httpx"
	"restaurant-system/internal/common/logger"
	"restaurant-system/internal/domain"
	"restaurant-system/internal/repository"
)

func Run(ctx context.Context, port, maxConc int) error {
	lg := logger.New("order-service")
	repo := repository.NewOrdersPG()

	mux := http.NewServeMux()
	mux.HandleFunc("/orders", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var req domain.CreateOrderRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad json", http.StatusBadRequest)
			return
		}
		_, total, _, err := repo.CreateOrderTx(r.Context(), req)
		if err != nil {
			http.Error(w, "db error", http.StatusInternalServerError)
			return
		}
		resp := domain.CreateOrderResponse{OrderNumber: "ORD_19700101_001", Status: "received", TotalAmount: total}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
		lg.Debug("order_received", map[string]any{"total": total})
	})

	srv := httpx.New(":"+strconv.Itoa(port), mux)
	return srv.Run(ctx)
}
