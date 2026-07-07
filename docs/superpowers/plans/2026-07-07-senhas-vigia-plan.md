# Sistema de PINs (senhas) por vigia para sinal de vida — Plano de Implementação

## Contexto

Hoje o vigia, a cada check-in periódico, envia um `tipo_senha` que é um **enum fixo
global** (`padrao|coacao|finalizacao|sabotagem`, `internal/model/turno.go:48`). "coacao"
dispara alerta crítico imediato com nível hardcoded (`1`) e destinatários vindos de uma
config por-empresa (`config_alerta_emergencia`, tipo fixo). Não há como o admin
diferenciar o que cada vigia digita nem criar variações — é um botão/opção nomeada, não
um código secreto.

O objetivo é substituir esse enum fixo por um **sistema de PINs configuráveis por
vigia**: cada vigia tem seus próprios códigos numéricos, cadastrados pelo admin, e o
significado de cada código só é conhecido pelo servidor (não pelo app) — permitindo o
padrão clássico de "senha de coação": sob ameaça, o vigia digita o PIN de emergência e a
ação (check-in, início, fim de turno) continua funcionando normalmente do ponto de vista
de quem está coagindo, mas dispara notificação silenciosa para os destinatários certos.

Sabotagem fica **totalmente fora de escopo** (endpoint próprio, detecção automática do
app, inalterado).

## Global Constraints

Estas regras valem para TODAS as tasks abaixo — releia antes de implementar qualquer uma:

1. **Nomenclatura**: a nova entidade se chama `SenhaVigia`/`senhas_vigia`. NÃO usar
   "Pin"/"pin" no nome de nada relacionado a ela — esse nome já é usado por
   `Turno.Pin`/`ReassociarRequest.Pin`, um conceito totalmente diferente (PIN gerado
   pelo servidor para troca de dispositivo, 15 min de validade, uso único). Confundir os
   dois é um bug de design.
2. **`checkins` separa dois conceitos hoje misturados em `tipo_senha`**: a AÇÃO
   (`evento`: inicio|checkin|finalizacao|sabotagem) e a CLASSIFICAÇÃO do PIN usado
   (`tipo_senha`, agora nullable: ok|emergencia|customizada, NULL para sabotagem).
3. **Nível vinculado ao PIN**: FK nullable `senhas_vigia.nivel_escalonamento_id →
   config_escalonamento(id)`. `NULL` = "nível máximo dinâmico", resolvido em runtime a
   cada disparo (nunca fixado na criação do PIN). PINs `ok`/`emergencia` NUNCA podem ter
   nível fixo (regra de negócio + CHECK constraint no banco); só `customizada` pode
   escolher um nível específico.
4. **A ação nunca falha por causa do PIN** — iniciar/checkin/finalizar sempre são
   persistidos com sucesso, independente do PIN ser reconhecido ou não. PIN não
   reconhecido ou vigia sem PINs cadastrados: a ação prossegue normalmente, sem alerta
   (equivalente a "ok"), só logado internamente (`slog`). Esses erros NUNCA viram
   resposta HTTP de erro.
5. **Ao finalizar com PIN de emergência/customizada, o turno termina como
   `"finalizado"`** (não fica crítico no mapa) — o alerta dispara em paralelo, com os
   destinatários corretos, independente do status final do turno.
6. **Toda empresa (nova ou já existente) tem um nível de escalonamento padrão com o
   admin como destinatário** — garante que "nível máximo dinâmico" sempre tenha alguém
   pra notificar.
7. **Multi-tenancy**: toda query nova filtra por `empresa_id`; todo handler novo extrai
   `empresaID` do JWT (`GetEmpresaID(ctx)`), nunca do body/path. Siga o padrão já usado
   em `internal/handler/usuario.go` e `internal/repository/user_repository.go`.
8. **Padrões de código a seguir** (não inventar convenção nova): repositórios com
   queries diretas via `pgxpool` e `errors.Is(err, pgx.ErrNoRows)` (ver
   `internal/repository/config_escalonamento_repository.go`); services com erros
   sentinela (`var Err... = errors.New(...)`) mapeados em handlers via `errors.Is`; RBAC
   sempre no roteador (`internal/app/app.go`) via `handler.RequireRole(...)`, nunca no
   service.
9. **Cada task termina com commit próprio**, build limpo (`go build ./...`) e testes
   unitários existentes passando (`go test ./...`, sem a tag `integration`).

## Task 1: Migrations

Criar 4 migrations novas em `migrations/`, seguindo o padrão de arquivos par
`NNNNNN_nome.up.sql`/`NNNNNN_nome.down.sql` já usado no diretório (ver
`migrations/000020_create_substituicoes.up.sql` como exemplo de estilo). Última
migration existente é `000020`; comece em `000021`.

### `migrations/000021_create_senhas_vigia.up.sql`
```sql
CREATE TABLE senhas_vigia (
    id                      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    empresa_id              UUID NOT NULL REFERENCES empresas(id),
    usuario_id              UUID NOT NULL REFERENCES usuarios(id),
    tipo                    VARCHAR(20) NOT NULL CHECK (tipo IN ('ok', 'emergencia', 'customizada')),
    codigo                  VARCHAR(6)  NOT NULL CHECK (codigo ~ '^[0-9]{4,6}$'),
    descricao               VARCHAR(255),
    nivel_escalonamento_id  UUID REFERENCES config_escalonamento(id) ON DELETE RESTRICT,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT ck_senhas_vigia_descricao_customizada
        CHECK (tipo <> 'customizada' OR descricao IS NOT NULL),
    CONSTRAINT ck_senhas_vigia_nivel_fixo_so_customizada
        CHECK (tipo = 'customizada' OR nivel_escalonamento_id IS NULL),
    UNIQUE (usuario_id, codigo)
);

-- Garante exatamente 1 PIN 'ok' e 1 'emergencia' por vigia; 'customizada' livre (0..N).
CREATE UNIQUE INDEX uq_senhas_vigia_ok         ON senhas_vigia(usuario_id) WHERE tipo = 'ok';
CREATE UNIQUE INDEX uq_senhas_vigia_emergencia ON senhas_vigia(usuario_id) WHERE tipo = 'emergencia';

CREATE INDEX idx_senhas_vigia_empresa ON senhas_vigia(empresa_id);
CREATE INDEX idx_senhas_vigia_nivel   ON senhas_vigia(nivel_escalonamento_id) WHERE nivel_escalonamento_id IS NOT NULL;
```
`migrations/000021_create_senhas_vigia.down.sql`:
```sql
DROP TABLE IF EXISTS senhas_vigia;
```

