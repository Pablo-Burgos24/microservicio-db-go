package presentation

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"microservicio-db-go/domain"
)

// Tipos de problema solo de presentación (detectados antes del caso de uso).
const (
	problemTypeTooLarge           = "too-large"
	problemTypeServiceUnavailable = "service-unavailable"
	problemTypeBusy               = "busy"
)

type problemDefinition struct {
	status int
	title  string
	detail string
}

// problemRegistry mapea slugs estables a su presentación RFC 9457.
// Los strings de detail son fijos y nunca derivados de valores de error internos.
var problemRegistry = map[string]problemDefinition{
	"document-not-found": {
		status: http.StatusNotFound,
		title:  "Documento no encontrado",
		detail: "El documento solicitado no existe.",
	},
	"invalid-document-id": {
		status: http.StatusBadRequest,
		title:  "ID de documento inválido",
		detail: "El identificador proporcionado no tiene un formato válido.",
	},
	"document-already-exists": {
		status: http.StatusConflict,
		title:  "Documento duplicado",
		detail: "El documento ya fue cargado previamente.",
	},
	domain.ErrorCodeInternal: {
		status: http.StatusInternalServerError,
		title:  "Error Interno del Servidor",
		detail: "Ha ocurrido un error inesperado en el servidor. Por favor, intente más tarde.",
	},
	problemTypeTooLarge: {
		status: http.StatusRequestEntityTooLarge,
		title:  "Archivo demasiado grande",
		detail: "El archivo subido supera el tamaño máximo permitido.",
	},
	problemTypeServiceUnavailable: {
		status: http.StatusServiceUnavailable,
		title:  "Servicio No Disponible",
		detail: "Una dependencia requerida (ej. la base de datos) está actualmente inaccesible.",
	},
	problemTypeBusy: {
		status: http.StatusServiceUnavailable,
		title:  "Servicio Ocupado",
		detail: "El servicio está al límite de capacidad; reintente la petición más tarde.",
	},
}

type problem struct {
	Type     string `json:"type"`
	Title    string `json:"title"`
	Status   int    `json:"status"`
	Detail   string `json:"detail"`
	Instance string `json:"instance"`
}

// writeProblem emite un documento RFC 9457 Problem Details para el slug dado.
func writeProblem(c *gin.Context, baseURL, slug, instance string) {
	def, ok := problemRegistry[slug]
	if !ok {
		def = problemRegistry[domain.ErrorCodeInternal]
	}
	writeProblemDoc(c, baseURL, slug, def.title, def.detail, def.status, instance)
}

// writeProblemDoc emite un documento RFC 9457 con una definición explícita.
func writeProblemDoc(c *gin.Context, baseURL, slug, title, detail string, status int, instance string) {
	c.Header("Content-Type", "application/problem+json")
	c.JSON(status, problem{
		Type:     strings.TrimRight(baseURL, "/") + "/" + slug,
		Title:    title,
		Status:   status,
		Detail:   detail,
		Instance: instance,
	})
}

// writeProblemWithDetail emite un problema RFC 9457 permitiendo sobrescribir el detail
// (útil para errores de validación con detalles específicos).
func writeProblemWithDetail(c *gin.Context, baseURL, slug, instance, customDetail string) {
	def, ok := problemRegistry[slug]
	if !ok {
		def = problemRegistry[domain.ErrorCodeInternal]
	}
	c.Header("Content-Type", "application/problem+json")
	c.JSON(def.status, problem{
		Type:     strings.TrimRight(baseURL, "/") + "/" + slug,
		Title:    def.title,
		Status:   def.status,
		Detail:   customDetail,
		Instance: instance,
	})
}