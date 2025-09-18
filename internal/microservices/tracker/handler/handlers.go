package handler

import "wheres-my-pizza/internal/microservices/tracker/service"

type Handler struct {
	TrackerHandler *TrackerHandler
}

func New(svc service.TrackerServiceInterface) *Handler {
	return &Handler{
		TrackerHandler: NewTrackerHandler(svc),
	}
}
