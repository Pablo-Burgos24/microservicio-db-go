package tests

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"microservicio-db-go/domain"
)

// MockDocumentRepository es un mock de domain.DocumentRepository para tests
type MockDocumentRepository struct {
	mock.Mock
}

func (m *MockDocumentRepository) ExistsByChecksum(ctx context.Context, checksum string) (bool, error) {
	args := m.Called(ctx, checksum)
	return args.Bool(0), args.Error(1)
}

func (m *MockDocumentRepository) Create(ctx context.Context, data *domain.DocumentCreate) (*domain.DocumentResponse, error) {
	args := m.Called(ctx, data)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.DocumentResponse), args.Error(1)
}

func (m *MockDocumentRepository) GetAll(ctx context.Context, query domain.PageQuery) ([]*domain.DocumentResponse, error) {
	args := m.Called(ctx, query)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.DocumentResponse), args.Error(1)
}

func (m *MockDocumentRepository) GetByID(ctx context.Context, docID string) (*domain.DocumentResponse, error) {
	args := m.Called(ctx, docID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.DocumentResponse), args.Error(1)
}

func (m *MockDocumentRepository) Update(ctx context.Context, docID string, changes map[string]any) (*domain.DocumentResponse, error) {
	args := m.Called(ctx, docID, changes)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.DocumentResponse), args.Error(1)
}

func (m *MockDocumentRepository) Delete(ctx context.Context, docID string) error {
	args := m.Called(ctx, docID)
	return args.Error(0)
}

func (m *MockDocumentRepository) EnsureIndexes(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func makeResponse() *domain.DocumentResponse {
	return &domain.DocumentResponse{
		ID:             "60d5ecb8b392d70008051234",
		Filename:       "test.pdf",
		TextContent:    "contenido",
		Checksum:       "abc123",
		FileSizeBytes:  1024,
		CreatedAt:      time.Now().UTC(),
	}
}

func TestDocumentUseCase_CreateDocument(t *testing.T) {
	mockRepo := new(MockDocumentRepository)
	uc := NewDocumentUseCase(mockRepo)

	ctx := context.Background()
	data := &domain.DocumentCreate{
		Filename:       "test.pdf",
		TextContent:    "contenido",
		Checksum:       "abc123",
		FileSizeBytes:  1024,
		CreatedAt:      time.Now().UTC(),
	}

	mockRepo.On("ExistsByChecksum", ctx, "abc123").Return(false, nil)
	mockRepo.On("Create", ctx, data).Return(makeResponse(), nil)

	result, err := uc.CreateDocument(ctx, data)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "test.pdf", result.Filename)
	assert.Equal(t, "abc123", result.Checksum)
	assert.Equal(t, int64(1024), result.FileSizeBytes)

	mockRepo.AssertExpectations(t)
}

func TestDocumentUseCase_CreateDocument_DuplicateChecksum(t *testing.T) {
	mockRepo := new(MockDocumentRepository)
	uc := NewDocumentUseCase(mockRepo)

	ctx := context.Background()
	data := &domain.DocumentCreate{
		Filename:       "test.pdf",
		TextContent:    "contenido",
		Checksum:       "abc123",
		FileSizeBytes:  1024,
		CreatedAt:      time.Now().UTC(),
	}

	// Pre-check encuentra duplicado
	mockRepo.On("ExistsByChecksum", ctx, "abc123").Return(true, nil)

	result, err := uc.CreateDocument(ctx, data)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.True(t, errors.Is(err, domain.ErrDocumentAlreadyExists))
	assert.Equal(t, domain.ErrorCodeDocumentAlreadyExists, err.(*domain.DocumentAlreadyExistsError).Code)

	mockRepo.AssertExpectations(t)
}

func TestDocumentUseCase_CreateDocument_RaceCondition(t *testing.T) {
	mockRepo := new(MockDocumentRepository)
	uc := NewDocumentUseCase(mockRepo)

	ctx := context.Background()
	data := &domain.DocumentCreate{
		Filename:       "test.pdf",
		TextContent:    "contenido",
		Checksum:       "abc123",
		FileSizeBytes:  1024,
		CreatedAt:      time.Now().UTC(),
	}

	// Pre-check pasa, pero create falla por carrera TOCTOU
	mockRepo.On("ExistsByChecksum", ctx, "abc123").Return(false, nil)
	mockRepo.On("Create", ctx, data).Return(nil, domain.NewDocumentAlreadyExistsError("abc123"))

	result, err := uc.CreateDocument(ctx, data)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.True(t, errors.Is(err, domain.ErrDocumentAlreadyExists))

	mockRepo.AssertExpectations(t)
}

