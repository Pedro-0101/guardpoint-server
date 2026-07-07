package worker

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/guardpoint/guardpoint-server/internal/repository"
	"github.com/guardpoint/guardpoint-server/internal/service"
)

type TimeoutChecker struct {
	db               *pgxpool.Pool
	alertaSvc        *service.AlertaService
	configRepo       *repository.ConfigEscalonamentoRepository
	escalaRepo       *repository.EscalaRepository
	substituicaoRepo *repository.SubstituicaoRepository
}

func NewTimeoutChecker(
	db *pgxpool.Pool,
	alertaSvc *service.AlertaService,
	configRepo *repository.ConfigEscalonamentoRepository,
	escalaRepo *repository.EscalaRepository,
	substituicaoRepo *repository.SubstituicaoRepository,
) *TimeoutChecker {
	return &TimeoutChecker{
		db:               db,
		alertaSvc:        alertaSvc,
		configRepo:       configRepo,
		escalaRepo:       escalaRepo,
		substituicaoRepo: substituicaoRepo,
	}
}

func (w *TimeoutChecker) Run(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	slog.Info("timeout checker worker started")

	for {
		select {
		case <-ctx.Done():
			slog.Info("timeout checker worker stopped")
			return
		case <-ticker.C:
			w.CheckOnce(ctx)
		}
	}
}

// CheckOnce roda um ciclo de verificacao (atrasos de check-in + no-show).
// Exportado para permitir teste deterministico sem o ticker.
func (w *TimeoutChecker) CheckOnce(ctx context.Context) {
	turnos, err := w.listTurnosAtivos(ctx)
	if err != nil {
		slog.Error("timeout checker: listar turnos ativos", "error", err)
	} else {
		for _, t := range turnos {
			w.processTurno(ctx, t)
		}
	}

	w.checkNoShow(ctx)
}

type turnoAtivoInfo struct {
	ID           uuid.UUID
	EmpresaID    uuid.UUID
	UsuarioID    uuid.UUID
	PostoID      uuid.UUID
	IntervaloMin int
	Status       string
}

func (w *TimeoutChecker) listTurnosAtivos(ctx context.Context) ([]turnoAtivoInfo, error) {
	query := `
		SELECT id, empresa_id, usuario_id, posto_id, intervalo_min, status
		FROM turnos
		WHERE status IN ('em_andamento', 'pausado', 'critico')
	`
	rows, err := w.db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query turnos ativos: %w", err)
	}
	defer rows.Close()

	var turnos []turnoAtivoInfo
	for rows.Next() {
		var t turnoAtivoInfo
		if err := rows.Scan(&t.ID, &t.EmpresaID, &t.UsuarioID, &t.PostoID, &t.IntervaloMin, &t.Status); err != nil {
			return nil, fmt.Errorf("scan turno ativo: %w", err)
		}
		turnos = append(turnos, t)
	}
	return turnos, rows.Err()
}

func (w *TimeoutChecker) processTurno(ctx context.Context, t turnoAtivoInfo) {
	ultimoTimestamp, err := w.getUltimoCheckin(ctx, t.ID)
	if err != nil {
		return
	}

	if ultimoTimestamp == nil {
		return
	}

	elapsed := time.Since(*ultimoTimestamp)
	intervaloDuration := time.Duration(t.IntervaloMin) * time.Minute

	if elapsed <= intervaloDuration {
		return
	}

	atrasoMinutos := int(elapsed.Minutes()) - t.IntervaloMin

	configs, err := w.configRepo.FindByEmpresa(ctx, t.EmpresaID)
	if err != nil || len(configs) == 0 {
		return
	}

	for _, cfg := range configs {
		if atrasoMinutos >= cfg.AtrasoMinutos {
			mensagem := fmt.Sprintf(
				"Atraso de %d minutos detectado no turno. Nivel %d de escalonamento.",
				atrasoMinutos, cfg.Nivel,
			)

			tipo := fmt.Sprintf("atraso_n%d", cfg.Nivel)
			_, err := w.alertaSvc.CreateAlerta(ctx, t.EmpresaID, t.ID, tipo, cfg.Nivel, mensagem)
			if err != nil {
				slog.Error("timeout checker: criar alerta", "error", err, "turno_id", t.ID)
				continue
			}

			slog.Info("timeout checker: alerta gerado",
				"turno_id", t.ID.String(),
				"tipo", tipo,
				"nivel", cfg.Nivel,
				"atraso_minutos", atrasoMinutos,
			)
		}
	}
}

