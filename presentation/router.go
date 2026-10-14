package presentation

import (
	"github.com/gin-gonic/gin"
)

// NewRouter conecta las rutas HTTP con middleware de recovery que emite
// documentos RFC 9457 en pánicos en lugar de respuestas internas crudas.
func NewRouter(docUC DocumentUseCase, healthUC HealthUseCase, cfg Config) *gin.Engine {
	r := gin.New()
	r.Use(gin.RecoveryWithWriter(nil, func(c *gin.Context, recovered any) {
		c.Abort()
		writeProblem(c, cfg.ErrBaseURL, serverErrorSlug, c.Request.URL.Path)
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
		// Ruta estática: debe registrarse junto a `/:id`; Gin prioriza el
		// segmento estático sobre el parámetro.
		api.GET("/checksum/:checksum", h.ExistsByChecksum)
		api.GET("/:id", h.GetDocument)
		api.PATCH("/:id", h.UpdateDocument)
		api.DELETE("/:id", h.DeleteDocument)
	}

	return r
}