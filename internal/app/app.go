// Package app monta o roteador HTTP e os servicos do GuardPoint. E usado pelo
// cmd/server e pelos testes de integracao, garantindo que os testes exercitem
// exatamente a mesma fiacao (rotas, RBAC, middlewares) da producao.
package app

import (
	"context"
	"crypto/subtle"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	httpSwagger "github.com/swaggo/http-swagger"

	"github.com/guardpoint/guardpoint-server/internal/auth"
	"github.com/guardpoint/guardpoint-server/internal/config"
	"github.com/guardpoint/guardpoint-server/internal/handler"
	"github.com/guardpoint/guardpoint-server/internal/metrics"
	"github.com/guardpoint/guardpoint-server/internal/middleware"
	"github.com/guardpoint/guardpoint-server/internal/repository"
	"github.com/guardpoint/guardpoint-server/internal/service"
	"github.com/guardpoint/guardpoint-server/internal/worker"
	"github.com/guardpoint/guardpoint-server/internal/ws"
)

type App struct {
	Router          chi.Router
	Hub             *ws.Hub
	JWTService      *auth.JWTService
	AlertaService   *service.AlertaService
	TurnoService    *service.TurnoService
	TimeoutChecker  *worker.TimeoutChecker
	AlertDispatcher *worker.AlertDispatcher
	SyncReconciler  *worker.SyncReconciler
	EmpresaRepo     *repository.EmpresaRepository
	UserRepo        *repository.UserRepository
}

