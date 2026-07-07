# Alerta por Escalonamento — Destinatários Configuráveis — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Substituir os campos genéricos `cargo_alvo`/`whatsapp_para` da configuração de escalonamento por listas de usuários específicos do sistema, tornar os destinatários das emergências (coação/sabotagem/no-show) configuráveis por tipo, e fechar automaticamente os alertas de atraso quando o vigia finalmente faz check-in.

**Architecture:** O pipeline existente (Turno.intervalo_min → TimeoutChecker → ConfigEscalonamento → AlertaService → canal → AlertDispatcher stub) é mantido. Muda apenas a resolução de destinatários (agora por `usuario_id`, não por telefone/cargo) e adiciona-se uma configuração paralela por tipo de emergência, além de fechamento automático de alertas no check-in.

**Tech Stack:** Go, chi router, pgx v5 (Postgres), golang-migrate, go-playground/validator, testes de integração com `-tags integration` contra Postgres efêmero.

## Global Constraints

- Todas as rotas de configuração (`/config/*`) exigem RBAC `admin` (via `handler.RequireRole("admin")`), já aplicado no grupo de rotas em `internal/app/app.go`.
- Toda query de repositório deve ser escopada por `empresa_id` (multi-tenancy) — exceção documentada apenas para lookups por `turno_id`/`id` que já são globalmente únicos (mesmo padrão hoje usado em `CloseAlertasFalsoPositivo`).
- Nenhuma query deve depender de bind de array (`= ANY($1)` com slice de `uuid.UUID`) — usar `JOIN` por `empresa_id` ou consulta por `config_id` escalar, que já são padrões comprovados no código existente.
- `usuario_ids` vazio é sempre rejeitado na validação de request (`validate:"required,min=1"`).
- Cada `usuario_id` enviado pelo admin deve pertencer à mesma empresa do token — validado no service via `UserRepository.FindByIDEmpresa`, retornando 400 se não pertencer.

---

### Task 1: Migration `000018` — schema de destinatários

**Files:**
- Create: `migrations/000018_config_alerta_destinatarios.up.sql`
- Create: `migrations/000018_config_alerta_destinatarios.down.sql`

**Interfaces:**
- Produces: tabelas `config_escalonamento_destinatarios`, `config_alerta_emergencia`, `config_alerta_emergencia_destinatarios`; remove colunas `config_escalonamento.whatsapp_para` e `config_escalonamento.cargo_alvo`.

- [ ] **Step 1: Escrever a migration up**

```sql
-- migrations/000018_config_alerta_destinatarios.up.sql
ALTER TABLE config_escalonamento DROP COLUMN whatsapp_para;
ALTER TABLE config_escalonamento DROP COLUMN cargo_alvo;

CREATE TABLE config_escalonamento_destinatarios (
    id                      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    config_escalonamento_id UUID NOT NULL REFERENCES config_escalonamento(id) ON DELETE CASCADE,
    usuario_id              UUID NOT NULL REFERENCES usuarios(id) ON DELETE CASCADE,
    created_at              TIMESTAMPTZ DEFAULT now(),
    UNIQUE(config_escalonamento_id, usuario_id)
);

CREATE INDEX IF NOT EXISTS idx_config_escalonamento_destinatarios_config
    ON config_escalonamento_destinatarios(config_escalonamento_id);

CREATE TABLE config_alerta_emergencia (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    empresa_id  UUID NOT NULL REFERENCES empresas(id),
    tipo        VARCHAR(20) NOT NULL CHECK (tipo IN ('coacao', 'sabotagem', 'no_show')),
    created_at  TIMESTAMPTZ DEFAULT now(),
    UNIQUE(empresa_id, tipo)
);

CREATE TABLE config_alerta_emergencia_destinatarios (
    id                          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    config_alerta_emergencia_id UUID NOT NULL REFERENCES config_alerta_emergencia(id) ON DELETE CASCADE,
    usuario_id                  UUID NOT NULL REFERENCES usuarios(id) ON DELETE CASCADE,
    created_at                  TIMESTAMPTZ DEFAULT now(),
    UNIQUE(config_alerta_emergencia_id, usuario_id)
);

CREATE INDEX IF NOT EXISTS idx_config_alerta_emergencia_destinatarios_config
    ON config_alerta_emergencia_destinatarios(config_alerta_emergencia_id);
```

- [ ] **Step 2: Escrever a migration down**

```sql
-- migrations/000018_config_alerta_destinatarios.down.sql
DROP TABLE IF EXISTS config_alerta_emergencia_destinatarios;
DROP TABLE IF EXISTS config_alerta_emergencia;
DROP TABLE IF EXISTS config_escalonamento_destinatarios;

ALTER TABLE config_escalonamento ADD COLUMN whatsapp_para VARCHAR(20);
ALTER TABLE config_escalonamento ADD COLUMN cargo_alvo VARCHAR(50);
```

- [ ] **Step 3: Aplicar localmente e verificar**

Suba um Postgres efêmero e rode as migrations (o `internal/testutil.SetupTestDB` já faz isso para os testes de integração — não é necessário rodar manualmente se você for direto para os testes da Task 9+). Para uma verificação rápida isolada:

Run: `go build ./...`
Expected: compila sem erros (a migration em si não é compilada, mas confirma que nada mais quebrou ainda).

- [ ] **Step 4: Commit**

```bash
git add migrations/000018_config_alerta_destinatarios.up.sql migrations/000018_config_alerta_destinatarios.down.sql
git commit -m "feat: migration para destinatarios de escalonamento e alertas de emergencia"
```

---

### Task 2: Modelo Go — `internal/model/alerta.go`

**Files:**
- Modify: `internal/model/alerta.go`

**Interfaces:**
- Consumes: nenhuma (modelos puros).
- Produces: `model.ConfigEscalonamento{ID, EmpresaID, Nivel, AtrasoMinutos, UsuarioIDs []uuid.UUID, CreatedAt}`, `model.ConfigAlertaEmergencia{ID, EmpresaID, Tipo string, UsuarioIDs []uuid.UUID, CreatedAt}`, `model.CreateConfigEscalonamentoRequest{Nivel, AtrasoMinutos, UsuarioIDs}`, `model.UpdateConfigEscalonamentoRequest{AtrasoMinutos, UsuarioIDs}`, `model.UpdateConfigAlertaEmergenciaRequest{UsuarioIDs}`, `model.PendingAlert{Alerta *Alerta, UsuarioIDs []uuid.UUID}`.

- [ ] **Step 1: Substituir os structs em `internal/model/alerta.go`**

Substitua todo o conteúdo do arquivo por:

```go
package model

import (
	"time"

	"github.com/google/uuid"
)

type Alerta struct {
	ID          uuid.UUID  `json:"id"`
	EmpresaID   uuid.UUID  `json:"empresa_id"`
	TurnoID     *uuid.UUID `json:"turno_id"`
	Tipo        string     `json:"tipo"`
	Nivel       int        `json:"nivel"`
	Status      string     `json:"status"`
	Mensagem    *string    `json:"mensagem,omitempty"`
	ResolvidoEm *time.Time `json:"resolvido_em,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
}

type ConfigEscalonamento struct {
	ID            uuid.UUID   `json:"id"`
	EmpresaID     uuid.UUID   `json:"empresa_id"`
	Nivel         int         `json:"nivel"`
	AtrasoMinutos int         `json:"atraso_minutos"`
	UsuarioIDs    []uuid.UUID `json:"usuario_ids"`
	CreatedAt     time.Time   `json:"created_at"`
}

// ConfigAlertaEmergencia define quais usuarios recebem um tipo especifico de
// alerta de emergencia (coacao, sabotagem, no_show), independente dos niveis
// de escalonamento por atraso.
type ConfigAlertaEmergencia struct {
	ID         uuid.UUID   `json:"id"`
	EmpresaID  uuid.UUID   `json:"empresa_id"`
	Tipo       string      `json:"tipo"`
	UsuarioIDs []uuid.UUID `json:"usuario_ids"`
	CreatedAt  time.Time   `json:"created_at"`
}

type AlertaFilter struct {
	Status  string `json:"status"`
	Tipo    string `json:"tipo"`
	TurnoID string `json:"turno_id"`
	Limit   int    `json:"limit"`
	Offset  int    `json:"offset"`
}

type AlertStatistics struct {
	TotalAbertos      int             `json:"total_abertos"`
	TotalReconhecidos int             `json:"total_reconhecidos"`
	TotalEncerrados   int             `json:"total_encerrados"`
	PorTipo           []AlertaPorTipo `json:"por_tipo"`
	PorHora           []AlertaPorHora `json:"por_hora"`
}

type AlertaPorTipo struct {
	Tipo       string `json:"tipo"`
	Quantidade int    `json:"quantidade"`
}

type AlertaPorHora struct {
	Hora       string `json:"hora"`
	Quantidade int    `json:"quantidade"`
}

type CreateConfigEscalonamentoRequest struct {
	Nivel         int         `json:"nivel" validate:"required,min=1,max=5"`
	AtrasoMinutos int         `json:"atraso_minutos" validate:"required,min=1,max=1440"`
	UsuarioIDs    []uuid.UUID `json:"usuario_ids" validate:"required,min=1"`
}

type UpdateConfigEscalonamentoRequest struct {
	AtrasoMinutos int         `json:"atraso_minutos" validate:"required,min=1,max=1440"`
	UsuarioIDs    []uuid.UUID `json:"usuario_ids" validate:"required,min=1"`
}

type UpdateConfigAlertaEmergenciaRequest struct {
	UsuarioIDs []uuid.UUID `json:"usuario_ids" validate:"required,min=1"`
}

