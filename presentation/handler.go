package presentation

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"microservicio-db-go/domain"
)

// DocumentUseCase es la costura sobre el caso de uso; satisfecha por
// *application.DocumentUseCase.
type DocumentUseCase interface {
	CreateDocument(ctx context.Context, data *domain.DocumentCreate) (*domain.DocumentResponse, error)
	ListDocuments(ctx context.Context, query domain.PageQuery) ([]*domain.DocumentResponse, error)
	GetDocument(ctx context.Context, docID string) (*domain.DocumentResponse, error)
	UpdateDocument(ctx context.Context, docID string, update *domain.DocumentUpdate) (*domain.DocumentResponse, error)
	DeleteDocument(ctx context.Context, docID string) error
	ExistsByChecksum(ctx context.Context, checksum string) (bool, error)
}

// HealthUseCase reporta la salud de dependencias; satisfecha por
// *application.HealthUseCase.
type HealthUseCase interface {
	IsHealthy(ctx context.Context) bool
}

// Handler conecta la capa HTTP Gin con la capa de aplicación.
// Solo parsea entrada HTTP, aplica límites y formatea respuestas RFC 9457.
type Handler struct {
	docUC       DocumentUseCase
	healthUC    HealthUseCase
	cfgProvider ConfigProvider
	inflight    chan struct{}
}

// defaultMaxInFlight limita subidas simultáneas bufferizadas cuando la Config
// inyectada deja MaxInFlight sin establecer.
const defaultMaxInFlight = 32

func NewHandler(docUC DocumentUseCase, healthUC HealthUseCase, provider ConfigProvider) *Handler {
	cfg := provider.Get()
	if cfg.MaxInFlight <= 0 {
		cfg.MaxInFlight = defaultMaxInFlight
	}
	return &Handler{
		docUC:       docUC,
		healthUC:    healthUC,
		cfgProvider: provider,
		inflight:    make(chan struct{}, cfg.MaxInFlight),
	}
}

// currentConfig returns the current config snapshot (supports hot-reload).
func (h *Handler) currentConfig() Config {
	return h.cfgProvider.Get()
}