func New(cfg *config.Config, pool *pgxpool.Pool) *App {
	hub := ws.NewHub()
	jwtService := auth.NewJWTService(cfg.JWTSecret)

	userRepo := repository.NewUserRepository(pool)
	empresaRepo := repository.NewEmpresaRepository(pool)
	sessaoDispositivoRepo := repository.NewSessaoDispositivoRepository(pool)
	postoRepo := repository.NewPostoRepository(pool)
	turnoRepo := repository.NewTurnoRepository(pool)
	checkinRepo := repository.NewCheckinRepository(pool)
	alertaRepo := repository.NewAlertaRepository(pool)
	configEscalonamentoRepo := repository.NewConfigEscalonamentoRepository(pool)
	escalaRepo := repository.NewEscalaRepository(pool)
	substituicaoRepo := repository.NewSubstituicaoRepository(pool)
	senhaVigiaRepo := repository.NewSenhaVigiaRepository(pool)

	authService := service.NewAuthService(jwtService, userRepo, empresaRepo, sessaoDispositivoRepo)
	authHandler := handler.NewAuthHandler(authService)

	postoService := service.NewPostoService(postoRepo)
	postoHandler := handler.NewPostoHandler(postoService)

	escalonamentoSvc := service.NewEscalonamentoService(configEscalonamentoRepo, userRepo)
	alertaService := service.NewAlertaService(alertaRepo, escalonamentoSvc, hub)

	turnoService := service.NewTurnoService(turnoRepo, checkinRepo, postoRepo, userRepo, sessaoDispositivoRepo, escalaRepo, substituicaoRepo, senhaVigiaRepo, alertaService, hub)
	syncReconciler := worker.NewSyncReconciler(alertaRepo, checkinRepo, turnoRepo, hub)
	turnoHandler := handler.NewTurnoHandler(turnoService, syncReconciler)

	senhaVigiaService := service.NewSenhaVigiaService(senhaVigiaRepo, userRepo, configEscalonamentoRepo)
	senhaVigiaHandler := handler.NewSenhaVigiaHandler(senhaVigiaService)

	usuarioService := service.NewUsuarioService(userRepo)
	usuarioHandler := handler.NewUsuarioHandler(usuarioService)

	dashboardRepo := repository.NewDashboardRepository(pool)
	dashboardService := service.NewDashboardService(dashboardRepo, alertaRepo)
	dashboardHandler := handler.NewDashboardHandler(dashboardService)

	alertaHandler := handler.NewAlertaHandler(alertaService)
	escalonamentoHandler := handler.NewEscalonamentoHandler(escalonamentoSvc)

	escalaService := service.NewEscalaService(escalaRepo)
	escalaHandler := handler.NewEscalaHandler(escalaService)

	empresaService := service.NewEmpresaService(empresaRepo, configEscalonamentoRepo)
	empresaHandler := handler.NewEmpresaHandler(empresaService)

	substituicaoService := service.NewSubstituicaoService(substituicaoRepo)
	substituicaoHandler := handler.NewSubstituicaoHandler(substituicaoService)

	timeoutChecker := worker.NewTimeoutChecker(pool, alertaService, configEscalonamentoRepo, escalaRepo, substituicaoRepo)
	alertDispatcher := worker.NewAlertDispatcher(alertaService.AlertChannel())

	r := chi.NewRouter()
	r.Use(chimw.RequestID)
	r.Use(metrics.Middleware)
	r.Use(chimw.Logger)
	r.Use(chimw.Recoverer)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins: cfg.CORSOrigins,
		AllowedMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders: []string{"Content-Type", "Authorization"},
		MaxAge:         86400,
	}))
	r.Use(chimw.Timeout(30 * time.Second))

	r.Route("/api/v1", func(r chi.Router) {
		r.Route("/auth", func(r chi.Router) {
			r.Post("/login", authHandler.Login)
			r.Post("/refresh", authHandler.Refresh)
			r.Post("/biometric/login", authHandler.BiometricLogin)

			r.Group(func(r chi.Router) {
				r.Use(middleware.AuthMiddleware(jwtService))
				r.Post("/logout", authHandler.Logout)
				r.Post("/biometric/register", authHandler.BiometricRegister)

				r.Group(func(r chi.Router) {
					r.Use(middleware.RequireRole("admin"))
					r.Post("/register", authHandler.Register)
				})
			})
		})

		r.Group(func(r chi.Router) {
			r.Use(middleware.AuthMiddleware(jwtService))

			r.Group(func(r chi.Router) {
				r.Use(middleware.RequireRole("admin", "supervisor"))
				r.Get("/dashboard/summary", dashboardHandler.Summary)
			})

			r.Route("/postos", func(r chi.Router) {
				r.Get("/", postoHandler.List)
				r.Get("/{id}", postoHandler.GetByID)

				r.Group(func(r chi.Router) {
					r.Use(middleware.RequireRole("admin"))
					r.Post("/", postoHandler.Create)
					r.Put("/{id}", postoHandler.Update)
					r.Delete("/{id}", postoHandler.Delete)
				})
			})

			r.Route("/turnos", func(r chi.Router) {
				r.Get("/", turnoHandler.List)
				r.Post("/iniciar", turnoHandler.Iniciar)
				r.Post("/checkin", turnoHandler.Checkin)
				r.Post("/finalizar", turnoHandler.Finalizar)
				r.Post("/sabotagem", turnoHandler.Sabotagem)
				r.Post("/reassociar", turnoHandler.Reassociar)
				r.Get("/status", turnoHandler.Status)
				r.Get("/{id}", turnoHandler.GetByID)
				r.Post("/{id}/revogar", turnoHandler.Revogar)
			})

			r.Post("/checkins/lote", turnoHandler.Lote)

			r.Route("/usuarios", func(r chi.Router) {
				r.Group(func(r chi.Router) {
					r.Use(middleware.RequireRole("admin", "supervisor"))
					r.Get("/", usuarioHandler.List)
					r.Get("/{id}", usuarioHandler.GetByID)
				})
				r.Group(func(r chi.Router) {
					r.Use(middleware.RequireRole("admin"))
					r.Post("/", usuarioHandler.Create)
					r.Put("/{id}", usuarioHandler.Update)
					r.Delete("/{id}", usuarioHandler.Delete)
				})
			})

			r.Route("/usuarios/{id}/senhas", func(r chi.Router) {
				r.Group(func(r chi.Router) {
					r.Use(middleware.RequireRole("admin", "supervisor"))
					r.Get("/", senhaVigiaHandler.List)
				})
				r.Group(func(r chi.Router) {
					r.Use(middleware.RequireRole("admin"))
					r.Post("/", senhaVigiaHandler.Create)
					r.Post("/batch", senhaVigiaHandler.BatchCreate)
					r.Put("/{senhaId}", senhaVigiaHandler.Update)
					r.Delete("/{senhaId}", senhaVigiaHandler.Delete)
				})
			})

			r.Route("/alertas", func(r chi.Router) {
				r.Use(middleware.RequireRole("admin", "supervisor"))
				r.Get("/", alertaHandler.List)
				r.Get("/estatisticas", alertaHandler.Estatisticas)
				r.Put("/{id}/reconhecer", alertaHandler.Reconhecer)
				r.Put("/{id}/encerrar", alertaHandler.Encerrar)
			})

			r.Route("/config", func(r chi.Router) {
				r.Use(middleware.RequireRole("admin"))
				r.Route("/escalonamento", func(r chi.Router) {
					r.Get("/", escalonamentoHandler.List)
					r.Post("/", escalonamentoHandler.Create)
					r.Get("/{id}", escalonamentoHandler.GetByID)
					r.Put("/{id}", escalonamentoHandler.Update)
					r.Put("/{id}/usuarios", escalonamentoHandler.UpdateUsuarios)
					r.Delete("/{id}", escalonamentoHandler.Delete)
				})
			})

			r.Route("/escalas", func(r chi.Router) {
				r.Use(middleware.RequireRole("admin", "supervisor"))
				r.Get("/", escalaHandler.List)
				r.Post("/", escalaHandler.Create)
				r.Post("/lote", escalaHandler.CreateLote)
				r.Put("/lote", escalaHandler.ReplaceLote)
				r.Get("/{id}", escalaHandler.GetByID)
				r.Put("/{id}", escalaHandler.Update)
				r.Delete("/{id}", escalaHandler.Delete)
			})

			r.Route("/substituicoes", func(r chi.Router) {
				r.Use(middleware.RequireRole("admin", "supervisor"))
				r.Get("/", substituicaoHandler.List)
				r.Post("/", substituicaoHandler.Create)
				r.Get("/{id}", substituicaoHandler.GetByID)
				r.Put("/{id}", substituicaoHandler.Update)
				r.Delete("/{id}", substituicaoHandler.Delete)
			})

			r.Route("/empresa", func(r chi.Router) {
				r.Use(middleware.RequireRole("admin"))
				r.Get("/", empresaHandler.Get)
				r.Put("/", empresaHandler.Update)
			})
		})
	})

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	r.Get("/ready", func(w http.ResponseWriter, r *http.Request) {
		pingCtx, pingCancel := context.WithTimeout(r.Context(), 3*time.Second)
		defer pingCancel()

		w.Header().Set("Content-Type", "application/json")
		if err := pool.Ping(pingCtx); err != nil {
			slog.Warn("readiness check failed", "error", err)
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"status":"not ready","error":"database unreachable"}`))
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ready"}`))
	})

	// METRICS_TOKEN vazio deixa /metrics aberto (ex.: rede privada do Railway);
	// definido, exige Authorization: Bearer <token> (B3).
	metricsHandler := promhttp.Handler()
	r.Get("/metrics", func(w http.ResponseWriter, r *http.Request) {
		if cfg.MetricsToken != "" {
			if subtle.ConstantTimeCompare([]byte(r.Header.Get("Authorization")), []byte("Bearer "+cfg.MetricsToken)) != 1 {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
		}
		metricsHandler.ServeHTTP(w, r)
	})

	r.Get("/ws", ws.HandleWebSocket(hub, jwtService, cfg.CORSOrigins))

	// Swagger UI disponivel apenas fora de producao para nao expor a doc da API publicamente.
	if cfg.Env != "production" {
		r.Get("/swagger/*", httpSwagger.WrapHandler)
	}

	return &App{
		Router:          r,
		Hub:             hub,
		JWTService:      jwtService,
		AlertaService:   alertaService,
		TurnoService:    turnoService,
		TimeoutChecker:  timeoutChecker,
		AlertDispatcher: alertDispatcher,
		SyncReconciler:  syncReconciler,
		EmpresaRepo:     empresaRepo,
		UserRepo:        userRepo,
	}
}
