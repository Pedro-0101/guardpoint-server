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

type ConfigAlertaEmergenciaRepository struct {
	db *pgxpool.Pool
}

func NewConfigAlertaEmergenciaRepository(db *pgxpool.Pool) *ConfigAlertaEmergenciaRepository {
	return &ConfigAlertaEmergenciaRepository{db: db}
}

func (r *ConfigAlertaEmergenciaRepository) FindByEmpresa(ctx context.Context, empresaID uuid.UUID) ([]model.ConfigAlertaEmergencia, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, empresa_id, tipo, created_at
		FROM config_alerta_emergencia
		WHERE empresa_id = $1
		ORDER BY tipo ASC
	`, empresaID)
	if err != nil {
		return nil, fmt.Errorf("listar config alerta emergencia: %w", err)
	}
	defer rows.Close()

	var configs []model.ConfigAlertaEmergencia
	for rows.Next() {
		var c model.ConfigAlertaEmergencia
		if err := rows.Scan(&c.ID, &c.EmpresaID, &c.Tipo, &c.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan config alerta emergencia: %w", err)
		}
		configs = append(configs, c)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	destinatarios, err := r.destinatariosPorEmpresa(ctx, empresaID)
	if err != nil {
		return nil, err
	}
	for i := range configs {
		configs[i].UsuarioIDs = destinatarios[configs[i].ID]
	}
	return configs, nil
}

func (r *ConfigAlertaEmergenciaRepository) destinatariosPorEmpresa(ctx context.Context, empresaID uuid.UUID) (map[uuid.UUID][]uuid.UUID, error) {
	rows, err := r.db.Query(ctx, `
		SELECT d.config_alerta_emergencia_id, d.usuario_id
		FROM config_alerta_emergencia_destinatarios d
		JOIN config_alerta_emergencia c ON c.id = d.config_alerta_emergencia_id
		WHERE c.empresa_id = $1
	`, empresaID)
	if err != nil {
		return nil, fmt.Errorf("listar destinatarios de emergencia: %w", err)
	}
	defer rows.Close()

	result := make(map[uuid.UUID][]uuid.UUID)
	for rows.Next() {
		var configID, usuarioID uuid.UUID
		if err := rows.Scan(&configID, &usuarioID); err != nil {
			return nil, fmt.Errorf("scan destinatario de emergencia: %w", err)
		}
		result[configID] = append(result[configID], usuarioID)
	}
	return result, rows.Err()
}

func (r *ConfigAlertaEmergenciaRepository) FindByEmpresaETipo(ctx context.Context, empresaID uuid.UUID, tipo string) (*model.ConfigAlertaEmergencia, error) {
	var c model.ConfigAlertaEmergencia
	err := r.db.QueryRow(ctx, `
		SELECT id, empresa_id, tipo, created_at
		FROM config_alerta_emergencia
		WHERE empresa_id = $1 AND tipo = $2
	`, empresaID, tipo).Scan(&c.ID, &c.EmpresaID, &c.Tipo, &c.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("buscar config alerta emergencia: %w", err)
	}

	rows, err := r.db.Query(ctx, `SELECT usuario_id FROM config_alerta_emergencia_destinatarios WHERE config_alerta_emergencia_id = $1`, c.ID)
	if err != nil {
		return nil, fmt.Errorf("listar destinatarios: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var usuarioID uuid.UUID
		if err := rows.Scan(&usuarioID); err != nil {
			return nil, fmt.Errorf("scan destinatario: %w", err)
		}
		c.UsuarioIDs = append(c.UsuarioIDs, usuarioID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return &c, nil
}

func (r *ConfigAlertaEmergenciaRepository) Upsert(ctx context.Context, c *model.ConfigAlertaEmergencia) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("iniciar transacao: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := tx.QueryRow(ctx, `
		INSERT INTO config_alerta_emergencia (empresa_id, tipo)
		VALUES ($1, $2)
		ON CONFLICT (empresa_id, tipo) DO UPDATE SET tipo = EXCLUDED.tipo
		RETURNING id, created_at
	`, c.EmpresaID, c.Tipo).Scan(&c.ID, &c.CreatedAt); err != nil {
		return fmt.Errorf("upsert config alerta emergencia: %w", err)
	}

	if _, err := tx.Exec(ctx, `DELETE FROM config_alerta_emergencia_destinatarios WHERE config_alerta_emergencia_id = $1`, c.ID); err != nil {
		return fmt.Errorf("limpar destinatarios: %w", err)
	}

	for _, usuarioID := range c.UsuarioIDs {
		if _, err := tx.Exec(ctx, `
			INSERT INTO config_alerta_emergencia_destinatarios (config_alerta_emergencia_id, usuario_id)
			VALUES ($1, $2)
		`, c.ID, usuarioID); err != nil {
			return fmt.Errorf("inserir destinatario: %w", err)
		}
	}

	return tx.Commit(ctx)
}
