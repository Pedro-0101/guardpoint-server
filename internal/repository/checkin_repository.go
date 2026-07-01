package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/guardpoint/guardpoint-server/internal/model"
)

type CheckinRepository struct {
	db *pgxpool.Pool
}

func NewCheckinRepository(db *pgxpool.Pool) *CheckinRepository {
	return &CheckinRepository{db: db}
}

func (r *CheckinRepository) Create(ctx context.Context, c *model.Checkin) error {
	query := `
		INSERT INTO checkins (turno_id, empresa_id, latitude, longitude, timestamp_criacao, tipo_senha, flag_geofence, origem_rede)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, timestamp_recebimento, created_at
	`
	return r.db.QueryRow(ctx, query,
		c.TurnoID, c.EmpresaID, c.Latitude, c.Longitude,
		c.TimestampCriacao, c.TipoSenha, c.FlagGeofence, c.OrigemRede,
	).Scan(&c.ID, &c.TimestampRecebimento, &c.CreatedAt)
}

func (r *CheckinRepository) FindUltimoByTurno(ctx context.Context, turnoID uuid.UUID) (*model.Checkin, error) {
	query := `
		SELECT id, turno_id, empresa_id, latitude, longitude, timestamp_criacao,
		       timestamp_recebimento, tipo_senha, flag_geofence, origem_rede, created_at
		FROM checkins
		WHERE turno_id = $1
		ORDER BY timestamp_criacao DESC
		LIMIT 1
	`
	var c model.Checkin
	err := r.db.QueryRow(ctx, query, turnoID).Scan(
		&c.ID, &c.TurnoID, &c.EmpresaID, &c.Latitude, &c.Longitude,
		&c.TimestampCriacao, &c.TimestampRecebimento, &c.TipoSenha,
		&c.FlagGeofence, &c.OrigemRede, &c.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("buscar ultimo checkin: %w", err)
	}
	return &c, nil
}

func (r *CheckinRepository) CountByTurnoHoje(ctx context.Context, turnoID uuid.UUID) (int, error) {
	query := `
		SELECT COUNT(*) FROM checkins
		WHERE turno_id = $1
		  AND timestamp_criacao::date = CURRENT_DATE
	`
	var count int
	if err := r.db.QueryRow(ctx, query, turnoID).Scan(&count); err != nil {
		return 0, fmt.Errorf("contar checkins hoje: %w", err)
	}
	return count, nil
}

func (r *CheckinRepository) ListByTurno(ctx context.Context, turnoID uuid.UUID) ([]model.Checkin, error) {
	query := `
		SELECT id, turno_id, empresa_id, latitude, longitude, timestamp_criacao,
		       timestamp_recebimento, tipo_senha, flag_geofence, origem_rede, created_at
		FROM checkins
		WHERE turno_id = $1
		ORDER BY timestamp_criacao ASC
	`
	rows, err := r.db.Query(ctx, query, turnoID)
	if err != nil {
		return nil, fmt.Errorf("listar checkins: %w", err)
	}
	defer rows.Close()

	var checkins []model.Checkin
	for rows.Next() {
		var c model.Checkin
		if err := rows.Scan(
			&c.ID, &c.TurnoID, &c.EmpresaID, &c.Latitude, &c.Longitude,
			&c.TimestampCriacao, &c.TimestampRecebimento, &c.TipoSenha,
			&c.FlagGeofence, &c.OrigemRede, &c.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan checkin: %w", err)
		}
		checkins = append(checkins, c)
	}
	return checkins, rows.Err()
}

func (r *CheckinRepository) FindUltimoByTurnoNoError(ctx context.Context, turnoID uuid.UUID) *model.Checkin {
	c, err := r.FindUltimoByTurno(ctx, turnoID)
	if err != nil {
		return nil
	}
	return c
}

func (r *CheckinRepository) FindUltimoTimestampByTurno(ctx context.Context, turnoID uuid.UUID) (*time.Time, error) {
	c, err := r.FindUltimoByTurno(ctx, turnoID)
	if err != nil {
		return nil, err
	}
	return &c.TimestampCriacao, nil
}
