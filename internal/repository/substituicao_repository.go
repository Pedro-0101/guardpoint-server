package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/guardpoint/guardpoint-server/internal/model"
)

type SubstituicaoRepository struct {
	db *pgxpool.Pool
}

func NewSubstituicaoRepository(db *pgxpool.Pool) *SubstituicaoRepository {
	return &SubstituicaoRepository{db: db}
}

func (r *SubstituicaoRepository) Create(ctx context.Context, s *model.Substituicao) error {
	query := `
		INSERT INTO substituicoes (empresa_id, usuario_id, posto_id, data_inicio, data_fim, hora_inicio, hora_fim, tolerancia_min, motivo)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id, ativo, created_at, updated_at
	`
	return r.db.QueryRow(ctx, query,
		s.EmpresaID, s.UsuarioID, s.PostoID, s.DataInicio, s.DataFim,
		s.HoraInicio, s.HoraFim, s.ToleranciaMin, s.Motivo,
	).Scan(&s.ID, &s.Ativo, &s.CreatedAt, &s.UpdatedAt)
}

func (r *SubstituicaoRepository) FindByID(ctx context.Context, empresaID, id uuid.UUID) (*model.Substituicao, error) {
	query := `
		SELECT s.id, s.empresa_id, s.usuario_id, s.posto_id,
		       s.data_inicio, s.data_fim, s.hora_inicio::text, s.hora_fim::text,
		       s.tolerancia_min, COALESCE(s.motivo, ''), s.ativo, s.created_at, s.updated_at,
		       u.nome AS usuario_nome, p.nome AS posto_nome
		FROM substituicoes s
		LEFT JOIN usuarios u ON u.id = s.usuario_id
		LEFT JOIN postos p ON p.id = s.posto_id
		WHERE s.id = $1 AND s.empresa_id = $2
	`
	var sub model.Substituicao
	err := r.db.QueryRow(ctx, query, id, empresaID).Scan(
		&sub.ID, &sub.EmpresaID, &sub.UsuarioID, &sub.PostoID,
		&sub.DataInicio, &sub.DataFim, &sub.HoraInicio, &sub.HoraFim,
		&sub.ToleranciaMin, &sub.Motivo, &sub.Ativo, &sub.CreatedAt, &sub.UpdatedAt,
		&sub.UsuarioNome, &sub.PostoNome,
	)
	if err != nil {
		return nil, fmt.Errorf("buscar substituicao: %w", err)
	}
	return &sub, nil
}

func (r *SubstituicaoRepository) List(ctx context.Context, empresaID uuid.UUID, filter model.SubstituicaoFilter) ([]model.Substituicao, int, error) {
	where := "WHERE s.empresa_id = $1"
	args := []interface{}{empresaID}
	argIdx := 2

	if filter.UsuarioID != "" {
		where += fmt.Sprintf(" AND s.usuario_id = $%d::uuid", argIdx)
		args = append(args, filter.UsuarioID)
		argIdx++
	}
	if filter.PostoID != "" {
		where += fmt.Sprintf(" AND s.posto_id = $%d::uuid", argIdx)
		args = append(args, filter.PostoID)
		argIdx++
	}
	if filter.Data != "" {
		where += fmt.Sprintf(" AND s.data_inicio <= $%d::date AND s.data_fim >= $%d::date", argIdx, argIdx+1)
		args = append(args, filter.Data, filter.Data)
		argIdx += 2
	}
	if filter.Ativo != nil {
		where += fmt.Sprintf(" AND s.ativo = $%d", argIdx)
		args = append(args, *filter.Ativo)
		argIdx++
	}

	var total int
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM substituicoes s %s", where)
	if err := r.db.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("contar substituicoes: %w", err)
	}

	if filter.Limit <= 0 {
		filter.Limit = 20
	}
	if filter.Offset < 0 {
		filter.Offset = 0
	}

	dataQuery := fmt.Sprintf(`
		SELECT s.id, s.empresa_id, s.usuario_id, s.posto_id,
		       s.data_inicio, s.data_fim, s.hora_inicio::text, s.hora_fim::text,
		       s.tolerancia_min, COALESCE(s.motivo, ''), s.ativo, s.created_at, s.updated_at,
		       u.nome AS usuario_nome, p.nome AS posto_nome
		FROM substituicoes s
		LEFT JOIN usuarios u ON u.id = s.usuario_id
		LEFT JOIN postos p ON p.id = s.posto_id
		%s
		ORDER BY s.data_inicio DESC, s.hora_inicio ASC
		LIMIT $%d OFFSET $%d
	`, where, argIdx, argIdx+1)
	args = append(args, filter.Limit, filter.Offset)

	rows, err := r.db.Query(ctx, dataQuery, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("listar substituicoes: %w", err)
	}
	defer rows.Close()

	var subs []model.Substituicao
	for rows.Next() {
		var sub model.Substituicao
		if err := rows.Scan(
			&sub.ID, &sub.EmpresaID, &sub.UsuarioID, &sub.PostoID,
			&sub.DataInicio, &sub.DataFim, &sub.HoraInicio, &sub.HoraFim,
			&sub.ToleranciaMin, &sub.Motivo, &sub.Ativo, &sub.CreatedAt, &sub.UpdatedAt,
			&sub.UsuarioNome, &sub.PostoNome,
		); err != nil {
			return nil, 0, fmt.Errorf("scan substituicao: %w", err)
		}
		subs = append(subs, sub)
	}
	return subs, total, rows.Err()
}

