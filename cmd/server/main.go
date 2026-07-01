package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"

	"github.com/guardpoint/guardpoint-server/internal/auth"
	"github.com/guardpoint/guardpoint-server/internal/config"
	"github.com/guardpoint/guardpoint-server/internal/db"
	"github.com/guardpoint/guardpoint-server/internal/handler"
	"github.com/guardpoint/guardpoint-server/internal/middleware"
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

	jwtService := auth.NewJWTService(cfg.JWTSecret)

	userRepo := repository.NewUserRepository(pool)
	empresaRepo := repository.NewEmpresaRepository(pool)
	sessaoDispositivoRepo := repository.NewSessaoDispositivoRepository(pool)
	postoRepo := repository.NewPostoRepository(pool)
	turnoRepo := repository.NewTurnoRepository(pool)
	checkinRepo := repository.NewCheckinRepository(pool)

	authService := service.NewAuthService(jwtService, userRepo, empresaRepo, sessaoDispositivoRepo)
	authHandler := handler.NewAuthHandler(authService)

	postoService := service.NewPostoService(postoRepo)
	postoHandler := handler.NewPostoHandler(postoService)

	turnoService := service.NewTurnoService(turnoRepo, checkinRepo, postoRepo, userRepo, sessaoDispositivoRepo)
	turnoHandler := handler.NewTurnoHandler(turnoService)

	dashboardService := service.NewDashboardService(pool)
	dashboardHandler := handler.NewDashboardHandler(dashboardService)

	if cfg.Env == "development" {
		if err := seed.Run(ctx, empresaRepo, userRepo); err != nil {
			slog.Error("seed failed", "error", err)
			os.Exit(1)
		}
	}

	r := chi.NewRouter()
	r.Use(chimw.RequestID)
	r.Use(chimw.Logger)
	r.Use(chimw.Recoverer)
	r.Use(middleware.CORS)
	r.Use(chimw.Timeout(30 * time.Second))

	r.Route("/api", func(r chi.Router) {
		r.Route("/auth", func(r chi.Router) {
			r.Post("/login", authHandler.Login)
			r.Post("/refresh", authHandler.Refresh)
			r.Post("/biometric/login", authHandler.BiometricLogin)

			r.Group(func(r chi.Router) {
				r.Use(handler.AuthMiddleware(jwtService))
				r.Post("/logout", authHandler.Logout)
				r.Post("/biometric/register", authHandler.BiometricRegister)

				r.Group(func(r chi.Router) {
					r.Use(handler.RequireRole("admin"))
					r.Post("/register", authHandler.Register)
				})
			})
		})

		r.Group(func(r chi.Router) {
			r.Use(handler.AuthMiddleware(jwtService))

			r.Get("/dashboard/summary", dashboardHandler.Summary)

			r.Route("/postos", func(r chi.Router) {
				r.Get("/", postoHandler.List)
				r.Post("/", postoHandler.Create)
				r.Get("/{id}", postoHandler.GetByID)
				r.Put("/{id}", postoHandler.Update)
				r.Delete("/{id}", postoHandler.Delete)
			})

			r.Route("/turnos", func(r chi.Router) {
				r.Post("/iniciar", turnoHandler.Iniciar)
				r.Post("/checkin", turnoHandler.Checkin)
				r.Post("/finalizar", turnoHandler.Finalizar)
				r.Post("/sabotagem", turnoHandler.Sabotagem)
				r.Get("/status", turnoHandler.Status)
				r.Get("/ativos", turnoHandler.Ativos)
				r.Get("/historico", turnoHandler.Historico)
				r.Get("/{id}", turnoHandler.GetByID)
				r.Post("/{id}/revogar", turnoHandler.Revogar)
			})

			r.Post("/checkins/lote", turnoHandler.Lote)
		})
	})

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      r,
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
