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

type PostoRepository struct {
	db *pgxpool.Pool
}

func NewPostoRepository(db *pgxpool.Pool) *PostoRepository {
	return &PostoRepository{db: db}
}

func (r *PostoRepository) Create(ctx context.Context, p *model.Posto) error {
	query := `
		INSERT INTO postos (empresa_id, nome, latitude, longitude, raio_m)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, ativo, created_at
	`
	return r.db.QueryRow(ctx, query,
		p.EmpresaID, p.Nome, p.Latitude, p.Longitude, p.RaioM,
	).Scan(&p.ID, &p.Ativo, &p.CreatedAt)
}

func (r *PostoRepository) FindByID(ctx context.Context, empresaID, id uuid.UUID) (*model.Posto, error) {
	query := `
		SELECT id, empresa_id, nome, latitude, longitude, raio_m, ativo, created_at
		FROM postos
		WHERE id = $1 AND empresa_id = $2
	`
	var p model.Posto
	err := r.db.QueryRow(ctx, query, id, empresaID).Scan(
		&p.ID, &p.EmpresaID, &p.Nome, &p.Latitude, &p.Longitude,
		&p.RaioM, &p.Ativo, &p.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("posto nao encontrado: %w", err)
		}
		return nil, fmt.Errorf("buscar posto: %w", err)
	}
	return &p, nil
}

func (r *PostoRepository) List(ctx context.Context, empresaID uuid.UUID, apenasAtivos bool) ([]model.Posto, error) {
	query := `
		SELECT id, empresa_id, nome, latitude, longitude, raio_m, ativo, created_at
		FROM postos
		WHERE empresa_id = $1
	`
	if apenasAtivos {
		query += ` AND ativo = true`
	}
	query += ` ORDER BY nome`

	rows, err := r.db.Query(ctx, query, empresaID)
	if err != nil {
		return nil, fmt.Errorf("listar postos: %w", err)
	}
	defer rows.Close()

	var postos []model.Posto
	for rows.Next() {
		var p model.Posto
		if err := rows.Scan(
			&p.ID, &p.EmpresaID, &p.Nome, &p.Latitude, &p.Longitude,
			&p.RaioM, &p.Ativo, &p.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan posto: %w", err)
		}
		postos = append(postos, p)
	}
	return postos, rows.Err()
}

func (r *PostoRepository) Update(ctx context.Context, empresaID, id uuid.UUID, p *model.Posto) error {
	query := `
		UPDATE postos
		SET nome = $1, latitude = $2, longitude = $3, raio_m = $4, ativo = $5
		WHERE id = $6 AND empresa_id = $7
		RETURNING id, empresa_id, nome, latitude, longitude, raio_m, ativo, created_at
	`
	return r.db.QueryRow(ctx, query,
		p.Nome, p.Latitude, p.Longitude, p.RaioM, p.Ativo, id, empresaID,
	).Scan(&p.ID, &p.EmpresaID, &p.Nome, &p.Latitude, &p.Longitude,
		&p.RaioM, &p.Ativo, &p.CreatedAt)
}
