package presentation

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"microservicio-db-go/domain"
)

type emptyListDocumentUseCase struct{}

func (emptyListDocumentUseCase) CreateDocument(context.Context, *domain.DocumentCreate) (*domain.DocumentResponse, error) {
	return nil, nil
}

func (emptyListDocumentUseCase) ListDocuments(context.Context, domain.PageQuery) ([]*domain.DocumentResponse, error) {
	return nil, nil
}

func (emptyListDocumentUseCase) GetDocument(context.Context, string) (*domain.DocumentResponse, error) {
	return nil, nil
}

func (emptyListDocumentUseCase) UpdateDocument(context.Context, string, *domain.DocumentUpdate) (*domain.DocumentResponse, error) {
	return nil, nil
}

func (emptyListDocumentUseCase) DeleteDocument(context.Context, string) error {
	return nil
}

type unhealthyStub struct{}

func (unhealthyStub) IsHealthy(context.Context) bool { return false }

func TestListDocumentsReturnsEmptyArray(t *testing.T) {
	router := NewRouter(emptyListDocumentUseCase{}, unhealthyStub{}, Config{})
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/documents?skip=0&limit=100", nil)

	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, response.Code)
	}
	if response.Body.String() != "[]" {
		t.Fatalf("expected empty JSON array, got %q", response.Body.String())
	}
}