### `migrations/000022_refactor_checkins_evento_senha.up.sql`
```sql
ALTER TABLE checkins ADD COLUMN evento VARCHAR(20);

UPDATE checkins SET evento = 'finalizacao' WHERE tipo_senha = 'finalizacao';
UPDATE checkins SET evento = 'sabotagem'   WHERE tipo_senha = 'sabotagem';
UPDATE checkins SET evento = 'checkin'     WHERE evento IS NULL;

ALTER TABLE checkins ALTER COLUMN evento SET NOT NULL;
ALTER TABLE checkins ADD CONSTRAINT ck_checkins_evento
    CHECK (evento IN ('inicio', 'checkin', 'finalizacao', 'sabotagem'));

-- Remapeia o vocabulario antigo para o novo (padrao->ok, coacao->emergencia);
-- finalizacao/sabotagem eram marcadores de acao, nao de senha, viram NULL.
UPDATE checkins SET tipo_senha = 'ok'         WHERE tipo_senha = 'padrao';
UPDATE checkins SET tipo_senha = 'emergencia' WHERE tipo_senha = 'coacao';
UPDATE checkins SET tipo_senha = NULL         WHERE tipo_senha IN ('finalizacao', 'sabotagem');

ALTER TABLE checkins ALTER COLUMN tipo_senha DROP NOT NULL;
ALTER TABLE checkins ADD CONSTRAINT ck_checkins_tipo_senha
    CHECK (tipo_senha IS NULL OR tipo_senha IN ('ok', 'emergencia', 'customizada'));

ALTER TABLE checkins ADD COLUMN senha_vigia_id UUID REFERENCES senhas_vigia(id) ON DELETE SET NULL;

CREATE INDEX idx_checkins_evento ON checkins(evento);
```
`migrations/000022_refactor_checkins_evento_senha.down.sql` precisa reverter na ordem
inversa: dropar `idx_checkins_evento`, dropar coluna `senha_vigia_id`, dropar constraint
`ck_checkins_tipo_senha`, remapear `tipo_senha` de volta (`ok`→`padrao`,
`emergencia`→`coacao`), popular `tipo_senha` para linhas com `evento IN
('finalizacao','sabotagem')` a partir de `evento` (já que essas linhas ficaram com
`tipo_senha IS NULL` no up), então `ALTER COLUMN tipo_senha SET NOT NULL`, dropar
constraint `ck_checkins_evento`, dropar coluna `evento`.

### `migrations/000023_remove_coacao_config_alerta_emergencia.up.sql`
```sql
DELETE FROM config_alerta_emergencia_destinatarios
WHERE config_alerta_emergencia_id IN (
    SELECT id FROM config_alerta_emergencia WHERE tipo = 'coacao'
);
DELETE FROM config_alerta_emergencia WHERE tipo = 'coacao';

ALTER TABLE config_alerta_emergencia DROP CONSTRAINT config_alerta_emergencia_tipo_check;
ALTER TABLE config_alerta_emergencia ADD CONSTRAINT config_alerta_emergencia_tipo_check
    CHECK (tipo IN ('sabotagem', 'no_show'));
```
**Antes de escrever isso**, rode `\d config_alerta_emergencia` (ou consulte
`migrations/000018_config_alerta_destinatarios.up.sql`, que criou a tabela) para
confirmar o nome real da constraint CHECK gerada pelo Postgres — pode não ser
`config_alerta_emergencia_tipo_check`. Ajuste o nome na migration para o valor real.
`down.sql`: reverte o CHECK para incluir `'coacao'` novamente (não recria os dados
deletados — comentar isso explicitamente no arquivo).

### `migrations/000024_default_nivel_escalonamento.up.sql`
Backfill para empresas já existentes sem nenhum nível de escalonamento configurado:
```sql
INSERT INTO config_escalonamento (empresa_id, nivel, atraso_minutos)
SELECT id, 1, 15 FROM empresas e
WHERE NOT EXISTS (SELECT 1 FROM config_escalonamento c WHERE c.empresa_id = e.id);

INSERT INTO config_escalonamento_destinatarios (config_escalonamento_id, usuario_id)
SELECT ce.id, u.id
FROM config_escalonamento ce
JOIN usuarios u ON u.empresa_id = ce.empresa_id AND u.role = 'admin'
WHERE ce.nivel = 1 AND ce.atraso_minutos = 15
  AND NOT EXISTS (SELECT 1 FROM config_escalonamento_destinatarios d WHERE d.config_escalonamento_id = ce.id);
```
`down.sql`: arquivo com só um comentário explicando que o rollback é intencionalmente
no-op (não é seguro apagar níveis que o admin possa ter reconfigurado manualmente após
o deploy).

**Verificação da task**: aplicar as 4 migrations up, em ordem, contra um Postgres local
ou efêmero (`docker run -d --name senhas-vigia-test-pg -e POSTGRES_PASSWORD=postgres -e
POSTGRES_DB=guardpoint_test -p 5434:5432 postgres:16-alpine`, depois `migrate -path
migrations -database "postgres://postgres:postgres@localhost:5434/guardpoint_test?sslmode=disable"
up`), depois aplicar down na ordem inversa, confirmando que nenhum comando falha.
Reportar os comandos exatos rodados e o resultado no relatório da task. Derrubar o
container ao final.

---

## Task 2: Modelos Go

Depende da Task 1 (nomes de coluna/tabela precisam bater com o schema criado).

