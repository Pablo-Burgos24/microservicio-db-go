package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"microservicio-db-go/application"
	"microservicio-db-go/config"
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
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("cargar config: %w", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	repo, err := mongo.NewRepository(ctx, cfg.MongoURI, cfg.MongoDB, cfg.MongoCollection)
	if err != nil {
		return fmt.Errorf("inicializar mongo: %w", err)
	}

	docUC := application.NewDocumentUseCase(repo)
	healthUC := application.NewHealthUseCase(repo)

	router := presentation.NewRouter(docUC, healthUC, presentation.Config{
		ErrBaseURL:        cfg.ErrBaseURL,
		MaxUploadBytes:    cfg.MaxUploadBytes,
		RequestTimeout:    cfg.RequestTimeout,
		MaxInFlight:       cfg.MaxInFlight,
	})

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
		_ = repo.Disconnect(context.Background())
		return fmt.Errorf("servidor: %w", err)
	case <-ctx.Done():
		log.Printf("señal de apagado recibida")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			_ = repo.Disconnect(context.Background())
			return fmt.Errorf("apagado grácil: %w", err)
		}
		if err := repo.Disconnect(context.Background()); err != nil {
			return fmt.Errorf("desconectar mongo: %w", err)
		}
		log.Printf("apagado completo")
		return nil
	}
}