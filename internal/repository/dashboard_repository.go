package repository

import (
	"context"
	"fmt"
	"time"

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

func (r *DashboardRepository) ListTurnosDetalhados(ctx context.Context, empresaID uuid.UUID, limit, offset int) ([]model.DashboardLinha, int, error) {
	query := `
		SELECT
			t.id,
			t.status,
			t.inicio_previsto,
			t.fim_previsto,
			t.inicio_real,
			t.intervalo_min,
			p.id,
			p.nome,
			p.latitude,
			p.longitude,
			p.raio_m,
			u.id,
			u.nome,
			uc.timestamp_criacao,
			COALESCE(uc.timestamp_criacao, t.inicio_real, t.inicio_previsto) + (t.intervalo_min * interval '1 minute'),
			(COALESCE(uc.timestamp_criacao, t.inicio_real, t.inicio_previsto) + (t.intervalo_min * interval '1 minute')) < now(),
			COUNT(*) OVER() AS total_count
		FROM turnos t
		JOIN postos p ON p.id = t.posto_id
		JOIN usuarios u ON u.id = t.usuario_id
		LEFT JOIN LATERAL (
			SELECT timestamp_criacao, timestamp_recebimento
			FROM checkins
			WHERE turno_id = t.id
			ORDER BY timestamp_criacao DESC
			LIMIT 1
		) uc ON true
		WHERE t.empresa_id = $1
		  AND t.status IN ('em_andamento', 'pausado', 'critico')
		ORDER BY
			COALESCE(uc.timestamp_criacao, t.inicio_real, t.inicio_previsto) + (t.intervalo_min * interval '1 minute') ASC
		LIMIT $2 OFFSET $3
	`

	rows, err := r.db.Query(ctx, query, empresaID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("listar turnos detalhados: %w", err)
	}
	defer rows.Close()

	var linhas []model.DashboardLinha
	var total int

	for rows.Next() {
		var l model.DashboardLinha
		var turnoID, postoID, vigiaID uuid.UUID
		var inicioPrevisto, fimPrevisto time.Time
		var inicioReal *time.Time
		var ultimoCheckin *time.Time
		var proximoCheckin time.Time

		if err := rows.Scan(
			&turnoID, &l.TurnoStatus, &inicioPrevisto, &fimPrevisto, &inicioReal,
			&l.IntervaloMin,
			&postoID, &l.PostoNome, &l.PostoLatitude, &l.PostoLongitude, &l.PostoRaioM,
			&vigiaID, &l.VigiaNome,
			&ultimoCheckin, &proximoCheckin, &l.Atrasado,
			&total,
		); err != nil {
			return nil, 0, fmt.Errorf("scan turno detalhado: %w", err)
		}

		l.TurnoID = turnoID.String()
		l.PostoID = postoID.String()
		l.VigiaID = vigiaID.String()
		l.InicioPrevisto = inicioPrevisto.Format(time.RFC3339)
		l.FimPrevisto = fimPrevisto.Format(time.RFC3339)
		if inicioReal != nil {
			l.InicioReal = inicioReal.Format(time.RFC3339)
		}
		if ultimoCheckin != nil {
			l.UltimoCheckin = ultimoCheckin.Format(time.RFC3339)
		}
		l.ProximoCheckin = proximoCheckin.Format(time.RFC3339)

		linhas = append(linhas, l)
	}

	return linhas, total, rows.Err()
}