func (w *TimeoutChecker) getUltimoCheckin(ctx context.Context, turnoID uuid.UUID) (*time.Time, error) {
	var ts time.Time
	err := w.db.QueryRow(ctx, `
		SELECT timestamp_criacao FROM checkins
		WHERE turno_id = $1
		ORDER BY timestamp_criacao DESC
		LIMIT 1
	`, turnoID).Scan(&ts)
	if err != nil {
		return nil, err
	}
	return &ts, nil
}

func (w *TimeoutChecker) checkNoShow(ctx context.Context) {
	now := time.Now()

	escalas, err := w.escalaRepo.FindEscalasSemTurno(ctx, now)
	if err != nil {
		slog.Error("timeout checker: buscar escalas sem turno", "error", err)
		return
	}

	substituicoes, err := w.substituicaoRepo.FindSubstituicoesSemTurno(ctx, now)
	if err != nil {
		slog.Error("timeout checker: buscar substituicoes sem turno", "error", err)
		return
	}

	all := make([]noShowEntry, 0, len(escalas)+len(substituicoes))
	for _, e := range escalas {
		all = append(all, noShowEntry{
			empresaID:     e.EmpresaID,
			usuarioID:     e.UsuarioID,
			postoID:       e.PostoID,
			horaInicio:    e.HoraInicio,
			toleranciaMin: e.ToleranciaMin,
		})
	}
	for _, s := range substituicoes {
		all = append(all, noShowEntry{
			empresaID:     s.EmpresaID,
			usuarioID:     s.UsuarioID,
			postoID:       s.PostoID,
			horaInicio:    s.HoraInicio,
			toleranciaMin: s.ToleranciaMin,
		})
	}

	for _, e := range all {
		existe, err := w.alertaJaExiste(ctx, e.empresaID, e.usuarioID, now)
		if err != nil {
			slog.Error("timeout checker: verificar alerta existente", "error", err, "usuario_id", e.usuarioID)
			continue
		}
		if existe {
			continue
		}

		tolerancia := e.toleranciaMin
		if tolerancia <= 0 {
			tolerancia = 15
		}

		mensagem := fmt.Sprintf(
			"No-show detectado: vigia nao iniciou turno ate %s (tolerancia de %d min). Escala as %s. [ref:%s]",
			now.Format("15:04"), tolerancia, e.horaInicio, e.usuarioID.String(),
		)

		_, err = w.alertaSvc.CreateAlertaImediato(ctx, e.empresaID, uuid.Nil, "no_show", 2, mensagem)
		if err != nil {
			slog.Error("timeout checker: criar alerta no-show", "error", err, "usuario_id", e.usuarioID)
			continue
		}

		slog.Info("timeout checker: alerta no-show gerado",
			"usuario_id", e.usuarioID.String(),
			"posto_id", e.postoID.String(),
			"hora_inicio", e.horaInicio,
		)
	}
}

type noShowEntry struct {
	empresaID     uuid.UUID
	usuarioID     uuid.UUID
	postoID       uuid.UUID
	horaInicio    string
	toleranciaMin int
}

func (w *TimeoutChecker) alertaJaExiste(ctx context.Context, empresaID, usuarioID uuid.UUID, data time.Time) (bool, error) {
	var count int
	err := w.db.QueryRow(ctx, `
		SELECT COUNT(*) FROM alertas
		WHERE empresa_id = $1
		  AND tipo = 'no_show'
		  AND status = 'aberto'
		  AND created_at::date = $2::date
		  AND COALESCE(mensagem, '') LIKE '%' || $3::text || '%'
	`, empresaID, data, usuarioID.String()).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}
