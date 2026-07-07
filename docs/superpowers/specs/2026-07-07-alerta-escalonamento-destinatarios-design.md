# Alerta por Tempo e Escalonamento — Destinatários Configuráveis

> Data: 2026-07-07
> Status: aprovado para implementação

## Contexto

O backend já implementa um pipeline completo de alerta por atraso e escalonamento:

- `Turno.intervalo_min` define o intervalo esperado entre check-ins de um vigia.
- `TimeoutChecker` (`internal/worker/timeout_checker.go`) roda a cada 30s, calcula o
  atraso de cada turno ativo e, para cada nível configurado em `ConfigEscalonamento`
  cujo `atraso_minutos` foi ultrapassado, cria um `Alerta` (dedup por turno+tipo).
- `AlertaService.CreateAlerta`/`CreateAlertaImediato` enfileiram o alerta num canal
  consumido pelo `AlertDispatcher`, que hoje é um **stub** (só loga, não envia nada).
- Já existe API REST completa (`/api/config/escalonamento`) para o admin configurar
  os níveis, protegida por RBAC (`admin` apenas).

O que falta, e é o objeto deste spec:

1. O destinatário de cada nível hoje é um `cargo_alvo` genérico (string) ou um
   `whatsapp_para` (telefone manual) — não é possível escolher usuários específicos
   do sistema.
2. Alertas imediatos (coação, sabotagem, no-show) hoje disparam para um **nível
   fixo hardcoded no código** (coação/sabotagem → nível 1, no-show → nível 2), sem
   configuração própria.
3. Quando o vigia finalmente dá o check-in atrasado, os alertas de atraso já
   abertos permanecem abertos até um admin/supervisor agir manualmente.

Fora de escopo (decidido explicitamente): implementar um canal de envio real
(WhatsApp, push, e-mail). Este spec só modela **quem** deve receber cada alerta;
o dispatcher continua stub, mas passa a logar destinatários (usuário) em vez de
telefone, preparado para a próxima fase.

## Decisões

- **Destinatários por usuário específico**, não por cargo genérico. `cargo_alvo`
  e `whatsapp_para` são removidos (não mantidos como campos mortos).
- **Dois contextos de destinatários, independentes**:
  - Escalonamento por atraso: por **nível** (`config_escalonamento`).
  - Emergências (coação, sabotagem, no-show): por **tipo**, configurável pelo
    admin em vez de hardcoded.
- **Nível 1 continua exigindo `atraso_minutos >= 1`** (não é necessário permitir 0
  — a diferença é desprezível já que o worker roda a cada 30s).
- **Alertas de atraso se auto-resolvem quando o check-in chega**: ao registrar um
  check-in (online ou via lote offline), qualquer alerta `atraso_nX` aberto ou
  reconhecido daquele turno é fechado automaticamente com status
  `resolvido_checkin`.
- **Alertas imediatos continuam indo para um único conjunto de destinatários por
  tipo** (não para a união de todos os níveis de escalonamento).

## Modelo de dados (migration `000018`)

```sql
-- 1. Remove campos antigos de config_escalonamento
ALTER TABLE config_escalonamento DROP COLUMN whatsapp_para;
ALTER TABLE config_escalonamento DROP COLUMN cargo_alvo;

-- 2. Destinatários por nível de escalonamento
CREATE TABLE config_escalonamento_destinatarios (
    id                      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    config_escalonamento_id UUID NOT NULL REFERENCES config_escalonamento(id) ON DELETE CASCADE,
    usuario_id              UUID NOT NULL REFERENCES usuarios(id) ON DELETE CASCADE,
    created_at              TIMESTAMPTZ DEFAULT now(),
    UNIQUE(config_escalonamento_id, usuario_id)
);

-- 3. Configuração de destinatários por tipo de emergência
CREATE TABLE config_alerta_emergencia (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    empresa_id  UUID NOT NULL REFERENCES empresas(id),
    tipo        VARCHAR(20) NOT NULL CHECK (tipo IN ('coacao', 'sabotagem', 'no_show')),
    created_at  TIMESTAMPTZ DEFAULT now(),
    UNIQUE(empresa_id, tipo)
);

-- 4. Destinatários por tipo de emergência
CREATE TABLE config_alerta_emergencia_destinatarios (
    id                          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    config_alerta_emergencia_id UUID NOT NULL REFERENCES config_alerta_emergencia(id) ON DELETE CASCADE,
    usuario_id                  UUID NOT NULL REFERENCES usuarios(id) ON DELETE CASCADE,
    created_at                  TIMESTAMPTZ DEFAULT now(),
    UNIQUE(config_alerta_emergencia_id, usuario_id)
);
```

Índices: `idx_config_escalonamento_destinatarios_config` em
`(config_escalonamento_id)` e `idx_config_alerta_emergencia_destinatarios_config`
em `(config_alerta_emergencia_id)`, para o carregamento dos destinatários por
config. Nenhuma mudança na tabela `alertas` (status já é `VARCHAR` sem `CHECK`
constraint — `resolvido_checkin` é apenas um novo valor de string, seguindo o
mesmo padrão de `falso_positivo`).

Tabelas de junção foram escolhidas em vez de uma coluna array de UUIDs porque
ganham `FK CASCADE` (usuário removido desaparece das listas automaticamente,
sem faxina manual) e seguem o padrão relacional já usado no projeto (escalas,
sessões de dispositivo).

## Modelo Go

`internal/model/alerta.go`:

