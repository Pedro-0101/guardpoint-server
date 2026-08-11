package worker

import (
	"context"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/guardpoint/guardpoint-server/internal/repository"
	"github.com/guardpoint/guardpoint-server/internal/timeutil"
)

type NaoRealizadoChecker struct {
	db               *pgxpool.Pool
	escalaRepo       *repository.EscalaRepository
	substituicaoRepo *repository.SubstituicaoRepository
}

func NewNaoRealizadoChecker(
	db *pgxpool.Pool,
	escalaRepo *repository.EscalaRepository,
	substituicaoRepo *repository.SubstituicaoRepository,
) *NaoRealizadoChecker {
	return &NaoRealizadoChecker{
		db:               db,
		escalaRepo:       escalaRepo,
		substituicaoRepo: substituicaoRepo,
	}
}

func (w *NaoRealizadoChecker) Run(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	slog.Info("nao-realizado checker worker started")

	for {
		select {
		case <-ctx.Done():
			slog.Info("nao-realizado checker worker stopped")
			return
		case <-ticker.C:
			w.CheckOnce(ctx)
		}
	}
}

func (w *NaoRealizadoChecker) CheckOnce(ctx context.Context) {
	now := timeutil.NowBRT()

	for diasAtras := 1; diasAtras <= 7; diasAtras++ {
		data := now.AddDate(0, 0, -diasAtras)

		escalas, err := w.escalaRepo.FindEscalasConcluidasSemTurno(ctx, data, now)
		if err != nil {
			slog.Error("nao-realizado checker: buscar escalas concluidas sem turno", "error", err, "data", data.Format("2006-01-02"))
			continue
		}

		for _, e := range escalas {
			w.criarTurnoNaoRealizado(ctx, e.EmpresaID, e.UsuarioID, e.PostoID, e.HoraInicio, e.HoraFim, data)
		}

		subs, err := w.substituicaoRepo.FindSubstituicoesConcluidasSemTurno(ctx, data, now)
		if err != nil {
			slog.Error("nao-realizado checker: buscar substituicoes concluidas sem turno", "error", err, "data", data.Format("2006-01-02"))
			continue
		}

		for _, s := range subs {
			w.criarTurnoNaoRealizado(ctx, s.EmpresaID, s.UsuarioID, s.PostoID, s.HoraInicio, s.HoraFim, data)
		}
	}
}

func (w *NaoRealizadoChecker) criarTurnoNaoRealizado(ctx context.Context, empresaID, usuarioID, postoID uuid.UUID, horaInicio, horaFim string, data time.Time) {
	dateStr := data.Format("2006-01-02")
	inicioPrevisto, err := parseHoraData(dateStr, horaInicio)
	if err != nil {
		slog.Error("nao-realizado checker: parse hora_inicio", "error", err, "hora", horaInicio)
		return
	}
	fimPrevisto, err := parseHoraData(dateStr, horaFim)
	if err != nil {
		slog.Error("nao-realizado checker: parse hora_fim", "error", err, "hora", horaFim)
		return
	}
	if !fimPrevisto.After(inicioPrevisto) {
		fimPrevisto = fimPrevisto.AddDate(0, 0, 1)
	}

	_, err = w.db.Exec(ctx, `
		INSERT INTO turnos (empresa_id, usuario_id, posto_id, status, inicio_previsto, fim_previsto, intervalo_min)
		VALUES ($1, $2, $3, 'nao_realizado', $4, $5, 30)
	`, empresaID, usuarioID, postoID, inicioPrevisto, fimPrevisto)
	if err != nil {
		slog.Error("nao-realizado checker: criar turno nao realizado", "error", err, "usuario_id", usuarioID)
		return
	}

	slog.Info("nao-realizado checker: turno nao realizado criado",
		"usuario_id", usuarioID.String(),
		"posto_id", postoID.String(),
		"data", dateStr,
		"inicio_previsto", inicioPrevisto,
		"fim_previsto", fimPrevisto,
	)
}
