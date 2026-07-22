package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
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

func (r *EscalaRepository) BeginTx(ctx context.Context) (pgx.Tx, error) {
	return r.db.Begin(ctx)
}

func (r *EscalaRepository) CreateWithTx(ctx context.Context, tx pgx.Tx, e *model.Escala) error {
	query := `
		INSERT INTO escalas (empresa_id, usuario_id, posto_id, dia_semana_inicio, dia_semana_fim, hora_inicio, hora_fim, tolerancia_min)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, ativo, created_at, updated_at
	`
	return tx.QueryRow(ctx, query,
		e.EmpresaID, e.UsuarioID, e.PostoID,
		e.DiaSemanaInicio, e.DiaSemanaFim, e.HoraInicio, e.HoraFim, e.ToleranciaMin,
	).Scan(&e.ID, &e.Ativo, &e.CreatedAt, &e.UpdatedAt)
}

func (r *EscalaRepository) DeleteAtivasPorUsuarioPosto(ctx context.Context, tx pgx.Tx, empresaID, usuarioID, postoID uuid.UUID) error {
	query := `
		UPDATE escalas
		SET ativo = false, updated_at = now()
		WHERE empresa_id = $1 AND usuario_id = $2 AND posto_id = $3 AND ativo = true
	`
	_, err := tx.Exec(ctx, query, empresaID, usuarioID, postoID)
	return err
}

func (r *EscalaRepository) Create(ctx context.Context, e *model.Escala) error {
	query := `
		INSERT INTO escalas (empresa_id, usuario_id, posto_id, dia_semana_inicio, dia_semana_fim, hora_inicio, hora_fim, tolerancia_min)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, ativo, created_at, updated_at
	`
	return r.db.QueryRow(ctx, query,
		e.EmpresaID, e.UsuarioID, e.PostoID,
		e.DiaSemanaInicio, e.DiaSemanaFim, e.HoraInicio, e.HoraFim, e.ToleranciaMin,
	).Scan(&e.ID, &e.Ativo, &e.CreatedAt, &e.UpdatedAt)
}

