package handlers

import (
	"encoding/json"
	"net/http"

	"wheres-my-pizza/internal/microservices/order/service"

	dto "wheres-my-pizza/internal/microservices/order/domain/dto"
)

type OrderHandler struct {
	service service.OrderServiceInterface
}

func NewOrderHandler(s service.OrderServiceInterface) *OrderHandler {
	return &OrderHandler{service: s}
}

func (oh *OrderHandler) AddOrder(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req dto.CreateOrderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON body", http.StatusBadRequest)
		return
	}

	// Call service layer
	resp, err := oh.service.AddOrder(req)
	if err != nil {
		http.Error(w, "Failed to create order: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Return JSON response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}
