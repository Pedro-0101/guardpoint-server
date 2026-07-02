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
	query := `
		INSERT INTO config_escalonamento (empresa_id, nivel, atraso_minutos, whatsapp_para, cargo_alvo)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, created_at
	`
	return r.db.QueryRow(ctx, query,
		c.EmpresaID, c.Nivel, c.AtrasoMinutos, c.WhatsappPara, c.CargoAlvo,
	).Scan(&c.ID, &c.CreatedAt)
}

func (r *ConfigEscalonamentoRepository) FindByEmpresa(ctx context.Context, empresaID uuid.UUID) ([]model.ConfigEscalonamento, error) {
	query := `
		SELECT id, empresa_id, nivel, atraso_minutos, whatsapp_para, cargo_alvo, created_at
		FROM config_escalonamento
		WHERE empresa_id = $1
		ORDER BY nivel ASC
	`
	rows, err := r.db.Query(ctx, query, empresaID)
	if err != nil {
		return nil, fmt.Errorf("listar config escalonamento: %w", err)
	}
	defer rows.Close()

	var configs []model.ConfigEscalonamento
	for rows.Next() {
		var c model.ConfigEscalonamento
		if err := rows.Scan(
			&c.ID, &c.EmpresaID, &c.Nivel, &c.AtrasoMinutos,
			&c.WhatsappPara, &c.CargoAlvo, &c.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan config escalonamento: %w", err)
		}
		configs = append(configs, c)
	}
	return configs, rows.Err()
}

func (r *ConfigEscalonamentoRepository) FindByEmpresaENivel(ctx context.Context, empresaID uuid.UUID, nivel int) (*model.ConfigEscalonamento, error) {
	query := `
		SELECT id, empresa_id, nivel, atraso_minutos, whatsapp_para, cargo_alvo, created_at
		FROM config_escalonamento
		WHERE empresa_id = $1 AND nivel = $2
	`
	var c model.ConfigEscalonamento
	err := r.db.QueryRow(ctx, query, empresaID, nivel).Scan(
		&c.ID, &c.EmpresaID, &c.Nivel, &c.AtrasoMinutos,
		&c.WhatsappPara, &c.CargoAlvo, &c.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("buscar config escalonamento: %w", err)
	}
	return &c, nil
}

func (r *ConfigEscalonamentoRepository) Update(ctx context.Context, id, empresaID uuid.UUID, c *model.ConfigEscalonamento) error {
	query := `
		UPDATE config_escalonamento
		SET atraso_minutos = $1, whatsapp_para = $2, cargo_alvo = $3
		WHERE id = $4 AND empresa_id = $5
		RETURNING id, empresa_id, nivel, atraso_minutos, whatsapp_para, cargo_alvo, created_at
	`
	return r.db.QueryRow(ctx, query,
		c.AtrasoMinutos, c.WhatsappPara, c.CargoAlvo, id, empresaID,
	).Scan(&c.ID, &c.EmpresaID, &c.Nivel, &c.AtrasoMinutos,
		&c.WhatsappPara, &c.CargoAlvo, &c.CreatedAt)
}

func (r *ConfigEscalonamentoRepository) Upsert(ctx context.Context, c *model.ConfigEscalonamento) error {
	query := `
		INSERT INTO config_escalonamento (empresa_id, nivel, atraso_minutos, whatsapp_para, cargo_alvo)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (empresa_id, nivel)
		DO UPDATE SET atraso_minutos = $3, whatsapp_para = $4, cargo_alvo = $5
		RETURNING id, created_at
	`
	return r.db.QueryRow(ctx, query,
		c.EmpresaID, c.Nivel, c.AtrasoMinutos, c.WhatsappPara, c.CargoAlvo,
	).Scan(&c.ID, &c.CreatedAt)
}

func (r *ConfigEscalonamentoRepository) Delete(ctx context.Context, id, empresaID uuid.UUID) error {
	query := `DELETE FROM config_escalonamento WHERE id = $1 AND empresa_id = $2`
	ct, err := r.db.Exec(ctx, query, id, empresaID)
	if err != nil {
		return fmt.Errorf("deletar config escalonamento: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return fmt.Errorf("config escalonamento nao encontrado")
	}
	return nil
}

func (r *ConfigEscalonamentoRepository) DeleteByEmpresa(ctx context.Context, empresaID uuid.UUID) error {
	_, err := r.db.Exec(ctx, `DELETE FROM config_escalonamento WHERE empresa_id = $1`, empresaID)
	if err != nil {
		return fmt.Errorf("deletar configs escalonamento: %w", err)
	}
	return nil
}

func (r *ConfigEscalonamentoRepository) ReplaceByEmpresa(ctx context.Context, empresaID uuid.UUID, configs []model.ConfigEscalonamento) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("iniciar transacao: %w", err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `DELETE FROM config_escalonamento WHERE empresa_id = $1`, empresaID); err != nil {
		return fmt.Errorf("deletar configs existentes: %w", err)
	}

	for i := range configs {
		configs[i].EmpresaID = empresaID
		query := `
			INSERT INTO config_escalonamento (empresa_id, nivel, atraso_minutos, whatsapp_para, cargo_alvo)
			VALUES ($1, $2, $3, $4, $5)
		`
		if _, err := tx.Exec(ctx, query,
			configs[i].EmpresaID, configs[i].Nivel, configs[i].AtrasoMinutos,
			configs[i].WhatsappPara, configs[i].CargoAlvo,
		); err != nil {
			return fmt.Errorf("inserir config nivel %d: %w", configs[i].Nivel, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commitar transacao: %w", err)
	}
	return nil
}
