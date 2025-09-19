package service

import (
	"context"

	"wheres-my-pizza/internal/microservices/tracker/models"
	"wheres-my-pizza/internal/microservices/tracker/repository"
)

type TrackerServiceInterface interface {
	GetOrderView(ctx context.Context, id string) (models.OrderView, bool, error)
	GetOrderTimeline(ctx context.Context, id string) ([]models.OrderEvent, error)
	GetWorkersStatus(ctx context.Context) ([]models.WorkerStatus, error)
}

type TrackerService struct {
	repo repository.TrackerRepoInterface
}

func NewTrackerService(repo repository.TrackerRepoInterface) *TrackerService {
	return &TrackerService{repo: repo}
}

func (s *TrackerService) GetOrderView(ctx context.Context, id string) (models.OrderView, bool, error) {
	return s.repo.GetOrderView(ctx, id)
}

func (s *TrackerService) GetOrderTimeline(ctx context.Context, id string) ([]models.OrderEvent, error) {
	return s.repo.GetOrderTimeline(ctx, id)
}

func (s *TrackerService) GetWorkersStatus(ctx context.Context) ([]models.WorkerStatus, error) {
	return s.repo.ListWorkersStatus(ctx)
}
