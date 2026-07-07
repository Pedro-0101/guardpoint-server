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

type AlertaRepository struct {
	db *pgxpool.Pool
}

func NewAlertaRepository(db *pgxpool.Pool) *AlertaRepository {
	return &AlertaRepository{db: db}
}

func (r *AlertaRepository) Create(ctx context.Context, a *model.Alerta) error {
	query := `
		INSERT INTO alertas (empresa_id, turno_id, tipo, nivel, status, mensagem)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, created_at
	`
	return r.db.QueryRow(ctx, query,
		a.EmpresaID, a.TurnoID, a.Tipo, a.Nivel, a.Status, a.Mensagem,
	).Scan(&a.ID, &a.CreatedAt)
}

func (r *AlertaRepository) FindByID(ctx context.Context, empresaID, id uuid.UUID) (*model.Alerta, error) {
	query := `
		SELECT id, empresa_id, turno_id, tipo, nivel, status, mensagem, resolvido_em, created_at
		FROM alertas
		WHERE id = $1 AND empresa_id = $2
	`
	var a model.Alerta
	err := r.db.QueryRow(ctx, query, id, empresaID).Scan(
		&a.ID, &a.EmpresaID, &a.TurnoID, &a.Tipo, &a.Nivel,
		&a.Status, &a.Mensagem, &a.ResolvidoEm, &a.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("alerta nao encontrado: %w", err)
		}
		return nil, fmt.Errorf("buscar alerta: %w", err)
	}
	return &a, nil
}

func (r *AlertaRepository) List(ctx context.Context, empresaID uuid.UUID, filter model.AlertaFilter) ([]model.Alerta, int, error) {
	where := "WHERE empresa_id = $1"
	args := []interface{}{empresaID}
	argIdx := 2

	if filter.Status != "" {
		where += fmt.Sprintf(" AND status = $%d", argIdx)
		args = append(args, filter.Status)
		argIdx++
	}
	if filter.Tipo != "" {
		where += fmt.Sprintf(" AND tipo = $%d", argIdx)
		args = append(args, filter.Tipo)
		argIdx++
	}
	if filter.TurnoID != "" {
		where += fmt.Sprintf(" AND turno_id = $%d::uuid", argIdx)
		args = append(args, filter.TurnoID)
		argIdx++
	}

	var total int
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM alertas %s", where)
	if err := r.db.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("contar alertas: %w", err)
	}

	if filter.Limit <= 0 {
		filter.Limit = 20
	}
	if filter.Offset < 0 {
		filter.Offset = 0
	}

	dataQuery := fmt.Sprintf(`
		SELECT id, empresa_id, turno_id, tipo, nivel, status, mensagem, resolvido_em, created_at
		FROM alertas %s ORDER BY created_at DESC LIMIT $%d OFFSET $%d
	`, where, argIdx, argIdx+1)
	args = append(args, filter.Limit, filter.Offset)

	rows, err := r.db.Query(ctx, dataQuery, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("listar alertas: %w", err)
	}
	defer rows.Close()

	var alertas []model.Alerta
	for rows.Next() {
		var a model.Alerta
		if err := rows.Scan(
			&a.ID, &a.EmpresaID, &a.TurnoID, &a.Tipo, &a.Nivel,
			&a.Status, &a.Mensagem, &a.ResolvidoEm, &a.CreatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("scan alerta: %w", err)
		}
		alertas = append(alertas, a)
	}
	return alertas, total, rows.Err()
}

