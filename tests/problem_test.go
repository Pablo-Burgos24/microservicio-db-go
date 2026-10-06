package tests

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestWriteProblem(t *testing.T) {
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodGet, "/test", nil)

	writeProblem(c, "https://errors.example.com", "document-not-found", "/test")

	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Equal(t, "application/problem+json", w.Header().Get("Content-Type"))

	var prob problem
	err := json.Unmarshal(w.Body.Bytes(), &prob)
	assert.NoError(t, err)
	assert.Equal(t, "https://errors.example.com/document-not-found", prob.Type)
	assert.Equal(t, "Documento no encontrado", prob.Title)
	assert.Equal(t, http.StatusNotFound, prob.Status)
	assert.Equal(t, "El documento solicitado no existe.", prob.Detail)
	assert.Equal(t, "/test", prob.Instance)
}

func TestWriteProblem_UnknownSlugDefaultsToInternal(t *testing.T) {
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodGet, "/test", nil)

	writeProblem(c, "https://errors.example.com", "unknown-slug", "/test")

	assert.Equal(t, http.StatusInternalServerError, w.Code)

	var prob problem
	err := json.Unmarshal(w.Body.Bytes(), &prob)
	assert.NoError(t, err)
	assert.Equal(t, "https://errors.example.com/server-error", prob.Type)
	assert.Equal(t, "Error Interno del Servidor", prob.Title)
	assert.Equal(t, http.StatusInternalServerError, prob.Status)
}

func TestWriteProblemWithDetail(t *testing.T) {
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodGet, "/test", nil)

	writeProblemWithDetail(c, "https://errors.example.com", "invalid-document-id", "/test", "Detalle personalizado")

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var prob problem
	err := json.Unmarshal(w.Body.Bytes(), &prob)
	assert.NoError(t, err)
	assert.Equal(t, "Detalle personalizado", prob.Detail)
}

func TestProblemRegistry_AllSlugsHaveDefinitions(t *testing.T) {
	requiredSlugs := []string{
		"document-not-found",
		"invalid-document-id",
		"document-already-exists",
		"server-error",
		"too-large",
		"service-unavailable",
		"busy",
	}

	for _, slug := range requiredSlugs {
		def, ok := problemRegistry[slug]
		assert.True(t, ok, "slug %q debe estar en registry", slug)
		assert.Greater(t, def.status, 0, "slug %q debe tener status", slug)
		assert.NotEmpty(t, def.title, "slug %q debe tener title", slug)
		assert.NotEmpty(t, def.detail, "slug %q debe tener detail", slug)
	}
}