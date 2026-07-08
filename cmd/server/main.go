// Package main inicia o servidor HTTP do GuardPoint.
//
// Accept/Produce/Security sao defaults globais do swagger; endpoints
// publicos sobrescrevem com "@Security" (sem valor) nos seus handlers.
//
// @title           GuardPoint API
// @version         1.0
// @description     API do sistema GuardPoint (controle de rondas/postos).
// @BasePath        /api/v1
// @accept          json
// @produce         json
// @security        BearerAuth
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/guardpoint/guardpoint-server/docs"
	"github.com/guardpoint/guardpoint-server/internal/app"
	"github.com/guardpoint/guardpoint-server/internal/config"
	"github.com/guardpoint/guardpoint-server/internal/db"
	"github.com/guardpoint/guardpoint-server/internal/repository"
	"github.com/guardpoint/guardpoint-server/internal/seed"
	"github.com/guardpoint/guardpoint-server/internal/service"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	logger := newLogger(cfg.LogLevel, cfg.LogFormat)
	slog.SetDefault(logger)

	slog.Info("starting guardpoint-server", "env", cfg.Env)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := db.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	slog.Info("database connected")

	if cfg.Env == "development" {
		empresaRepo := repository.NewEmpresaRepository(pool)
		userRepo := repository.NewUserRepository(pool)
		configEscalonamentoRepo := repository.NewConfigEscalonamentoRepository(pool)
		empresaService := service.NewEmpresaService(empresaRepo, configEscalonamentoRepo)
		if err := seed.Run(ctx, empresaRepo, userRepo, empresaService); err != nil {
			slog.Error("seed failed", "error", err)
			os.Exit(1)
		}
	}

	a := app.New(cfg, pool)

	workerCtx, workerCancel := context.WithCancel(context.Background())
	defer workerCancel()

	go a.TimeoutChecker.Run(workerCtx)
	go a.AlertDispatcher.Run(workerCtx)

	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      a.Router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		slog.Info("server listening", "port", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server failed", "error", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("shutting down workers...")
	workerCancel()

	slog.Info("shutting down server...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("server forced to shutdown", "error", err)
	}

	slog.Info("server stopped")
}

func newLogger(level, format string) *slog.Logger {
	var slogLevel slog.Level
	switch level {
	case "debug":
		slogLevel = slog.LevelDebug
	case "warn":
		slogLevel = slog.LevelWarn
	case "error":
		slogLevel = slog.LevelError
	default:
		slogLevel = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{
		Level: slogLevel,
	}

	if format == "json" {
		return slog.New(slog.NewJSONHandler(os.Stdout, opts))
	}
	return slog.New(slog.NewTextHandler(os.Stdout, opts))
}
