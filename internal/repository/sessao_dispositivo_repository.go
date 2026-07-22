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

type SessaoDispositivoRepository struct {
	db *pgxpool.Pool
}

func NewSessaoDispositivoRepository(db *pgxpool.Pool) *SessaoDispositivoRepository {
	return &SessaoDispositivoRepository{db: db}
}

func (r *SessaoDispositivoRepository) FindByDeviceID(ctx context.Context, empresaID, deviceID string) (*model.SessaoDispositivo, error) {
	query := `
		SELECT id, usuario_id, empresa_id, device_id, device_secret_hash, criado_em
		FROM sessoes_dispositivo
		WHERE device_id = $1 AND empresa_id = $2::uuid
	`

	var s model.SessaoDispositivo
	err := r.db.QueryRow(ctx, query, deviceID, empresaID).Scan(
		&s.ID, &s.UsuarioID, &s.EmpresaID, &s.DeviceID, &s.DeviceSecretHash, &s.CriadoEm,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("sessao dispositivo nao encontrada: %w", err)
		}
		return nil, fmt.Errorf("buscar sessao dispositivo: %w", err)
	}

	return &s, nil
}

// Create faz upsert por (empresa_id, device_id): re-registrar biometria no
// mesmo aparelho atualiza o dono e o segredo em vez de duplicar a linha.
func (r *SessaoDispositivoRepository) Create(ctx context.Context, s *model.SessaoDispositivo) error {
	query := `
		INSERT INTO sessoes_dispositivo (usuario_id, empresa_id, device_id, device_secret_hash)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (empresa_id, device_id)
		DO UPDATE SET usuario_id = EXCLUDED.usuario_id,
		              device_secret_hash = EXCLUDED.device_secret_hash,
		              criado_em = now()
		RETURNING id, criado_em
	`

	return r.db.QueryRow(ctx, query,
		s.UsuarioID, s.EmpresaID, s.DeviceID, s.DeviceSecretHash,
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

func (r *SessaoDispositivoRepository) DeleteByUsuario(ctx context.Context, empresaID, usuarioID uuid.UUID) error {
	query := `DELETE FROM sessoes_dispositivo WHERE usuario_id = $1 AND empresa_id = $2`

	ct, err := r.db.Exec(ctx, query, usuarioID, empresaID)
	if err != nil {
		return fmt.Errorf("remover sessoes do usuario: %w", err)
	}

	_ = ct.RowsAffected()
	return nil
}
