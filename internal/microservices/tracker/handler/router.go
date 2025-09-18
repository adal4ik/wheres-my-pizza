package handler

import "net/http"

// Только эндпоинты из ТЗ
func Router(h *Handler) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/tracking/orders/{order_id}/status", h.TrackerHandler.GetStatus)
	mux.HandleFunc("GET /api/v1/tracking/orders/{order_id}/timeline", h.TrackerHandler.GetTimeline)
	return mux
}
