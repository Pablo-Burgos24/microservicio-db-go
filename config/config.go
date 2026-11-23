package config

import (
	"fmt"
	"os"
	"strconv"
	"sync/atomic"
	"time"

	"microservicio-db-go/consul"
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

	// ConfigWatcher allows hot-reloading of tunable parameters from Consul KV.
	// If nil, falls back to environment variables only.
	ConfigWatcher *consul.ConfigWatcher
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

	// Apply env var overrides with validation (used as fallback when Consul unavailable)
	if err := applyEnvOverrides(cfg); err != nil {
		return nil, err
	}

	if _, err := strconv.Atoi(cfg.Port); err != nil {
		return nil, fmt.Errorf("PORT debe ser numérico, recibido %q", cfg.Port)
	}

	return cfg, nil
}

// LoadWithWatcher loads config and initializes Consul KV watcher for hot-reload.
func LoadWithWatcher(consulAddress, consulToken string) (*Config, *consul.ConfigWatcher, error) {
	cfg, err := Load()
	if err != nil {
		return nil, nil, err
	}

	// Create config watcher for hot-reloadable parameters
	watcher, err := consul.NewConfigWatcher(consul.ConfigWatcherConfig{
		ConsulAddress: consulAddress,
		ConsulToken:   consulToken,
		Prefix:        "config/microservicio-db/",
		OnChange:      func(kv map[string]string) { /* Hot-reload handled via atomic access */ },
	})
	if err != nil {
		// Non-fatal: log and continue without hot-reload
		return cfg, nil, err
	}

	cfg.ConfigWatcher = watcher
	return cfg, watcher, nil
}

// GetMaxUploadBytes returns the current max upload bytes (Consul KV > env > default).
func (c *Config) GetMaxUploadBytes() int64 {
	if c.ConfigWatcher != nil {
		return c.ConfigWatcher.GetInt64("MAX_UPLOAD_MB", "MAX_UPLOAD_MB", defaultMaxUploadBytes/1024/1024) * 1024 * 1024
	}
	return atomic.LoadInt64(&c.MaxUploadBytes)
}

// GetMaxInFlight returns the current max in-flight requests (Consul KV > env > default).
func (c *Config) GetMaxInFlight() int {
	if c.ConfigWatcher != nil {
		return c.ConfigWatcher.GetInt("MAX_IN_FLIGHT", "MAX_IN_FLIGHT", defaultMaxInFlight)
	}
	return c.MaxInFlight
}

// GetRequestTimeout returns the current request timeout (Consul KV > env > default).
func (c *Config) GetRequestTimeout() time.Duration {
	if c.ConfigWatcher != nil {
		return c.ConfigWatcher.GetDuration("REQUEST_TIMEOUT", "REQUEST_TIMEOUT", defaultRequestTimeout)
	}
	return c.RequestTimeout
}

// GetShutdownTimeout returns the current shutdown timeout (Consul KV > env > default).
func (c *Config) GetShutdownTimeout() time.Duration {
	if c.ConfigWatcher != nil {
		return c.ConfigWatcher.GetDuration("SHUTDOWN_TIMEOUT", "SHUTDOWN_TIMEOUT", defaultShutdownTimeout)
	}
	return c.ShutdownTimeout
}

// GetErrBaseURL returns the current error base URL (Consul KV > env > default).
func (c *Config) GetErrBaseURL() string {
	if c.ConfigWatcher != nil {
		return c.ConfigWatcher.GetString("ERR_BASE_URL", "ERR_BASE_URL", defaultErrBaseURL)
	}
	return c.ErrBaseURL
}

// applyEnvOverrides applies environment variable overrides with validation (used as fallback).
func applyEnvOverrides(cfg *Config) error {
	if raw := os.Getenv("MAX_UPLOAD_MB"); raw != "" {
		v, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || v <= 0 {
			return fmt.Errorf("MAX_UPLOAD_MB debe ser un entero positivo de megabytes (ej. 25), recibido %q", raw)
		}
		cfg.MaxUploadBytes = v * 1024 * 1024
	}
	if raw := os.Getenv("MAX_IN_FLIGHT"); raw != "" {
		v, err := strconv.Atoi(raw)
		if err != nil || v <= 0 {
			return fmt.Errorf("MAX_IN_FLIGHT debe ser un entero positivo, recibido %q", raw)
		}
		cfg.MaxInFlight = v
	}
	if raw := os.Getenv("REQUEST_TIMEOUT"); raw != "" {
		v, err := time.ParseDuration(raw)
		if err != nil || v <= 0 {
			return fmt.Errorf("REQUEST_TIMEOUT debe ser una duración positiva (ej. 10s), recibido %q", raw)
		}
		cfg.RequestTimeout = v
	}
	if raw := os.Getenv("SHUTDOWN_TIMEOUT"); raw != "" {
		v, err := time.ParseDuration(raw)
		if err != nil || v <= 0 {
			return fmt.Errorf("SHUTDOWN_TIMEOUT debe ser una duración positiva (ej. 15s), recibido %q", raw)
		}
		cfg.ShutdownTimeout = v
	}
	if raw := os.Getenv("ERR_BASE_URL"); raw != "" {
		if len(raw) < 8 || (raw[:7] != "http://" && raw[:8] != "https://") {
			return fmt.Errorf("ERR_BASE_URL debe ser una URI http(s), recibido %q", raw)
		}
		cfg.ErrBaseURL = raw
	}
	return nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// ConfigSnapshot provides a point-in-time view of hot-reloadable config.
type ConfigSnapshot struct {
	MaxUploadBytes  int64
	MaxInFlight     int
	RequestTimeout  time.Duration
	ShutdownTimeout time.Duration
	ErrBaseURL      string
}

// Snapshot returns a consistent snapshot of hot-reloadable config.
func (c *Config) Snapshot() ConfigSnapshot {
	return ConfigSnapshot{
		MaxUploadBytes:  c.GetMaxUploadBytes(),
		MaxInFlight:     c.GetMaxInFlight(),
		RequestTimeout:  c.GetRequestTimeout(),
		ShutdownTimeout: c.GetShutdownTimeout(),
		ErrBaseURL:      c.GetErrBaseURL(),
	}
}