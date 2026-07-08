package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
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
		INSERT INTO checkins (turno_id, empresa_id, latitude, longitude, timestamp_criacao, evento, tipo_senha, senha_vigia_id, flag_geofence, origem_rede, cliente_checkin_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING id, timestamp_recebimento, created_at
	`
	return r.db.QueryRow(ctx, query,
		c.TurnoID, c.EmpresaID, c.Latitude, c.Longitude,
		c.TimestampCriacao, c.Evento, c.TipoSenha, c.SenhaVigiaID, c.FlagGeofence, c.OrigemRede, c.ClienteCheckinID,
	).Scan(&c.ID, &c.TimestampRecebimento, &c.CreatedAt)
}

func (r *CheckinRepository) CreateIdempotent(ctx context.Context, c *model.Checkin) (bool, error) {
	query := `
		INSERT INTO checkins (turno_id, empresa_id, latitude, longitude, timestamp_criacao, evento, tipo_senha, senha_vigia_id, flag_geofence, origem_rede, cliente_checkin_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		ON CONFLICT ON CONSTRAINT uq_checkins_cliente DO NOTHING
		RETURNING id, timestamp_recebimento, created_at
	`
	err := r.db.QueryRow(ctx, query,
		c.TurnoID, c.EmpresaID, c.Latitude, c.Longitude,
		c.TimestampCriacao, c.Evento, c.TipoSenha, c.SenhaVigiaID, c.FlagGeofence, c.OrigemRede, c.ClienteCheckinID,
	).Scan(&c.ID, &c.TimestampRecebimento, &c.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return false, fmt.Errorf("criar checkin idempotente: %w", err)
	}
	return true, nil
}

func (r *CheckinRepository) FindUltimoByTurno(ctx context.Context, turnoID uuid.UUID) (*model.Checkin, error) {
	query := `
		SELECT id, turno_id, empresa_id, latitude, longitude, timestamp_criacao,
		       timestamp_recebimento, evento, tipo_senha, senha_vigia_id, flag_geofence, origem_rede, cliente_checkin_id, created_at
		FROM checkins
		WHERE turno_id = $1
		ORDER BY timestamp_criacao DESC
		LIMIT 1
	`
	var c model.Checkin
	err := r.db.QueryRow(ctx, query, turnoID).Scan(
		&c.ID, &c.TurnoID, &c.EmpresaID, &c.Latitude, &c.Longitude,
		&c.TimestampCriacao, &c.TimestampRecebimento, &c.Evento, &c.TipoSenha, &c.SenhaVigiaID,
		&c.FlagGeofence, &c.OrigemRede, &c.ClienteCheckinID, &c.CreatedAt,
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
		       timestamp_recebimento, evento, tipo_senha, senha_vigia_id, flag_geofence, origem_rede, cliente_checkin_id, created_at
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
			&c.TimestampCriacao, &c.TimestampRecebimento, &c.Evento, &c.TipoSenha, &c.SenhaVigiaID,
			&c.FlagGeofence, &c.OrigemRede, &c.ClienteCheckinID, &c.CreatedAt,
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
