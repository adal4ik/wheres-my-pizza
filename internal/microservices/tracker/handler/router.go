package handler

import "net/http"

// Только эндпоинты из ТЗ
func Router(h *Handler) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET tracking/orders/{order_id}/status", h.TrackerHandler.GetStatus)
	mux.HandleFunc("GET /tracking/orders/{order_id}/timeline", h.TrackerHandler.GetTimeline)
	mux.HandleFunc("GET /workers/status", h.TrackerHandler.GetWorkersStatus)
	return mux
}