func (r *SubstituicaoRepository) Update(ctx context.Context, empresaID, id uuid.UUID, s *model.Substituicao) error {
	query := `
		UPDATE substituicoes
		SET usuario_id = $1, posto_id = $2,
		    data_inicio = $3, data_fim = $4,
		    hora_inicio = $5, hora_fim = $6,
		    tolerancia_min = $7, motivo = $8, ativo = $9,
		    updated_at = now()
		WHERE id = $10 AND empresa_id = $11
		RETURNING id, empresa_id, usuario_id, posto_id,
		          data_inicio, data_fim,
		          hora_inicio::text, hora_fim::text,
		          tolerancia_min, COALESCE(motivo, ''), ativo, created_at, updated_at
	`
	return r.db.QueryRow(ctx, query,
		s.UsuarioID, s.PostoID, s.DataInicio, s.DataFim,
		s.HoraInicio, s.HoraFim, s.ToleranciaMin, s.Motivo, s.Ativo,
		id, empresaID,
	).Scan(
		&s.ID, &s.EmpresaID, &s.UsuarioID, &s.PostoID,
		&s.DataInicio, &s.DataFim,
		&s.HoraInicio, &s.HoraFim,
		&s.ToleranciaMin, &s.Motivo, &s.Ativo, &s.CreatedAt, &s.UpdatedAt,
	)
}

func (r *SubstituicaoRepository) FindAtivaByUsuarioPostoData(ctx context.Context, empresaID, usuarioID, postoID uuid.UUID, data time.Time) (*model.Substituicao, error) {
	query := `
		SELECT id, empresa_id, usuario_id, posto_id,
		       data_inicio, data_fim, hora_inicio::text, hora_fim::text,
		       tolerancia_min, COALESCE(motivo, ''), ativo, created_at, updated_at
		FROM substituicoes
		WHERE empresa_id = $1
		  AND usuario_id = $2
		  AND posto_id = $3
		  AND ativo = true
		  AND data_inicio <= ($4 AT TIME ZONE 'America/Sao_Paulo')::date
		  AND data_fim >= ($4 AT TIME ZONE 'America/Sao_Paulo')::date
		LIMIT 1
	`
	var sub model.Substituicao
	err := r.db.QueryRow(ctx, query,
		empresaID, usuarioID, postoID, data,
	).Scan(
		&sub.ID, &sub.EmpresaID, &sub.UsuarioID, &sub.PostoID,
		&sub.DataInicio, &sub.DataFim, &sub.HoraInicio, &sub.HoraFim,
		&sub.ToleranciaMin, &sub.Motivo, &sub.Ativo, &sub.CreatedAt, &sub.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("buscar substituicao ativa: %w", err)
	}
	return &sub, nil
}