func (r *EscalaRepository) FindByID(ctx context.Context, empresaID, id uuid.UUID) (*model.Escala, error) {
	query := `
		SELECT e.id, e.empresa_id, e.usuario_id, e.posto_id,
		       e.dia_semana_inicio, e.dia_semana_fim, e.hora_inicio::text, e.hora_fim::text,
		       e.tolerancia_min, e.ativo, e.created_at, e.updated_at,
		       u.nome AS usuario_nome, p.nome AS posto_nome
		FROM escalas e
		LEFT JOIN usuarios u ON u.id = e.usuario_id
		LEFT JOIN postos p ON p.id = e.posto_id
		WHERE e.id = $1 AND e.empresa_id = $2
	`
	var esc model.Escala
	err := r.db.QueryRow(ctx, query, id, empresaID).Scan(
		&esc.ID, &esc.EmpresaID, &esc.UsuarioID, &esc.PostoID,
		&esc.DiaSemanaInicio, &esc.DiaSemanaFim, &esc.HoraInicio, &esc.HoraFim,
		&esc.ToleranciaMin, &esc.Ativo, &esc.CreatedAt, &esc.UpdatedAt,
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

	if filter.Limit <= 0 {
		filter.Limit = 20
	}
	if filter.Offset < 0 {
		filter.Offset = 0
	}

	// When filtering by a specific user, fall back to row-based pagination
	if filter.UsuarioID != "" {
		var total int
		countQuery := fmt.Sprintf("SELECT COUNT(*) FROM escalas e %s", where)
		if err := r.db.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
			return nil, 0, fmt.Errorf("contar escalas: %w", err)
		}

		dataQuery := fmt.Sprintf(`
			SELECT e.id, e.empresa_id, e.usuario_id, e.posto_id,
			       e.dia_semana_inicio, e.dia_semana_fim, e.hora_inicio::text, e.hora_fim::text,
			       e.tolerancia_min, e.ativo, e.created_at, e.updated_at,
			       u.nome AS usuario_nome, p.nome AS posto_nome
			FROM escalas e
			LEFT JOIN usuarios u ON u.id = e.usuario_id
			LEFT JOIN postos p ON p.id = e.posto_id
			%s
			ORDER BY e.created_at DESC
			LIMIT $%d OFFSET $%d
		`, where, argIdx, argIdx+1)
		dataArgs := append(args, filter.Limit, filter.Offset)

		rows, err := r.db.Query(ctx, dataQuery, dataArgs...)
		if err != nil {
			return nil, 0, fmt.Errorf("listar escalas: %w", err)
		}
		defer rows.Close()

		var escalas []model.Escala
		for rows.Next() {
			var esc model.Escala
			if err := rows.Scan(
				&esc.ID, &esc.EmpresaID, &esc.UsuarioID, &esc.PostoID,
				&esc.DiaSemanaInicio, &esc.DiaSemanaFim, &esc.HoraInicio, &esc.HoraFim,
				&esc.ToleranciaMin, &esc.Ativo, &esc.CreatedAt, &esc.UpdatedAt,
				&esc.UsuarioNome, &esc.PostoNome,
			); err != nil {
				return nil, 0, fmt.Errorf("scan escala: %w", err)
			}
			escalas = append(escalas, esc)
		}
		return escalas, total, rows.Err()
	}

	// Paginate by distinct user: count total users, fetch paginated user IDs,
	// then return ALL escalas for those users.
	var totalUsers int
	countQuery := fmt.Sprintf("SELECT COUNT(DISTINCT e.usuario_id) FROM escalas e %s", where)
	if err := r.db.QueryRow(ctx, countQuery, args...).Scan(&totalUsers); err != nil {
		return nil, 0, fmt.Errorf("contar usuarios: %w", err)
	}

	if totalUsers == 0 {
		return []model.Escala{}, 0, nil
	}

	userQuery := fmt.Sprintf(`
		SELECT e.usuario_id
		FROM escalas e
		%s
		GROUP BY e.usuario_id
		ORDER BY MAX(e.created_at) DESC
		LIMIT $%d OFFSET $%d
	`, where, argIdx, argIdx+1)
	userArgs := append(args, filter.Limit, filter.Offset)

	uRows, err := r.db.Query(ctx, userQuery, userArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("listar usuarios de escala: %w", err)
	}
	defer uRows.Close()

	var userIDs []uuid.UUID
	for uRows.Next() {
		var uid uuid.UUID
		if err := uRows.Scan(&uid); err != nil {
			return nil, 0, fmt.Errorf("scan usuario_id: %w", err)
		}
		userIDs = append(userIDs, uid)
	}
	if err := uRows.Err(); err != nil {
		return nil, 0, err
	}

	if len(userIDs) == 0 {
		return []model.Escala{}, totalUsers, nil
	}

	dataQuery := fmt.Sprintf(`
		SELECT e.id, e.empresa_id, e.usuario_id, e.posto_id,
		       e.dia_semana_inicio, e.dia_semana_fim, e.hora_inicio::text, e.hora_fim::text,
		       e.tolerancia_min, e.ativo, e.created_at, e.updated_at,
		       u.nome AS usuario_nome, p.nome AS posto_nome
		FROM escalas e
		LEFT JOIN usuarios u ON u.id = e.usuario_id
		LEFT JOIN postos p ON p.id = e.posto_id
		%s AND e.usuario_id = ANY($%d)
		ORDER BY e.usuario_id, e.created_at DESC
	`, where, argIdx)
	dataArgs := append(args, userIDs)

	rows, err := r.db.Query(ctx, dataQuery, dataArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("listar escalas: %w", err)
	}
	defer rows.Close()

	var escalas []model.Escala
	for rows.Next() {
		var esc model.Escala
		if err := rows.Scan(
			&esc.ID, &esc.EmpresaID, &esc.UsuarioID, &esc.PostoID,
			&esc.DiaSemanaInicio, &esc.DiaSemanaFim, &esc.HoraInicio, &esc.HoraFim,
			&esc.ToleranciaMin, &esc.Ativo, &esc.CreatedAt, &esc.UpdatedAt,
			&esc.UsuarioNome, &esc.PostoNome,
		); err != nil {
			return nil, 0, fmt.Errorf("scan escala: %w", err)
		}
		escalas = append(escalas, esc)
	}
	return escalas, totalUsers, rows.Err()
}

