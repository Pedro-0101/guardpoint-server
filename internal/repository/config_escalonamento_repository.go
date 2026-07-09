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

type ConfigEscalonamentoRepository struct {
	db *pgxpool.Pool
}

func NewConfigEscalonamentoRepository(db *pgxpool.Pool) *ConfigEscalonamentoRepository {
	return &ConfigEscalonamentoRepository{db: db}
}

func (r *ConfigEscalonamentoRepository) Create(ctx context.Context, c *model.ConfigEscalonamento) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("iniciar transacao: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	query := `
		INSERT INTO config_escalonamento (empresa_id, atraso_minutos, descricao)
		VALUES ($1, $2, $3)
		RETURNING id, created_at
	`
	if err := tx.QueryRow(ctx, query, c.EmpresaID, c.AtrasoMinutos, c.Descricao).Scan(&c.ID, &c.CreatedAt); err != nil {
		return fmt.Errorf("criar config escalonamento: %w", err)
	}

	if err := inserirDestinatariosEscalonamento(ctx, tx, c.ID, c.UsuarioIDs); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func (r *ConfigEscalonamentoRepository) FindByEmpresa(ctx context.Context, empresaID uuid.UUID) (*model.ConfigEscalonamento, error) {
	var c model.ConfigEscalonamento
	err := r.db.QueryRow(ctx, `
		SELECT id, empresa_id, atraso_minutos, descricao, created_at
		FROM config_escalonamento
		WHERE empresa_id = $1
	`, empresaID).Scan(&c.ID, &c.EmpresaID, &c.AtrasoMinutos, &c.Descricao, &c.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("buscar config escalonamento: %w", err)
	}

	rows, err := r.db.Query(ctx, `SELECT usuario_id FROM config_escalonamento_destinatarios WHERE config_escalonamento_id = $1`, c.ID)
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

func (r *ConfigEscalonamentoRepository) Upsert(ctx context.Context, c *model.ConfigEscalonamento) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("iniciar transacao: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	query := `
		INSERT INTO config_escalonamento (empresa_id, atraso_minutos, descricao)
		VALUES ($1, $2, $3)
		ON CONFLICT (empresa_id)
		DO UPDATE SET atraso_minutos = $2, descricao = $3
		RETURNING id, created_at
	`
	if err := tx.QueryRow(ctx, query, c.EmpresaID, c.AtrasoMinutos, c.Descricao).Scan(&c.ID, &c.CreatedAt); err != nil {
		return fmt.Errorf("atualizar config escalonamento: %w", err)
	}

	if _, err := tx.Exec(ctx, `DELETE FROM config_escalonamento_destinatarios WHERE config_escalonamento_id = $1`, c.ID); err != nil {
		return fmt.Errorf("limpar destinatarios: %w", err)
	}
	if err := inserirDestinatariosEscalonamento(ctx, tx, c.ID, c.UsuarioIDs); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func (r *ConfigEscalonamentoRepository) Delete(ctx context.Context, empresaID uuid.UUID) error {
	_, err := r.db.Exec(ctx, `DELETE FROM config_escalonamento WHERE empresa_id = $1`, empresaID)
	if err != nil {
		return fmt.Errorf("deletar config escalonamento: %w", err)
	}
	return nil
}

func inserirDestinatariosEscalonamento(ctx context.Context, tx pgx.Tx, configID uuid.UUID, usuarioIDs []uuid.UUID) error {
	for _, usuarioID := range usuarioIDs {
		if _, err := tx.Exec(ctx, `
			INSERT INTO config_escalonamento_destinatarios (config_escalonamento_id, usuario_id)
			VALUES ($1, $2)
		`, configID, usuarioID); err != nil {
			return fmt.Errorf("inserir destinatario de escalonamento: %w", err)
		}
	}
	return nil
}
