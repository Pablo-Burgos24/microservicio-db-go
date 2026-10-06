package presentation

import (
	"github.com/gin-gonic/gin"

	"microservicio-db-go/domain"
)

// NewRouter conecta las rutas HTTP con middleware de recovery que emite
// documentos RFC 9457 en pánicos en lugar de respuestas internas crudas.
func NewRouter(docUC DocumentUseCase, healthUC HealthUseCase, cfg Config) *gin.Engine {
	r := gin.New()
	r.Use(gin.RecoveryWithWriter(nil, func(c *gin.Context, recovered any) {
		c.Abort()
		writeProblem(c, cfg.ErrBaseURL, domain.ErrorCodeInternal, c.Request.URL.Path)
	}))

	h := NewHandler(docUC, healthUC, cfg)

	// Health endpoints
	r.GET("/health/live", h.HealthLiveness)
	r.GET("/health", h.HealthReadiness)

	// Document endpoints
	api := r.Group("/api/v1/documents")
	{
		api.POST("", h.CreateDocument)
		api.GET("", h.ListDocuments)
		api.GET("/:id", h.GetDocument)
		api.PATCH("/:id", h.UpdateDocument)
		api.DELETE("/:id", h.DeleteDocument)
	}

	return r
}