### `internal/model/senha_vigia.go` (novo arquivo)
```go
package model

import (
	"time"

	"github.com/google/uuid"
)

type SenhaVigia struct {
	ID                    uuid.UUID  `json:"id"`
	EmpresaID             uuid.UUID  `json:"empresa_id"`
	UsuarioID             uuid.UUID  `json:"usuario_id"`
	Tipo                  string     `json:"tipo"` // ok | emergencia | customizada
	Codigo                string     `json:"codigo"`
	Descricao             *string    `json:"descricao,omitempty"`
	NivelEscalonamentoID  *uuid.UUID `json:"nivel_escalonamento_id,omitempty"` // nil = nivel maximo dinamico
	CreatedAt             time.Time  `json:"created_at"`
	UpdatedAt             time.Time  `json:"updated_at"`
}

type CreateSenhaVigiaRequest struct {
	Tipo                 string  `json:"tipo" validate:"required,oneof=ok emergencia customizada"`
	Codigo               string  `json:"codigo" validate:"required,numeric,min=4,max=6"`
	Descricao            *string `json:"descricao" validate:"required_if=Tipo customizada,omitempty,max=255"`
	NivelEscalonamentoID *string `json:"nivel_escalonamento_id" validate:"omitempty,uuid"`
}

type UpdateSenhaVigiaRequest struct {
	Codigo               *string `json:"codigo" validate:"omitempty,numeric,min=4,max=6"`
	Descricao            *string `json:"descricao" validate:"omitempty,max=255"`
	NivelEscalonamentoID *string `json:"nivel_escalonamento_id" validate:"omitempty,uuid"`
	NivelDinamico        *bool   `json:"nivel_dinamico,omitempty"` // true = forca nivel_escalonamento_id = NULL
}
```
Nota: `validate:"required,numeric,min=4,max=6"` no campo `Codigo string` valida
COMPRIMENTO da string (4 a 6 caracteres) via `go-playground/validator`, que é o
comportamento desejado aqui — confirme esse entendimento lendo outros usos de
`min`/`max` em campos string no mesmo arquivo (`turno.go`) antes de assumir que
funciona igual para `int`.

### `internal/model/turno.go` (alterações)
```go
type IniciarTurnoRequest struct {
	PostoID      string  `json:"posto_id" validate:"required,uuid"`
	DeviceID     string  `json:"device_id" validate:"required"`
	IntervaloMin int     `json:"intervalo_min" validate:"omitempty,min=1,max=120"`
	Latitude     float64 `json:"latitude" validate:"required,latitude"`
	Longitude    float64 `json:"longitude" validate:"required,longitude"`
	Senha        string  `json:"senha" validate:"required,numeric,min=4,max=6"`
}

type CheckinRequest struct {
	TurnoID          string  `json:"turno_id" validate:"required,uuid"`
	DeviceID         string  `json:"device_id" validate:"required"`
	Latitude         float64 `json:"latitude" validate:"required,latitude"`
	Longitude        float64 `json:"longitude" validate:"required,longitude"`
	Senha            string  `json:"senha" validate:"required,numeric,min=4,max=6"`
	Timestamp        string  `json:"timestamp" validate:"required"`
	ClienteCheckinID string  `json:"cliente_checkin_id" validate:"omitempty,uuid"`
}

type FinalizarTurnoRequest struct {
	TurnoID   string  `json:"turno_id" validate:"required,uuid"`
	DeviceID  string  `json:"device_id" validate:"required"`
	Latitude  float64 `json:"latitude" validate:"required,latitude"`
	Longitude float64 `json:"longitude" validate:"required,longitude"`
	Senha     string  `json:"senha" validate:"required,numeric,min=4,max=6"`
	Timestamp string  `json:"timestamp" validate:"required"`
}
```
Remover o campo `TipoSenha` de `CheckinRequest` por completo (era
`validate:"required,oneof=padrao coacao finalizacao sabotagem"`). `SabotagemRequest`
**não muda** (fora de escopo).

### `internal/model/checkin.go` (alterações)
```go
type Checkin struct {
	ID                   uuid.UUID  `json:"id"`
	TurnoID              uuid.UUID  `json:"turno_id"`
	EmpresaID            uuid.UUID  `json:"empresa_id"`
	Latitude             float64    `json:"latitude"`
	Longitude            float64    `json:"longitude"`
	TimestampCriacao     time.Time  `json:"timestamp_criacao"`
	TimestampRecebimento time.Time  `json:"timestamp_recebimento"`
	Evento               string     `json:"evento"`               // inicio | checkin | finalizacao | sabotagem
	TipoSenha            *string    `json:"tipo_senha,omitempty"` // ok | emergencia | customizada (nil p/ sabotagem)
	SenhaVigiaID         *uuid.UUID `json:"senha_vigia_id,omitempty"`
	FlagGeofence         *string    `json:"flag_geofence,omitempty"`
	OrigemRede           string     `json:"origem_rede"`
	ClienteCheckinID     *string    `json:"cliente_checkin_id,omitempty"`
	CreatedAt            time.Time  `json:"created_at"`
}
```
`TipoSenha` muda de `string` para `*string` — isso é uma mudança de tipo, não só um
campo novo.

**Verificação da task**: `go build ./...` limpo. Não crie repository/service/handler
ainda — só os modelos. Se `go vet`/`go build` acusar erros em arquivos que já usam
`Checkin.TipoSenha` como `string` (ex. `internal/repository/checkin_repository.go`,
`internal/service/turno_service.go`), é esperado que quebrem agora — essas correções são
das Tasks 3 e 7, não desta. Reporte quais arquivos ficaram quebrados (não conserte).

---

## Task 3: Repository

Depende das Tasks 1 e 2.

### `internal/repository/senha_vigia_repository.go` (novo)
Siga o padrão de `internal/repository/config_escalonamento_repository.go` (queries
diretas com `pgxpool`, `errors.Is(err, pgx.ErrNoRows)` para "não encontrado", sempre
filtrando por `empresa_id`). Métodos:
```go
func NewSenhaVigiaRepository(db *pgxpool.Pool) *SenhaVigiaRepository

func (r *SenhaVigiaRepository) Create(ctx context.Context, s *model.SenhaVigia) error
func (r *SenhaVigiaRepository) ListByUsuario(ctx context.Context, empresaID, usuarioID uuid.UUID) ([]model.SenhaVigia, error)
func (r *SenhaVigiaRepository) FindByID(ctx context.Context, empresaID, id uuid.UUID) (*model.SenhaVigia, error)
// FindByUsuarioECodigo retorna (nil, nil) se nao achar (nao e erro) -- usado na
// resolucao do check-in, onde "nao achou" e um caso de negocio normal, nao uma falha.
func (r *SenhaVigiaRepository) FindByUsuarioECodigo(ctx context.Context, empresaID, usuarioID uuid.UUID, codigo string) (*model.SenhaVigia, error)
func (r *SenhaVigiaRepository) CountByUsuario(ctx context.Context, empresaID, usuarioID uuid.UUID) (int, error)
func (r *SenhaVigiaRepository) Update(ctx context.Context, id, empresaID uuid.UUID, s *model.SenhaVigia) error
func (r *SenhaVigiaRepository) Delete(ctx context.Context, id, empresaID uuid.UUID) error
```
`Update` faz `SET codigo = $, descricao = $, nivel_escalonamento_id = $, updated_at =
now()`. `Delete` usa `Exec` + checa `RowsAffected() == 0` para erro "não encontrado"
(mesmo padrão de `internal/repository/sessao_dispositivo_repository.go` método
`DeleteByDeviceID`).