func (r *AlertaRepository) UpdateStatus(ctx context.Context, id, empresaID uuid.UUID, status string, resolvidoEm *time.Time) error {
	query := `UPDATE alertas SET status = $1, resolvido_em = $2 WHERE id = $3 AND empresa_id = $4`
	ct, err := r.db.Exec(ctx, query, status, resolvidoEm, id, empresaID)
	if err != nil {
		return fmt.Errorf("atualizar alerta: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return fmt.Errorf("alerta nao encontrado")
	}
	return nil
}

func (r *AlertaRepository) CountAbertos(ctx context.Context, empresaID uuid.UUID) (int, error) {
	var count int
	err := r.db.QueryRow(ctx, `
		SELECT COUNT(*) FROM alertas
		WHERE empresa_id = $1 AND status = 'aberto'
	`, empresaID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("contar alertas abertos: %w", err)
	}
	return count, nil
}

func (r *AlertaRepository) ListRecentes(ctx context.Context, empresaID uuid.UUID, limit int) ([]model.AlertaRecente, error) {
	query := `
		SELECT id, tipo, turno_id, nivel, COALESCE(mensagem, ''), created_at
		FROM alertas
		WHERE empresa_id = $1 AND status = 'aberto'
		ORDER BY created_at DESC
		LIMIT $2
	`
	rows, err := r.db.Query(ctx, query, empresaID, limit)
	if err != nil {
		return nil, fmt.Errorf("listar alertas recentes: %w", err)
	}
	defer rows.Close()

	var alertas []model.AlertaRecente
	for rows.Next() {
		var ar model.AlertaRecente
		var id uuid.UUID
		var turnoID *uuid.UUID
		var createdAt time.Time
		if err := rows.Scan(&id, &ar.Tipo, &turnoID, &ar.Nivel, &ar.Mensagem, &createdAt); err != nil {
			return nil, fmt.Errorf("scan alerta recente: %w", err)
		}
		ar.ID = id.String()
		if turnoID != nil {
			ar.TurnoID = turnoID.String()
		}
		ar.CreatedAt = createdAt.Format(time.RFC3339)
		alertas = append(alertas, ar)
	}
	return alertas, rows.Err()
}

func (r *AlertaRepository) CountByTurnoETipo(ctx context.Context, turnoID uuid.UUID, tipo string) (int, error) {
	var count int
	err := r.db.QueryRow(ctx, `
		SELECT COUNT(*) FROM alertas
		WHERE turno_id = $1 AND tipo = $2 AND status IN ('aberto', 'reconhecido')
	`, turnoID, tipo).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("contar alertas por turno e tipo: %w", err)
	}
	return count, nil
}

func (r *AlertaRepository) CountPorTipo(ctx context.Context, empresaID uuid.UUID) ([]model.AlertaPorTipo, error) {
	rows, err := r.db.Query(ctx, `
		SELECT tipo, COUNT(*) FROM alertas
		WHERE empresa_id = $1
		GROUP BY tipo ORDER BY COUNT(*) DESC
	`, empresaID)
	if err != nil {
		return nil, fmt.Errorf("contar alertas por tipo: %w", err)
	}
	defer rows.Close()

	var result []model.AlertaPorTipo
	for rows.Next() {
		var apt model.AlertaPorTipo
		if err := rows.Scan(&apt.Tipo, &apt.Quantidade); err != nil {
			return nil, fmt.Errorf("scan alerta por tipo: %w", err)
		}
		result = append(result, apt)
	}
	return result, rows.Err()
}

func (r *AlertaRepository) CloseAlertasFalsoPositivo(ctx context.Context, turnoID uuid.UUID) (int64, error) {
	now := time.Now()
	query := `
		UPDATE alertas SET status = 'falso_positivo', resolvido_em = $1
		WHERE turno_id = $2
		  AND tipo LIKE 'atraso_%'
		  AND status IN ('aberto', 'reconhecido')
	`
	ct, err := r.db.Exec(ctx, query, now, turnoID)
	if err != nil {
		return 0, fmt.Errorf("marcar alertas falso positivo: %w", err)
	}
	return ct.RowsAffected(), nil
}

// CloseAlertasResolvidoCheckin fecha alertas de atraso ('atraso_%') abertos ou
// reconhecidos do turno quando um novo check-in chega, resetando o relogio do
// deadman's switch. E idempotente: se nao houver alerta aberto, nao afeta linhas.
func (r *AlertaRepository) CloseAlertasResolvidoCheckin(ctx context.Context, turnoID uuid.UUID) (int64, error) {
	now := time.Now()
	query := `
		UPDATE alertas SET status = 'resolvido_checkin', resolvido_em = $1
		WHERE turno_id = $2
		  AND tipo LIKE 'atraso_%'
		  AND status IN ('aberto', 'reconhecido')
	`
	ct, err := r.db.Exec(ctx, query, now, turnoID)
	if err != nil {
		return 0, fmt.Errorf("marcar alertas resolvido por checkin: %w", err)
	}
	return ct.RowsAffected(), nil
}

func (r *AlertaRepository) CountPorHora(ctx context.Context, empresaID uuid.UUID) ([]model.AlertaPorHora, error) {
	rows, err := r.db.Query(ctx, `
		SELECT TO_CHAR(created_at, 'HH24:00'), COUNT(*)
		FROM alertas
		WHERE empresa_id = $1 AND created_at >= now() - interval '24 hours'
		GROUP BY TO_CHAR(created_at, 'HH24:00')
		ORDER BY TO_CHAR(created_at, 'HH24:00')
	`, empresaID)
	if err != nil {
		return nil, fmt.Errorf("contar alertas por hora: %w", err)
	}
	defer rows.Close()

	var result []model.AlertaPorHora
	for rows.Next() {
		var aph model.AlertaPorHora
		if err := rows.Scan(&aph.Hora, &aph.Quantidade); err != nil {
			return nil, fmt.Errorf("scan alerta por hora: %w", err)
		}
		result = append(result, aph)
	}
	return result, rows.Err()
}
