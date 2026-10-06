package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config es la configuración validada en tiempo de ejecución para el servicio.
type Config struct {
	Port              string
	MongoURI          string
	MongoDB           string
	MongoCollection   string
	MaxUploadBytes    int64
	MaxInFlight       int
	RequestTimeout    time.Duration
	ShutdownTimeout   time.Duration
	ErrBaseURL        string
}

const (
	defaultPort            = "8080"
	defaultDB              = "pdf_db"
	defaultCollection      = "extracted_texts"
	defaultMaxUploadBytes  = 25 * 1024 * 1024 // 25 MB
	defaultMaxInFlight     = 32
	defaultRequestTimeout  = 10 * time.Second
	defaultShutdownTimeout = 15 * time.Second
	defaultErrBaseURL      = "https://errors.example.com"
)

// Load lee y valida el entorno. Falla con un mensaje accionable nombrando la variable infractora.
func Load() (*Config, error) {
	cfg := &Config{
		Port:             getEnv("PORT", defaultPort),
		MongoURI:         os.Getenv("MONGODB_URI"),
		MongoDB:          getEnv("MONGODB_DB", defaultDB),
		MongoCollection:  getEnv("MONGODB_COLLECTION", defaultCollection),
		MaxUploadBytes:   defaultMaxUploadBytes,
		MaxInFlight:      defaultMaxInFlight,
		RequestTimeout:   defaultRequestTimeout,
		ShutdownTimeout:  defaultShutdownTimeout,
		ErrBaseURL:       getEnv("ERR_BASE_URL", defaultErrBaseURL),
	}

	if cfg.MongoURI == "" {
		return nil, fmt.Errorf("MONGODB_URI es requerida")
	}

	if raw := os.Getenv("MAX_UPLOAD_MB"); raw != "" {
		v, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || v <= 0 {
			return nil, fmt.Errorf("MAX_UPLOAD_MB debe ser un entero positivo de megabytes (ej. 25), recibido %q", raw)
		}
		cfg.MaxUploadBytes = v * 1024 * 1024
	}

	if raw := os.Getenv("MAX_IN_FLIGHT"); raw != "" {
		v, err := strconv.Atoi(raw)
		if err != nil || v <= 0 {
			return nil, fmt.Errorf("MAX_IN_FLIGHT debe ser un entero positivo, recibido %q", raw)
		}
		cfg.MaxInFlight = v
	}

	if raw := os.Getenv("REQUEST_TIMEOUT"); raw != "" {
		v, err := time.ParseDuration(raw)
		if err != nil || v <= 0 {
			return nil, fmt.Errorf("REQUEST_TIMEOUT debe ser una duración positiva (ej. 10s), recibido %q", raw)
		}
		cfg.RequestTimeout = v
	}

	if raw := os.Getenv("SHUTDOWN_TIMEOUT"); raw != "" {
		v, err := time.ParseDuration(raw)
		if err != nil || v <= 0 {
			return nil, fmt.Errorf("SHUTDOWN_TIMEOUT debe ser una duración positiva (ej. 15s), recibido %q", raw)
		}
		cfg.ShutdownTimeout = v
	}

	if raw := os.Getenv("ERR_BASE_URL"); raw != "" {
		if len(raw) < 8 || (raw[:7] != "http://" && raw[:8] != "https://") {
			return nil, fmt.Errorf("ERR_BASE_URL debe ser una URI http(s), recibido %q", raw)
		}
		cfg.ErrBaseURL = raw
	}

	if _, err := strconv.Atoi(cfg.Port); err != nil {
		return nil, fmt.Errorf("PORT debe ser numérico, recibido %q", cfg.Port)
	}

	return cfg, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}