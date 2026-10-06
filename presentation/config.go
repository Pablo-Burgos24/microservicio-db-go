package presentation

import "time"

// Config contiene la configuración de la capa de presentación inyectada por
// la raíz de composición.
type Config struct {
	// ErrBaseURL es la URI base para los valores "type" de problemas RFC 9457.
	ErrBaseURL string
	// MaxUploadBytes limita el tamaño aceptado de subida multipart.
	MaxUploadBytes int64
	// RequestTimeout acota una sola request, incluyendo espera en cola.
	RequestTimeout time.Duration
	// MaxInFlight limita subidas simultáneas bufferizadas por handlers.
	// Cero cae a un default seguro (ver Handler).
	MaxInFlight int
}