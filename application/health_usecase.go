package application

import (
	"context"

	"microservicio-db-go/domain"
)

// HealthUseCase responde si la base de datos está disponible (readiness probe).
type HealthUseCase struct {
	repo domain.HealthRepository
}

func NewHealthUseCase(repo domain.HealthRepository) *HealthUseCase {
	return &HealthUseCase{repo: repo}
}

func (uc *HealthUseCase) IsHealthy(ctx context.Context) bool {
	return uc.repo.Ping(ctx) == nil
}