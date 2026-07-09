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
		INSERT INTO config_escalonamento (empresa_id, atraso_minutos, descricao, sistema)
		VALUES ($1, $2, $3, $4)
		RETURNING id, created_at
	`
	if err := tx.QueryRow(ctx, query, c.EmpresaID, c.AtrasoMinutos, c.Descricao, c.Sistema).Scan(&c.ID, &c.CreatedAt); err != nil {
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
		SELECT id, empresa_id, atraso_minutos, descricao, sistema, created_at
		FROM config_escalonamento
		WHERE empresa_id = $1
		ORDER BY created_at ASC
		LIMIT 1
	`, empresaID).Scan(&c.ID, &c.EmpresaID, &c.AtrasoMinutos, &c.Descricao, &c.Sistema, &c.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("buscar config escalonamento: %w", err)
	}

	c.UsuarioIDs = r.listarDestinatarios(ctx, c.ID)
	return &c, nil
}

func (r *ConfigEscalonamentoRepository) FindByIDEmpresa(ctx context.Context, id, empresaID uuid.UUID) (*model.ConfigEscalonamento, error) {
	var c model.ConfigEscalonamento
	err := r.db.QueryRow(ctx, `
		SELECT id, empresa_id, atraso_minutos, descricao, sistema, created_at
		FROM config_escalonamento
		WHERE id = $1 AND empresa_id = $2
	`, id, empresaID).Scan(&c.ID, &c.EmpresaID, &c.AtrasoMinutos, &c.Descricao, &c.Sistema, &c.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("buscar config escalonamento por id: %w", err)
	}

	c.UsuarioIDs = r.listarDestinatarios(ctx, c.ID)
	return &c, nil
}

func (r *ConfigEscalonamentoRepository) ListByEmpresa(ctx context.Context, empresaID uuid.UUID) ([]model.ConfigEscalonamento, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, empresa_id, atraso_minutos, descricao, sistema, created_at
		FROM config_escalonamento
		WHERE empresa_id = $1
		ORDER BY created_at ASC
	`, empresaID)
	if err != nil {
		return nil, fmt.Errorf("listar configs escalonamento: %w", err)
	}
	defer rows.Close()

	var configs []model.ConfigEscalonamento
	for rows.Next() {
		var c model.ConfigEscalonamento
		if err := rows.Scan(&c.ID, &c.EmpresaID, &c.AtrasoMinutos, &c.Descricao, &c.Sistema, &c.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan config escalonamento: %w", err)
		}
		c.UsuarioIDs = r.listarDestinatarios(ctx, c.ID)
		configs = append(configs, c)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if configs == nil {
		configs = []model.ConfigEscalonamento{}
	}
	return configs, nil
}

func (r *ConfigEscalonamentoRepository) Update(ctx context.Context, c *model.ConfigEscalonamento) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("iniciar transacao: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	query := `
		UPDATE config_escalonamento
		SET atraso_minutos = $1, descricao = $2
		WHERE id = $3 AND empresa_id = $4
	`
	tag, err := tx.Exec(ctx, query, c.AtrasoMinutos, c.Descricao, c.ID, c.EmpresaID)
	if err != nil {
		return fmt.Errorf("atualizar config escalonamento: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("config escalonamento nao encontrada")
	}

	if _, err := tx.Exec(ctx, `DELETE FROM config_escalonamento_destinatarios WHERE config_escalonamento_id = $1`, c.ID); err != nil {
		return fmt.Errorf("limpar destinatarios: %w", err)
	}
	if err := inserirDestinatariosEscalonamento(ctx, tx, c.ID, c.UsuarioIDs); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func (r *ConfigEscalonamentoRepository) UpdateUsuarios(ctx context.Context, configID uuid.UUID, usuarioIDs []uuid.UUID) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("iniciar transacao: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, `DELETE FROM config_escalonamento_destinatarios WHERE config_escalonamento_id = $1`, configID); err != nil {
		return fmt.Errorf("limpar destinatarios: %w", err)
	}
	if err := inserirDestinatariosEscalonamento(ctx, tx, configID, usuarioIDs); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func (r *ConfigEscalonamentoRepository) DeleteByID(ctx context.Context, id, empresaID uuid.UUID) error {
	tag, err := r.db.Exec(ctx, `DELETE FROM config_escalonamento WHERE id = $1 AND empresa_id = $2`, id, empresaID)
	if err != nil {
		return fmt.Errorf("deletar config escalonamento: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("config escalonamento nao encontrada")
	}
	return nil
}

func (r *ConfigEscalonamentoRepository) listarDestinatarios(ctx context.Context, configID uuid.UUID) []uuid.UUID {
	rows, err := r.db.Query(ctx, `SELECT usuario_id FROM config_escalonamento_destinatarios WHERE config_escalonamento_id = $1`, configID)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var ids []uuid.UUID
	for rows.Next() {
		var usuarioID uuid.UUID
		if err := rows.Scan(&usuarioID); err != nil {
			return nil
		}
		ids = append(ids, usuarioID)
	}
	if ids == nil {
		ids = []uuid.UUID{}
	}
	return ids
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
