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
	db         *pgxpool.Pool
	alertaSvc  *service.AlertaService
	configRepo *repository.ConfigEscalonamentoRepository
}

func NewTimeoutChecker(
	db *pgxpool.Pool,
	alertaSvc *service.AlertaService,
	configRepo *repository.ConfigEscalonamentoRepository,
) *TimeoutChecker {
	return &TimeoutChecker{
		db:         db,
		alertaSvc:  alertaSvc,
		configRepo: configRepo,
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
			w.check(ctx)
		}
	}
}

func (w *TimeoutChecker) check(ctx context.Context) {
	turnos, err := w.listTurnosAtivos(ctx)
	if err != nil {
		slog.Error("timeout checker: listar turnos ativos", "error", err)
		return
	}

	for _, t := range turnos {
		w.processTurno(ctx, t)
	}
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
