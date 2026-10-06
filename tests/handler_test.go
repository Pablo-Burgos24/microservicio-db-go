package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"microservicio-db-go/application"
	"microservicio-db-go/domain"
)

// MockDocumentUseCase es un mock de DocumentUseCase para tests
type MockDocumentUseCase struct {
	mock.Mock
}

func (m *MockDocumentUseCase) CreateDocument(ctx context.Context, data *domain.DocumentCreate) (*domain.DocumentResponse, error) {
	args := m.Called(ctx, data)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.DocumentResponse), args.Error(1)
}

func (m *MockDocumentUseCase) ListDocuments(ctx context.Context, query domain.PageQuery) ([]*domain.DocumentResponse, error) {
	args := m.Called(ctx, query)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.DocumentResponse), args.Error(1)
}

func (m *MockDocumentUseCase) GetDocument(ctx context.Context, docID string) (*domain.DocumentResponse, error) {
	args := m.Called(ctx, docID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.DocumentResponse), args.Error(1)
}

func (m *MockDocumentUseCase) UpdateDocument(ctx context.Context, docID string, update *domain.DocumentUpdate) (*domain.DocumentResponse, error) {
	args := m.Called(ctx, docID, update)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.DocumentResponse), args.Error(1)
}

func (m *MockDocumentUseCase) DeleteDocument(ctx context.Context, docID string) error {
	args := m.Called(ctx, docID)
	return args.Error(0)
}

// MockHealthUseCase es un mock de HealthUseCase para tests
type MockHealthUseCase struct {
	mock.Mock
}

