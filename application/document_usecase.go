package application

import (
	"context"

	"microservicio-db-go/domain"
)

// DocumentUseCase orquesta la lógica de negocio de documentos sobre sus puertos (DIP).
type DocumentUseCase struct {
	repo domain.DocumentRepository
}

func NewDocumentUseCase(repo domain.DocumentRepository) *DocumentUseCase {
	return &DocumentUseCase{repo: repo}
}

func (uc *DocumentUseCase) EnsureIndexes(ctx context.Context) error {
	return uc.repo.EnsureIndexes(ctx)
}

func (uc *DocumentUseCase) CreateDocument(ctx context.Context, data *domain.DocumentCreate) (*domain.DocumentResponse, error) {
	// Pre-check para evitar la inserción costosa si ya existe.
	// La garantía atómica la da el índice único + DuplicateKeyError del repo
	// (red de seguridad anti-carrera TOCTOU), no este chequeo.
	exists, err := uc.repo.ExistsByChecksum(ctx, data.Checksum)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, domain.NewDocumentAlreadyExistsError(data.Checksum)
	}
	return uc.repo.Create(ctx, data)
}

func (uc *DocumentUseCase) ListDocuments(ctx context.Context, query domain.PageQuery) ([]*domain.DocumentResponse, error) {
	return uc.repo.GetAll(ctx, query)
}

func (uc *DocumentUseCase) GetDocument(ctx context.Context, docID string) (*domain.DocumentResponse, error) {
	return uc.repo.GetByID(ctx, docID)
}

func (uc *DocumentUseCase) UpdateDocument(ctx context.Context, docID string, update *domain.DocumentUpdate) (*domain.DocumentResponse, error) {
	changes := update.GetChanges()
	if len(changes) == 0 {
		return uc.GetDocument(ctx, docID)
	}
	return uc.repo.Update(ctx, docID, changes)
}

func (uc *DocumentUseCase) DeleteDocument(ctx context.Context, docID string) error {
	return uc.repo.Delete(ctx, docID)
}