func (r *EscalaRepository) ListAtivasByEmpresa(ctx context.Context, empresaID uuid.UUID, usuarioIDs, postoIDs []string) ([]model.Escala, error) {
	where := "WHERE e.empresa_id = $1 AND e.ativo = true"
	args := []interface{}{empresaID}

	if len(usuarioIDs) > 0 {
		base := len(args)
		placeholders := make([]string, len(usuarioIDs))
		for i, id := range usuarioIDs {
			placeholders[i] = fmt.Sprintf("$%d::uuid", base+i+1)
			args = append(args, id)
		}
		where += fmt.Sprintf(" AND e.usuario_id IN (%s)", strings.Join(placeholders, ", "))
	}
	if len(postoIDs) > 0 {
		base := len(args)
		placeholders := make([]string, len(postoIDs))
		for i, id := range postoIDs {
			placeholders[i] = fmt.Sprintf("$%d::uuid", base+i+1)
			args = append(args, id)
		}
		where += fmt.Sprintf(" AND e.posto_id IN (%s)", strings.Join(placeholders, ", "))
	}

	query := fmt.Sprintf(`
		SELECT e.id, e.empresa_id, e.usuario_id, e.posto_id,
		       e.dia_semana_inicio, e.dia_semana_fim, e.hora_inicio::text, e.hora_fim::text,
		       e.tolerancia_min, e.ativo, e.created_at, e.updated_at,
		       u.nome AS usuario_nome, p.nome AS posto_nome
		FROM escalas e
		LEFT JOIN usuarios u ON u.id = e.usuario_id
		LEFT JOIN postos p ON p.id = e.posto_id
		%s
		ORDER BY e.dia_semana_inicio, e.hora_inicio
	`, where)

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("listar escalas ativas: %w", err)
	}
	defer rows.Close()

	var escalas []model.Escala
	for rows.Next() {
		var esc model.Escala
		if err := rows.Scan(
			&esc.ID, &esc.EmpresaID, &esc.UsuarioID, &esc.PostoID,
			&esc.DiaSemanaInicio, &esc.DiaSemanaFim, &esc.HoraInicio, &esc.HoraFim,
			&esc.ToleranciaMin, &esc.Ativo, &esc.CreatedAt, &esc.UpdatedAt,
			&esc.UsuarioNome, &esc.PostoNome,
		); err != nil {
			return nil, fmt.Errorf("scan escala: %w", err)
		}
		escalas = append(escalas, esc)
	}
	return escalas, rows.Err()
}

func (r *EscalaRepository) Update(ctx context.Context, empresaID, id uuid.UUID, e *model.Escala) error {
	query := `
		UPDATE escalas
		SET usuario_id = $1, posto_id = $2,
		    dia_semana_inicio = $3, dia_semana_fim = $4,
		    hora_inicio = $5, hora_fim = $6, tolerancia_min = $7,
		    ativo = $8, updated_at = now()
		WHERE id = $9 AND empresa_id = $10
		RETURNING id, empresa_id, usuario_id, posto_id,
		          dia_semana_inicio, dia_semana_fim,
		          hora_inicio::text, hora_fim::text, tolerancia_min, ativo, created_at, updated_at
	`
	return r.db.QueryRow(ctx, query,
		e.UsuarioID, e.PostoID,
		e.DiaSemanaInicio, e.DiaSemanaFim,
		e.HoraInicio, e.HoraFim, e.ToleranciaMin,
		e.Ativo, id, empresaID,
	).Scan(
		&e.ID, &e.EmpresaID, &e.UsuarioID, &e.PostoID,
		&e.DiaSemanaInicio, &e.DiaSemanaFim,
		&e.HoraInicio, &e.HoraFim,
		&e.ToleranciaMin, &e.Ativo, &e.CreatedAt, &e.UpdatedAt,
	)
}

