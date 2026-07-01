package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/guardpoint/guardpoint-server/internal/model"
)

type TurnoRepository struct {
	db *pgxpool.Pool
}

func NewTurnoRepository(db *pgxpool.Pool) *TurnoRepository {
	return &TurnoRepository{db: db}
}

func (r *TurnoRepository) Create(ctx context.Context, t *model.Turno) error {
	query := `
		INSERT INTO turnos (empresa_id, usuario_id, posto_id, status, inicio_previsto, fim_previsto, inicio_real, token_sessao, intervalo_min)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id, created_at
	`
	return r.db.QueryRow(ctx, query,
		t.EmpresaID, t.UsuarioID, t.PostoID, t.Status,
		t.InicioPrevisto, t.FimPrevisto, t.InicioReal,
		t.TokenSessao, t.IntervaloMin,
	).Scan(&t.ID, &t.CreatedAt)
}

func (r *TurnoRepository) FindAtivoByUsuario(ctx context.Context, empresaID, usuarioID uuid.UUID) (*model.Turno, error) {
	query := `
		SELECT id, empresa_id, usuario_id, posto_id, status, inicio_previsto, fim_previsto,
		       inicio_real, fim_real, token_sessao, intervalo_min, created_at
		FROM turnos
		WHERE empresa_id = $1 AND usuario_id = $2 AND status IN ('em_andamento', 'pausado', 'critico')
		LIMIT 1
	`
	var t model.Turno
	err := r.db.QueryRow(ctx, query, empresaID, usuarioID).Scan(
		&t.ID, &t.EmpresaID, &t.UsuarioID, &t.PostoID, &t.Status,
		&t.InicioPrevisto, &t.FimPrevisto, &t.InicioReal, &t.FimReal,
		&t.TokenSessao, &t.IntervaloMin, &t.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("buscar turno ativo: %w", err)
	}
	return &t, nil
}

func (r *TurnoRepository) FindByID(ctx context.Context, empresaID, id uuid.UUID) (*model.Turno, error) {
	query := `
		SELECT id, empresa_id, usuario_id, posto_id, status, inicio_previsto, fim_previsto,
		       inicio_real, fim_real, token_sessao, intervalo_min, created_at
		FROM turnos
		WHERE id = $1 AND empresa_id = $2
	`
	var t model.Turno
	err := r.db.QueryRow(ctx, query, id, empresaID).Scan(
		&t.ID, &t.EmpresaID, &t.UsuarioID, &t.PostoID, &t.Status,
		&t.InicioPrevisto, &t.FimPrevisto, &t.InicioReal, &t.FimReal,
		&t.TokenSessao, &t.IntervaloMin, &t.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("turno nao encontrado: %w", err)
		}
		return nil, fmt.Errorf("buscar turno: %w", err)
	}
	return &t, nil
}

func (r *TurnoRepository) UpdateStatus(ctx context.Context, id, empresaID uuid.UUID, status string, fimReal *time.Time) error {
	query := `UPDATE turnos SET status = $1, fim_real = $2 WHERE id = $3 AND empresa_id = $4`
	ct, err := r.db.Exec(ctx, query, status, fimReal, id, empresaID)
	if err != nil {
		return fmt.Errorf("atualizar status turno: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return fmt.Errorf("turno nao encontrado")
	}
	return nil
}

func (r *TurnoRepository) ListAtivos(ctx context.Context, empresaID uuid.UUID) ([]model.Turno, error) {
	query := `
		SELECT id, empresa_id, usuario_id, posto_id, status, inicio_previsto, fim_previsto,
		       inicio_real, fim_real, token_sessao, intervalo_min, created_at
		FROM turnos
		WHERE empresa_id = $1 AND status IN ('em_andamento', 'pausado', 'critico')
		ORDER BY inicio_real DESC
	`
	rows, err := r.db.Query(ctx, query, empresaID)
	if err != nil {
		return nil, fmt.Errorf("listar turnos ativos: %w", err)
	}
	defer rows.Close()

	var turnos []model.Turno
	for rows.Next() {
		var t model.Turno
		if err := rows.Scan(
			&t.ID, &t.EmpresaID, &t.UsuarioID, &t.PostoID, &t.Status,
			&t.InicioPrevisto, &t.FimPrevisto, &t.InicioReal, &t.FimReal,
			&t.TokenSessao, &t.IntervaloMin, &t.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan turno: %w", err)
		}
		turnos = append(turnos, t)
	}
	return turnos, rows.Err()
}

func (r *TurnoRepository) ListHistorico(ctx context.Context, empresaID uuid.UUID, filter model.HistoricoFilter) ([]model.Turno, int, error) {
	where := "WHERE empresa_id = $1"
	args := []interface{}{empresaID}
	argIdx := 2

	if filter.Status != "" {
		where += fmt.Sprintf(" AND status = $%d", argIdx)
		args = append(args, filter.Status)
		argIdx++
	}
	if filter.UsuarioID != "" {
		where += fmt.Sprintf(" AND usuario_id = $%d::uuid", argIdx)
		args = append(args, filter.UsuarioID)
		argIdx++
	}
	if filter.PostoID != "" {
		where += fmt.Sprintf(" AND posto_id = $%d::uuid", argIdx)
		args = append(args, filter.PostoID)
		argIdx++
	}
	if filter.DataInicio != "" {
		where += fmt.Sprintf(" AND inicio_previsto >= $%d::timestamptz", argIdx)
		args = append(args, filter.DataInicio)
		argIdx++
	}
	if filter.DataFim != "" {
		where += fmt.Sprintf(" AND inicio_previsto <= $%d::timestamptz", argIdx)
		args = append(args, filter.DataFim)
		argIdx++
	}

	var total int
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM turnos %s", where)
	if err := r.db.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("contar turnos: %w", err)
	}

	if filter.Limit <= 0 {
		filter.Limit = 20
	}
	if filter.Offset < 0 {
		filter.Offset = 0
	}

	dataQuery := fmt.Sprintf(`
		SELECT id, empresa_id, usuario_id, posto_id, status, inicio_previsto, fim_previsto,
		       inicio_real, fim_real, token_sessao, intervalo_min, created_at
		FROM turnos %s ORDER BY created_at DESC LIMIT $%d OFFSET $%d
	`, where, argIdx, argIdx+1)
	args = append(args, filter.Limit, filter.Offset)

	rows, err := r.db.Query(ctx, dataQuery, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("listar historico turnos: %w", err)
	}
	defer rows.Close()

	var turnos []model.Turno
	for rows.Next() {
		var t model.Turno
		if err := rows.Scan(
			&t.ID, &t.EmpresaID, &t.UsuarioID, &t.PostoID, &t.Status,
			&t.InicioPrevisto, &t.FimPrevisto, &t.InicioReal, &t.FimReal,
			&t.TokenSessao, &t.IntervaloMin, &t.CreatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("scan turno: %w", err)
		}
		turnos = append(turnos, t)
	}
	return turnos, total, rows.Err()
}

func (r *TurnoRepository) RevogarToken(ctx context.Context, id, empresaID uuid.UUID) error {
	query := `UPDATE turnos SET token_sessao = NULL, status = 'finalizado', fim_real = now() WHERE id = $1 AND empresa_id = $2`
	ct, err := r.db.Exec(ctx, query, id, empresaID)
	if err != nil {
		return fmt.Errorf("revogar turno: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return fmt.Errorf("turno nao encontrado")
	}
	return nil
}
