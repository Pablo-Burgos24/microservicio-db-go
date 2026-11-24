package presentation

import (
	"sync/atomic"

	"github.com/gin-gonic/gin"
)

// ConfigProvider provides access to current configuration (supports hot-reload).
type ConfigProvider interface {
	Get() Config
}

// StaticConfigProvider wraps a static Config.
type StaticConfigProvider struct {
	config atomic.Value // holds Config
}

func NewStaticConfigProvider(cfg Config) *StaticConfigProvider {
	p := &StaticConfigProvider{}
	p.config.Store(cfg)
	return p
}

func (p *StaticConfigProvider) Get() Config {
	return p.config.Load().(Config)
}

func (p *StaticConfigProvider) Update(cfg Config) {
	p.config.Store(cfg)
}

// NewRouter conecta las rutas HTTP con middleware de recovery que emite
// documentos RFC 9457 en pánicos en lugar de respuestas internas crudas.
func NewRouter(docUC DocumentUseCase, healthUC HealthUseCase, provider ConfigProvider) *gin.Engine {
	r := gin.New()
	r.Use(gin.RecoveryWithWriter(nil, func(c *gin.Context, recovered any) {
		c.Abort()
		writeProblem(c, provider.Get().ErrBaseURL, serverErrorSlug, c.Request.URL.Path)
	}))

	h := NewHandler(docUC, healthUC, provider)

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
	api.PATCH("/:id/summary", h.UpdateSummary)
	api.DELETE("/:id", h.DeleteDocument)
	}

	return r
}