func TestDocumentUseCase_ListDocuments(t *testing.T) {
	mockRepo := new(MockDocumentRepository)
	uc := NewDocumentUseCase(mockRepo)

	ctx := context.Background()
	query := domain.PageQuery{Skip: 5, Limit: 25}
	expected := []*domain.DocumentResponse{makeResponse()}

	mockRepo.On("GetAll", ctx, query).Return(expected, nil)

	result, err := uc.ListDocuments(ctx, query)

	assert.NoError(t, err)
	assert.Equal(t, expected, result)

	mockRepo.AssertExpectations(t)
}

func TestDocumentUseCase_GetDocument(t *testing.T) {
	mockRepo := new(MockDocumentRepository)
	uc := NewDocumentUseCase(mockRepo)

	ctx := context.Background()
	docID := "60d5ecb8b392d70008051234"
	expected := makeResponse()

	mockRepo.On("GetByID", ctx, docID).Return(expected, nil)

	result, err := uc.GetDocument(ctx, docID)

	assert.NoError(t, err)
	assert.Equal(t, expected, result)

	mockRepo.AssertExpectations(t)
}

func TestDocumentUseCase_GetDocument_NotFound(t *testing.T) {
	mockRepo := new(MockDocumentRepository)
	uc := NewDocumentUseCase(mockRepo)

	ctx := context.Background()
	docID := "60d5ecb8b392d70008051234"

	mockRepo.On("GetByID", ctx, docID).Return(nil, domain.NewDocumentNotFoundError(docID))

	result, err := uc.GetDocument(ctx, docID)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.True(t, errors.Is(err, domain.ErrDocumentNotFound))

	mockRepo.AssertExpectations(t)
}

func TestDocumentUseCase_UpdateDocument(t *testing.T) {
	mockRepo := new(MockDocumentRepository)
	uc := NewDocumentUseCase(mockRepo)

	ctx := context.Background()
	docID := "60d5ecb8b392d70008051234"
	update := &domain.DocumentUpdate{
		Filename: stringPtr("nuevo.pdf"),
	}
	expected := makeResponse()
	expected.Filename = "nuevo.pdf"

	mockRepo.On("Update", ctx, docID, map[string]any{"filename": "nuevo.pdf"}).Return(expected, nil)

	result, err := uc.UpdateDocument(ctx, docID, update)

	assert.NoError(t, err)
	assert.Equal(t, "nuevo.pdf", result.Filename)

	mockRepo.AssertExpectations(t)
}

func TestDocumentUseCase_UpdateDocument_EmptyUpdate(t *testing.T) {
	mockRepo := new(MockDocumentRepository)
	uc := NewDocumentUseCase(mockRepo)

	ctx := context.Background()
	docID := "60d5ecb8b392d70008051234"
	update := &domain.DocumentUpdate{} // Sin cambios
	expected := makeResponse()

	// Debe llamar a GetByID, no a Update
	mockRepo.On("GetByID", ctx, docID).Return(expected, nil)

	result, err := uc.UpdateDocument(ctx, docID, update)

	assert.NoError(t, err)
	assert.Equal(t, expected, result)

	mockRepo.AssertExpectations(t)
	mockRepo.AssertNotCalled(t, "Update")
}

func TestDocumentUseCase_DeleteDocument(t *testing.T) {
	mockRepo := new(MockDocumentRepository)
	uc := NewDocumentUseCase(mockRepo)

	ctx := context.Background()
	docID := "60d5ecb8b392d70008051234"

	mockRepo.On("Delete", ctx, docID).Return(nil)

	err := uc.DeleteDocument(ctx, docID)

	assert.NoError(t, err)

	mockRepo.AssertExpectations(t)
}

func TestDocumentUseCase_EnsureIndexes(t *testing.T) {
	mockRepo := new(MockDocumentRepository)
	uc := NewDocumentUseCase(mockRepo)

	ctx := context.Background()

	mockRepo.On("EnsureIndexes", ctx).Return(nil)

	err := uc.EnsureIndexes(ctx)

	assert.NoError(t, err)

	mockRepo.AssertExpectations(t)
}

func stringPtr(s string) *string {
	return &s
}