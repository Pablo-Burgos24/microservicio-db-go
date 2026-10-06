package tests

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestLoad_Defaults(t *testing.T) {
	os.Clearenv()
	os.Setenv("MONGODB_URI", "mongodb://localhost:27017")

	cfg, err := Load()
	assert.NoError(t, err)
	assert.Equal(t, defaultPort, cfg.Port)
	assert.Equal(t, defaultDB, cfg.MongoDB)
	assert.Equal(t, defaultCollection, cfg.MongoCollection)
	assert.Equal(t, defaultMaxUploadBytes, cfg.MaxUploadBytes)
	assert.Equal(t, defaultRequestTimeout, cfg.RequestTimeout)
	assert.Equal(t, defaultShutdownTimeout, cfg.ShutdownTimeout)
	assert.Equal(t, defaultErrBaseURL, cfg.ErrBaseURL)
}

func TestLoad_CustomValues(t *testing.T) {
	os.Clearenv()
	os.Setenv("MONGODB_URI", "mongodb://user:pass@mongo:27017")
	os.Setenv("PORT", "9090")
	os.Setenv("MONGODB_DB", "custom_db")
	os.Setenv("MONGODB_COLLECTION", "custom_col")
	os.Setenv("MAX_UPLOAD_MB", "50")
	os.Setenv("REQUEST_TIMEOUT", "30s")
	os.Setenv("SHUTDOWN_TIMEOUT", "20s")
	os.Setenv("ERR_BASE_URL", "https://custom.errors.com")

	cfg, err := Load()
	assert.NoError(t, err)
	assert.Equal(t, "9090", cfg.Port)
	assert.Equal(t, "custom_db", cfg.MongoDB)
	assert.Equal(t, "custom_col", cfg.MongoCollection)
	assert.Equal(t, int64(50*1024*1024), cfg.MaxUploadBytes)
	assert.Equal(t, 30*time.Second, cfg.RequestTimeout)
	assert.Equal(t, 20*time.Second, cfg.ShutdownTimeout)
	assert.Equal(t, "https://custom.errors.com", cfg.ErrBaseURL)
}

func TestLoad_MissingMongoURI(t *testing.T) {
	os.Clearenv()
	os.Unsetenv("MONGODB_URI")

	cfg, err := Load()
	assert.Error(t, err)
	assert.Nil(t, cfg)
	assert.Contains(t, err.Error(), "MONGODB_URI es requerida")
}

func TestLoad_InvalidMaxUploadMB(t *testing.T) {
	os.Clearenv()
	os.Setenv("MONGODB_URI", "mongodb://localhost:27017")
	os.Setenv("MAX_UPLOAD_MB", "invalid")

	cfg, err := Load()
	assert.Error(t, err)
	assert.Nil(t, cfg)
	assert.Contains(t, err.Error(), "MAX_UPLOAD_MB debe ser un entero positivo")
}

func TestLoad_InvalidRequestTimeout(t *testing.T) {
	os.Clearenv()
	os.Setenv("MONGODB_URI", "mongodb://localhost:27017")
	os.Setenv("REQUEST_TIMEOUT", "invalid")

	cfg, err := Load()
	assert.Error(t, err)
	assert.Nil(t, cfg)
	assert.Contains(t, err.Error(), "REQUEST_TIMEOUT debe ser una duración positiva")
}

func TestLoad_InvalidShutdownTimeout(t *testing.T) {
	os.Clearenv()
	os.Setenv("MONGODB_URI", "mongodb://localhost:27017")
	os.Setenv("SHUTDOWN_TIMEOUT", "invalid")

	cfg, err := Load()
	assert.Error(t, err)
	assert.Nil(t, cfg)
	assert.Contains(t, err.Error(), "SHUTDOWN_TIMEOUT debe ser una duración positiva")
}

func TestLoad_InvalidErrBaseURL(t *testing.T) {
	os.Clearenv()
	os.Setenv("MONGODB_URI", "mongodb://localhost:27017")
	os.Setenv("ERR_BASE_URL", "ftp://invalid.com")

	cfg, err := Load()
	assert.Error(t, err)
	assert.Nil(t, cfg)
	assert.Contains(t, err.Error(), "ERR_BASE_URL debe ser una URI http(s)")
}

func TestLoad_InvalidPort(t *testing.T) {
	os.Clearenv()
	os.Setenv("MONGODB_URI", "mongodb://localhost:27017")
	os.Setenv("PORT", "abc")

	cfg, err := Load()
	assert.Error(t, err)
	assert.Nil(t, cfg)
	assert.Contains(t, err.Error(), "PORT debe ser numérico")
}