func (r *SubstituicaoRepository) ListAtivasByDateRange(ctx context.Context, empresaID uuid.UUID, dataInicio, dataFim string, usuarioIDs, postoIDs []string) ([]model.Substituicao, error) {
	where := "WHERE s.empresa_id = $1 AND s.ativo = true"
	args := []interface{}{empresaID}

	if dataInicio != "" && dataFim != "" {
		idx := len(args) + 1
		where += fmt.Sprintf(" AND s.data_inicio <= $%d::date AND s.data_fim >= $%d::date", idx, idx+1)
		args = append(args, dataFim, dataInicio)
	}
	if len(usuarioIDs) > 0 {
		base := len(args)
		placeholders := make([]string, len(usuarioIDs))
		for i, id := range usuarioIDs {
			placeholders[i] = fmt.Sprintf("$%d::uuid", base+i+1)
			args = append(args, id)
		}
		where += fmt.Sprintf(" AND s.usuario_id IN (%s)", strings.Join(placeholders, ", "))
	}
	if len(postoIDs) > 0 {
		base := len(args)
		placeholders := make([]string, len(postoIDs))
		for i, id := range postoIDs {
			placeholders[i] = fmt.Sprintf("$%d::uuid", base+i+1)
			args = append(args, id)
		}
		where += fmt.Sprintf(" AND s.posto_id IN (%s)", strings.Join(placeholders, ", "))
	}

	query := fmt.Sprintf(`
		SELECT s.id, s.empresa_id, s.usuario_id, s.posto_id,
		       s.data_inicio, s.data_fim, s.hora_inicio::text, s.hora_fim::text,
		       s.tolerancia_min, COALESCE(s.motivo, ''), s.ativo, s.created_at, s.updated_at,
		       u.nome AS usuario_nome, p.nome AS posto_nome
		FROM substituicoes s
		LEFT JOIN usuarios u ON u.id = s.usuario_id
		LEFT JOIN postos p ON p.id = s.posto_id
		%s
		ORDER BY s.data_inicio, s.hora_inicio
	`, where)

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("listar substituicoes ativas: %w", err)
	}
	defer rows.Close()

	var subs []model.Substituicao
	for rows.Next() {
		var sub model.Substituicao
		if err := rows.Scan(
			&sub.ID, &sub.EmpresaID, &sub.UsuarioID, &sub.PostoID,
			&sub.DataInicio, &sub.DataFim, &sub.HoraInicio, &sub.HoraFim,
			&sub.ToleranciaMin, &sub.Motivo, &sub.Ativo, &sub.CreatedAt, &sub.UpdatedAt,
			&sub.UsuarioNome, &sub.PostoNome,
		); err != nil {
			return nil, fmt.Errorf("scan substituicao: %w", err)
		}
		subs = append(subs, sub)
	}
	return subs, rows.Err()
}

// SubstituicaoSemTurno e a projecao usada pelo worker de no-show.
type SubstituicaoSemTurno struct {
	ID            uuid.UUID
	EmpresaID     uuid.UUID
	UsuarioID     uuid.UUID
	PostoID       uuid.UUID
	HoraInicio    string
	HoraFim       string
	ToleranciaMin int
}

func (r *SubstituicaoRepository) FindSubstituicoesSemTurno(ctx context.Context, horaCorte time.Time) ([]SubstituicaoSemTurno, error) {
	query := `
		SELECT s.id, s.empresa_id, s.usuario_id, s.posto_id, s.hora_inicio::text, s.hora_fim::text, s.tolerancia_min
		FROM substituicoes s
		WHERE s.ativo = true
		  AND s.data_inicio <= $1::date AND s.data_fim >= $1::date
		  AND ($1::date + s.hora_inicio + (s.tolerancia_min || ' minutes')::interval) <= ($1::date + $2::time)
		  AND NOT EXISTS (
		      SELECT 1 FROM turnos t
		      WHERE t.usuario_id = s.usuario_id
		        AND t.posto_id = s.posto_id
		        AND t.empresa_id = s.empresa_id
		        AND t.status IN ('em_andamento', 'pausado', 'critico', 'atrasado')
		  )
	`
	rows, err := r.db.Query(ctx, query,
		horaCorte.Format("2006-01-02"), horaCorte.Format("15:04:05"),
	)
	if err != nil {
		return nil, fmt.Errorf("buscar substituicoes sem turno: %w", err)
	}
	defer rows.Close()

	var result []SubstituicaoSemTurno
	for rows.Next() {
		var s SubstituicaoSemTurno
		if err := rows.Scan(&s.ID, &s.EmpresaID, &s.UsuarioID, &s.PostoID, &s.HoraInicio, &s.HoraFim, &s.ToleranciaMin); err != nil {
			return nil, fmt.Errorf("scan substituicao sem turno: %w", err)
		}
		result = append(result, s)
	}
	return result, rows.Err()
}
