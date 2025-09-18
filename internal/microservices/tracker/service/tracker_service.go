package service

import (
	"context"
	"time"

	"wheres-my-pizza/internal/microservices/tracker/models"
	"wheres-my-pizza/internal/microservices/tracker/repository"
)

type TrackerServiceInterface interface {
	Apply(ctx context.Context, ev models.OrderEvent) error
	GetOrderView(ctx context.Context, id string) (models.OrderView, bool, error)
	GetOrderTimeline(ctx context.Context, id string, limit, offset int) ([]models.OrderEvent, error)
}

type TrackerService struct {
	repo repository.TrackerRepoInterface
}

func NewTrackerService(repo repository.TrackerRepoInterface) *TrackerService {
	return &TrackerService{repo: repo}
}

func (s *TrackerService) Apply(ctx context.Context, ev models.OrderEvent) error {
	status := mapEventToStatus(ev.EventType)
	_ = s.repo.AppendEvent(ctx, ev)

	v := models.OrderView{
		OrderID:     ev.OrderID,
		Status:      status,
		UpdatedAt:   time.Now().UTC(),
		LastEventAt: ev.OccurredAt,
	}
	if name, ok := ev.Payload["customer_name"].(string); ok {
		v.CustomerName = name
	}
	return s.repo.UpsertOrderView(ctx, v)
}

func (s *TrackerService) GetOrderView(ctx context.Context, id string) (models.OrderView, bool, error) {
	return s.repo.GetOrderView(ctx, id)
}

func (s *TrackerService) GetOrderTimeline(ctx context.Context, id string, limit, offset int) ([]models.OrderEvent, error) {
	return s.repo.GetOrderTimeline(ctx, id, limit, offset)
}

func mapEventToStatus(t string) models.OrderStatus {
	switch t {
	case "order.created":
		return models.StatusCreated
	case "kitchen.accepted":
		return models.StatusAccepted
	case "kitchen.ready":
		return models.StatusReady
	case "courier.picked_up":
		return models.StatusPickedUp
	case "order.delivered":
		return models.StatusDelivered
	case "order.canceled":
		return models.StatusCanceled
	default:
		return models.StatusCreated
	}
}
