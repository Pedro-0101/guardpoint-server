package service

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/guardpoint/guardpoint-server/internal/model"
	"github.com/guardpoint/guardpoint-server/internal/repository"
)

type DashboardService struct {
	db         *pgxpool.Pool
	alertaRepo *repository.AlertaRepository
}

func NewDashboardService(db *pgxpool.Pool, alertaRepo *repository.AlertaRepository) *DashboardService {
	return &DashboardService{db: db, alertaRepo: alertaRepo}
}

func (s *DashboardService) Summary(ctx context.Context, empresaID string) (*model.DashboardSummary, error) {
	summary := &model.DashboardSummary{}

	turnosAtivos, err := s.countTurnosAtivos(ctx, empresaID)
	if err == nil {
		summary.TurnosAtivos = turnosAtivos
	}

	checkinsUltimaHora, err := s.countCheckinsUltimaHora(ctx, empresaID)
	if err == nil {
		summary.CheckinsUltimaHora = checkinsUltimaHora
	}

	desviosRota, err := s.countDesviosRota(ctx, empresaID)
	if err == nil {
		summary.DesviosRota = desviosRota
	}

	parsedEmpresaID, err := uuid.Parse(empresaID)
	if err == nil {
		alertasAbertos, err := s.alertaRepo.CountAbertos(ctx, parsedEmpresaID)
		if err == nil {
			summary.AlertasAbertos = alertasAbertos
		}

		recentes, err := s.alertaRepo.ListRecentes(ctx, parsedEmpresaID, 5)
		if err == nil {
			summary.AlertasRecentes = recentes
		}
	}

	turnosPorPosto, err := s.aggregateTurnosPorPosto(ctx, empresaID)
	if err == nil {
		summary.TurnosPorPosto = turnosPorPosto
	}

	summary.AlertasRecentes = safeSlice(summary.AlertasRecentes, func() []model.AlertaRecente { return []model.AlertaRecente{} })
	summary.TurnosPorPosto = safeSlice(summary.TurnosPorPosto, func() []model.TurnoPorPosto { return []model.TurnoPorPosto{} })

	return summary, nil
}

func (s *DashboardService) countTurnosAtivos(ctx context.Context, empresaID string) (int, error) {
	var count int
	err := s.db.QueryRow(ctx, `
		SELECT COUNT(*) FROM turnos
		WHERE empresa_id = $1 AND status IN ('em_andamento', 'pausado', 'critico')
	`, empresaID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("contar turnos ativos: %w", err)
	}
	return count, nil
}

func (s *DashboardService) countCheckinsUltimaHora(ctx context.Context, empresaID string) (int, error) {
	var count int
	err := s.db.QueryRow(ctx, `
		SELECT COUNT(*) FROM checkins
		WHERE empresa_id = $1 AND timestamp_recebimento >= now() - interval '1 hour'
	`, empresaID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("contar checkins ultima hora: %w", err)
	}
	return count, nil
}

func (s *DashboardService) countDesviosRota(ctx context.Context, empresaID string) (int, error) {
	var count int
	err := s.db.QueryRow(ctx, `
		SELECT COUNT(*) FROM checkins
		WHERE empresa_id = $1 AND flag_geofence = 'desvio_rota'
		  AND timestamp_recebimento >= now() - interval '24 hours'
	`, empresaID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("contar desvios rota: %w", err)
	}
	return count, nil
}

func (s *DashboardService) aggregateTurnosPorPosto(ctx context.Context, empresaID string) ([]model.TurnoPorPosto, error) {
	query := `
		SELECT t.posto_id, COALESCE(p.nome, ''), COUNT(*)
		FROM turnos t
		LEFT JOIN postos p ON p.id = t.posto_id
		WHERE t.empresa_id = $1 AND t.status IN ('em_andamento', 'pausado', 'critico')
		GROUP BY t.posto_id, p.nome
		ORDER BY COUNT(*) DESC
	`
	rows, err := s.db.Query(ctx, query, empresaID)
	if err != nil {
		return nil, fmt.Errorf("agregar turnos por posto: %w", err)
	}
	defer rows.Close()

	var result []model.TurnoPorPosto
	for rows.Next() {
		var tpp model.TurnoPorPosto
		var id uuid.UUID
		if err := rows.Scan(&id, &tpp.PostoNome, &tpp.Quantidade); err != nil {
			return nil, fmt.Errorf("scan turno por posto: %w", err)
		}
		tpp.PostoID = id.String()
		result = append(result, tpp)
	}
	return result, rows.Err()
}

func safeSlice[T any](s []T, fallback func() []T) []T {
	if s == nil {
		return fallback()
	}
	return s
}
