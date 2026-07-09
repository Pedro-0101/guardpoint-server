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

type TurnoRepository struct {
	db *pgxpool.Pool
}

func NewTurnoRepository(db *pgxpool.Pool) *TurnoRepository {
	return &TurnoRepository{db: db}
}

func (r *TurnoRepository) Create(ctx context.Context, t *model.Turno) error {
	query := `
		INSERT INTO turnos (empresa_id, usuario_id, posto_id, status, inicio_previsto, fim_previsto, inicio_real, token_sessao, device_id, intervalo_min)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id, created_at
	`
	return r.db.QueryRow(ctx, query,
		t.EmpresaID, t.UsuarioID, t.PostoID, t.Status,
		t.InicioPrevisto, t.FimPrevisto, t.InicioReal,
		t.TokenSessao, t.DeviceID, t.IntervaloMin,
	).Scan(&t.ID, &t.CreatedAt)
}

func (r *TurnoRepository) FindAtivoByUsuario(ctx context.Context, empresaID, usuarioID uuid.UUID) (*model.Turno, error) {
	query := `
		SELECT id, empresa_id, usuario_id, posto_id, status, inicio_previsto, fim_previsto,
		       inicio_real, fim_real, token_sessao, device_id, intervalo_min, created_at
		FROM turnos
		WHERE empresa_id = $1 AND usuario_id = $2 AND status IN ('em_andamento', 'pausado', 'critico')
		LIMIT 1
	`
	var t model.Turno
	err := r.db.QueryRow(ctx, query, empresaID, usuarioID).Scan(
		&t.ID, &t.EmpresaID, &t.UsuarioID, &t.PostoID, &t.Status,
		&t.InicioPrevisto, &t.FimPrevisto, &t.InicioReal, &t.FimReal,
		&t.TokenSessao, &t.DeviceID, &t.IntervaloMin, &t.CreatedAt,
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
		       inicio_real, fim_real, token_sessao, device_id, intervalo_min, created_at
		FROM turnos
		WHERE id = $1 AND empresa_id = $2
	`
	var t model.Turno
	err := r.db.QueryRow(ctx, query, id, empresaID).Scan(
		&t.ID, &t.EmpresaID, &t.UsuarioID, &t.PostoID, &t.Status,
		&t.InicioPrevisto, &t.FimPrevisto, &t.InicioReal, &t.FimReal,
		&t.TokenSessao, &t.DeviceID, &t.IntervaloMin, &t.CreatedAt,
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
		       inicio_real, fim_real, token_sessao, device_id, intervalo_min, created_at
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
			&t.TokenSessao, &t.DeviceID, &t.IntervaloMin, &t.CreatedAt,
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
		       inicio_real, fim_real, token_sessao, device_id, intervalo_min, created_at
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
			&t.TokenSessao, &t.DeviceID, &t.IntervaloMin, &t.CreatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("scan turno: %w", err)
		}
		turnos = append(turnos, t)
	}
	return turnos, total, rows.Err()
}

func (r *TurnoRepository) ListTurnos(ctx context.Context, empresaID uuid.UUID, filter model.TurnoFilter) ([]model.Turno, error) {
	where := "WHERE t.empresa_id = $1"
	args := []interface{}{empresaID}

	statuses := parseStatusFilter(filter.Status)
	if len(statuses) > 0 {
		base := len(args)
		placeholders := make([]string, len(statuses))
		for i, s := range statuses {
			placeholders[i] = fmt.Sprintf("$%d", base+i+1)
			args = append(args, s)
		}
		where += fmt.Sprintf(" AND t.status IN (%s)", strings.Join(placeholders, ", "))
	}

	if filter.DataInicio != "" {
		where += fmt.Sprintf(" AND t.inicio_previsto >= $%d::timestamptz", len(args)+1)
		args = append(args, filter.DataInicio)
	}
	if filter.DataFim != "" {
		where += fmt.Sprintf(" AND t.inicio_previsto <= $%d::timestamptz", len(args)+1)
		args = append(args, filter.DataFim)
	}
	if filter.UsuarioID != "" {
		where += fmt.Sprintf(" AND t.usuario_id = $%d::uuid", len(args)+1)
		args = append(args, filter.UsuarioID)
	}
	if filter.PostoID != "" {
		where += fmt.Sprintf(" AND t.posto_id = $%d::uuid", len(args)+1)
		args = append(args, filter.PostoID)
	}

	query := fmt.Sprintf(`
		SELECT t.id, t.empresa_id, t.usuario_id, t.posto_id, t.status,
		       t.inicio_previsto, t.fim_previsto, t.inicio_real, t.fim_real,
		       t.token_sessao, t.device_id, t.intervalo_min, t.created_at,
		       u.nome AS usuario_nome, p.nome AS posto_nome
		FROM turnos t
		LEFT JOIN usuarios u ON u.id = t.usuario_id
		LEFT JOIN postos p ON p.id = t.posto_id
		%s
	`, where)

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("listar turnos: %w", err)
	}
	defer rows.Close()

	var turnos []model.Turno
	for rows.Next() {
		var t model.Turno
		if err := rows.Scan(
			&t.ID, &t.EmpresaID, &t.UsuarioID, &t.PostoID, &t.Status,
			&t.InicioPrevisto, &t.FimPrevisto, &t.InicioReal, &t.FimReal,
			&t.TokenSessao, &t.DeviceID, &t.IntervaloMin, &t.CreatedAt,
			&t.UsuarioNome, &t.PostoNome,
		); err != nil {
			return nil, fmt.Errorf("scan turno: %w", err)
		}
		turnos = append(turnos, t)
	}
	return turnos, rows.Err()
}

func parseStatusFilter(status string) []string {
	if status == "" {
		return nil
	}
	parts := strings.Split(status, ",")
	var result []string
	for _, s := range parts {
		s = strings.TrimSpace(s)
		if s == "" || s == "agendado" {
			continue
		}
		result = append(result, s)
	}
	return result
}

func (r *TurnoRepository) ListTurnosByDateRange(ctx context.Context, empresaID uuid.UUID, dataInicio, dataFim, usuarioID, postoID string) ([]model.Turno, error) {
	where := "WHERE t.empresa_id = $1"
	args := []interface{}{empresaID}

	if dataInicio != "" {
		where += fmt.Sprintf(" AND t.inicio_previsto >= $%d::timestamptz", len(args)+1)
		args = append(args, dataInicio)
	}
	if dataFim != "" {
		where += fmt.Sprintf(" AND t.inicio_previsto <= $%d::timestamptz", len(args)+1)
		args = append(args, dataFim)
	}
	if usuarioID != "" {
		where += fmt.Sprintf(" AND t.usuario_id = $%d::uuid", len(args)+1)
		args = append(args, usuarioID)
	}
	if postoID != "" {
		where += fmt.Sprintf(" AND t.posto_id = $%d::uuid", len(args)+1)
		args = append(args, postoID)
	}

	query := fmt.Sprintf(`
		SELECT t.id, t.empresa_id, t.usuario_id, t.posto_id, t.status,
		       t.inicio_previsto, t.fim_previsto, t.inicio_real, t.fim_real,
		       t.token_sessao, t.device_id, t.intervalo_min, t.created_at,
		       u.nome AS usuario_nome, p.nome AS posto_nome
		FROM turnos t
		LEFT JOIN usuarios u ON u.id = t.usuario_id
		LEFT JOIN postos p ON p.id = t.posto_id
		%s
	`, where)

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("listar turnos por data: %w", err)
	}
	defer rows.Close()

	var turnos []model.Turno
	for rows.Next() {
		var t model.Turno
		if err := rows.Scan(
			&t.ID, &t.EmpresaID, &t.UsuarioID, &t.PostoID, &t.Status,
			&t.InicioPrevisto, &t.FimPrevisto, &t.InicioReal, &t.FimReal,
			&t.TokenSessao, &t.DeviceID, &t.IntervaloMin, &t.CreatedAt,
			&t.UsuarioNome, &t.PostoNome,
		); err != nil {
			return nil, fmt.Errorf("scan turno: %w", err)
		}
		turnos = append(turnos, t)
	}
	return turnos, rows.Err()
}

func (r *TurnoRepository) RevogarToken(ctx context.Context, id, empresaID uuid.UUID, pin string, pinValidoAte time.Time) error {
	query := `UPDATE turnos SET token_sessao = NULL, device_id = NULL, pin = $3, pin_valido_ate = $4 WHERE id = $1 AND empresa_id = $2`
	ct, err := r.db.Exec(ctx, query, id, empresaID, pin, pinValidoAte)
	if err != nil {
		return fmt.Errorf("revogar turno: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return fmt.Errorf("turno nao encontrado")
	}
	return nil
}

// FindAtivoComPinByUsuario busca o turno ativo do usuario incluindo pin e
// pin_valido_ate; usado apenas no fluxo de reassociacao por PIN.
func (r *TurnoRepository) FindAtivoComPinByUsuario(ctx context.Context, empresaID, usuarioID uuid.UUID) (*model.Turno, error) {
	query := `
		SELECT id, empresa_id, usuario_id, posto_id, status, inicio_previsto, fim_previsto,
		       inicio_real, fim_real, token_sessao, device_id, intervalo_min, pin, pin_valido_ate, created_at
		FROM turnos
		WHERE empresa_id = $1 AND usuario_id = $2 AND status IN ('em_andamento', 'pausado', 'critico')
		LIMIT 1
	`
	var t model.Turno
	err := r.db.QueryRow(ctx, query, empresaID, usuarioID).Scan(
		&t.ID, &t.EmpresaID, &t.UsuarioID, &t.PostoID, &t.Status,
		&t.InicioPrevisto, &t.FimPrevisto, &t.InicioReal, &t.FimReal,
		&t.TokenSessao, &t.DeviceID, &t.IntervaloMin, &t.Pin, &t.PinValidoAte, &t.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("buscar turno ativo com pin: %w", err)
	}
	return &t, nil
}

// Reassociar vincula o turno a um novo dispositivo apos o resgate por PIN,
// gerando nova sessao e consumindo o PIN.
func (r *TurnoRepository) Reassociar(ctx context.Context, id, empresaID uuid.UUID, deviceID, tokenSessao string) error {
	query := `
		UPDATE turnos
		SET device_id = $3, token_sessao = $4, pin = NULL, pin_valido_ate = NULL
		WHERE id = $1 AND empresa_id = $2
	`
	ct, err := r.db.Exec(ctx, query, id, empresaID, deviceID, tokenSessao)
	if err != nil {
		return fmt.Errorf("reassociar turno: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return fmt.Errorf("turno nao encontrado")
	}
	return nil
}
