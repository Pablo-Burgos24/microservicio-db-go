package domain

import (
	"context"
)

// DocumentRepository es el contrato mínimo que la capa de aplicación exige a su repositorio.
// Aislado de la implementación concreta de MongoDB (DIP).
type DocumentRepository interface {
	ExistsByChecksum(ctx context.Context, checksum string) (bool, error)
	Create(ctx context.Context, data *DocumentCreate) (*DocumentResponse, error)
	GetAll(ctx context.Context, query PageQuery) ([]*DocumentResponse, error)
	GetByID(ctx context.Context, docID string) (*DocumentResponse, error)
	Update(ctx context.Context, docID string, changes map[string]any) (*DocumentResponse, error)
	Delete(ctx context.Context, docID string) error
	EnsureIndexes(ctx context.Context) error
}

// HealthRepository es el contrato de verificación de disponibilidad.
type HealthRepository interface {
	Ping(ctx context.Context) error
}