type PendingAlert struct {
	Alerta     *Alerta     `json:"alerta"`
	UsuarioIDs []uuid.UUID `json:"usuario_ids"`
}
```

- [ ] **Step 2: Compilar (vai falhar — esperado)**

Run: `go build ./...`
Expected: FAIL — `internal/repository/config_escalonamento_repository.go` e `internal/service/alerta_service.go` ainda referenciam `WhatsappPara`/`CargoAlvo`. Isso é esperado; essas referências serão corrigidas nas próximas tasks.

- [ ] **Step 3: Commit**

```bash
git add internal/model/alerta.go
git commit -m "feat: modelo de destinatarios por usuario para escalonamento e emergencias"
```

---

### Task 3: Reescrever `ConfigEscalonamentoRepository`

**Files:**
- Modify: `internal/repository/config_escalonamento_repository.go`

**Interfaces:**
- Consumes: `model.ConfigEscalonamento` (Task 2).
- Produces: `ConfigEscalonamentoRepository.{Create, FindByEmpresa, FindByEmpresaENivel, Update, Upsert, Delete, DeleteByEmpresa, ReplaceByEmpresa}` com a mesma assinatura pública de antes (apenas o conteúdo dos structs muda), populando `UsuarioIDs`.

- [ ] **Step 1: Substituir o arquivo inteiro**

```go
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

type ConfigEscalonamentoRepository struct {
	db *pgxpool.Pool
}

func NewConfigEscalonamentoRepository(db *pgxpool.Pool) *ConfigEscalonamentoRepository {
	return &ConfigEscalonamentoRepository{db: db}
}

func (r *ConfigEscalonamentoRepository) Create(ctx context.Context, c *model.ConfigEscalonamento) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("iniciar transacao: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	query := `
		INSERT INTO config_escalonamento (empresa_id, nivel, atraso_minutos)
		VALUES ($1, $2, $3)
		RETURNING id, created_at
	`
	if err := tx.QueryRow(ctx, query, c.EmpresaID, c.Nivel, c.AtrasoMinutos).Scan(&c.ID, &c.CreatedAt); err != nil {
		return fmt.Errorf("criar config escalonamento: %w", err)
	}

	if err := inserirDestinatariosEscalonamento(ctx, tx, c.ID, c.UsuarioIDs); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func (r *ConfigEscalonamentoRepository) FindByEmpresa(ctx context.Context, empresaID uuid.UUID) ([]model.ConfigEscalonamento, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, empresa_id, nivel, atraso_minutos, created_at
		FROM config_escalonamento
		WHERE empresa_id = $1
		ORDER BY nivel ASC
	`, empresaID)
	if err != nil {
		return nil, fmt.Errorf("listar config escalonamento: %w", err)
	}
	defer rows.Close()

	var configs []model.ConfigEscalonamento
	for rows.Next() {
		var c model.ConfigEscalonamento
		if err := rows.Scan(&c.ID, &c.EmpresaID, &c.Nivel, &c.AtrasoMinutos, &c.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan config escalonamento: %w", err)
		}
		configs = append(configs, c)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	destinatarios, err := r.destinatariosPorEmpresa(ctx, empresaID)
	if err != nil {
		return nil, err
	}
	for i := range configs {
		configs[i].UsuarioIDs = destinatarios[configs[i].ID]
	}
	return configs, nil
}

// destinatariosPorEmpresa busca, num unico round-trip, os usuario_id de todas
// as configs de escalonamento da empresa, agrupados por config_escalonamento_id.
// Evita N+1 sem depender de bind de array (`= ANY($1)` com slice de uuid.UUID).
func (r *ConfigEscalonamentoRepository) destinatariosPorEmpresa(ctx context.Context, empresaID uuid.UUID) (map[uuid.UUID][]uuid.UUID, error) {
	rows, err := r.db.Query(ctx, `
		SELECT d.config_escalonamento_id, d.usuario_id
		FROM config_escalonamento_destinatarios d
		JOIN config_escalonamento c ON c.id = d.config_escalonamento_id
		WHERE c.empresa_id = $1
	`, empresaID)
	if err != nil {
		return nil, fmt.Errorf("listar destinatarios de escalonamento: %w", err)
	}
	defer rows.Close()

	result := make(map[uuid.UUID][]uuid.UUID)
	for rows.Next() {
		var configID, usuarioID uuid.UUID
		if err := rows.Scan(&configID, &usuarioID); err != nil {
			return nil, fmt.Errorf("scan destinatario de escalonamento: %w", err)
		}
		result[configID] = append(result[configID], usuarioID)
	}
	return result, rows.Err()
}

