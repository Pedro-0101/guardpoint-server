package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/guardpoint/guardpoint-server/internal/model"
)

type EscalaRepository struct {
	db *pgxpool.Pool
}

func NewEscalaRepository(db *pgxpool.Pool) *EscalaRepository {
	return &EscalaRepository{db: db}
}

func (r *EscalaRepository) Create(ctx context.Context, e *model.Escala) error {
	query := `
		INSERT INTO escalas (empresa_id, usuario_id, posto_id, data_inicio, data_fim, hora_inicio, hora_fim, dias_semana, tolerancia_min)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id, ativo, created_at, updated_at
	`
	return r.db.QueryRow(ctx, query,
		e.EmpresaID, e.UsuarioID, e.PostoID, e.DataInicio, e.DataFim,
		e.HoraInicio, e.HoraFim, e.DiasSemana, e.ToleranciaMin,
	).Scan(&e.ID, &e.Ativo, &e.CreatedAt, &e.UpdatedAt)
}

func (r *EscalaRepository) FindByID(ctx context.Context, empresaID, id uuid.UUID) (*model.Escala, error) {
	query := `
		SELECT e.id, e.empresa_id, e.usuario_id, e.posto_id,
		       e.data_inicio, e.data_fim, e.hora_inicio::text, e.hora_fim::text,
		       e.dias_semana, e.tolerancia_min, e.ativo, e.created_at, e.updated_at,
		       u.nome AS usuario_nome, p.nome AS posto_nome
		FROM escalas e
		LEFT JOIN usuarios u ON u.id = e.usuario_id
		LEFT JOIN postos p ON p.id = e.posto_id
		WHERE e.id = $1 AND e.empresa_id = $2
	`
	var esc model.Escala
	err := r.db.QueryRow(ctx, query, id, empresaID).Scan(
		&esc.ID, &esc.EmpresaID, &esc.UsuarioID, &esc.PostoID,
		&esc.DataInicio, &esc.DataFim, &esc.HoraInicio, &esc.HoraFim,
		&esc.DiasSemana, &esc.ToleranciaMin, &esc.Ativo, &esc.CreatedAt, &esc.UpdatedAt,
		&esc.UsuarioNome, &esc.PostoNome,
	)
	if err != nil {
		return nil, fmt.Errorf("buscar escala: %w", err)
	}
	return &esc, nil
}

func (r *EscalaRepository) List(ctx context.Context, empresaID uuid.UUID, filter model.EscalaFilter) ([]model.Escala, int, error) {
	where := "WHERE e.empresa_id = $1"
	args := []interface{}{empresaID}
	argIdx := 2

	if filter.UsuarioID != "" {
		where += fmt.Sprintf(" AND e.usuario_id = $%d::uuid", argIdx)
		args = append(args, filter.UsuarioID)
		argIdx++
	}
	if filter.PostoID != "" {
		where += fmt.Sprintf(" AND e.posto_id = $%d::uuid", argIdx)
		args = append(args, filter.PostoID)
		argIdx++
	}
	if filter.Ativo != nil {
		where += fmt.Sprintf(" AND e.ativo = $%d", argIdx)
		args = append(args, *filter.Ativo)
		argIdx++
	}
	if filter.Data != "" {
		where += fmt.Sprintf(" AND e.data_inicio <= $%d::date AND e.data_fim >= $%d::date", argIdx, argIdx+1)
		args = append(args, filter.Data, filter.Data)
		argIdx += 2
	}

	var total int
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM escalas e %s", where)
	if err := r.db.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("contar escalas: %w", err)
	}

	if filter.Limit <= 0 {
		filter.Limit = 20
	}
	if filter.Offset < 0 {
		filter.Offset = 0
	}

	dataQuery := fmt.Sprintf(`
		SELECT e.id, e.empresa_id, e.usuario_id, e.posto_id,
		       e.data_inicio, e.data_fim, e.hora_inicio::text, e.hora_fim::text,
		       e.dias_semana, e.tolerancia_min, e.ativo, e.created_at, e.updated_at,
		       u.nome AS usuario_nome, p.nome AS posto_nome
		FROM escalas e
		LEFT JOIN usuarios u ON u.id = e.usuario_id
		LEFT JOIN postos p ON p.id = e.posto_id
		%s
		ORDER BY e.created_at DESC
		LIMIT $%d OFFSET $%d
	`, where, argIdx, argIdx+1)
	args = append(args, filter.Limit, filter.Offset)

	rows, err := r.db.Query(ctx, dataQuery, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("listar escalas: %w", err)
	}
	defer rows.Close()

	var escalas []model.Escala
	for rows.Next() {
		var esc model.Escala
		if err := rows.Scan(
			&esc.ID, &esc.EmpresaID, &esc.UsuarioID, &esc.PostoID,
			&esc.DataInicio, &esc.DataFim, &esc.HoraInicio, &esc.HoraFim,
			&esc.DiasSemana, &esc.ToleranciaMin, &esc.Ativo, &esc.CreatedAt, &esc.UpdatedAt,
			&esc.UsuarioNome, &esc.PostoNome,
		); err != nil {
			return nil, 0, fmt.Errorf("scan escala: %w", err)
		}
		escalas = append(escalas, esc)
	}
	return escalas, total, rows.Err()
}