### `internal/repository/config_escalonamento_repository.go` (novos métodos, adicionar ao arquivo existente)
```go
// FindByID busca um nivel de escalonamento especifico pertencente a empresa.
func (r *ConfigEscalonamentoRepository) FindByID(ctx context.Context, id, empresaID uuid.UUID) (*model.ConfigEscalonamento, error)

// FindMaiorNivel retorna a config do MAIOR nivel configurado para a empresa no
// momento da chamada (resolucao em runtime, nao cacheada). Retorna (nil, nil) se a
// empresa nao tem nenhum nivel de escalonamento configurado.
func (r *ConfigEscalonamentoRepository) FindMaiorNivel(ctx context.Context, empresaID uuid.UUID) (*model.ConfigEscalonamento, error)
```
`FindMaiorNivel` resolve `SELECT MAX(nivel) FROM config_escalonamento WHERE empresa_id =
$1`, e se não for NULL, delega para o `FindByEmpresaENivel` já existente no mesmo
arquivo (reaproveitar, não duplicar a query de busca+destinatários).

### `internal/repository/checkin_repository.go` (alterações no arquivo existente)
As 4 queries (`Create`, `CreateIdempotent`, `FindUltimoByTurno`, `ListByTurno`) passam a
incluir as colunas `evento, tipo_senha, senha_vigia_id` na lista de colunas do
INSERT/SELECT e no `Scan` correspondente. `TipoSenha` agora é `*string` — o driver pgx
lida com ponteiro nulo automaticamente, não precisa de tratamento especial no Scan além
de escanear para `&c.TipoSenha`.

**Verificação da task**: `go build ./...` limpo (as camadas de service/handler que ainda
não foram atualizadas continuam quebradas até a Task 7 — isso é esperado, reporte quais
pacotes ainda não compilam e por quê). Escreva testes unitários para
`SenhaVigiaRepository` e os 2 métodos novos de `ConfigEscalonamentoRepository` **apenas
se houver testes unitários de repository já existentes no projeto usando algum mock/fake
de `pgxpool`** — confirme isso lendo o diretório `internal/repository/` antes de decidir
(hoje `internal/repository` não tem nenhum arquivo `_test.go`, então é provável que a
cobertura desses repositórios seja só via os testes de integração da Task 10, não
unitária; se for esse o caso, não invente uma infraestrutura de teste unitário nova só
para esta task).

---

## Task 4: Provisionamento padrão de empresa

Depende das Tasks 1-3.

### `internal/service/empresa_service.go` (adicionar ao arquivo existente)
```go
// ProvisionarPadrao cria o nivel de escalonamento inicial (nivel=1, atraso=15min) com
// o usuario informado (o admin recem-criado da empresa) como destinatario. Chamado
// logo apos a criacao do primeiro admin de uma empresa, garantindo que "nivel maximo
// dinamico" (usado pelas senhas de emergencia/customizada) sempre tenha alguem pra
// notificar.
func (s *EmpresaService) ProvisionarPadrao(ctx context.Context, empresaID, adminID uuid.UUID) error
```
Implementação: cria (ou reaproveita, se já existir por algum motivo) um
`config_escalonamento{Nivel: 1, AtrasoMinutos: 15, UsuarioIDs: []uuid.UUID{adminID}}`
via `ConfigEscalonamentoRepository.Create` (repositório já existe, `EmpresaService`
precisa ganhar essa dependência no construtor — atualize `NewEmpresaService` e o wiring
em `internal/app/app.go`). Trate o caso de já existir nível 1 para essa empresa (ex.: se
chamado mais de uma vez) de forma idempotente — não deve falhar nem duplicar.

### `internal/seed/seed.go` (alterações no arquivo existente)
Depois de criar `empresa` (se não existia) e `admin` (linhas ~28 e ~53 do arquivo
atual), chamar `empresaService.ProvisionarPadrao(ctx, empresa.ID, admin.ID)`. Isso muda
a assinatura de `seed.Run` — hoje é `Run(ctx, empresaRepo, userRepo)`; precisa passar a
receber também o `*service.EmpresaService` (ou o `*repository.ConfigEscalonamentoRepository`,
avalie qual é mais coeso com o resto do arquivo). Atualizar o call site de `seed.Run` em
`cmd/server/main.go` (ou onde for chamado) de acordo.

**Verificação da task**: `go build ./...` limpo (service/handler de turno ainda quebrados
até Task 7, ok). Rodar o seed contra um Postgres local (`make db-up` se disponível, ou
Postgres efêmero) e confirmar via SQL que a empresa demo ganhou 1 linha em
`config_escalonamento` e 1 em `config_escalonamento_destinatarios` apontando pro admin
recém-criado. Reportar as queries de verificação e o resultado.

---

## Task 5: SenhaVigiaService (CRUD administrativo)

Depende das Tasks 1-3.

### `internal/service/senha_vigia_service.go` (novo)
```go
type SenhaVigiaService struct {
	senhaRepo  *repository.SenhaVigiaRepository
	userRepo   *repository.UserRepository
	configRepo *repository.ConfigEscalonamentoRepository
}

func NewSenhaVigiaService(senhaRepo *repository.SenhaVigiaRepository, userRepo *repository.UserRepository, configRepo *repository.ConfigEscalonamentoRepository) *SenhaVigiaService

func (s *SenhaVigiaService) List(ctx context.Context, empresaID, usuarioID uuid.UUID) ([]model.SenhaVigia, error)
func (s *SenhaVigiaService) Create(ctx context.Context, empresaID, usuarioID uuid.UUID, req model.CreateSenhaVigiaRequest) (*model.SenhaVigia, error)
func (s *SenhaVigiaService) Update(ctx context.Context, empresaID, usuarioID, senhaID uuid.UUID, req model.UpdateSenhaVigiaRequest) (*model.SenhaVigia, error)
func (s *SenhaVigiaService) Delete(ctx context.Context, empresaID, usuarioID, senhaID uuid.UUID) error
```

