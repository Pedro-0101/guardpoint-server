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

type SenhaVigiaRepository struct {
	db *pgxpool.Pool
}

func NewSenhaVigiaRepository(db *pgxpool.Pool) *SenhaVigiaRepository {
	return &SenhaVigiaRepository{db: db}
}

func (r *SenhaVigiaRepository) Create(ctx context.Context, s *model.SenhaVigia) error {
	query := `
		INSERT INTO senhas_vigia (empresa_id, usuario_id, tipo, codigo, nivel_escalonamento_id)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, created_at, updated_at
	`
	if err := r.db.QueryRow(ctx, query,
		s.EmpresaID, s.UsuarioID, s.Tipo, s.Codigo, s.NivelEscalonamentoID,
	).Scan(&s.ID, &s.CreatedAt, &s.UpdatedAt); err != nil {
		return fmt.Errorf("criar senha vigia: %w", err)
	}
	return nil
}

func (r *SenhaVigiaRepository) ListByUsuario(ctx context.Context, empresaID, usuarioID uuid.UUID) ([]model.SenhaVigia, error) {
	query := `
		SELECT id, empresa_id, usuario_id, tipo, codigo, nivel_escalonamento_id, created_at, updated_at
		FROM senhas_vigia
		WHERE empresa_id = $1 AND usuario_id = $2
		ORDER BY tipo ASC, created_at ASC
	`
	rows, err := r.db.Query(ctx, query, empresaID, usuarioID)
	if err != nil {
		return nil, fmt.Errorf("listar senhas vigia: %w", err)
	}
	defer rows.Close()

	var senhas []model.SenhaVigia
	for rows.Next() {
		var s model.SenhaVigia
		if err := rows.Scan(
			&s.ID, &s.EmpresaID, &s.UsuarioID, &s.Tipo, &s.Codigo, &s.NivelEscalonamentoID,
			&s.CreatedAt, &s.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan senha vigia: %w", err)
		}
		senhas = append(senhas, s)
	}
	return senhas, rows.Err()
}

// ListByEmpresa retorna todas as senhas da empresa, independente do vigia --
// usado para validar unicidade de nivel_escalonamento_id entre vigias.
func (r *SenhaVigiaRepository) ListByEmpresa(ctx context.Context, empresaID uuid.UUID) ([]model.SenhaVigia, error) {
	query := `
		SELECT id, empresa_id, usuario_id, tipo, codigo, nivel_escalonamento_id, created_at, updated_at
		FROM senhas_vigia
		WHERE empresa_id = $1
	`
	rows, err := r.db.Query(ctx, query, empresaID)
	if err != nil {
		return nil, fmt.Errorf("listar senhas vigia da empresa: %w", err)
	}
	defer rows.Close()

	var senhas []model.SenhaVigia
	for rows.Next() {
		var s model.SenhaVigia
		if err := rows.Scan(
			&s.ID, &s.EmpresaID, &s.UsuarioID, &s.Tipo, &s.Codigo, &s.NivelEscalonamentoID,
			&s.CreatedAt, &s.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan senha vigia: %w", err)
		}
		senhas = append(senhas, s)
	}
	return senhas, rows.Err()
}

func (r *SenhaVigiaRepository) FindByID(ctx context.Context, empresaID, id uuid.UUID) (*model.SenhaVigia, error) {
	query := `
		SELECT id, empresa_id, usuario_id, tipo, codigo, nivel_escalonamento_id, created_at, updated_at
		FROM senhas_vigia
		WHERE id = $1 AND empresa_id = $2
	`
	var s model.SenhaVigia
	err := r.db.QueryRow(ctx, query, id, empresaID).Scan(
		&s.ID, &s.EmpresaID, &s.UsuarioID, &s.Tipo, &s.Codigo, &s.NivelEscalonamentoID,
		&s.CreatedAt, &s.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("buscar senha vigia: %w", err)
	}
	return &s, nil
}

// FindByUsuarioECodigo retorna (nil, nil) se nao achar (nao e erro) -- usado na
// resolucao do check-in, onde "nao achou" e um caso de negocio normal, nao uma falha.
func (r *SenhaVigiaRepository) FindByUsuarioECodigo(ctx context.Context, empresaID, usuarioID uuid.UUID, codigo string) (*model.SenhaVigia, error) {
	query := `
		SELECT id, empresa_id, usuario_id, tipo, codigo, nivel_escalonamento_id, created_at, updated_at
		FROM senhas_vigia
		WHERE empresa_id = $1 AND usuario_id = $2 AND codigo = $3
	`
	var s model.SenhaVigia
	err := r.db.QueryRow(ctx, query, empresaID, usuarioID, codigo).Scan(
		&s.ID, &s.EmpresaID, &s.UsuarioID, &s.Tipo, &s.Codigo, &s.NivelEscalonamentoID,
		&s.CreatedAt, &s.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("buscar senha vigia por codigo: %w", err)
	}
	return &s, nil
}

func (r *SenhaVigiaRepository) CountByUsuario(ctx context.Context, empresaID, usuarioID uuid.UUID) (int, error) {
	query := `SELECT COUNT(*) FROM senhas_vigia WHERE empresa_id = $1 AND usuario_id = $2`
	var count int
	if err := r.db.QueryRow(ctx, query, empresaID, usuarioID).Scan(&count); err != nil {
		return 0, fmt.Errorf("contar senhas vigia: %w", err)
	}
	return count, nil
}

func (r *SenhaVigiaRepository) Update(ctx context.Context, id, empresaID uuid.UUID, s *model.SenhaVigia) error {
	query := `
		UPDATE senhas_vigia
		SET codigo = $1, nivel_escalonamento_id = $2, updated_at = now()
		WHERE id = $3 AND empresa_id = $4
		RETURNING id, empresa_id, usuario_id, tipo, codigo, nivel_escalonamento_id, created_at, updated_at
	`
	err := r.db.QueryRow(ctx, query, s.Codigo, s.NivelEscalonamentoID, id, empresaID).Scan(
		&s.ID, &s.EmpresaID, &s.UsuarioID, &s.Tipo, &s.Codigo, &s.NivelEscalonamentoID,
		&s.CreatedAt, &s.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("atualizar senha vigia: %w", err)
	}
	return nil
}

func (r *SenhaVigiaRepository) Delete(ctx context.Context, id, empresaID uuid.UUID) error {
	query := `DELETE FROM senhas_vigia WHERE id = $1 AND empresa_id = $2`
	ct, err := r.db.Exec(ctx, query, id, empresaID)
	if err != nil {
		return fmt.Errorf("deletar senha vigia: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return fmt.Errorf("senha vigia nao encontrada")
	}
	return nil
}
