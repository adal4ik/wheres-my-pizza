package service

import (
	"wheres-my-pizza/internal/microservices/tracker/repository"
)

type Service struct {
	repo repository.TrackerRepoInterface
}

func NewService(repo repository.TrackerRepoInterface) *Service {
	return &Service{repo: repo}
}