Erros sentinela novos, neste arquivo:
```go
var (
	ErrSenhaNaoEncontrada               = errors.New("senha nao encontrada")
	ErrSenhaCodigoDuplicado             = errors.New("codigo ja usado por outra senha deste vigia")
	ErrSenhaTipoJaExiste                = errors.New("vigia ja possui uma senha deste tipo")
	ErrSenhaObrigatoriaNaoRemovivel     = errors.New("senha obrigatoria (ok/emergencia) nao pode ser removida")
	ErrSenhaCampoNaoEditavelParaTipo    = errors.New("campo nao editavel para este tipo de senha")
	ErrNivelEscalonamentoNaoEncontrado  = errors.New("nivel de escalonamento nao encontrado")
	ErrNivelInvalidoParaTipo            = errors.New("nivel de escalonamento nao pode ser definido para senha ok/emergencia")
	ErrUsuarioNaoPertenceAEmpresa       = errors.New("usuario nao pertence a esta empresa") // reaproveitar se ja existir esse erro em outro arquivo do pacote service
)
```
Se `ErrUsuarioNaoPertenceAEmpresa` já existir em `internal/service/alerta_service.go`
(confirme lendo o arquivo), REUSE o mesmo em vez de declarar de novo — não duplicar
sentinela no mesmo pacote.

Regras de `Create`:
1. `userRepo.FindByIDEmpresa(ctx, usuarioID, empresaID)` → `ErrUsuarioNaoPertenceAEmpresa` se não achar.
2. `Tipo in (ok, emergencia)` + `NivelEscalonamentoID != nil` no request → `ErrNivelInvalidoParaTipo`.
3. `Tipo == customizada`: `Descricao` obrigatória (a tag `required_if` já valida no
   handler, mas reforce no service); se `NivelEscalonamentoID` vier preenchido, validar
   via `configRepo.FindByID(ctx, nivelID, empresaID)` → `ErrNivelEscalonamentoNaoEncontrado`
   se não achar/não pertencer à empresa.
4. Para `ok`/`emergencia`: antes do INSERT, checar via `senhaRepo.ListByUsuario` (ou um
   `CountByUsuario` filtrado — decida a forma mais simples) se já existe um do mesmo
   tipo para aquele vigia → `ErrSenhaTipoJaExiste`.
5. Checar código duplicado no mesmo vigia (`ListByUsuario` + comparação, ou deixar o
   `UNIQUE(usuario_id, codigo)` do banco estourar e mapear a violação) →
   `ErrSenhaCodigoDuplicado`.

Regras de `Update`: buscar a senha existente escopada por `empresaID+usuarioID+senhaID`
→ `ErrSenhaNaoEncontrada`; se `Tipo in (ok, emergencia)` e o request trouxer
`Descricao`/`NivelEscalonamentoID`/`NivelDinamico` não-nulos → `ErrSenhaCampoNaoEditavelParaTipo`
(só `Codigo` é aplicável); se `Tipo == customizada`, aplicar os campos informados,
validando `NivelEscalonamentoID` como no `Create` quando presente, e tratando
`NivelDinamico: true` como "forçar `NivelEscalonamentoID = nil`" (documentar essa
precedência se ambos vierem preenchidos ao mesmo tempo — `NivelDinamico` deve vencer).

Regras de `Delete`: buscar a senha; se `Tipo in (ok, emergencia)` →
`ErrSenhaObrigatoriaNaoRemovivel`; senão deletar.

**Verificação da task**: escrever testes unitários de `SenhaVigiaService` usando o
padrão de mock/fake já estabelecido no projeto para testar services isoladamente — leia
`internal/service/*_test.go` (se existir algum) antes de decidir a abordagem; se não
houver nenhum teste unitário de service no projeto hoje (provável, dado que
`internal/repository` também não tem), siga o padrão observado em
`internal/service` (arquivo `turno_window_test.go` foi citado como existente — leia-o
primeiro) para saber que estilo de teste (unitário puro vs. precisa de DB) o projeto já
usa antes de escrever os testes desta task. `go build ./...` limpo.

---

## Task 6: AlertaService

Depende das Tasks 1-3. Pode rodar em paralelo de conteúdo com a Task 5 (não há
dependência direta entre elas), mas dispare sequencialmente conforme a regra do processo
(nunca dois implementadores em paralelo).

### `internal/service/alerta_service.go` (alterações no arquivo existente)
- Remover `"coacao"` da slice `tiposEmergencia` (linha ~25) → fica
  `[]string{"sabotagem", "no_show"}`.
- Novo método público:
```go
// CreateAlertaPorSenha cria um alerta imediato (sem dedupe, mesmo padrao de
// CreateAlertaImediato) cujos destinatarios vem do nivel de escalonamento vinculado
// ao PIN: nivel especifico (senha.NivelEscalonamentoID) ou nivel maximo dinamico da
// empresa (NivelEscalonamentoID == nil), sempre resolvido em runtime no momento do
// disparo -- nunca fixado na criacao do PIN.
func (s *AlertaService) CreateAlertaPorSenha(ctx context.Context, empresaID, turnoID uuid.UUID, tipo string, senha *model.SenhaVigia, mensagem string) (*model.Alerta, error) {
	var cfg *model.ConfigEscalonamento
	var err error
	if senha.NivelEscalonamentoID != nil {
		cfg, err = s.configRepo.FindByID(ctx, *senha.NivelEscalonamentoID, empresaID)
	} else {
		cfg, err = s.configRepo.FindMaiorNivel(ctx, empresaID)
	}
	if err != nil {
		return nil, fmt.Errorf("resolver nivel da senha: %w", err)
	}
	var nivel int
	var usuarioIDs []uuid.UUID
	if cfg != nil {
		nivel = cfg.Nivel
		usuarioIDs = cfg.UsuarioIDs
	} else {
		slog.Error("empresa sem nivel de escalonamento configurado; alerta de senha criado sem destinatarios",
			"empresa_id", empresaID, "turno_id", turnoID)
	}
	return s.criarAlerta(ctx, empresaID, turnoID, tipo, nivel, mensagem, usuarioIDs)
}
```
(`s.configRepo` já existe no struct do `AlertaService` — confirme o nome exato do campo
lendo o arquivo antes de referenciar; `criarAlerta` é o núcleo privado já existente,
reaproveitar, não duplicar.)
- `DeleteEscalonamento`: como `senhas_vigia.nivel_escalonamento_id` agora tem `ON DELETE
  RESTRICT` (Task 1), deletar um nível referenciado por um PIN customizado vai falhar
  com erro de FK do Postgres. Capturar isso (`pgconn.PgError`, checar `.Code ==
  "23503"` — import `github.com/jackc/pgx/v5/pgconn`) e retornar um novo sentinel
  `ErrNivelEscalonamentoEmUso = errors.New("nivel de escalonamento em uso por uma senha de vigia")`
  em vez de propagar o erro cru do Postgres. Mapear esse erro para HTTP 409 no handler
  correspondente (`internal/handler/alerta.go`, método `DeleteEscalonamento`).

