package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/guardpoint/guardpoint-server/internal/model"
)

type DashboardRepository struct {
	db *pgxpool.Pool
}

func NewDashboardRepository(db *pgxpool.Pool) *DashboardRepository {
	return &DashboardRepository{db: db}
}

func (r *DashboardRepository) CountTurnosAtivos(ctx context.Context, empresaID uuid.UUID) (int, error) {
	var count int
	err := r.db.QueryRow(ctx, `
		SELECT COUNT(*) FROM turnos
		WHERE empresa_id = $1 AND status IN ('em_andamento', 'pausado', 'critico')
	`, empresaID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("contar turnos ativos: %w", err)
	}
	return count, nil
}

func (r *DashboardRepository) CountCheckinsUltimaHora(ctx context.Context, empresaID uuid.UUID) (int, error) {
	var count int
	err := r.db.QueryRow(ctx, `
		SELECT COUNT(*) FROM checkins
		WHERE empresa_id = $1 AND timestamp_recebimento >= now() - interval '1 hour'
	`, empresaID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("contar checkins ultima hora: %w", err)
	}
	return count, nil
}

func (r *DashboardRepository) CountDesviosRota(ctx context.Context, empresaID uuid.UUID) (int, error) {
	var count int
	err := r.db.QueryRow(ctx, `
		SELECT COUNT(*) FROM checkins
		WHERE empresa_id = $1 AND flag_geofence = 'desvio_rota'
		  AND timestamp_recebimento >= now() - interval '24 hours'
	`, empresaID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("contar desvios rota: %w", err)
	}
	return count, nil
}

func (r *DashboardRepository) AggregateTurnosPorPosto(ctx context.Context, empresaID uuid.UUID) ([]model.TurnoPorPosto, error) {
	query := `
		SELECT t.posto_id, COALESCE(p.nome, ''), COUNT(*)
		FROM turnos t
		LEFT JOIN postos p ON p.id = t.posto_id
		WHERE t.empresa_id = $1 AND t.status IN ('em_andamento', 'pausado', 'critico')
		GROUP BY t.posto_id, p.nome
		ORDER BY COUNT(*) DESC
	`
	rows, err := r.db.Query(ctx, query, empresaID)
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
