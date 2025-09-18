package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
	"wheres-my-pizza/internal/microservices/tracker/service"
)

type TrackerHandler struct {
	service service.TrackerServiceInterface
}

func NewTrackerHandler(svc service.TrackerServiceInterface) *TrackerHandler {
	return &TrackerHandler{service: svc}
}

func (h *TrackerHandler) GetStatus(w http.ResponseWriter, r *http.Request) {
	id := param(r, "order_id")
	v, ok, err := h.service.GetOrderView(r.Context(), id)
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "db_error", err.Error())
		return
	}
	if !ok {
		writeProblem(w, http.StatusNotFound, "not_found", "order not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"order_id": v.OrderID, "status": v.Status, "updated_at": v.UpdatedAt,
	})
}

func (h *TrackerHandler) GetTimeline(w http.ResponseWriter, r *http.Request) {
	id := param(r, "order_id")
	limit := atoiDefault(r.URL.Query().Get("limit"), 50)
	offset := atoiDefault(r.URL.Query().Get("offset"), 0)
	events, err := h.service.GetOrderTimeline(r.Context(), id, limit, offset)
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "db_error", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"order_id": id, "events": events})
}

// writeJSON — отдаёт JSON с нужным статусом
func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

// writeProblem — единый формат ошибок (RFC7807 Problem+JSON, упрощённый)
func writeProblem(w http.ResponseWriter, code int, typ, detail string) {
	resp := map[string]any{
		"type":   typ,                   // машинно-читаемый код ошибки
		"title":  http.StatusText(code), // человеко-читаемый заголовок
		"status": code,                  // HTTP статус
		"detail": detail,                // подробности
	}
	writeJSON(w, code, resp)
}

// param — достаёт {order_id} из маршрута (нужен Go 1.22+ и ServeMux с шаблонами)
func param(r *http.Request, key string) string {
	return r.PathValue(key)
}

// atoiDefault — безопасный парсер int с дефолтом
func atoiDefault(s string, d int) int {
	if s == "" {
		return d
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return d
	}
	return n
}