**Verificação da task**: `go build ./...` limpo. Rodar os testes unitários/integração
existentes de `alerta_service.go` (procure por `alerta_test.go`,
`internal/integration/alerta_test.go`) e confirmar que a remoção de `"coacao"` de
`tiposEmergencia` não quebra nada que não seja esperado (é aceitável que
`internal/integration/alerta_test.go` tenha casos que agora falham porque testavam
`coacao` como tipo de emergência válido — **não corrija esses testes nesta task**, isso
é da Task 10; apenas reporte quais testes passaram a falhar e por quê).

---

## Task 7: TurnoService — integração do PIN nos 4 pontos de entrada

Depende das Tasks 1, 2, 3, 5 e 6 (usa `SenhaVigiaRepository`, `SenhaVigiaService` não é
usado diretamente aqui — só o repository —, e `AlertaService.CreateAlertaPorSenha`).
Esta é a task mais sensível: mexe no fluxo central de negócio do sistema.

### `internal/service/turno_service.go` (alterações no arquivo existente)

Novo campo `senhaVigiaRepo *repository.SenhaVigiaRepository` no struct `TurnoService` e
no construtor `NewTurnoService` (propagar depois em `internal/app/app.go` — mas o
wiring completo de `app.go` é da Task 8; nesta task, só ajuste a assinatura do
construtor e deixe `app.go` quebrado se for o caso, reporte isso).

Novos erros sentinela (NUNCA retornados ao chamador HTTP — só usados internamente e
logados):
```go
var (
	ErrSenhaVigiaInvalida         = errors.New("senha invalida")
	ErrVigiaSemSenhasConfiguradas = errors.New("vigia sem senhas configuradas")
)
```

Novo helper privado:
```go
// resolverSenha busca o PIN do vigia (escopado por usuario_id) que bate com o codigo
// informado. Distingue "vigia sem nenhum PIN cadastrado" de "codigo nao corresponde a
// nenhum PIN".
func (s *TurnoService) resolverSenha(ctx context.Context, empresaID, usuarioID uuid.UUID, codigo string) (*model.SenhaVigia, error) {
	total, err := s.senhaVigiaRepo.CountByUsuario(ctx, empresaID, usuarioID)
	if err != nil {
		return nil, fmt.Errorf("contar senhas do vigia: %w", err)
	}
	if total == 0 {
		return nil, ErrVigiaSemSenhasConfiguradas
	}
	senha, err := s.senhaVigiaRepo.FindByUsuarioECodigo(ctx, empresaID, usuarioID, codigo)
	if err != nil {
		return nil, fmt.Errorf("buscar senha: %w", err)
	}
	if senha == nil {
		return nil, ErrSenhaVigiaInvalida
	}
	return senha, nil
}

// aplicarConsequenciaSenha resolve o PIN e, se nao for 'ok', marca o turno como
// critico e dispara o alerta via AlertaService.CreateAlertaPorSenha. Erros de
// resolucao (PIN invalido/vigia sem PINs) sao SEMPRE ENGOLIDOS aqui (so logados) --
// a acao chamadora ja foi ou sera persistida com sucesso independente do resultado.
// Retorna o *model.SenhaVigia resolvido (ou nil) para popular
// Checkin.TipoSenha/SenhaVigiaID antes do INSERT.
func (s *TurnoService) aplicarConsequenciaSenha(ctx context.Context, empresaIDStr string, empresaID, turnoID, usuarioID uuid.UUID, codigo string) *model.SenhaVigia {
	senha, err := s.resolverSenha(ctx, empresaID, usuarioID, codigo)
	if err != nil {
		slog.Warn("senha de vigia nao resolvida", "error", err, "turno_id", turnoID, "usuario_id", usuarioID)
		return nil
	}
	if senha.Tipo == "ok" {
		return senha
	}

	_ = s.turnoRepo.UpdateStatus(ctx, turnoID, empresaID, "critico", nil)

	tipoAlerta := "senha_emergencia"
	mensagem := "Senha de emergencia detectada"
	if senha.Tipo == "customizada" {
		tipoAlerta = "senha_customizada"
		desc := ""
		if senha.Descricao != nil {
			desc = *senha.Descricao
		}
		mensagem = "Senha customizada detectada: " + desc
	}

	if _, err := s.alertaService.CreateAlertaPorSenha(ctx, empresaID, turnoID, tipoAlerta, senha, mensagem); err != nil {
		slog.Error("criar alerta de senha", "error", err, "turno_id", turnoID)
	}
	s.hub.Broadcast(empresaIDStr, ws.NewStatusChangeEvent(turnoID.String(), "critico"))
	return senha
}
```

Integração nos 4 pontos (leia `internal/service/turno_service.go` inteiro antes de
editar, para preservar exatamente a lógica de negócio existente ao redor — escala,
geofence, janela deslizante):

- **`Iniciar`**: o request ganha `Latitude`/`Longitude` (já definidos na Task 2). Depois
  de `s.turnoRepo.Create(ctx, turno)` bem-sucedido (turno já tem `.ID`), calcular
  `flagGeofence := s.calcularGeofence(ctx, turno.PostoID, parsedEmpresaID, req.Latitude,
  req.Longitude)`, chamar `aplicarConsequenciaSenha(ctx, empresaID, parsedEmpresaID,
  turno.ID, parsedUserID, req.Senha)`, montar e persistir
  `Checkin{TurnoID: turno.ID, EmpresaID: parsedEmpresaID, Latitude: req.Latitude,
  Longitude: req.Longitude, TimestampCriacao: now, Evento: "inicio",
  TipoSenha/SenhaVigiaID: derivados da senha resolvida (nil se não resolveu),
  FlagGeofence: flagGeofence, OrigemRede: "online"}` via `s.checkinRepo.Create`. Se
  `aplicarConsequenciaSenha` retornou uma senha com `Tipo != "ok"`, atualizar
  `turno.Status = "critico"` **em memória** antes do `return turno, nil` — senão a
  resposta HTTP do endpoint mentiria sobre o status real (mesmo cuidado que `Finalizar`
  já toma hoje). O broadcast existente `ws.NewStatusChangeEvent(turno.ID.String(),
  "em_andamento")` (linha ~145) continua acontecendo sempre; se a senha for de
  emergência/customizada, `aplicarConsequenciaSenha` já dispara um segundo broadcast de
  `"critico"` por conta própria — não duplicar.