func (r *ConfigEscalonamentoRepository) FindByEmpresaENivel(ctx context.Context, empresaID uuid.UUID, nivel int) (*model.ConfigEscalonamento, error) {
	var c model.ConfigEscalonamento
	err := r.db.QueryRow(ctx, `
		SELECT id, empresa_id, nivel, atraso_minutos, created_at
		FROM config_escalonamento
		WHERE empresa_id = $1 AND nivel = $2
	`, empresaID, nivel).Scan(&c.ID, &c.EmpresaID, &c.Nivel, &c.AtrasoMinutos, &c.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("buscar config escalonamento: %w", err)
	}

	rows, err := r.db.Query(ctx, `SELECT usuario_id FROM config_escalonamento_destinatarios WHERE config_escalonamento_id = $1`, c.ID)
	if err != nil {
		return nil, fmt.Errorf("listar destinatarios: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var usuarioID uuid.UUID
		if err := rows.Scan(&usuarioID); err != nil {
			return nil, fmt.Errorf("scan destinatario: %w", err)
		}
		c.UsuarioIDs = append(c.UsuarioIDs, usuarioID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return &c, nil
}

func (r *ConfigEscalonamentoRepository) Update(ctx context.Context, id, empresaID uuid.UUID, c *model.ConfigEscalonamento) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("iniciar transacao: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	query := `
		UPDATE config_escalonamento
		SET atraso_minutos = $1
		WHERE id = $2 AND empresa_id = $3
		RETURNING id, empresa_id, nivel, atraso_minutos, created_at
	`
	if err := tx.QueryRow(ctx, query, c.AtrasoMinutos, id, empresaID).Scan(
		&c.ID, &c.EmpresaID, &c.Nivel, &c.AtrasoMinutos, &c.CreatedAt,
	); err != nil {
		return fmt.Errorf("atualizar config escalonamento: %w", err)
	}

	if _, err := tx.Exec(ctx, `DELETE FROM config_escalonamento_destinatarios WHERE config_escalonamento_id = $1`, c.ID); err != nil {
		return fmt.Errorf("limpar destinatarios: %w", err)
	}
	if err := inserirDestinatariosEscalonamento(ctx, tx, c.ID, c.UsuarioIDs); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func (r *ConfigEscalonamentoRepository) Upsert(ctx context.Context, c *model.ConfigEscalonamento) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("iniciar transacao: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	query := `
		INSERT INTO config_escalonamento (empresa_id, nivel, atraso_minutos)
		VALUES ($1, $2, $3)
		ON CONFLICT (empresa_id, nivel)
		DO UPDATE SET atraso_minutos = $3
		RETURNING id, created_at
	`
	if err := tx.QueryRow(ctx, query, c.EmpresaID, c.Nivel, c.AtrasoMinutos).Scan(&c.ID, &c.CreatedAt); err != nil {
		return fmt.Errorf("atualizar config escalonamento: %w", err)
	}

	if _, err := tx.Exec(ctx, `DELETE FROM config_escalonamento_destinatarios WHERE config_escalonamento_id = $1`, c.ID); err != nil {
		return fmt.Errorf("limpar destinatarios: %w", err)
	}
	if err := inserirDestinatariosEscalonamento(ctx, tx, c.ID, c.UsuarioIDs); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func (r *ConfigEscalonamentoRepository) Delete(ctx context.Context, id, empresaID uuid.UUID) error {
	query := `DELETE FROM config_escalonamento WHERE id = $1 AND empresa_id = $2`
	ct, err := r.db.Exec(ctx, query, id, empresaID)
	if err != nil {
		return fmt.Errorf("deletar config escalonamento: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return fmt.Errorf("config escalonamento nao encontrado")
	}
	return nil
}

func (r *ConfigEscalonamentoRepository) DeleteByEmpresa(ctx context.Context, empresaID uuid.UUID) error {
	_, err := r.db.Exec(ctx, `DELETE FROM config_escalonamento WHERE empresa_id = $1`, empresaID)
	if err != nil {
		return fmt.Errorf("deletar configs escalonamento: %w", err)
	}
	return nil
}

func (r *ConfigEscalonamentoRepository) ReplaceByEmpresa(ctx context.Context, empresaID uuid.UUID, configs []model.ConfigEscalonamento) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("iniciar transacao: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, `DELETE FROM config_escalonamento WHERE empresa_id = $1`, empresaID); err != nil {
		return fmt.Errorf("deletar configs existentes: %w", err)
	}

	for i := range configs {
		configs[i].EmpresaID = empresaID
		var configID uuid.UUID
		query := `
			INSERT INTO config_escalonamento (empresa_id, nivel, atraso_minutos)
			VALUES ($1, $2, $3)
			RETURNING id
		`
		if err := tx.QueryRow(ctx, query,
			configs[i].EmpresaID, configs[i].Nivel, configs[i].AtrasoMinutos,
		).Scan(&configID); err != nil {
			return fmt.Errorf("inserir config nivel %d: %w", configs[i].Nivel, err)
		}
		configs[i].ID = configID

		if err := inserirDestinatariosEscalonamento(ctx, tx, configID, configs[i].UsuarioIDs); err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}

func inserirDestinatariosEscalonamento(ctx context.Context, tx pgx.Tx, configID uuid.UUID, usuarioIDs []uuid.UUID) error {
	for _, usuarioID := range usuarioIDs {
		if _, err := tx.Exec(ctx, `
			INSERT INTO config_escalonamento_destinatarios (config_escalonamento_id, usuario_id)
			VALUES ($1, $2)
		`, configID, usuarioID); err != nil {
			return fmt.Errorf("inserir destinatario de escalonamento: %w", err)
		}
	}
	return nil
}
```

- [ ] **Step 2: Compilar (ainda vai falhar por causa do service — esperado)**

Run: `go build ./...`
Expected: FAIL apenas em `internal/service/alerta_service.go` (referencias a `WhatsappPara`/`CargoAlvo`/`strPtr`). Corrigido na Task 6.

- [ ] **Step 3: Commit**

```bash
git add internal/repository/config_escalonamento_repository.go
git commit -m "refactor: destinatarios de escalonamento por usuario, sem cargo/whatsapp"
```

---

### Task 4: Novo `ConfigAlertaEmergenciaRepository`

**Files:**
- Create: `internal/repository/config_alerta_emergencia_repository.go`

**Interfaces:**
- Consumes: `model.ConfigAlertaEmergencia` (Task 2).
- Produces: `ConfigAlertaEmergenciaRepository.{FindByEmpresa(ctx, empresaID) ([]model.ConfigAlertaEmergencia, error), FindByEmpresaETipo(ctx, empresaID, tipo string) (*model.ConfigAlertaEmergencia, error), Upsert(ctx, c *model.ConfigAlertaEmergencia) error}`.

- [ ] **Step 1: Criar o arquivo**

```go
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

type ConfigAlertaEmergenciaRepository struct {
	db *pgxpool.Pool
}

func NewConfigAlertaEmergenciaRepository(db *pgxpool.Pool) *ConfigAlertaEmergenciaRepository {
	return &ConfigAlertaEmergenciaRepository{db: db}
}

func (r *ConfigAlertaEmergenciaRepository) FindByEmpresa(ctx context.Context, empresaID uuid.UUID) ([]model.ConfigAlertaEmergencia, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, empresa_id, tipo, created_at
		FROM config_alerta_emergencia
		WHERE empresa_id = $1
		ORDER BY tipo ASC
	`, empresaID)
	if err != nil {
		return nil, fmt.Errorf("listar config alerta emergencia: %w", err)
	}
	defer rows.Close()

	var configs []model.ConfigAlertaEmergencia
	for rows.Next() {
		var c model.ConfigAlertaEmergencia
		if err := rows.Scan(&c.ID, &c.EmpresaID, &c.Tipo, &c.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan config alerta emergencia: %w", err)
		}
		configs = append(configs, c)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	destinatarios, err := r.destinatariosPorEmpresa(ctx, empresaID)
	if err != nil {
		return nil, err
	}
	for i := range configs {
		configs[i].UsuarioIDs = destinatarios[configs[i].ID]
	}
	return configs, nil
}

func (r *ConfigAlertaEmergenciaRepository) destinatariosPorEmpresa(ctx context.Context, empresaID uuid.UUID) (map[uuid.UUID][]uuid.UUID, error) {
	rows, err := r.db.Query(ctx, `
		SELECT d.config_alerta_emergencia_id, d.usuario_id
		FROM config_alerta_emergencia_destinatarios d
		JOIN config_alerta_emergencia c ON c.id = d.config_alerta_emergencia_id
		WHERE c.empresa_id = $1
	`, empresaID)
	if err != nil {
		return nil, fmt.Errorf("listar destinatarios de emergencia: %w", err)
	}
	defer rows.Close()

	result := make(map[uuid.UUID][]uuid.UUID)
	for rows.Next() {
		var configID, usuarioID uuid.UUID
		if err := rows.Scan(&configID, &usuarioID); err != nil {
			return nil, fmt.Errorf("scan destinatario de emergencia: %w", err)
		}
		result[configID] = append(result[configID], usuarioID)
	}
	return result, rows.Err()
}

func (r *ConfigAlertaEmergenciaRepository) FindByEmpresaETipo(ctx context.Context, empresaID uuid.UUID, tipo string) (*model.ConfigAlertaEmergencia, error) {
	var c model.ConfigAlertaEmergencia
	err := r.db.QueryRow(ctx, `
		SELECT id, empresa_id, tipo, created_at
		FROM config_alerta_emergencia
		WHERE empresa_id = $1 AND tipo = $2
	`, empresaID, tipo).Scan(&c.ID, &c.EmpresaID, &c.Tipo, &c.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("buscar config alerta emergencia: %w", err)
	}

	rows, err := r.db.Query(ctx, `SELECT usuario_id FROM config_alerta_emergencia_destinatarios WHERE config_alerta_emergencia_id = $1`, c.ID)
	if err != nil {
		return nil, fmt.Errorf("listar destinatarios: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var usuarioID uuid.UUID
		if err := rows.Scan(&usuarioID); err != nil {
			return nil, fmt.Errorf("scan destinatario: %w", err)
		}
		c.UsuarioIDs = append(c.UsuarioIDs, usuarioID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return &c, nil
}

func (r *ConfigAlertaEmergenciaRepository) Upsert(ctx context.Context, c *model.ConfigAlertaEmergencia) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("iniciar transacao: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := tx.QueryRow(ctx, `
		INSERT INTO config_alerta_emergencia (empresa_id, tipo)
		VALUES ($1, $2)
		ON CONFLICT (empresa_id, tipo) DO UPDATE SET tipo = EXCLUDED.tipo
		RETURNING id, created_at
	`, c.EmpresaID, c.Tipo).Scan(&c.ID, &c.CreatedAt); err != nil {
		return fmt.Errorf("upsert config alerta emergencia: %w", err)
	}

	if _, err := tx.Exec(ctx, `DELETE FROM config_alerta_emergencia_destinatarios WHERE config_alerta_emergencia_id = $1`, c.ID); err != nil {
		return fmt.Errorf("limpar destinatarios: %w", err)
	}

	for _, usuarioID := range c.UsuarioIDs {
		if _, err := tx.Exec(ctx, `
			INSERT INTO config_alerta_emergencia_destinatarios (config_alerta_emergencia_id, usuario_id)
			VALUES ($1, $2)
		`, c.ID, usuarioID); err != nil {
			return fmt.Errorf("inserir destinatario: %w", err)
		}
	}

	return tx.Commit(ctx)
}
```

- [ ] **Step 2: Compilar**

Run: `go build ./...`
Expected: FAIL apenas em `internal/service/alerta_service.go` (ainda não foi atualizado — Task 6).

- [ ] **Step 3: Commit**

```bash
git add internal/repository/config_alerta_emergencia_repository.go
git commit -m "feat: repository de destinatarios por tipo de alerta de emergencia"
```

---

### Task 5: `AlertaRepository.CloseAlertasResolvidoCheckin`

**Files:**
- Modify: `internal/repository/alerta_repository.go`

**Interfaces:**
- Produces: `AlertaRepository.CloseAlertasResolvidoCheckin(ctx, turnoID uuid.UUID) (int64, error)`.

- [ ] **Step 1: Adicionar o metodo logo apos `CloseAlertasFalsoPositivo`**

Em `internal/repository/alerta_repository.go`, adicione (após a função `CloseAlertasFalsoPositivo`, linha ~219):

```go
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
```

- [ ] **Step 2: Compilar**

Run: `go build ./internal/repository/...`
Expected: PASS (o pacote `repository` compila sozinho; o restante do binário ainda falha por causa do service, corrigido na Task 6).

- [ ] **Step 3: Commit**

```bash
git add internal/repository/alerta_repository.go
git commit -m "feat: fechar alertas de atraso automaticamente ao receber checkin"
```

---

### Task 6: Reescrever `AlertaService`

**Files:**
- Modify: `internal/service/alerta_service.go`

**Interfaces:**
- Consumes: `ConfigEscalonamentoRepository` (Task 3), `ConfigAlertaEmergenciaRepository` (Task 4), `AlertaRepository.CloseAlertasResolvidoCheckin` (Task 5), `UserRepository.FindByIDEmpresa(ctx, empresaID, id uuid.UUID) (*model.User, error)` (já existe em `internal/repository/user_repository.go:67`).
- Produces: `NewAlertaService(alertaRepo, configRepo, configEmergenciaRepo, turnoRepo, checkinRepo, userRepo, hub) *AlertaService`; `AlertaService.{CreateAlerta, CreateAlertaImediato, ResolverAlertasAtraso(ctx, turnoID uuid.UUID) error, GetAlertasEmergencia(ctx, empresaID string) ([]model.ConfigAlertaEmergencia, error), UpdateAlertaEmergencia(ctx, empresaID, tipo string, req model.UpdateConfigAlertaEmergenciaRequest) (*model.ConfigAlertaEmergencia, error)}`; erros `ErrUsuarioNaoPertenceAEmpresa`, `ErrTipoEmergenciaInvalido`.

- [ ] **Step 1: Substituir o arquivo inteiro**

```go
package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/guardpoint/guardpoint-server/internal/model"
	"github.com/guardpoint/guardpoint-server/internal/repository"
	"github.com/guardpoint/guardpoint-server/internal/ws"
)

var (
	ErrAlertaNaoEncontrado          = errors.New("alerta nao encontrado")
	ErrAlertaTransicaoInvalida      = errors.New("transicao de status do alerta invalida")
	ErrConfigEscalonamentoDuplicado = errors.New("nivel de escalonamento ja existe para esta empresa")
	ErrUsuarioNaoPertenceAEmpresa   = errors.New("usuario nao pertence a empresa")
	ErrTipoEmergenciaInvalido       = errors.New("tipo de alerta de emergencia invalido")
)

var tiposEmergencia = []string{"coacao", "sabotagem", "no_show"}

type AlertaService struct {
	alertaRepo           *repository.AlertaRepository
	configRepo           *repository.ConfigEscalonamentoRepository
	configEmergenciaRepo *repository.ConfigAlertaEmergenciaRepository
	turnoRepo            *repository.TurnoRepository
	checkinRepo          *repository.CheckinRepository
	userRepo             *repository.UserRepository
	alertChannel         chan *model.PendingAlert
	hub                  *ws.Hub
}

func NewAlertaService(
	alertaRepo *repository.AlertaRepository,
	configRepo *repository.ConfigEscalonamentoRepository,
	configEmergenciaRepo *repository.ConfigAlertaEmergenciaRepository,
	turnoRepo *repository.TurnoRepository,
	checkinRepo *repository.CheckinRepository,
	userRepo *repository.UserRepository,
	hub *ws.Hub,
) *AlertaService {
	return &AlertaService{
		alertaRepo:           alertaRepo,
		configRepo:           configRepo,
		configEmergenciaRepo: configEmergenciaRepo,
		turnoRepo:            turnoRepo,
		checkinRepo:          checkinRepo,
		userRepo:             userRepo,
		alertChannel:         make(chan *model.PendingAlert, 100),
		hub:                  hub,
	}
}

func (s *AlertaService) AlertChannel() <-chan *model.PendingAlert {
	return s.alertChannel
}

// CreateAlerta cria um alerta de escalonamento por atraso, com deduplicacao
// por (turno, tipo). Os destinatarios vem da configuracao do nivel informado.
func (s *AlertaService) CreateAlerta(ctx context.Context, empresaID, turnoID uuid.UUID, tipo string, nivel int, mensagem string) (*model.Alerta, error) {
	count, err := s.alertaRepo.CountByTurnoETipo(ctx, turnoID, tipo)
	if err != nil {
		return nil, fmt.Errorf("verificar duplicidade: %w", err)
	}
	if count > 0 {
		return nil, nil
	}

	usuarioIDs, err := s.destinatariosPorNivel(ctx, empresaID, nivel)
	if err != nil {
		return nil, fmt.Errorf("resolver destinatarios: %w", err)
	}

	return s.criarAlerta(ctx, empresaID, turnoID, tipo, nivel, mensagem, usuarioIDs)
}

// CreateAlertaImediato cria um alerta de emergencia (coacao, sabotagem,
// no-show), sem deduplicacao. Os destinatarios vem da configuracao especifica
// do tipo de emergencia (config_alerta_emergencia), independente dos niveis
// de escalonamento por atraso.
func (s *AlertaService) CreateAlertaImediato(ctx context.Context, empresaID, turnoID uuid.UUID, tipo string, nivel int, mensagem string) (*model.Alerta, error) {
	usuarioIDs, err := s.destinatariosPorTipoEmergencia(ctx, empresaID, tipo)
	if err != nil {
		return nil, fmt.Errorf("resolver destinatarios: %w", err)
	}

	return s.criarAlerta(ctx, empresaID, turnoID, tipo, nivel, mensagem, usuarioIDs)
}

func (s *AlertaService) criarAlerta(ctx context.Context, empresaID, turnoID uuid.UUID, tipo string, nivel int, mensagem string, usuarioIDs []uuid.UUID) (*model.Alerta, error) {
	msg := &mensagem
	if mensagem == "" {
		msg = nil
	}

	turnoRef, turnoStr := nullableTurno(turnoID)

	alerta := &model.Alerta{
		EmpresaID: empresaID,
		TurnoID:   turnoRef,
		Tipo:      tipo,
		Nivel:     nivel,
		Status:    "aberto",
		Mensagem:  msg,
	}

	if err := s.alertaRepo.Create(ctx, alerta); err != nil {
		return nil, fmt.Errorf("criar alerta: %w", err)
	}

	s.hub.Broadcast(empresaID.String(), ws.NewAlertEvent(alerta.ID.String(), tipo, turnoStr, nivel))

	if len(usuarioIDs) > 0 {
		select {
		case s.alertChannel <- &model.PendingAlert{Alerta: alerta, UsuarioIDs: usuarioIDs}:
		default:
		}
	}

	return alerta, nil
}

func (s *AlertaService) destinatariosPorNivel(ctx context.Context, empresaID uuid.UUID, nivel int) ([]uuid.UUID, error) {
	cfg, err := s.configRepo.FindByEmpresaENivel(ctx, empresaID, nivel)
	if err != nil {
		return nil, err
	}
	if cfg == nil {
		return nil, nil
	}
	return cfg.UsuarioIDs, nil
}

func (s *AlertaService) destinatariosPorTipoEmergencia(ctx context.Context, empresaID uuid.UUID, tipo string) ([]uuid.UUID, error) {
	cfg, err := s.configEmergenciaRepo.FindByEmpresaETipo(ctx, empresaID, tipo)
	if err != nil {
		return nil, err
	}
	if cfg == nil {
		return nil, nil
	}
	return cfg.UsuarioIDs, nil
}

// ResolverAlertasAtraso fecha os alertas de atraso abertos do turno quando um
// check-in chega (online ou via lote offline), resetando o deadman's switch.
func (s *AlertaService) ResolverAlertasAtraso(ctx context.Context, turnoID uuid.UUID) error {
	if _, err := s.alertaRepo.CloseAlertasResolvidoCheckin(ctx, turnoID); err != nil {
		return fmt.Errorf("resolver alertas de atraso: %w", err)
	}
	return nil
}

func (s *AlertaService) List(ctx context.Context, empresaID string, filter model.AlertaFilter) ([]model.Alerta, int, error) {
	parsedEmpresaID, err := uuid.Parse(empresaID)
	if err != nil {
		return nil, 0, fmt.Errorf("empresa_id invalido: %w", err)
	}
	return s.alertaRepo.List(ctx, parsedEmpresaID, filter)
}

func (s *AlertaService) Reconhecer(ctx context.Context, empresaID, alertaID string) error {
	parsedEmpresaID, err := uuid.Parse(empresaID)
	if err != nil {
		return fmt.Errorf("empresa_id invalido: %w", err)
	}
	parsedAlertaID, err := uuid.Parse(alertaID)
	if err != nil {
		return fmt.Errorf("alerta_id invalido: %w", err)
	}

	alerta, err := s.alertaRepo.FindByID(ctx, parsedEmpresaID, parsedAlertaID)
	if err != nil {
		return ErrAlertaNaoEncontrado
	}

	if alerta.Status != "aberto" {
		return fmt.Errorf("%w: alerta nao esta aberto", ErrAlertaTransicaoInvalida)
	}

	if err := s.alertaRepo.UpdateStatus(ctx, parsedAlertaID, parsedEmpresaID, "reconhecido", nil); err != nil {
		return fmt.Errorf("reconhecer alerta: %w", err)
	}
	return nil
}

func (s *AlertaService) Encerrar(ctx context.Context, empresaID, alertaID string) error {
	parsedEmpresaID, err := uuid.Parse(empresaID)
	if err != nil {
		return fmt.Errorf("empresa_id invalido: %w", err)
	}
	parsedAlertaID, err := uuid.Parse(alertaID)
	if err != nil {
		return fmt.Errorf("alerta_id invalido: %w", err)
	}

	alerta, err := s.alertaRepo.FindByID(ctx, parsedEmpresaID, parsedAlertaID)
	if err != nil {
		return ErrAlertaNaoEncontrado
	}

	if alerta.Status == "encerrado" {
		return fmt.Errorf("%w: alerta ja esta encerrado", ErrAlertaTransicaoInvalida)
	}

	now := time.Now()
	if err := s.alertaRepo.UpdateStatus(ctx, parsedAlertaID, parsedEmpresaID, "encerrado", &now); err != nil {
		return fmt.Errorf("encerrar alerta: %w", err)
	}
	return nil
}

func (s *AlertaService) GetEstatisticas(ctx context.Context, empresaID string) (*model.AlertStatistics, error) {
	parsedEmpresaID, err := uuid.Parse(empresaID)
	if err != nil {
		return nil, fmt.Errorf("empresa_id invalido: %w", err)
	}

	stats := &model.AlertStatistics{}

	alertas, total, err := s.alertaRepo.List(ctx, parsedEmpresaID, model.AlertaFilter{Limit: 1000})
	if err != nil {
		return nil, fmt.Errorf("listar alertas: %w", err)
	}
	_ = total

	for _, a := range alertas {
		switch a.Status {
		case "aberto":
			stats.TotalAbertos++
		case "reconhecido":
			stats.TotalReconhecidos++
		case "encerrado":
			stats.TotalEncerrados++
		}
	}

	porTipo, err := s.alertaRepo.CountPorTipo(ctx, parsedEmpresaID)
	if err == nil {
		stats.PorTipo = porTipo
	}

	porHora, err := s.alertaRepo.CountPorHora(ctx, parsedEmpresaID)
	if err == nil {
		stats.PorHora = porHora
	}

	return stats, nil
}

func (s *AlertaService) GetEscalonamento(ctx context.Context, empresaID string) ([]model.ConfigEscalonamento, error) {
	parsedEmpresaID, err := uuid.Parse(empresaID)
	if err != nil {
		return nil, fmt.Errorf("empresa_id invalido: %w", err)
	}
	return s.configRepo.FindByEmpresa(ctx, parsedEmpresaID)
}

func (s *AlertaService) CreateEscalonamento(ctx context.Context, empresaID string, req model.CreateConfigEscalonamentoRequest) (*model.ConfigEscalonamento, error) {
	parsedEmpresaID, err := uuid.Parse(empresaID)
	if err != nil {
		return nil, fmt.Errorf("empresa_id invalido: %w", err)
	}

	if err := s.validarUsuariosDaEmpresa(ctx, parsedEmpresaID, req.UsuarioIDs); err != nil {
		return nil, err
	}

	existing, err := s.configRepo.FindByEmpresaENivel(ctx, parsedEmpresaID, req.Nivel)
	if err != nil {
		return nil, fmt.Errorf("verificar nivel existente: %w", err)
	}
	if existing != nil {
		c := &model.ConfigEscalonamento{
			EmpresaID:     parsedEmpresaID,
			Nivel:         req.Nivel,
			AtrasoMinutos: req.AtrasoMinutos,
			UsuarioIDs:    req.UsuarioIDs,
		}
		if err := s.configRepo.Upsert(ctx, c); err != nil {
			return nil, fmt.Errorf("atualizar config escalonamento: %w", err)
		}
		return c, nil
	}

	c := &model.ConfigEscalonamento{
		EmpresaID:     parsedEmpresaID,
		Nivel:         req.Nivel,
		AtrasoMinutos: req.AtrasoMinutos,
		UsuarioIDs:    req.UsuarioIDs,
	}

	if err := s.configRepo.Create(ctx, c); err != nil {
		return nil, fmt.Errorf("criar config escalonamento: %w", err)
	}
	return c, nil
}

func (s *AlertaService) UpdateEscalonamento(ctx context.Context, empresaID, configID string, req model.UpdateConfigEscalonamentoRequest) (*model.ConfigEscalonamento, error) {
	parsedEmpresaID, err := uuid.Parse(empresaID)
	if err != nil {
		return nil, fmt.Errorf("empresa_id invalido: %w", err)
	}
	parsedConfigID, err := uuid.Parse(configID)
	if err != nil {
		return nil, fmt.Errorf("config_id invalido: %w", err)
	}

	if err := s.validarUsuariosDaEmpresa(ctx, parsedEmpresaID, req.UsuarioIDs); err != nil {
		return nil, err
	}

	c := &model.ConfigEscalonamento{
		AtrasoMinutos: req.AtrasoMinutos,
		UsuarioIDs:    req.UsuarioIDs,
	}

	if err := s.configRepo.Update(ctx, parsedConfigID, parsedEmpresaID, c); err != nil {
		return nil, fmt.Errorf("atualizar config escalonamento: %w", err)
	}
	return c, nil
}

func (s *AlertaService) DeleteEscalonamento(ctx context.Context, empresaID, configID string) error {
	parsedEmpresaID, err := uuid.Parse(empresaID)
	if err != nil {
		return fmt.Errorf("empresa_id invalido: %w", err)
	}
	parsedConfigID, err := uuid.Parse(configID)
	if err != nil {
		return fmt.Errorf("config_id invalido: %w", err)
	}
	return s.configRepo.Delete(ctx, parsedConfigID, parsedEmpresaID)
}

func (s *AlertaService) ReplaceEscalonamento(ctx context.Context, empresaID string, reqs []model.CreateConfigEscalonamentoRequest) ([]model.ConfigEscalonamento, error) {
	parsedEmpresaID, err := uuid.Parse(empresaID)
	if err != nil {
		return nil, fmt.Errorf("empresa_id invalido: %w", err)
	}

	configs := make([]model.ConfigEscalonamento, 0, len(reqs))
	for _, req := range reqs {
		if err := s.validarUsuariosDaEmpresa(ctx, parsedEmpresaID, req.UsuarioIDs); err != nil {
			return nil, err
		}
		configs = append(configs, model.ConfigEscalonamento{
			Nivel:         req.Nivel,
			AtrasoMinutos: req.AtrasoMinutos,
			UsuarioIDs:    req.UsuarioIDs,
		})
	}

	if err := s.configRepo.ReplaceByEmpresa(ctx, parsedEmpresaID, configs); err != nil {
		return nil, fmt.Errorf("substituir configs: %w", err)
	}

	result, err := s.configRepo.FindByEmpresa(ctx, parsedEmpresaID)
	if err != nil {
		return nil, fmt.Errorf("buscar configs apos replace: %w", err)
	}
	return result, nil
}

// GetAlertasEmergencia sempre retorna os 3 tipos fixos (coacao, sabotagem,
// no_show), com lista de usuarios vazia para os que ainda nao tem configuracao
// salva.
func (s *AlertaService) GetAlertasEmergencia(ctx context.Context, empresaID string) ([]model.ConfigAlertaEmergencia, error) {
	parsedEmpresaID, err := uuid.Parse(empresaID)
	if err != nil {
		return nil, fmt.Errorf("empresa_id invalido: %w", err)
	}

	existentes, err := s.configEmergenciaRepo.FindByEmpresa(ctx, parsedEmpresaID)
	if err != nil {
		return nil, fmt.Errorf("listar config alerta emergencia: %w", err)
	}

	porTipo := make(map[string]model.ConfigAlertaEmergencia, len(existentes))
	for _, c := range existentes {
		porTipo[c.Tipo] = c
	}

	resultado := make([]model.ConfigAlertaEmergencia, 0, len(tiposEmergencia))
	for _, tipo := range tiposEmergencia {
		if c, ok := porTipo[tipo]; ok {
			resultado = append(resultado, c)
			continue
		}
		resultado = append(resultado, model.ConfigAlertaEmergencia{
			EmpresaID:  parsedEmpresaID,
			Tipo:       tipo,
			UsuarioIDs: []uuid.UUID{},
		})
	}
	return resultado, nil
}

func (s *AlertaService) UpdateAlertaEmergencia(ctx context.Context, empresaID, tipo string, req model.UpdateConfigAlertaEmergenciaRequest) (*model.ConfigAlertaEmergencia, error) {
	if !tipoEmergenciaValido(tipo) {
		return nil, ErrTipoEmergenciaInvalido
	}

	parsedEmpresaID, err := uuid.Parse(empresaID)
	if err != nil {
		return nil, fmt.Errorf("empresa_id invalido: %w", err)
	}

	if err := s.validarUsuariosDaEmpresa(ctx, parsedEmpresaID, req.UsuarioIDs); err != nil {
		return nil, err
	}

	c := &model.ConfigAlertaEmergencia{
		EmpresaID:  parsedEmpresaID,
		Tipo:       tipo,
		UsuarioIDs: req.UsuarioIDs,
	}
	if err := s.configEmergenciaRepo.Upsert(ctx, c); err != nil {
		return nil, fmt.Errorf("atualizar config alerta emergencia: %w", err)
	}
	return c, nil
}

func tipoEmergenciaValido(tipo string) bool {
	for _, t := range tiposEmergencia {
		if t == tipo {
			return true
		}
	}
	return false
}

func (s *AlertaService) validarUsuariosDaEmpresa(ctx context.Context, empresaID uuid.UUID, usuarioIDs []uuid.UUID) error {
	for _, usuarioID := range usuarioIDs {
		if _, err := s.userRepo.FindByIDEmpresa(ctx, empresaID, usuarioID); err != nil {
			return fmt.Errorf("%w: %s", ErrUsuarioNaoPertenceAEmpresa, usuarioID)
		}
	}
	return nil
}

// nullableTurno converte um turnoID em ponteiro nulo quando for o UUID zero
// (caso dos alertas de no-show, que nao possuem turno associado).
// Retorna tambem a representacao string usada no evento WebSocket ("" quando nulo).
func nullableTurno(turnoID uuid.UUID) (*uuid.UUID, string) {
	if turnoID == uuid.Nil {
		return nil, ""
	}
	id := turnoID
	return &id, id.String()
}
```

- [ ] **Step 2: Compilar**

Run: `go build ./...`
Expected: FAIL apenas em `internal/handler/alerta.go` e `internal/app/app.go` (ainda chamam `NewAlertaService` com a assinatura antiga, e o handler ainda referencia campos removidos). Corrigido nas próximas tasks.

- [ ] **Step 3: Commit**

```bash
git add internal/service/alerta_service.go
git commit -m "refactor: resolver destinatarios por usuario no escalonamento e por tipo nas emergencias"
```

---

### Task 7: Handler e wiring — `internal/handler/alerta.go` + `internal/app/app.go`

**Files:**
- Modify: `internal/handler/alerta.go`
- Modify: `internal/app/app.go`

**Interfaces:**
- Consumes: `service.AlertaService` novos métodos (Task 6).
- Produces: rotas `GET /api/v1/config/alertas-emergencia`, `PUT /api/v1/config/alertas-emergencia/{tipo}`; `AlertaHandler.{GetAlertasEmergencia, PutAlertaEmergencia}`.

- [ ] **Step 1: Adicionar tratamento de erro nos handlers de escalonamento existentes**

Em `internal/handler/alerta.go`, na função `CreateEscalonamento` (por volta da linha 191), substitua:

```go
	config, err := h.alertaService.CreateEscalonamento(r.Context(), empresaID, req)
	if err != nil {
		slog.Error("create escalonamento failed", "error", err)
		writeError(w, http.StatusInternalServerError, "erro ao criar configuracao")
		return
	}
```

por:

```go
	config, err := h.alertaService.CreateEscalonamento(r.Context(), empresaID, req)
	if err != nil {
		if errors.Is(err, service.ErrUsuarioNaoPertenceAEmpresa) {
			writeError(w, http.StatusBadRequest, "usuario_id invalido para esta empresa")
			return
		}
		slog.Error("create escalonamento failed", "error", err)
		writeError(w, http.StatusInternalServerError, "erro ao criar configuracao")
		return
	}
```

Faça o mesmo ajuste em `UpdateEscalonamento` (em torno da linha 225) e `PutEscalonamento` (em torno da linha 284), trocando `h.alertaService.UpdateEscalonamento(...)` / `h.alertaService.ReplaceEscalonamento(...)` pela mesma checagem de `errors.Is(err, service.ErrUsuarioNaoPertenceAEmpresa)` antes do log genérico.

- [ ] **Step 2: Adicionar os dois novos handlers ao final do arquivo**

```go
// GetAlertasEmergencia godoc
// @Summary      Lista os destinatarios configurados por tipo de alerta de emergencia (somente admin)
// @Tags         config
// @Success      200 {array} model.ConfigAlertaEmergencia
// @Failure      500 {object} map[string]string
// @Router       /config/alertas-emergencia [get]
func (h *AlertaHandler) GetAlertasEmergencia(w http.ResponseWriter, r *http.Request) {
	empresaID := GetEmpresaID(r.Context())

	configs, err := h.alertaService.GetAlertasEmergencia(r.Context(), empresaID)
	if err != nil {
		slog.Error("get alertas emergencia failed", "error", err)
		writeError(w, http.StatusInternalServerError, "erro ao carregar configuracao")
		return
	}

	writeJSON(w, http.StatusOK, configs)
}

// PutAlertaEmergencia godoc
// @Summary      Define os destinatarios de um tipo de alerta de emergencia (somente admin)
// @Tags         config
// @Param        tipo path string true "Tipo de emergencia (coacao, sabotagem, no_show)"
// @Param        request body model.UpdateConfigAlertaEmergenciaRequest true "Lista de usuarios destinatarios"
// @Success      200 {object} model.ConfigAlertaEmergencia
// @Failure      400 {object} map[string]string
// @Failure      500 {object} map[string]string
// @Router       /config/alertas-emergencia/{tipo} [put]
func (h *AlertaHandler) PutAlertaEmergencia(w http.ResponseWriter, r *http.Request) {
	empresaID := GetEmpresaID(r.Context())
	tipo := chi.URLParam(r, "tipo")

	var req model.UpdateConfigAlertaEmergenciaRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "json invalido")
		return
	}

	if err := h.validate.Struct(req); err != nil {
		writeValidationError(w, err)
		return
	}

	config, err := h.alertaService.UpdateAlertaEmergencia(r.Context(), empresaID, tipo, req)
	if err != nil {
		if errors.Is(err, service.ErrTipoEmergenciaInvalido) {
			writeError(w, http.StatusBadRequest, "tipo de emergencia invalido")
			return
		}
		if errors.Is(err, service.ErrUsuarioNaoPertenceAEmpresa) {
			writeError(w, http.StatusBadRequest, "usuario_id invalido para esta empresa")
			return
		}
		slog.Error("put alerta emergencia failed", "error", err)
		writeError(w, http.StatusInternalServerError, "erro ao salvar configuracao")
		return
	}

	writeJSON(w, http.StatusOK, config)
}
```

- [ ] **Step 3: Atualizar `internal/app/app.go`**

Adicione o repositório novo logo após a linha que cria `configEscalonamentoRepo` (linha 54):

```go
	configEscalonamentoRepo := repository.NewConfigEscalonamentoRepository(pool)
	configAlertaEmergenciaRepo := repository.NewConfigAlertaEmergenciaRepository(pool)
```

Atualize a construção do `alertaService` (linha 63) para a nova assinatura:

```go
	alertaService := service.NewAlertaService(alertaRepo, configEscalonamentoRepo, configAlertaEmergenciaRepo, turnoRepo, checkinRepo, userRepo, hub)
```

Adicione as duas rotas novas dentro do bloco `r.Route("/config", ...)` (linha ~165-172):

```go
		r.Route("/config", func(r chi.Router) {
			r.Use(handler.RequireRole("admin"))
			r.Get("/escalonamento", alertaHandler.GetEscalonamento)
			r.Put("/escalonamento", alertaHandler.PutEscalonamento)
			r.Post("/escalonamento", alertaHandler.CreateEscalonamento)
			r.Put("/escalonamento/{id}", alertaHandler.UpdateEscalonamento)
			r.Delete("/escalonamento/{id}", alertaHandler.DeleteEscalonamento)
			r.Get("/alertas-emergencia", alertaHandler.GetAlertasEmergencia)
			r.Put("/alertas-emergencia/{tipo}", alertaHandler.PutAlertaEmergencia)
		})
```

- [ ] **Step 4: Compilar**

Run: `go build ./...`
Expected: PASS (todo o binário compila).

Run: `go vet ./...`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/handler/alerta.go internal/app/app.go
git commit -m "feat: endpoints de configuracao de destinatarios por tipo de emergencia"
```

---

### Task 8: Auto-resolução de alertas no check-in — `TurnoService`

**Files:**
- Modify: `internal/service/turno_service.go`

**Interfaces:**
- Consumes: `AlertaService.ResolverAlertasAtraso(ctx, turnoID uuid.UUID) error` (Task 6).
- Produces: efeito colateral — check-in (online e lote) fecha alertas de atraso abertos do turno.

- [ ] **Step 1: Adicionar o import `log/slog`**

No topo de `internal/service/turno_service.go`, adicione `"log/slog"` ao bloco de imports (em ordem alfabética, junto com `"math"`):

```go
import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"math/big"
	"time"

	"github.com/google/uuid"

	"github.com/guardpoint/guardpoint-server/internal/model"
	"github.com/guardpoint/guardpoint-server/internal/repository"
	"github.com/guardpoint/guardpoint-server/internal/ws"
)
```

- [ ] **Step 2: Chamar `ResolverAlertasAtraso` apos o check-in online**

Em `TurnoService.Checkin`, logo apos o bloco:

```go
	if err := s.checkinRepo.Create(ctx, checkin); err != nil {
		return nil, fmt.Errorf("criar checkin: %w", err)
	}
```

adicione:

```go
	if err := s.checkinRepo.Create(ctx, checkin); err != nil {
		return nil, fmt.Errorf("criar checkin: %w", err)
	}

	if err := s.alertaService.ResolverAlertasAtraso(ctx, parsedTurnoID); err != nil {
		slog.Error("resolver alertas de atraso apos checkin", "error", err, "turno_id", parsedTurnoID.String())
	}
```

- [ ] **Step 3: Chamar `ResolverAlertasAtraso` no lote offline**

Em `TurnoService.ProcessarLote`, dentro do loop `for _, req := range checkins`, logo apos o bloco que grava o check-in (tanto o caminho `CreateIdempotent` quanto o `Create`):

```go
		if req.ClienteCheckinID != "" {
			cid := req.ClienteCheckinID
			checkin.ClienteCheckinID = &cid
			if _, err := s.checkinRepo.CreateIdempotent(ctx, checkin); err != nil {
				continue
			}
		} else {
			if err := s.checkinRepo.Create(ctx, checkin); err != nil {
				continue
			}
		}

		if err := s.alertaService.ResolverAlertasAtraso(ctx, parsedTurnoID); err != nil {
			slog.Error("resolver alertas de atraso apos checkin em lote", "error", err, "turno_id", parsedTurnoID.String())
		}
```

- [ ] **Step 4: Compilar**

Run: `go build ./...`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/service/turno_service.go
git commit -m "feat: fechar alertas de atraso automaticamente ao registrar checkin"
```

---

### Task 9: Corrigir teste existente que usa `whatsapp_para`

**Files:**
- Modify: `internal/integration/worker_test.go`

**Interfaces:**
- Consumes: `cenario.e.criarUsuario` (já existe em `helpers_test.go:64`), endpoint `PUT /api/v1/config/escalonamento`.

**Contexto:** este teste hoje quebra a compilação/execução porque envia `whatsapp_para`, campo que não existe mais. Precisa de usuários reais para preencher `usuario_ids`.

- [ ] **Step 1: Atualizar `TestTimeoutCheckerAtrasos`**

Em `internal/integration/worker_test.go`, substitua o corpo da função (linhas 16-51) por:

```go
// D3: TimeoutChecker gera atraso_nX conforme o atraso acumulado e nao duplica.
func TestTimeoutCheckerAtrasos(t *testing.T) {
	c := novoCenario(t)
	turno := c.iniciarTurno()

	supervisor := c.e.criarUsuario(c.empresa.ID, "Supervisor N1", "sup.n1@a.com", "senha123", "supervisor", true)
	gerente := c.e.criarUsuario(c.empresa.ID, "Gerente N2", "gerente.n2@a.com", "senha123", "supervisor", true)
	diretor := c.e.criarUsuario(c.empresa.ID, "Diretor N3", "diretor.n3@a.com", "senha123", "admin", true)

	c.e.reqJSON(http.MethodPut, "/api/v1/config/escalonamento", c.adminToken, []map[string]any{
		{"nivel": 1, "atraso_minutos": 5, "usuario_ids": []string{supervisor.ID.String()}},
		{"nivel": 2, "atraso_minutos": 15, "usuario_ids": []string{gerente.ID.String()}},
		{"nivel": 3, "atraso_minutos": 60, "usuario_ids": []string{diretor.ID.String()}},
	}, http.StatusOK, nil)

	// ultimo check-in ha 50 min; intervalo de 30 min => atraso de ~20 min
	c.e.reqJSON(http.MethodPost, "/api/v1/turnos/checkin", c.vigiaToken,
		c.checkinBody(turno.ID, "padrao", time.Now().Add(-50*time.Minute)), http.StatusOK, nil)

	c.e.app.TimeoutChecker.CheckOnce(t.Context())

	if n := c.contarAlertas("atraso_n1"); n != 1 {
		t.Errorf("alertas atraso_n1 = %d, esperado 1", n)
	}
	if n := c.contarAlertas("atraso_n2"); n != 1 {
		t.Errorf("alertas atraso_n2 = %d, esperado 1", n)
	}
	if n := c.contarAlertas("atraso_n3"); n != 0 {
		t.Errorf("alertas atraso_n3 = %d, esperado 0 (atraso de 20 min < 60)", n)
	}

	t.Run("segundo ciclo nao duplica", func(t *testing.T) {
		c.e.app.TimeoutChecker.CheckOnce(t.Context())
		if n := c.contarAlertas("atraso_n1"); n != 1 {
			t.Errorf("alertas atraso_n1 apos segundo ciclo = %d, esperado 1", n)
		}
		if n := c.contarAlertas("atraso_n2"); n != 1 {
			t.Errorf("alertas atraso_n2 apos segundo ciclo = %d, esperado 1", n)
		}
	})
}
```

(As demais funções do arquivo — `TestTimeoutCheckerNoShow`, `TestFindEscalasSemTurnoNoturna` — não usam `whatsapp_para` e ficam inalteradas.)

- [ ] **Step 2: Rodar os testes de integração**

Run: `go test -tags integration -p 1 ./internal/integration/... -run TestTimeoutChecker -v`
Expected: PASS (ambos os subtestes de `TestTimeoutCheckerAtrasos` e `TestTimeoutCheckerNoShow`).

- [ ] **Step 3: Commit**

```bash
git add internal/integration/worker_test.go
git commit -m "test: atualiza TestTimeoutCheckerAtrasos para usuario_ids"
```

---

### Task 10: Testes de integração — CRUD de escalonamento com destinatários

**Files:**
- Create: `internal/integration/alerta_test.go`

**Interfaces:**
- Consumes: `cenario` (helpers_test.go), endpoints `/api/v1/config/escalonamento`.

- [ ] **Step 1: Criar o arquivo com o teste de CRUD e validação cross-empresa**

```go
//go:build integration

package integration

import (
	"net/http"
	"testing"

	"github.com/guardpoint/guardpoint-server/internal/model"
)

// Destinatarios de escalonamento devem ser usuarios reais da mesma empresa.
func TestConfigEscalonamentoDestinatarios(t *testing.T) {
	c := novoCenario(t)

	supervisor := c.e.criarUsuario(c.empresa.ID, "Supervisor B", "sup.b@a.com", "senha123", "supervisor", true)

	t.Run("cria nivel com destinatarios", func(t *testing.T) {
		var config model.ConfigEscalonamento
		c.e.reqJSON(http.MethodPost, "/api/v1/config/escalonamento", c.adminToken, map[string]any{
			"nivel":          1,
			"atraso_minutos": 10,
			"usuario_ids":    []string{supervisor.ID.String()},
		}, http.StatusCreated, &config)

		if len(config.UsuarioIDs) != 1 || config.UsuarioIDs[0] != supervisor.ID {
			t.Fatalf("usuario_ids = %v, esperado [%s]", config.UsuarioIDs, supervisor.ID)
		}
	})

	t.Run("rejeita usuario de outra empresa", func(t *testing.T) {
		outraEmpresa := c.e.criarEmpresa("Empresa Outra", "22222222000191")
		usuarioOutraEmpresa := c.e.criarUsuario(outraEmpresa.ID, "Estranho", "estranho@b.com", "senha123", "supervisor", true)

		c.e.reqJSON(http.MethodPost, "/api/v1/config/escalonamento", c.adminToken, map[string]any{
			"nivel":          2,
			"atraso_minutos": 20,
			"usuario_ids":    []string{usuarioOutraEmpresa.ID.String()},
		}, http.StatusBadRequest, nil)
	})

	t.Run("rejeita lista vazia de destinatarios", func(t *testing.T) {
		c.e.reqJSON(http.MethodPost, "/api/v1/config/escalonamento", c.adminToken, map[string]any{
			"nivel":          3,
			"atraso_minutos": 30,
			"usuario_ids":    []string{},
		}, http.StatusBadRequest, nil)
	})

	t.Run("GET retorna destinatarios salvos", func(t *testing.T) {
		var configs []model.ConfigEscalonamento
		c.e.reqJSON(http.MethodGet, "/api/v1/config/escalonamento", c.adminToken, nil, http.StatusOK, &configs)

		if len(configs) != 1 {
			t.Fatalf("configs = %d, esperado 1 (apenas o nivel 1 criado com sucesso)", len(configs))
		}
		if len(configs[0].UsuarioIDs) != 1 || configs[0].UsuarioIDs[0] != supervisor.ID {
			t.Fatalf("usuario_ids = %v, esperado [%s]", configs[0].UsuarioIDs, supervisor.ID)
		}
	})
}
```

- [ ] **Step 2: Rodar o teste**

Run: `go test -tags integration -p 1 ./internal/integration/... -run TestConfigEscalonamentoDestinatarios -v`
Expected: PASS em todos os subtestes.

- [ ] **Step 3: Commit**

```bash
git add internal/integration/alerta_test.go
git commit -m "test: CRUD de escalonamento com destinatarios por usuario"
```

---

### Task 11: Testes de integração — configuração de alertas de emergência

**Files:**
- Modify: `internal/integration/alerta_test.go`

**Interfaces:**
- Consumes: endpoints `/api/v1/config/alertas-emergencia`.

- [ ] **Step 1: Adicionar o teste ao final do arquivo**

```go
// Configuracao de destinatarios por tipo de alerta de emergencia.
func TestConfigAlertaEmergencia(t *testing.T) {
	c := novoCenario(t)

	t.Run("GET inicial retorna os 3 tipos vazios", func(t *testing.T) {
		var configs []model.ConfigAlertaEmergencia
		c.e.reqJSON(http.MethodGet, "/api/v1/config/alertas-emergencia", c.adminToken, nil, http.StatusOK, &configs)

		if len(configs) != 3 {
			t.Fatalf("configs = %d, esperado 3 (coacao, sabotagem, no_show)", len(configs))
		}
		for _, cfg := range configs {
			if len(cfg.UsuarioIDs) != 0 {
				t.Errorf("tipo %s: usuario_ids = %v, esperado vazio", cfg.Tipo, cfg.UsuarioIDs)
			}
		}
	})

	t.Run("PUT define destinatarios de coacao", func(t *testing.T) {
		gerente := c.e.criarUsuario(c.empresa.ID, "Gerente Emergencia", "gerente.emerg@a.com", "senha123", "admin", true)

		var config model.ConfigAlertaEmergencia
		c.e.reqJSON(http.MethodPut, "/api/v1/config/alertas-emergencia/coacao", c.adminToken, map[string]any{
			"usuario_ids": []string{gerente.ID.String()},
		}, http.StatusOK, &config)

		if len(config.UsuarioIDs) != 1 || config.UsuarioIDs[0] != gerente.ID {
			t.Fatalf("usuario_ids = %v, esperado [%s]", config.UsuarioIDs, gerente.ID)
		}
	})

	t.Run("PUT com tipo invalido retorna 400", func(t *testing.T) {
		outro := c.e.criarUsuario(c.empresa.ID, "Outro", "outro.tipo@a.com", "senha123", "admin", true)
		c.e.reqJSON(http.MethodPut, "/api/v1/config/alertas-emergencia/tipo_invalido", c.adminToken, map[string]any{
			"usuario_ids": []string{outro.ID.String()},
		}, http.StatusBadRequest, nil)
	})
}
```

- [ ] **Step 2: Rodar o teste**

Run: `go test -tags integration -p 1 ./internal/integration/... -run TestConfigAlertaEmergencia -v`
Expected: PASS em todos os subtestes.

- [ ] **Step 3: Commit**

```bash
git add internal/integration/alerta_test.go
git commit -m "test: configuracao de destinatarios por tipo de alerta de emergencia"
```

---

### Task 12: Teste de integração — emergência usa destinatários configurados por tipo

**Files:**
- Modify: `internal/integration/alerta_test.go`

**Interfaces:**
- Consumes: `c.e.app.AlertaService.AlertChannel()` (já público em `internal/app/app.go:34`), `POST /api/v1/turnos/checkin` com `tipo_senha=coacao`.

- [ ] **Step 1: Adicionar o teste ao final do arquivo**

```go
// Alerta imediato (coacao) deve enfileirar PendingAlert com os usuario_ids
// configurados especificamente para o tipo "coacao", nao os do escalonamento.
func TestAlertaImediatoUsaDestinatariosPorTipo(t *testing.T) {
	c := novoCenario(t)
	turno := c.iniciarTurno()

	gerente := c.e.criarUsuario(c.empresa.ID, "Gerente Coacao", "gerente.coacao@a.com", "senha123", "admin", true)
	c.e.reqJSON(http.MethodPut, "/api/v1/config/alertas-emergencia/coacao", c.adminToken, map[string]any{
		"usuario_ids": []string{gerente.ID.String()},
	}, http.StatusOK, nil)

	c.e.reqJSON(http.MethodPost, "/api/v1/turnos/checkin", c.vigiaToken,
		c.checkinBody(turno.ID, "coacao", time.Now()), http.StatusOK, nil)

	select {
	case pending := <-c.e.app.AlertaService.AlertChannel():
		if pending.Alerta.Tipo != "coacao" {
			t.Fatalf("tipo do alerta = %s, esperado coacao", pending.Alerta.Tipo)
		}
		if len(pending.UsuarioIDs) != 1 || pending.UsuarioIDs[0] != gerente.ID {
			t.Fatalf("usuario_ids = %v, esperado [%s]", pending.UsuarioIDs, gerente.ID)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("nenhum PendingAlert recebido no canal em 2s")
	}
}
```

Adicione `"time"` ao bloco de imports do arquivo, caso ainda não esteja lá (necessário para `time.Now()` e `time.After`/`time.Second`).

- [ ] **Step 2: Rodar o teste**

Run: `go test -tags integration -p 1 ./internal/integration/... -run TestAlertaImediatoUsaDestinatariosPorTipo -v`
Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add internal/integration/alerta_test.go
git commit -m "test: alerta imediato usa destinatarios configurados por tipo de emergencia"
```

---

### Task 13: Teste de integração — check-in resolve alertas de atraso automaticamente

**Files:**
- Modify: `internal/integration/alerta_test.go`

**Interfaces:**
- Consumes: `TimeoutChecker.CheckOnce`, `contarAlertas` (helper existente, aceita apenas `tipo` — usar `e.reqJSON` diretamente para filtrar também por `status`).

- [ ] **Step 1: Adicionar o teste ao final do arquivo**

```go
// Quando o vigia finalmente da checkin, os alertas de atraso abertos daquele
// turno devem fechar sozinhos como 'resolvido_checkin'.
func TestAlertaAtrasoResolvidoNoCheckin(t *testing.T) {
	c := novoCenario(t)
	turno := c.iniciarTurno()

	supervisor := c.e.criarUsuario(c.empresa.ID, "Supervisor Resolve", "sup.resolve@a.com", "senha123", "supervisor", true)
	c.e.reqJSON(http.MethodPut, "/api/v1/config/escalonamento", c.adminToken, []map[string]any{
		{"nivel": 1, "atraso_minutos": 5, "usuario_ids": []string{supervisor.ID.String()}},
	}, http.StatusOK, nil)

	// primeiro checkin ha 50 min; intervalo de 30 min => atraso de ~20 min, dispara nivel 1
	c.e.reqJSON(http.MethodPost, "/api/v1/turnos/checkin", c.vigiaToken,
		c.checkinBody(turno.ID, "padrao", time.Now().Add(-50*time.Minute)), http.StatusOK, nil)

	c.e.app.TimeoutChecker.CheckOnce(t.Context())

	if n := c.contarAlertas("atraso_n1"); n != 1 {
		t.Fatalf("alertas atraso_n1 antes do checkin = %d, esperado 1", n)
	}

	var abertos struct {
		Total int `json:"total"`
	}
	c.e.reqJSON(http.MethodGet, "/api/v1/alertas?tipo=atraso_n1&status=aberto", c.adminToken, nil, http.StatusOK, &abertos)
	if abertos.Total != 1 {
		t.Fatalf("alertas atraso_n1 status=aberto antes do checkin = %d, esperado 1", abertos.Total)
	}

	// vigia finalmente da checkin em dia
	c.e.reqJSON(http.MethodPost, "/api/v1/turnos/checkin", c.vigiaToken,
		c.checkinBody(turno.ID, "padrao", time.Now()), http.StatusOK, nil)

	c.e.reqJSON(http.MethodGet, "/api/v1/alertas?tipo=atraso_n1&status=aberto", c.adminToken, nil, http.StatusOK, &abertos)
	if abertos.Total != 0 {
		t.Fatalf("alertas atraso_n1 status=aberto apos checkin = %d, esperado 0", abertos.Total)
	}

	var resolvidos struct {
		Total int `json:"total"`
	}
	c.e.reqJSON(http.MethodGet, "/api/v1/alertas?tipo=atraso_n1&status=resolvido_checkin", c.adminToken, nil, http.StatusOK, &resolvidos)
	if resolvidos.Total != 1 {
		t.Fatalf("alertas atraso_n1 status=resolvido_checkin apos checkin = %d, esperado 1", resolvidos.Total)
	}
}
```

- [ ] **Step 2: Rodar o teste**

Run: `go test -tags integration -p 1 ./internal/integration/... -run TestAlertaAtrasoResolvidoNoCheckin -v`
Expected: PASS

- [ ] **Step 3: Rodar a suite de integracao completa**

Run: `go test -tags integration -p 1 ./...`
Expected: PASS em todos os pacotes.

- [ ] **Step 4: Commit**

```bash
git add internal/integration/alerta_test.go
git commit -m "test: checkin resolve automaticamente alertas de atraso abertos"
```

---

### Task 14: Regenerar documentação Swagger e atualizar PLANNING.md

**Files:**
- Modify: `docs/swagger.json`, `docs/swagger.yaml`, `docs/docs.go` (gerados)
- Modify: `PLANNING.md` (seção 4.7)

**Interfaces:**
- Nenhuma nova — apenas sincroniza os artefatos gerados com as anotações `@Router`/`@Param` adicionadas na Task 7, e atualiza a documentação de schema para não ficar desatualizada.

- [ ] **Step 1: Rodar a geração de docs**

Run: `make docs`
Expected: regenera `docs/swagger.json`, `docs/swagger.yaml`, `docs/docs.go` sem erros, incluindo as novas rotas `/config/alertas-emergencia` e `/config/alertas-emergencia/{tipo}`.

(Se `make` não estiver disponível no ambiente, rode diretamente: `go run github.com/swaggo/swag/cmd/swag@v1.16.6 init -g cmd/server/main.go -o docs`.)

- [ ] **Step 2: Atualizar a seção 4.7 do `PLANNING.md`**

Em `PLANNING.md`, na seção "4.7. Configuração de Escalonamento" (linha ~162-172), substitua o bloco SQL antigo por:

```sql
CREATE TABLE config_escalonamento (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    empresa_id      UUID        NOT NULL REFERENCES empresas(id),
    nivel           INTEGER     NOT NULL, -- N1, N2, N3...
    atraso_minutos  INTEGER     NOT NULL, -- minutos de atraso para disparar
    created_at      TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE config_escalonamento_destinatarios (
    config_escalonamento_id UUID NOT NULL REFERENCES config_escalonamento(id) ON DELETE CASCADE,
    usuario_id              UUID NOT NULL REFERENCES usuarios(id) ON DELETE CASCADE
);
-- destinatarios de emergencia (coacao, sabotagem, no_show) seguem o mesmo
-- padrao em config_alerta_emergencia / config_alerta_emergencia_destinatarios,
-- independentes dos niveis de escalonamento por atraso.
```

- [ ] **Step 3: Verificar que o CI de docs vai passar**

Run: `git diff --exit-code docs/`
Expected: FAIL antes de `git add` (esperado — há mudanças a commitar). Depois do `git add`, o comando de CI (`git diff --exit-code docs/` após rodar `make docs` de novo) deve ficar limpo — isso só é validado de fato no CI, mas confirme localmente que rodar `make docs` uma segunda vez não produz nenhum diff adicional.

- [ ] **Step 4: Commit**

```bash
git add docs/swagger.json docs/swagger.yaml docs/docs.go PLANNING.md
git commit -m "docs: regenera swagger e atualiza PLANNING.md com destinatarios por usuario"
```

---

## Self-Review

**Spec coverage:**
- Modelo de dados (tabelas + colunas removidas) → Task 1, 2. ✓
- Destinatários por usuário no escalonamento → Task 3, 6, 7. ✓
- Destinatários por tipo de emergência (configurável, substitui nível hardcoded) → Task 4, 6, 7. ✓
- Auto-resolução no check-in → Task 5, 8. ✓
- Validação de usuário pertencente à empresa → Task 6 (`validarUsuariosDaEmpresa`), testado na Task 10. ✓
- API (`GET/POST/PUT/DELETE /config/escalonamento`, `GET/PUT /config/alertas-emergencia`) → Task 7. ✓
- `PendingAlert.UsuarioIDs` substituindo `WhatsappPara` → Task 2, 6. ✓
- Testes (unit de validação embutido no fluxo de service; integração de worker, CRUD, emergência, auto-resolução) → Tasks 9-13. ✓
- Docs sincronizadas (CI de swagger) → Task 14. ✓

**Placeholder scan:** nenhum "TBD"/"TODO"/"implementar depois" — todo código é completo e literal.

**Type consistency:** `model.ConfigEscalonamento.UsuarioIDs`, `model.ConfigAlertaEmergencia.UsuarioIDs`, `model.PendingAlert.UsuarioIDs` usam `[]uuid.UUID` de forma consistente em todas as tasks (model, repository, service, handler, testes). `AlertaService.ResolverAlertasAtraso(ctx, turnoID uuid.UUID) error` é chamado com a mesma assinatura nas Tasks 6 e 8. `ConfigAlertaEmergenciaRepository`/`ConfigEscalonamentoRepository` mantêm nomes de método idênticos aos usados no service (`FindByEmpresa`, `FindByEmpresaENivel`, `FindByEmpresaETipo`, `Upsert`, `Create`, `Update`, `Delete`, `ReplaceByEmpresa`).