// CreateDocument maneja POST /api/v1/documents
func (h *Handler) CreateDocument(c *gin.Context) {
	instance := c.Request.URL.Path
	cfg := h.currentConfig()

	// Control de admisión: falla rápido si hay demasiadas subidas en vuelo
	select {
	case h.inflight <- struct{}{}:
		defer func() { <-h.inflight }()
	default:
		writeProblem(c, cfg.ErrBaseURL, problemTypeBusy, instance)
		return
	}

	// Límite de tamaño de request
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, cfg.MaxUploadBytes)

	var req struct {
		Filename      string `json:"filename" binding:"required"`
		TextContent   string `json:"text_content"`
		Checksum      string `json:"checksum" binding:"required"`
		FileSizeBytes int64  `json:"file_size_bytes" binding:"required,gt=0"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		if isTooLarge(err) {
			writeProblem(c, cfg.ErrBaseURL, problemTypeTooLarge, instance)
			return
		}
		writeProblemWithDetail(c, cfg.ErrBaseURL, "invalid-document-id", instance, "Cuerpo de petición inválido: "+err.Error())
		return
	}

	// M2: una extracción vacía (PDF sólo imagen) es una entrada válida del
	// extractor, así que se rechaza explícitamente con 422 en vez de un 400
	// genérico por `required`.
	if req.TextContent == "" {
		writeProblem(c, cfg.ErrBaseURL, problemTypeEmptyExtraction, instance)
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), cfg.RequestTimeout)
	defer cancel()

	data := &domain.DocumentCreate{
		Filename:       req.Filename,
		TextContent:    req.TextContent,
		Checksum:       req.Checksum,
		FileSizeBytes:  req.FileSizeBytes,
		CreatedAt:      time.Now().UTC(),
	}

	doc, err := h.docUC.CreateDocument(ctx, data)
	if err != nil {
		h.writeDomainError(c, err, instance)
		return
	}

	c.JSON(http.StatusCreated, doc)
}

// ListDocuments maneja GET /api/v1/documents
func (h *Handler) ListDocuments(c *gin.Context) {
	cfg := h.currentConfig()
	var query domain.PageQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		writeProblemWithDetail(c, cfg.ErrBaseURL, "invalid-document-id", c.Request.URL.Path, "Parámetros de consulta inválidos: "+err.Error())
		return
	}
	// M1: aplica el default de `limit` cuando el cliente lo omite.
	query.Normalize()

	ctx, cancel := context.WithTimeout(c.Request.Context(), cfg.RequestTimeout)
	defer cancel()

	docs, err := h.docUC.ListDocuments(ctx, query)
	if err != nil {
		h.writeDomainError(c, err, c.Request.URL.Path)
		return
	}

	if docs == nil {
		docs = []*domain.DocumentResponse{}
	}
	c.JSON(http.StatusOK, docs)
}

// ExistsByChecksum maneja GET /api/v1/documents/checksum/:checksum.
// Devuelve 200 si existe un documento con ese checksum, 404 si no.
func (h *Handler) ExistsByChecksum(c *gin.Context) {
	cfg := h.currentConfig()
	checksum := c.Param("checksum")
	instance := c.Request.URL.Path

	ctx, cancel := context.WithTimeout(c.Request.Context(), cfg.RequestTimeout)
	defer cancel()

	exists, err := h.docUC.ExistsByChecksum(ctx, checksum)
	if err != nil {
		h.writeDomainError(c, err, instance)
		return
	}
	if !exists {
		writeProblem(c, cfg.ErrBaseURL, "document-not-found", instance)
		return
	}
	c.JSON(http.StatusOK, gin.H{"checksum": checksum, "exists": true})
}

// GetDocument maneja GET /api/v1/documents/:id
func (h *Handler) GetDocument(c *gin.Context) {
	cfg := h.currentConfig()
	docID := c.Param("id")
	instance := c.Request.URL.Path

	ctx, cancel := context.WithTimeout(c.Request.Context(), cfg.RequestTimeout)
	defer cancel()

	doc, err := h.docUC.GetDocument(ctx, docID)
	if err != nil {
		h.writeDomainError(c, err, instance)
		return
	}

	c.JSON(http.StatusOK, doc)
}

// UpdateDocument maneja PATCH /api/v1/documents/:id
func (h *Handler) UpdateDocument(c *gin.Context) {
	cfg := h.currentConfig()
	docID := c.Param("id")
	instance := c.Request.URL.Path

	var update domain.DocumentUpdate
	if err := c.ShouldBindJSON(&update); err != nil {
		writeProblemWithDetail(c, cfg.ErrBaseURL, "invalid-document-id", instance, "Cuerpo de petición inválido: "+err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), cfg.RequestTimeout)
	defer cancel()

	doc, err := h.docUC.UpdateDocument(ctx, docID, &update)
	if err != nil {
		h.writeDomainError(c, err, instance)
		return
	}

	c.JSON(http.StatusOK, doc)
}

// UpdateSummary maneja PATCH /api/v1/documents/:id/summary
// Actualiza únicamente el campo summary del documento (usado por microservicio-resume).
func (h *Handler) UpdateSummary(c *gin.Context) {
	cfg := h.currentConfig()
	docID := c.Param("id")
	instance := c.Request.URL.Path

	var req struct {
		Summary string `json:"summary" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		writeProblemWithDetail(c, cfg.ErrBaseURL, "invalid-document-id", instance, "Cuerpo de petición inválido: "+err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), cfg.RequestTimeout)
	defer cancel()

	doc, err := h.docUC.UpdateSummary(ctx, docID, req.Summary)
	if err != nil {
		h.writeDomainError(c, err, instance)
		return
	}

	c.JSON(http.StatusOK, doc)
}

// DeleteDocument maneja DELETE /api/v1/documents/:id
func (h *Handler) DeleteDocument(c *gin.Context) {
	cfg := h.currentConfig()
	docID := c.Param("id")
	instance := c.Request.URL.Path

	ctx, cancel := context.WithTimeout(c.Request.Context(), cfg.RequestTimeout)
	defer cancel()

	err := h.docUC.DeleteDocument(ctx, docID)
	if err != nil {
		h.writeDomainError(c, err, instance)
		return
	}

	c.Status(http.StatusNoContent)
}

// HealthLiveness maneja GET /health/live (liveness probe)
func (h *Handler) HealthLiveness(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// HealthReadiness maneja GET /health (readiness probe)
func (h *Handler) HealthReadiness(c *gin.Context) {
	cfg := h.currentConfig()
	instance := c.Request.URL.Path
	ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
	defer cancel()

	if h.healthUC.IsHealthy(ctx) {
		c.JSON(http.StatusOK, gin.H{
			"status":   "ok",
			"app":      "ok",
			"database": "ok",
		})
		return
	}

	writeProblem(c, cfg.ErrBaseURL, problemTypeServiceUnavailable, instance)
}

func (h *Handler) writeDomainError(c *gin.Context, err error, instance string) {
	cfg := h.currentConfig()
	// SlugFor mapea errores internos/desconocidos a "server-error"; el registry
	// de problemas nunca filtra detalles internos.
	writeProblem(c, cfg.ErrBaseURL, domain.SlugFor(err), instance)
}

// isTooLarge detecta el corte de http.MaxBytesReader.
func isTooLarge(err error) bool {
	var maxErr *http.MaxBytesError
	if errors.As(err, &maxErr) {
		return true
	}
	return err != nil && strings.Contains(err.Error(), "message too large")
}