- **`Checkin`**: mover a resolução da senha e a montagem do `Checkin` para **antes** do
  único `s.checkinRepo.Create` (elimina o insert duplo). A ordem de captura do
  `anterior` (checkin anterior, para a janela deslizante — comentário explícito sobre o
  bug A2 em `turno_service.go:249-250`) **precisa continuar acontecendo antes do
  insert**, exatamente como hoje — não mude essa ordem. Populate
  `Checkin{Evento: "checkin", TipoSenha/SenhaVigiaID: ...}` no lugar de
  `TipoSenha: req.TipoSenha`. Remover o bloco `if req.TipoSenha == "coacao" { ... }`
  (linhas 268-272) e substituir por uma chamada a `aplicarConsequenciaSenha` **depois**
  do insert (mesma posição relativa que o bloco de coação ocupava hoje).
- **`Finalizar`**: mesmo padrão — resolver a senha e montar
  `Checkin{Evento: "finalizacao", ...}` antes do insert único. Chamar
  `aplicarConsequenciaSenha` para o efeito colateral (alerta), e **sempre**, por último,
  aplicar `s.turnoRepo.UpdateStatus(ctx, parsedTurnoID, parsedEmpresaID, "finalizado",
  &now)` — isso sobrescreve qualquer `"critico"` transitório que
  `aplicarConsequenciaSenha` tenha setado. O turno sempre termina como `"finalizado"`; o
  alerta é quem carrega a urgência, não o status final do turno.
- **`ProcessarLote`**: mesmo padrão do `Checkin` — cada item do lote resolve sua própria
  senha (o campo `Senha` de cada `CheckinRequest` do array), populando
  `Evento: "checkin"`, `OrigemRede: "offline_sincronizado"`. Substituir o bloco de
  coação equivalente (linhas ~735-739) por `aplicarConsequenciaSenha`.

**Verificação da task**: `go build ./...` e `go vet ./...` limpos em TODO o projeto
(essa task deve deixar o projeto compilando de novo, já que é onde as últimas peças se
encaixam — exceto handler/rotas/app.go, que são da Task 8). Rodar `go test ./...`
(unitários, sem tag integration) e confirmar que passam. NÃO tente rodar
`internal/integration/...` ainda (precisa dos ajustes de mocks/helpers da Task 10) — só
confirme que compila com `go build -tags integration ./...` ou equivalente, sem
necessariamente passar.

---

## Task 8: Handler + Rotas

Depende das Tasks 1-7 (precisa que tudo compile).

### `internal/handler/senha_vigia.go` (novo)
Siga o padrão exato de `internal/handler/usuario.go` (mesma estrutura de método,
mesma forma de pegar `empresaID` do JWT via `GetEmpresaID(r.Context())`, mesmo uso de
`chi.URLParam`, `h.validate.Struct(req)`, `writeValidationError`/`writeError`/`writeJSON`,
mapeamento de erros sentinela via `errors.Is`):
```go
type SenhaVigiaHandler struct {
	service  *service.SenhaVigiaService
	validate *validator.Validate
}

func NewSenhaVigiaHandler(s *service.SenhaVigiaService) *SenhaVigiaHandler

func (h *SenhaVigiaHandler) List(w http.ResponseWriter, r *http.Request)   // GET /usuarios/{id}/senhas
func (h *SenhaVigiaHandler) Create(w http.ResponseWriter, r *http.Request) // POST /usuarios/{id}/senhas
func (h *SenhaVigiaHandler) Update(w http.ResponseWriter, r *http.Request) // PUT /usuarios/{id}/senhas/{senhaId}
func (h *SenhaVigiaHandler) Delete(w http.ResponseWriter, r *http.Request) // DELETE /usuarios/{id}/senhas/{senhaId}
```
Mapeamento de erros: `ErrUsuarioNaoPertenceAEmpresa`→404, `ErrSenhaNaoEncontrada`→404,
`ErrSenhaCodigoDuplicado`/`ErrSenhaTipoJaExiste`/`ErrNivelInvalidoParaTipo`/
`ErrSenhaCampoNaoEditavelParaTipo`→400, `ErrSenhaObrigatoriaNaoRemovivel`→409,
`ErrNivelEscalonamentoNaoEncontrado`→400.

### `internal/app/app.go` (alterações no arquivo existente)
Wiring completo:
```go
senhaVigiaRepo := repository.NewSenhaVigiaRepository(pool)
// ...
senhaVigiaService := service.NewSenhaVigiaService(senhaVigiaRepo, userRepo, configEscalonamentoRepo)
senhaVigiaHandler := handler.NewSenhaVigiaHandler(senhaVigiaService)
```
Atualizar o construtor de `turnoService := service.NewTurnoService(...)` para passar
`senhaVigiaRepo` (a assinatura mudou na Task 7 — confirme a ordem exata de parâmetros
lendo o construtor atual). Atualizar `empresaService := service.NewEmpresaService(...)`
para passar a dependência nova que a Task 4 adicionou. Atualizar a chamada de
`seed.Run(...)` para o novo formato de assinatura da Task 4.

Rotas — dentro do bloco `/usuarios` já existente e já protegido por
`handler.RequireRole("admin")` (`internal/app/app.go`, procure `r.Route("/usuarios",
...)`):
```go
r.Route("/{id}/senhas", func(r chi.Router) {
	r.Get("/", senhaVigiaHandler.List)
	r.Post("/", senhaVigiaHandler.Create)
	r.Put("/{senhaId}", senhaVigiaHandler.Update)
	r.Delete("/{senhaId}", senhaVigiaHandler.Delete)
})
```

### `internal/handler/turno.go`
Não deveria precisar de nenhuma alteração de mapeamento de erro (por design,
`ErrSenhaVigiaInvalida`/`ErrVigiaSemSenhasConfiguradas` nunca saem do service). Só
confirme que os handlers de Iniciar/Checkin/Finalizar continuam decodificando o body
genericamente (sem lista explícita de campos) — se algum handler tiver validação
explícita de campo por campo em vez de `h.validate.Struct(req)`, ajuste para que os
campos novos (`latitude`/`longitude`/`senha`) fluam automaticamente.

