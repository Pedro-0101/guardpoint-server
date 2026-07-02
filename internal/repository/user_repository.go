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

type UserRepository struct {
	db *pgxpool.Pool
}

func NewUserRepository(db *pgxpool.Pool) *UserRepository {
	return &UserRepository{db: db}
}

func (r *UserRepository) FindByEmail(ctx context.Context, email string) (*model.User, error) {
	query := `
		SELECT id, empresa_id, nome, email, senha_hash, role, telefone, ativo, created_at, updated_at
		FROM usuarios
		WHERE email = $1
	`

	var u model.User
	err := r.db.QueryRow(ctx, query, email).Scan(
		&u.ID, &u.EmpresaID, &u.Nome, &u.Email, &u.SenhaHash,
		&u.Role, &u.Telefone, &u.Ativo, &u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("usuario nao encontrado: %w", err)
		}
		return nil, fmt.Errorf("buscar usuario por email: %w", err)
	}

	return &u, nil
}

func (r *UserRepository) FindByID(ctx context.Context, id uuid.UUID) (*model.User, error) {
	query := `
		SELECT id, empresa_id, nome, email, senha_hash, role, telefone, ativo, created_at, updated_at
		FROM usuarios
		WHERE id = $1
	`

	var u model.User
	err := r.db.QueryRow(ctx, query, id).Scan(
		&u.ID, &u.EmpresaID, &u.Nome, &u.Email, &u.SenhaHash,
		&u.Role, &u.Telefone, &u.Ativo, &u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("usuario nao encontrado: %w", err)
		}
		return nil, fmt.Errorf("buscar usuario por id: %w", err)
	}

	return &u, nil
}

func (r *UserRepository) FindByIDEmpresa(ctx context.Context, empresaID, id uuid.UUID) (*model.User, error) {
	query := `
		SELECT id, empresa_id, nome, email, senha_hash, role, telefone, ativo, created_at, updated_at
		FROM usuarios
		WHERE id = $1 AND empresa_id = $2
	`

	var u model.User
	err := r.db.QueryRow(ctx, query, id, empresaID).Scan(
		&u.ID, &u.EmpresaID, &u.Nome, &u.Email, &u.SenhaHash,
		&u.Role, &u.Telefone, &u.Ativo, &u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("usuario nao encontrado: %w", err)
		}
		return nil, fmt.Errorf("buscar usuario por id e empresa: %w", err)
	}

	return &u, nil
}

func (r *UserRepository) Create(ctx context.Context, u *model.User) error {
	query := `
		INSERT INTO usuarios (empresa_id, nome, email, senha_hash, role, telefone)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, ativo, created_at, updated_at
	`

	return r.db.QueryRow(ctx, query,
		u.EmpresaID, u.Nome, u.Email, u.SenhaHash, u.Role, u.Telefone,
	).Scan(&u.ID, &u.Ativo, &u.CreatedAt, &u.UpdatedAt)
}

func (r *UserRepository) ListByEmpresa(ctx context.Context, empresaID uuid.UUID) ([]model.User, error) {
	query := `
		SELECT id, empresa_id, nome, email, senha_hash, role, telefone, ativo, created_at, updated_at
		FROM usuarios
		WHERE empresa_id = $1
		ORDER BY nome
	`

	rows, err := r.db.Query(ctx, query, empresaID)
	if err != nil {
		return nil, fmt.Errorf("listar usuarios: %w", err)
	}
	defer rows.Close()

	var usuarios []model.User
	for rows.Next() {
		var u model.User
		if err := rows.Scan(
			&u.ID, &u.EmpresaID, &u.Nome, &u.Email, &u.SenhaHash,
			&u.Role, &u.Telefone, &u.Ativo, &u.CreatedAt, &u.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan usuario: %w", err)
		}
		usuarios = append(usuarios, u)
	}
	return usuarios, rows.Err()
}

func (r *UserRepository) Update(ctx context.Context, empresaID, id uuid.UUID, u *model.User) error {
	query := `
		UPDATE usuarios
		SET nome = $1, email = $2, senha_hash = $3, role = $4, ativo = $5, updated_at = now()
		WHERE id = $6 AND empresa_id = $7
		RETURNING id, empresa_id, nome, email, senha_hash, role, telefone, ativo, created_at, updated_at
	`

	return r.db.QueryRow(ctx, query,
		u.Nome, u.Email, u.SenhaHash, u.Role, u.Ativo, id, empresaID,
	).Scan(
		&u.ID, &u.EmpresaID, &u.Nome, &u.Email, &u.SenhaHash,
		&u.Role, &u.Telefone, &u.Ativo, &u.CreatedAt, &u.UpdatedAt,
	)
}
