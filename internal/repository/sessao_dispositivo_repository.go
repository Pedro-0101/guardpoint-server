package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/guardpoint/guardpoint-server/internal/model"
)

type SessaoDispositivoRepository struct {
	db *pgxpool.Pool
}

func NewSessaoDispositivoRepository(db *pgxpool.Pool) *SessaoDispositivoRepository {
	return &SessaoDispositivoRepository{db: db}
}

func (r *SessaoDispositivoRepository) FindByDeviceID(ctx context.Context, empresaID, deviceID string) (*model.SessaoDispositivo, error) {
	query := `
		SELECT id, usuario_id, empresa_id, device_id, criado_em
		FROM sessoes_dispositivo
		WHERE device_id = $1 AND empresa_id = $2::uuid
	`

	var s model.SessaoDispositivo
	err := r.db.QueryRow(ctx, query, deviceID, empresaID).Scan(
		&s.ID, &s.UsuarioID, &s.EmpresaID, &s.DeviceID, &s.CriadoEm,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("sessao dispositivo nao encontrada: %w", err)
		}
		return nil, fmt.Errorf("buscar sessao dispositivo: %w", err)
	}

	return &s, nil
}

func (r *SessaoDispositivoRepository) Create(ctx context.Context, s *model.SessaoDispositivo) error {
	query := `
		INSERT INTO sessoes_dispositivo (usuario_id, empresa_id, device_id)
		VALUES ($1, $2, $3)
		RETURNING id, criado_em
	`

	return r.db.QueryRow(ctx, query,
		s.UsuarioID, s.EmpresaID, s.DeviceID,
	).Scan(&s.ID, &s.CriadoEm)
}

func (r *SessaoDispositivoRepository) DeleteByDeviceID(ctx context.Context, empresaID, deviceID string) error {
	query := `DELETE FROM sessoes_dispositivo WHERE device_id = $1 AND empresa_id = $2::uuid`

	ct, err := r.db.Exec(ctx, query, deviceID, empresaID)
	if err != nil {
		return fmt.Errorf("remover sessao dispositivo: %w", err)
	}

	if ct.RowsAffected() == 0 {
		return fmt.Errorf("sessao dispositivo nao encontrada")
	}

	return nil
}
