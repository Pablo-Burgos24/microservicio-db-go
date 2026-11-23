package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"microservicio-db-go/application"
	"microservicio-db-go/config"
	"microservicio-db-go/consul"
	"microservicio-db-go/infrastructure/mongo"
	"microservicio-db-go/presentation"
)

func main() {
	if err := run(); err != nil {
		log.Printf("fatal: %v", err)
		os.Exit(1)
	}
}

func run() error {
	consulAddress := getEnv("CONSUL_HTTP_ADDR", "consul:8500")
	consulToken := os.Getenv("CONSUL_HTTP_TOKEN")

	// Load config with optional Consul KV watcher for hot-reload
	cfg, configWatcher, err := config.LoadWithWatcher(consulAddress, consulToken)
	if err != nil {
		return fmt.Errorf("cargar config: %w", err)
	}
	// Stop config watcher on shutdown
	defer func() {
		if configWatcher != nil {
			configWatcher.Stop()
		}
	}()

	// Initialize Consul registrar
	port, err := consul.ParsePort(cfg.Port)
	if err != nil {
		return fmt.Errorf("invalid port: %w", err)
	}

	consulCfg := consul.Config{
		ServiceName:   "microservicio-db",
		ServicePort:   port,
		HealthPath:    "/health/live", // Use liveness probe (doesn't check MongoDB)
		ConsulAddress: consulAddress,
		ConsulToken:   consulToken,
	}

	registrar, err := consul.NewRegistrar(consulCfg)
	if err != nil {
		log.Printf("warning: failed to create Consul registrar: %v", err)
		// Continue without Consul registration in case of config issues
		registrar = nil
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Register with Consul before starting the server
	if registrar != nil {
		if err := registrar.Register(ctx); err != nil {
			log.Printf("warning: failed to register with Consul: %v", err)
			registrar = nil // Don't try to deregister later
		} else {
			// Start shutdown waiter in background
			go registrar.WaitForShutdown()
		}
	}

	// Initialize Consul service discovery for external dependencies (optional)
	// For MongoDB (external), use regular discovery. For service-to-service, use Connect sidecar.
	discoveryCfg := consul.DiscoveryConfig{
		ConsulAddress:      consulAddress,
		ConsulToken:        consulToken,
		DNSPort:            8600,
		CacheTTL:           30 * time.Second,
		UseConnectSidecar:  false, // MongoDB doesn't have a sidecar
	}
	discovery, _ := consul.NewServiceDiscovery(discoveryCfg) // Ignore error, fallback to env var

	// Resolve MongoDB connection via Consul discovery, fallback to config
	mongoURI := cfg.MongoURI
	if discovery != nil {
		if addr, err := discovery.ResolveService(ctx, "mongo"); err == nil {
			// Build MongoDB URI from discovered address
			// Preserve auth from original URI if present
			mongoURI = buildMongoURI(cfg.MongoURI, addr.Host, addr.Port)
			log.Printf("Resolved MongoDB via Consul: %s:%d", addr.Host, addr.Port)
		} else {
			log.Printf("Warning: Failed to resolve MongoDB via Consul, using config: %v", err)
		}
	}

	repo, err := mongo.NewRepository(ctx, mongoURI, cfg.MongoDB, cfg.MongoCollection)
	if err != nil {
		if registrar != nil {
			registrar.SignalShutdown()
		}
		return fmt.Errorf("inicializar mongo: %w", err)
	}

	docUC := application.NewDocumentUseCase(repo)
	healthUC := application.NewHealthUseCase(repo)

	// Convert config snapshot to presentation.Config
	toPresentationConfig := func(s config.ConfigSnapshot) presentation.Config {
		return presentation.Config{
			ErrBaseURL:     s.ErrBaseURL,
			MaxUploadBytes: s.MaxUploadBytes,
			RequestTimeout: s.RequestTimeout,
			MaxInFlight:    s.MaxInFlight,
		}
	}

	// Use ConfigProvider for hot-reload support
	configProvider := presentation.NewStaticConfigProvider(toPresentationConfig(cfg.Snapshot()))
	
	// If config watcher is available, update provider on config changes
	if cfg.ConfigWatcher != nil {
		go func() {
			for {
				// Watch for config changes by polling the watcher's config
				// The watcher already does blocking queries, we just need to propagate changes
				time.Sleep(5 * time.Second)
				configProvider.Update(toPresentationConfig(cfg.Snapshot()))
			}
		}()
	}
	
	router := presentation.NewRouter(docUC, healthUC, configProvider)

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()
	log.Printf("escuchando en :%s", cfg.Port)

	select {
	case err := <-errCh:
		if registrar != nil {
			registrar.SignalShutdown()
		}
		_ = repo.Disconnect(context.Background())
		return fmt.Errorf("servidor: %w", err)
	case <-ctx.Done():
		log.Printf("señal de apagado recibida")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			if registrar != nil {
				registrar.SignalShutdown()
			}
			_ = repo.Disconnect(context.Background())
			return fmt.Errorf("apagado grácil: %w", err)
		}
		if registrar != nil {
			registrar.SignalShutdown()
		}
		if err := repo.Disconnect(context.Background()); err != nil {
			return fmt.Errorf("desconectar mongo: %w", err)
		}
		log.Printf("apagado completo")
		return nil
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// buildMongoURI constructs a MongoDB connection URI from the original URI (for auth)
// and the discovered host:port from Consul.
func buildMongoURI(originalURI, host string, port int) string {
	// If original URI has credentials, preserve them
	// Format: mongodb://[username:password@]host[:port][/database][?options]
	if strings.Contains(originalURI, "@") {
		// Extract credentials part
		parts := strings.SplitN(originalURI, "@", 2)
		if len(parts) == 2 {
			return parts[0] + "@" + host + ":" + strconv.Itoa(port) + "/" + strings.SplitN(parts[1], "/", 2)[1]
		}
	}
	// No credentials, just use host:port with default database
	return "mongodb://" + host + ":" + strconv.Itoa(port)
}