func (m *MockHealthUseCase) IsHealthy(ctx context.Context) bool {
	args := m.Called(ctx)
	return args.Bool(0)
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

func setupRouter(docUC DocumentUseCase, healthUC HealthUseCase) *gin.Engine {
	gin.SetMode(gin.TestMode)
	return NewRouter(docUC, healthUC, Config{
		ErrBaseURL:        "https://errors.example.com",
		MaxUploadBytes:    25 * 1024 * 1024,
		RequestTimeout:    10 * time.Second,
		MaxInFlight:       32,
	})
}

func TestHandler_CreateDocument_Success(t *testing.T) {
	mockDocUC := new(MockDocumentUseCase)
	mockHealthUC := new(MockHealthUseCase)

	router := setupRouter(mockDocUC, mockHealthUC)

	reqBody := map[string]any{
		"filename":       "test.pdf",
		"text_content":   "contenido",
		"checksum":       "abc123",
		"file_size_bytes": 1024,
	}
	bodyBytes, _ := json.Marshal(reqBody)

	mockDocUC.On("CreateDocument", mock.Anything, mock.MatchedBy(func(d *domain.DocumentCreate) bool {
		return d.Filename == "test.pdf" && d.Checksum == "abc123"
	})).Return(makeResponse(), nil)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/documents", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	var resp domain.DocumentResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, "test.pdf", resp.Filename)
	assert.Equal(t, "abc123", resp.Checksum)

	mockDocUC.AssertExpectations(t)
}

func TestHandler_CreateDocument_DuplicateChecksum(t *testing.T) {
	mockDocUC := new(MockDocumentUseCase)
	mockHealthUC := new(MockHealthUseCase)

	router := setupRouter(mockDocUC, mockHealthUC)

	reqBody := map[string]any{
		"filename":       "test.pdf",
		"text_content":   "contenido",
		"checksum":       "abc123",
		"file_size_bytes": 1024,
	}
	bodyBytes, _ := json.Marshal(reqBody)

	mockDocUC.On("CreateDocument", mock.Anything, mock.Anything).Return(
		nil, domain.NewDocumentAlreadyExistsError("abc123"))

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/documents", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusConflict, w.Code)
	assert.Equal(t, "application/problem+json", w.Header().Get("Content-Type"))

	var prob problem
	err := json.Unmarshal(w.Body.Bytes(), &prob)
	assert.NoError(t, err)
	assert.Equal(t, "document-already-exists", prob.Type)
	assert.Equal(t, http.StatusConflict, prob.Status)

	mockDocUC.AssertExpectations(t)
}

func TestHandler_CreateDocument_InvalidBody(t *testing.T) {
	mockDocUC := new(MockDocumentUseCase)
	mockHealthUC := new(MockHealthUseCase)

	router := setupRouter(mockDocUC, mockHealthUC)

	// Body inválido (falta campos requeridos)
	reqBody := map[string]any{
		"filename": "test.pdf",
	}
	bodyBytes, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/documents", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Equal(t, "application/problem+json", w.Header().Get("Content-Type"))

	mockDocUC.AssertNotCalled(t, "CreateDocument")
}

func TestHandler_ListDocuments(t *testing.T) {
	mockDocUC := new(MockDocumentUseCase)
	mockHealthUC := new(MockHealthUseCase)

	router := setupRouter(mockDocUC, mockHealthUC)

	expected := []*domain.DocumentResponse{makeResponse()}
	mockDocUC.On("ListDocuments", mock.Anything, domain.PageQuery{Skip: 0, Limit: 100}).Return(expected, nil)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/documents", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp []domain.DocumentResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Len(t, resp, 1)

	mockDocUC.AssertExpectations(t)
}

func TestHandler_GetDocument_Success(t *testing.T) {
	mockDocUC := new(MockDocumentUseCase)
	mockHealthUC := new(MockHealthUseCase)

	router := setupRouter(mockDocUC, mockHealthUC)

	docID := "60d5ecb8b392d70008051234"
	mockDocUC.On("GetDocument", mock.Anything, docID).Return(makeResponse(), nil)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/documents/"+docID, nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp domain.DocumentResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, docID, resp.ID)

	mockDocUC.AssertExpectations(t)
}

func TestHandler_GetDocument_NotFound(t *testing.T) {
	mockDocUC := new(MockDocumentUseCase)
	mockHealthUC := new(MockHealthUseCase)

	router := setupRouter(mockDocUC, mockHealthUC)

	docID := "60d5ecb8b392d70008051234"
	mockDocUC.On("GetDocument", mock.Anything, docID).Return(
		nil, domain.NewDocumentNotFoundError(docID))

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/documents/"+docID, nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Equal(t, "application/problem+json", w.Header().Get("Content-Type"))

	var prob problem
	err := json.Unmarshal(w.Body.Bytes(), &prob)
	assert.NoError(t, err)
	assert.Equal(t, "document-not-found", prob.Type)

	mockDocUC.AssertExpectations(t)
}

func TestHandler_GetDocument_InvalidID(t *testing.T) {
	mockDocUC := new(MockDocumentUseCase)
	mockHealthUC := new(MockHealthUseCase)

	router := setupRouter(mockDocUC, mockHealthUC)

	docID := "invalid-id"
	mockDocUC.On("GetDocument", mock.Anything, docID).Return(
		nil, domain.NewInvalidDocumentIdError(docID))

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/documents/"+docID, nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Equal(t, "application/problem+json", w.Header().Get("Content-Type"))

	var prob problem
	err := json.Unmarshal(w.Body.Bytes(), &prob)
	assert.NoError(t, err)
	assert.Equal(t, "invalid-document-id", prob.Type)

	mockDocUC.AssertExpectations(t)
}

func TestHandler_UpdateDocument_Success(t *testing.T) {
	mockDocUC := new(MockDocumentUseCase)
	mockHealthUC := new(MockHealthUseCase)

	router := setupRouter(mockDocUC, mockHealthUC)

	docID := "60d5ecb8b392d70008051234"
	reqBody := map[string]any{
		"filename": "nuevo.pdf",
	}
	bodyBytes, _ := json.Marshal(reqBody)

	updated := makeResponse()
	updated.Filename = "nuevo.pdf"
	mockDocUC.On("UpdateDocument", mock.Anything, docID, mock.MatchedBy(func(u *domain.DocumentUpdate) bool {
		return u.Filename != nil && *u.Filename == "nuevo.pdf"
	})).Return(updated, nil)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPatch, "/api/v1/documents/"+docID, bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp domain.DocumentResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, "nuevo.pdf", resp.Filename)

	mockDocUC.AssertExpectations(t)
}