func (r *EscalaRepository) Update(ctx context.Context, empresaID, id uuid.UUID, e *model.Escala) error {
	query := `
		UPDATE escalas
		SET usuario_id = $1, posto_id = $2, data_inicio = $3, data_fim = $4,
		    hora_inicio = $5, hora_fim = $6, dias_semana = $7, tolerancia_min = $8,
		    ativo = $9, updated_at = now()
		WHERE id = $10 AND empresa_id = $11
		RETURNING id, empresa_id, usuario_id, posto_id, data_inicio, data_fim,
		          hora_inicio::text, hora_fim::text, dias_semana, tolerancia_min, ativo, created_at, updated_at
	`
	return r.db.QueryRow(ctx, query,
		e.UsuarioID, e.PostoID, e.DataInicio, e.DataFim,
		e.HoraInicio, e.HoraFim, e.DiasSemana, e.ToleranciaMin,
		e.Ativo, id, empresaID,
	).Scan(
		&e.ID, &e.EmpresaID, &e.UsuarioID, &e.PostoID,
		&e.DataInicio, &e.DataFim, &e.HoraInicio, &e.HoraFim,
		&e.DiasSemana, &e.ToleranciaMin, &e.Ativo, &e.CreatedAt, &e.UpdatedAt,
	)
}

func (r *EscalaRepository) FindAtivaByUsuarioPostoData(ctx context.Context, empresaID, usuarioID, postoID uuid.UUID, data time.Time, diaSemana int16) (*model.Escala, error) {
	query := `
		SELECT id, empresa_id, usuario_id, posto_id,
		       data_inicio, data_fim, hora_inicio::text, hora_fim::text,
		       dias_semana, tolerancia_min, ativo, created_at, updated_at
		FROM escalas
		WHERE empresa_id = $1
		  AND usuario_id = $2
		  AND posto_id = $3
		  AND ativo = true
		  AND data_inicio <= $4::date
		  AND data_fim >= $4::date
		  AND $5 = ANY(dias_semana)
		LIMIT 1
	`
	var esc model.Escala
	err := r.db.QueryRow(ctx, query,
		empresaID, usuarioID, postoID, data, diaSemana,
	).Scan(
		&esc.ID, &esc.EmpresaID, &esc.UsuarioID, &esc.PostoID,
		&esc.DataInicio, &esc.DataFim, &esc.HoraInicio, &esc.HoraFim,
		&esc.DiasSemana, &esc.ToleranciaMin, &esc.Ativo, &esc.CreatedAt, &esc.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("buscar escala ativa: %w", err)
	}
	return &esc, nil
}

func (r *EscalaRepository) FindEscalasSemTurno(ctx context.Context, horaCorte time.Time) ([]escalaSemTurno, error) {
	query := `
		SELECT e.id, e.empresa_id, e.usuario_id, e.posto_id, e.data_inicio,
		       e.hora_inicio::text, e.tolerancia_min
		FROM escalas e
		WHERE e.ativo = true
		  AND e.data_inicio <= $1::date
		  AND e.data_fim >= $1::date
		  AND $2 = ANY(e.dias_semana)
		  AND (e.hora_inicio + (e.tolerancia_min || ' minutes')::interval) <= $1::time
		  AND NOT EXISTS (
		      SELECT 1 FROM turnos t
		      WHERE t.usuario_id = e.usuario_id
		        AND t.posto_id = e.posto_id
		        AND t.empresa_id = e.empresa_id
		        AND t.status IN ('em_andamento', 'pausado', 'critico')
		  )
	`
	rows, err := r.db.Query(ctx, query, horaCorte, int16(horaCorte.Weekday()))
	if err != nil {
		return nil, fmt.Errorf("buscar escalas sem turno: %w", err)
	}
	defer rows.Close()

	var result []escalaSemTurno
	for rows.Next() {
		var e escalaSemTurno
		if err := rows.Scan(&e.ID, &e.EmpresaID, &e.UsuarioID, &e.PostoID, &e.DataInicio, &e.HoraInicio, &e.ToleranciaMin); err != nil {
			return nil, fmt.Errorf("scan escala sem turno: %w", err)
		}
		result = append(result, e)
	}
	return result, rows.Err()
}

type escalaSemTurno struct {
	ID            uuid.UUID
	EmpresaID     uuid.UUID
	UsuarioID     uuid.UUID
	PostoID       uuid.UUID
	DataInicio    time.Time
	HoraInicio    string
	ToleranciaMin int
}