func (r *EscalaRepository) FindAtivaByUsuarioPostoDia(ctx context.Context, empresaID, usuarioID, postoID uuid.UUID, diaSemana int16) (*model.Escala, error) {
	query := `
		SELECT id, empresa_id, usuario_id, posto_id,
		       dia_semana_inicio, dia_semana_fim, hora_inicio::text, hora_fim::text,
		       tolerancia_min, ativo, created_at, updated_at
		FROM escalas
		WHERE empresa_id = $1
		  AND usuario_id = $2
		  AND posto_id = $3
		  AND ativo = true
		  AND dia_semana_inicio = $4
		LIMIT 1
	`
	var esc model.Escala
	err := r.db.QueryRow(ctx, query,
		empresaID, usuarioID, postoID, diaSemana,
	).Scan(
		&esc.ID, &esc.EmpresaID, &esc.UsuarioID, &esc.PostoID,
		&esc.DiaSemanaInicio, &esc.DiaSemanaFim, &esc.HoraInicio, &esc.HoraFim,
		&esc.ToleranciaMin, &esc.Ativo, &esc.CreatedAt, &esc.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("buscar escala ativa: %w", err)
	}
	return &esc, nil
}

// FindEscalasSemTurno busca escalas cuja tolerancia de inicio ja estourou sem
// que exista turno ativo (no-show).
func (r *EscalaRepository) FindEscalasSemTurno(ctx context.Context, horaCorte time.Time) ([]EscalaSemTurno, error) {
	ontem := horaCorte.AddDate(0, 0, -1)
	query := `
		SELECT e.id, e.empresa_id, e.usuario_id, e.posto_id, e.hora_inicio::text, e.hora_fim::text, e.tolerancia_min
		FROM escalas e
		WHERE e.ativo = true
		  AND (
		      (
		          e.dia_semana_inicio = $1
		          AND ($2::date + e.hora_inicio + (e.tolerancia_min || ' minutes')::interval) <= ($2::date + $3::time)
		          AND (e.dia_semana_fim != e.dia_semana_inicio OR $3::time < e.hora_fim)
		      )
		      OR
		      (
		          e.dia_semana_inicio = $4
		          AND e.dia_semana_fim != e.dia_semana_inicio
		          AND ($5::date + e.hora_inicio + (e.tolerancia_min || ' minutes')::interval) <= ($2::date + $3::time)
		          AND $3::time < e.hora_fim
		      )
		  )
		  AND NOT EXISTS (
		      SELECT 1 FROM turnos t
		      WHERE t.usuario_id = e.usuario_id
		        AND t.posto_id = e.posto_id
		        AND t.empresa_id = e.empresa_id
		        AND t.status IN ('em_andamento', 'pausado', 'critico', 'atrasado')
		  )
	`
	rows, err := r.db.Query(ctx, query,
		int16(horaCorte.Weekday()), horaCorte.Format("2006-01-02"), horaCorte.Format("15:04:05"),
		int16(ontem.Weekday()), ontem.Format("2006-01-02"),
	)
	if err != nil {
		return nil, fmt.Errorf("buscar escalas sem turno: %w", err)
	}
	defer rows.Close()

	var result []EscalaSemTurno
	for rows.Next() {
		var e EscalaSemTurno
		if err := rows.Scan(&e.ID, &e.EmpresaID, &e.UsuarioID, &e.PostoID, &e.HoraInicio, &e.HoraFim, &e.ToleranciaMin); err != nil {
			return nil, fmt.Errorf("scan escala sem turno: %w", err)
		}
		result = append(result, e)
	}
	return result, rows.Err()
}

// EscalaSemTurno e a projecao retornada por FindEscalasSemTurno para o worker
// de no-show.
type EscalaSemTurno struct {
	ID            uuid.UUID
	EmpresaID     uuid.UUID
	UsuarioID     uuid.UUID
	PostoID       uuid.UUID
	HoraInicio    string
	HoraFim       string
	ToleranciaMin int
}