func TestHandler_UpdateDocument_EmptyUpdate(t *testing.T) {
	mockDocUC := new(MockDocumentUseCase)
	mockHealthUC := new(MockHealthUseCase)

	router := setupRouter(mockDocUC, mockHealthUC)

	docID := "60d5ecb8b392d70008051234"
	reqBody := map[string]any{}
	bodyBytes, _ := json.Marshal(reqBody)

	expected := makeResponse()
	mockDocUC.On("UpdateDocument", mock.Anything, docID, mock.MatchedBy(func(u *domain.DocumentUpdate) bool {
		return u.Filename == nil && u.TextContent == nil
	})).Return(expected, nil)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPatch, "/api/v1/documents/"+docID, bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp domain.DocumentResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, expected.ID, resp.ID)

	mockDocUC.AssertExpectations(t)
}

func TestHandler_DeleteDocument_Success(t *testing.T) {
	mockDocUC := new(MockDocumentUseCase)
	mockHealthUC := new(MockHealthUseCase)

	router := setupRouter(mockDocUC, mockHealthUC)

	docID := "60d5ecb8b392d70008051234"
	mockDocUC.On("DeleteDocument", mock.Anything, docID).Return(nil)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodDelete, "/api/v1/documents/"+docID, nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
	assert.Empty(t, w.Body.String())

	mockDocUC.AssertExpectations(t)
}

func TestHandler_DeleteDocument_NotFound(t *testing.T) {
	mockDocUC := new(MockDocumentUseCase)
	mockHealthUC := new(MockHealthUseCase)

	router := setupRouter(mockDocUC, mockHealthUC)

	docID := "60d5ecb8b392d70008051234"
	mockDocUC.On("DeleteDocument", mock.Anything, docID).Return(
		domain.NewDocumentNotFoundError(docID))

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodDelete, "/api/v1/documents/"+docID, nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Equal(t, "application/problem+json", w.Header().Get("Content-Type"))

	mockDocUC.AssertExpectations(t)
}

func TestHandler_HealthLiveness(t *testing.T) {
	mockDocUC := new(MockDocumentUseCase)
	mockHealthUC := new(MockHealthUseCase)

	router := setupRouter(mockDocUC, mockHealthUC)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/health/live", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, "ok", resp["status"])
}

func TestHandler_HealthReadiness_Healthy(t *testing.T) {
	mockDocUC := new(MockDocumentUseCase)
	mockHealthUC := new(MockHealthUseCase)

	router := setupRouter(mockDocUC, mockHealthUC)

	mockHealthUC.On("IsHealthy", mock.Anything).Return(true)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/health", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, "ok", resp["status"])
	assert.Equal(t, "ok", resp["app"])
	assert.Equal(t, "ok", resp["database"])

	mockHealthUC.AssertExpectations(t)
}

func TestHandler_HealthReadiness_Unhealthy(t *testing.T) {
	mockDocUC := new(MockDocumentUseCase)
	mockHealthUC := new(MockHealthUseCase)

	router := setupRouter(mockDocUC, mockHealthUC)

	mockHealthUC.On("IsHealthy", mock.Anything).Return(false)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/health", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	assert.Equal(t, "application/problem+json", w.Header().Get("Content-Type"))

	var prob problem
	err := json.Unmarshal(w.Body.Bytes(), &prob)
	assert.NoError(t, err)
	assert.Equal(t, "service-unavailable", prob.Type)

	mockHealthUC.AssertExpectations(t)
}

func TestHandler_TooLargeRequest(t *testing.T) {
	mockDocUC := new(MockDocumentUseCase)
	mockHealthUC := new(MockHealthUseCase)

	router := setupRouter(mockDocUC, mockHealthUC)

	// Request body muy grande (simulando MaxBytesReader)
	largeBody := bytes.Repeat([]byte("x"), 30*1024*1024) // 30MB > 25MB default

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/documents", bytes.NewReader(largeBody))
	req.Header.Set("Content-Type", "application/json")
	req.ContentLength = int64(len(largeBody))
	router.ServeHTTP(w, req)

	// Debe responder 413 Request Entity Too Large
	assert.Equal(t, http.StatusRequestEntityTooLarge, w.Code)
	assert.Equal(t, "application/problem+json", w.Header().Get("Content-Type"))

	var prob problem
	err := json.Unmarshal(w.Body.Bytes(), &prob)
	assert.NoError(t, err)
	assert.Equal(t, "too-large", prob.Type)
}