```go
type ConfigEscalonamento struct {
    ID            uuid.UUID   `json:"id"`
    EmpresaID     uuid.UUID   `json:"empresa_id"`
    Nivel         int         `json:"nivel"`
    AtrasoMinutos int         `json:"atraso_minutos"`
    UsuarioIDs    []uuid.UUID `json:"usuario_ids"`
    CreatedAt     time.Time   `json:"created_at"`
}

type ConfigAlertaEmergencia struct {
    ID         uuid.UUID   `json:"id"`
    EmpresaID  uuid.UUID   `json:"empresa_id"`
    Tipo       string      `json:"tipo"` // coacao | sabotagem | no_show
    UsuarioIDs []uuid.UUID `json:"usuario_ids"`
    CreatedAt  time.Time   `json:"created_at"`
}

type CreateConfigEscalonamentoRequest struct {
    Nivel         int         `json:"nivel" validate:"required,min=1,max=5"`
    AtrasoMinutos int         `json:"atraso_minutos" validate:"required,min=1,max=1440"`
    UsuarioIDs    []uuid.UUID `json:"usuario_ids" validate:"required,min=1,dive,required"`
}

type UpdateConfigEscalonamentoRequest struct {
    AtrasoMinutos int         `json:"atraso_minutos" validate:"required,min=1,max=1440"`
    UsuarioIDs    []uuid.UUID `json:"usuario_ids" validate:"required,min=1,dive,required"`
}

type UpdateConfigAlertaEmergenciaRequest struct {
    UsuarioIDs []uuid.UUID `json:"usuario_ids" validate:"required,min=1,dive,required"`
}

type PendingAlert struct {
    Alerta     *Alerta     `json:"alerta"`
    UsuarioIDs []uuid.UUID `json:"usuario_ids"`
}
```

`usuario_ids` vazio é rejeitado na validação de struct; a checagem de que cada
ID pertence à mesma empresa do admin autenticado acontece no service (contra
`UsuarioRepository`), retornando 400 se algum ID não pertencer/não existir.

## Resolução de destinatários

- `AlertaService.CreateAlerta` (caminho de atraso por nível): busca
  `usuario_ids` via `ConfigEscalonamentoDestinatariosRepository.FindByConfig`
  do nível correspondente.
- `AlertaService.CreateAlertaImediato` (coação, sabotagem, no-show): passa a
  receber o `tipo` e buscar `usuario_ids` em
  `ConfigAlertaEmergenciaRepository.FindByEmpresaETipo`, substituindo o nível
  fixo hoje hardcoded em `turno_service.go:245,623,712` e
  `timeout_checker.go:193`. Não filtra mais por `nivel` de escalonamento — o
  `nivel` continua sendo persistido no `Alerta` (mantém `1` para
  coação/sabotagem, `2` para no-show, apenas como classificação/severidade
  exibida na UI), mas a lista de destinatários vem exclusivamente da config de
  emergência.
- `PendingAlert.UsuarioIDs` substitui `WhatsappPara`. O `AlertDispatcher` (stub)
  passa a logar a lista de UUIDs de usuário em vez do telefone.

## Auto-resolução no check-in

Novo método no `AlertaRepository`, no mesmo padrão de `CloseAlertasFalsoPositivo`
(`internal/repository/alerta_repository.go:206`):

```go
func (r *AlertaRepository) CloseAlertasResolvidoCheckin(ctx context.Context, turnoID uuid.UUID) (int64, error) {
    query := `
        UPDATE alertas SET status = 'resolvido_checkin', resolvido_em = $1
        WHERE turno_id = $2
          AND tipo LIKE 'atraso_%'
          AND status IN ('aberto', 'reconhecido')
    `
    // ...
}
```

Chamado a partir de `TurnoService.Checkin` (logo após `checkinRepo.Create`, ao
lado do cálculo de `atrasado`) e de `TurnoService.ProcessarLote` (uma vez por
turno, no fim do processamento do lote — mesmo ponto onde hoje já se chama
`SyncReconciler.Reconcile`). A chamada é idempotente: se não houver alerta
aberto para o turno, a `UPDATE` não afeta nenhuma linha.

## API

Handlers em `internal/handler/alerta.go`, mesmo RBAC atual (`admin` apenas):

- `GET /api/config/escalonamento` — inalterado na rota, resposta agora inclui
  `usuario_ids` em vez de `whatsapp_para`/`cargo_alvo`.
- `POST/PUT/DELETE /api/config/escalonamento[/{id}]` — corpo passa a exigir
  `usuario_ids` em vez dos campos antigos.
- `PUT /api/config/escalonamento` (replace-all) — idem.
- **Novo** `GET /api/config/alertas-emergencia` — lista os 3 tipos
  (`coacao`, `sabotagem`, `no_show`) com seus `usuario_ids` (lista vazia se
  ainda não configurado).
- **Novo** `PUT /api/config/alertas-emergencia/{tipo}` — define a lista de
  usuários daquele tipo (upsert; `tipo` restrito ao enum via validação de rota).

## Testes

- Unitário: validação de `usuario_ids` (vazio rejeitado; ID de outra empresa
  rejeitado); resolução de destinatários por nível e por tipo de emergência.
- Integração (`internal/integration/worker_test.go`, já existe suíte
  equivalente): `TimeoutChecker` gera alerta e resolve destinatários
  configurados no nível certo; check-in fecha alerta como `resolvido_checkin`;
  emergência (coação/sabotagem/no-show) usa destinatários configurados por
  tipo, não hardcoded.
- Migration: aplicar `000018.up.sql` e `000018.down.sql` em Postgres efêmero,
  confirmando que dados existentes de `config_escalonamento` sobrevivem ao
  `DROP COLUMN` (down precisa recriar as colunas antigas como nullable, já que
  não há dado histórico de usuário para popular de volta).