### `internal/handler/alerta.go`
Adicionar o mapeamento de `service.ErrNivelEscalonamentoEmUso` (criado na Task 6) → 409,
no método `DeleteEscalonamento`.

**Verificação da task**: `go build ./...` e `go vet ./...` limpos no projeto inteiro,
sem exceções desta vez. Rodar `go test ./...` (unitários) e confirmar que passam.
Rodar o servidor localmente (`go run ./cmd/server/` contra um Postgres com as
migrations aplicadas) e fazer pelo menos 1 chamada manual real com `curl`/similar a
`POST /api/v1/usuarios/{id}/senhas` (usando um usuário vigia de teste e um token admin
válido) confirmando que retorna 201 com o PIN criado — reporte o comando exato e a
resposta no relatório da task.

---

## Task 9: Regeneração de docs Swagger

Depende da Task 8 (rotas/handlers finalizados).

Rodar o alvo do `Makefile` (`make docs`, que executa `go run
github.com/swaggo/swag/cmd/swag@v1.16.6 init -g cmd/server/main.go -o docs`) e commitar
`docs/docs.go`, `docs/swagger.json`, `docs/swagger.yaml` atualizados. Adicionar
anotações Swagger nos novos handlers de `internal/handler/senha_vigia.go` seguindo o
mesmo estilo de anotação já usado em `internal/handler/usuario.go` (comentários
`// @Summary`, `// @Tags`, `// @Param`, `// @Success`, `// @Router` etc. logo acima de
cada método) — sem essas anotações, `swag init` não vai gerar a documentação das rotas
novas corretamente.

**Verificação da task**: rodar `make docs` (ou o comando `swag init` equivalente) e
confirmar que `docs/docs.go`/`docs/swagger.json`/`docs/swagger.yaml` mudam e incluem as
rotas `/usuarios/{id}/senhas*`. Rodar `go build ./...` de novo (a regeneração não deveria
quebrar o build). Se o CI valida docs desatualizados via `git diff --exit-code` (procure
em `.github/workflows/*.yml`), confirme localmente que rodar `make docs` duas vezes
seguidas produz o mesmo resultado (idempotente) antes de commitar.

---

## Task 10: Testes de integração

Depende de todas as tasks anteriores (é a última, precisa do sistema completo
funcionando).

Ajustar os testes de integração existentes impactados pela mudança de `tipo_senha`
(enum fixo) para `senha` (PIN por vigia):

- **`internal/integration/helpers_test.go`**:
  - `iniciarTurno()` (linha ~216-223 no código-fonte original) precisa passar
    `latitude`/`longitude`/`senha` no corpo da requisição.
  - `checkinBody(turnoID, tipo, ts)` (linha ~225-234): o parâmetro `tipo` (que hoje
    recebe `"padrao"`/`"coacao"`/etc.) precisa virar `senha` (o código PIN real
    cadastrado no cenário de teste).
  - Novo helper `criarSenhaVigia(usuarioID uuid.UUID, tipo, codigo string, descricao
    *string, nivelID *uuid.UUID) *model.SenhaVigia` batendo em `POST
    /usuarios/{id}/senhas`.
  - `novoCenario` (ou onde quer que o cenário de teste padrão seja montado) deve
    cadastrar os PINs `ok` e `emergencia` do vigia de teste **antes** de qualquer
    chamada de `iniciarTurno`/`checkinBody`, usando códigos fixos conhecidos pelos
    testes (ex.: `ok="1234"`, `emergencia="9999"`).
- **`internal/integration/turno_test.go`**: linha ~214, `if ck.TipoSenha ==
  "finalizacao"` precisa virar `if ck.Evento == "finalizacao"` (já que `TipoSenha` agora
  é `*string` com semântica `ok/emergencia/customizada`, nunca mais `"finalizacao"`).
  Reescreva qualquer chamada `checkinBody(turno.ID, "padrao", ...)` /
  `checkinBody(turno.ID, "coacao", ...)` para usar os códigos de PIN reais do cenário
  (`"1234"`/`"9999"` ou o que a Task tiver definido no helper).
- **`internal/integration/worker_test.go`, `internal/integration/alerta_test.go`**:
  revisar qualquer teste de `PUT /config/alertas-emergencia/coacao` (deve passar a
  retornar 400, já que `"coacao"` não é mais um tipo válido) e qualquer teste de coação
  via `POST /turnos/checkin` com `tipo_senha=coacao` (reescrever para usar PIN
  `emergencia` cadastrado, e verificar o alerta resultante com `tipo="senha_emergencia"`
  em vez de `tipo="coacao"`).

Escrever, além dos ajustes acima, pelo menos estes 3 testes novos (podem ir em
`internal/integration/turno_test.go` ou um arquivo novo
`internal/integration/senha_vigia_test.go`, o que for mais coerente com a organização
atual — leia `internal/integration/` antes de decidir):
1. PIN customizado vinculado a um nível de escalonamento específico → o alerta
   resultante só notifica os destinatários **daquele** nível, não do nível máximo.
2. Vigia sem nenhum PIN cadastrado → `iniciar`/`checkin`/`finalizar` prosseguem
   normalmente (200/201), sem nenhum alerta criado.
3. Tentar deletar um nível de escalonamento referenciado por um PIN customizado → 409.

**Verificação da task**: `make test-db-up` (sobe Postgres efêmero na porta 5433),
`make test-integration` (ou `go test ./... -tags integration -p 1 -count=1 -race`),
confirmar suíte inteira passando, depois `make test-db-down`. Reportar a saída completa
do comando de teste no relatório.

---

## Verificação end-to-end (após Task 10)
- `go build ./...` e `go vet ./...` limpos.
- `make test` (unitários) e `make test-integration` (integração) passando.
- Fluxo manual via servidor local: criar vigia → cadastrar PIN `ok` (`1234`) e
  `emergencia` (`9999`) via `/usuarios/{id}/senhas` → iniciar turno com `senha=1234`
  (sucesso, sem alerta) → checkin com `senha=9999` (sucesso, turno vira `critico`,
  alerta aparece em `GET /alertas` com destinatários do nível máximo, evento `new_alert`
  chega via WebSocket) → finalizar com `senha=9999` (turno termina `"finalizado"`,
  alerta ainda assim disparado).
