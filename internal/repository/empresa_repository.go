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

type EmpresaRepository struct {
	db *pgxpool.Pool
}

func NewEmpresaRepository(db *pgxpool.Pool) *EmpresaRepository {
	return &EmpresaRepository{db: db}
}

func (r *EmpresaRepository) FindByCNPJ(ctx context.Context, cnpj string) (*model.Empresa, error) {
	query := `
		SELECT id, nome, cnpj, ativa, created_at
		FROM empresas
		WHERE cnpj = $1
	`

	var e model.Empresa
	err := r.db.QueryRow(ctx, query, cnpj).Scan(
		&e.ID, &e.Nome, &e.CNPJ, &e.Ativa, &e.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("empresa nao encontrada: %w", err)
		}
		return nil, fmt.Errorf("buscar empresa por cnpj: %w", err)
	}

	return &e, nil
}

func (r *EmpresaRepository) Create(ctx context.Context, e *model.Empresa) error {
	query := `
		INSERT INTO empresas (nome, cnpj)
		VALUES ($1, $2)
		RETURNING id, ativa, created_at
	`

	return r.db.QueryRow(ctx, query, e.Nome, e.CNPJ).Scan(
		&e.ID, &e.Ativa, &e.CreatedAt,
	)
}

func (r *EmpresaRepository) FindByID(ctx context.Context, id uuid.UUID) (*model.Empresa, error) {
	query := `
		SELECT id, nome, cnpj, ativa, alerta_sonoro, created_at, updated_at
		FROM empresas
		WHERE id = $1
	`
	var e model.Empresa
	err := r.db.QueryRow(ctx, query, id).Scan(
		&e.ID, &e.Nome, &e.CNPJ, &e.Ativa, &e.AlertaSonoro, &e.CreatedAt, &e.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("empresa nao encontrada: %w", err)
		}
		return nil, fmt.Errorf("buscar empresa por id: %w", err)
	}
	return &e, nil
}

func (r *EmpresaRepository) Update(ctx context.Context, id uuid.UUID, nome string, alertaSonoro bool) (*model.Empresa, error) {
	query := `
		UPDATE empresas
		SET nome = $1, alerta_sonoro = $2, updated_at = now()
		WHERE id = $3
		RETURNING id, nome, cnpj, ativa, alerta_sonoro, created_at, updated_at
	`
	var e model.Empresa
	err := r.db.QueryRow(ctx, query, nome, alertaSonoro, id).Scan(
		&e.ID, &e.Nome, &e.CNPJ, &e.Ativa, &e.AlertaSonoro, &e.CreatedAt, &e.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("atualizar empresa: %w", err)
	}
	return &e, nil
}
