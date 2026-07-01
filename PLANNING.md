# guardpoint-server — Plano de Desenvolvimento (Backend Go)

## 1. Visão Geral do Módulo

O `guardpoint-server` é o núcleo da plataforma GuardPoint. Trata-se de uma API RESTful de alta performance escrita em Go, responsável por toda a lógica de negócio, processamento concorrente, autenticação, comunicação em tempo real e persistência dos dados. É o componente central que orquestra a comunicação entre o aplicativo Android (vigias) e o painel gerencial Angular (supervisores/gestores).

## 2. Stack Tecnológica

| Componente       | Tecnologia                                         |
| ---------------- | -------------------------------------------------- |
| Linguagem        | Go 1.22+                                           |
| Roteador HTTP    | `chi` ou `gin`                                     |
| ORM / SQL        | `sqlc` + `pgx` (recomendado) ou `gorm`             |
| Banco de Dados   | PostgreSQL 16                                      |
| Autenticação     | JWT (`golang-jwt`)                                 |
| WebSockets       | `gorilla/websocket` ou `nhooyr.io/websocket`       |
| Filas/Workers    | Goroutines + Channels nativos                      |
| Geoprocessamento | `Haversine` em Go puro ou PostGIS (se ativado)     |
| Infraestrutura   | Docker, Docker Compose (dev); Railway (produção)   |
| CI/CD            | GitHub Actions + Railway                           |

## 3. Estrutura de Diretórios

```
guardpoint-server/
├── cmd/
│   └── server/
│       └── main.go              # Entrypoint
├── internal/
│   ├── config/                  # Carregamento de .env / flags
│   ├── auth/                    # JWT: geração, validação, middleware
│   ├── handler/                 # Handlers HTTP (controllers)
│   │   ├── auth.go
│   │   ├── shift.go             # Turnos
│   │   ├── checkin.go           # Reafirmação (check-in)
│   │   ├── alert.go             # Alertas
│   │   ├── device.go            # Sessão de dispositivo
│   │   └── schedule.go          # Escalas / Agenda
│   ├── middleware/              # RBAC, logging, CORS, rate-limit
│   ├── model/                   # Structs de domínio (DTOs, entidades)
│   ├── repository/              # Acesso a dados (sqlc queries)
│   ├── service/                 # Lógica de negócio
│   │   ├── checkin_service.go
│   │   ├── alert_service.go
│   │   ├── schedule_service.go
│   │   └── geofence_service.go  # Cálculo Haversine
│   ├── worker/                  # Motor de Escalonamento e Workers
│   │   ├── timeout_checker.go   # Varredura de atrasos
│   │   ├── alert_dispatcher.go  # Disparo de WebHooks WhatsApp
│   │   └── sync_reconciler.go   # Reconciliação offline
│   └── ws/                      # WebSocket Hub e Clients
│       ├── hub.go
│       └── client.go
├── migrations/                  # Migrations SQL (golang-migrate)
├── docker-compose.yml
├── Dockerfile
├── go.mod
└── go.sum
```

## 4. Modelo de Dados (Entidades Principais)

### 4.1. Empresa
```sql
CREATE TABLE empresas (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    nome        VARCHAR(255) NOT NULL,
    cnpj        VARCHAR(14)  NOT NULL UNIQUE,
    ativa       BOOLEAN      DEFAULT true,
    created_at  TIMESTAMPTZ  DEFAULT now()
);
```

### 4.2. Usuário
```sql
CREATE TABLE usuarios (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    empresa_id    UUID         NOT NULL REFERENCES empresas(id),
    nome          VARCHAR(255) NOT NULL,
    email         VARCHAR(255) NOT NULL UNIQUE,
    senha_hash    VARCHAR(255) NOT NULL,
    role          VARCHAR(50)  NOT NULL, -- 'admin', 'supervisor', 'vigia'
    telefone      VARCHAR(20),
    ativo         BOOLEAN      DEFAULT true,
    created_at    TIMESTAMPTZ  DEFAULT now()
);
CREATE INDEX idx_usuarios_empresa ON usuarios(empresa_id);
```

### 4.3. Posto / Obra
```sql
CREATE TABLE postos (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    empresa_id  UUID         NOT NULL REFERENCES empresas(id),
    nome        VARCHAR(255) NOT NULL,
    latitude    DOUBLE PRECISION NOT NULL,
    longitude   DOUBLE PRECISION NOT NULL,
    raio_m      INTEGER      DEFAULT 100,  -- raio de tolerância em metros
    ativo       BOOLEAN      DEFAULT true,
    created_at  TIMESTAMPTZ  DEFAULT now()
);
```

### 4.4. Turno
```sql
CREATE TABLE turnos (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    empresa_id      UUID        NOT NULL REFERENCES empresas(id),
    usuario_id      UUID        NOT NULL REFERENCES usuarios(id),
    posto_id        UUID        NOT NULL REFERENCES postos(id),
    status          VARCHAR(20) NOT NULL DEFAULT 'agendado',
        -- 'agendado', 'em_andamento', 'pausado', 'finalizado', 'critico'
    inicio_previsto TIMESTAMPTZ NOT NULL,
    fim_previsto    TIMESTAMPTZ NOT NULL,
    inicio_real     TIMESTAMPTZ,
    fim_real        TIMESTAMPTZ,
    token_sessao    VARCHAR(255),         -- sessão atrelada ao dispositivo
    intervalo_min   INTEGER     DEFAULT 30, -- minutos entre check-ins
    created_at      TIMESTAMPTZ DEFAULT now()
);
```

### 4.5. Check-in (Log de Reafirmação)
```sql
CREATE TABLE checkins (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    turno_id        UUID        NOT NULL REFERENCES turnos(id),
    empresa_id      UUID        NOT NULL REFERENCES empresas(id),
    latitude        DOUBLE PRECISION NOT NULL,
    longitude       DOUBLE PRECISION NOT NULL,
    timestamp_criacao TIMESTAMPTZ NOT NULL,  -- HORA DO CELULAR (confiável)
    timestamp_recebimento TIMESTAMPTZ DEFAULT now(),
    tipo_senha      VARCHAR(20) NOT NULL,
        -- 'padrao', 'coacao', 'finalizacao'
    flag_geofence   VARCHAR(20),
        -- 'ok', 'desvio_rota'
    origem_rede     VARCHAR(20) DEFAULT 'online',
        -- 'online', 'offline_sincronizado'
    created_at      TIMESTAMPTZ DEFAULT now()
);
CREATE INDEX idx_checkins_turno ON checkins(turno_id, timestamp_criacao);
```

### 4.6. Alerta
```sql
CREATE TABLE alertas (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    empresa_id      UUID        NOT NULL REFERENCES empresas(id),
    turno_id        UUID        NOT NULL REFERENCES turnos(id),
    tipo            VARCHAR(30) NOT NULL,
        -- 'atraso_n1', 'atraso_n2', 'atraso_n3', 'coacao', 'sabotagem',
        -- 'desvio_rota', 'offline', 'falha_infra'
    nivel           INTEGER     NOT NULL,
    status          VARCHAR(20) DEFAULT 'aberto',
        -- 'aberto', 'reconhecido', 'encerrado', 'falso_positivo'
    mensagem        TEXT,
    resolvido_em    TIMESTAMPTZ,
    created_at      TIMESTAMPTZ DEFAULT now()
);
```

### 4.7. Configuração de Escalonamento
```sql
CREATE TABLE config_escalonamento (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    empresa_id      UUID        NOT NULL REFERENCES empresas(id),
    nivel           INTEGER     NOT NULL, -- N1, N2, N3...
    atraso_minutos  INTEGER     NOT NULL, -- minutos de atraso para disparar
    whatsapp_para   VARCHAR(20) NOT NULL, -- telefone de destino
    cargo_alvo      VARCHAR(50),          -- 'supervisor', 'gerente'
    created_at      TIMESTAMPTZ DEFAULT now()
);
```

### 4.8. Dispositivo / Sessão
```sql
CREATE TABLE sessoes_dispositivo (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    usuario_id      UUID        NOT NULL REFERENCES usuarios(id),
    empresa_id      UUID        NOT NULL REFERENCES empresas(id),
    device_id       VARCHAR(255) NOT NULL,
    token           VARCHAR(255) NOT NULL UNIQUE,
    ativo           BOOLEAN     DEFAULT true,
    revogado_em     TIMESTAMPTZ,
    created_at      TIMESTAMPTZ DEFAULT now()
);
```

## 5. Endpoints da API REST

### 5.1. Autenticação
| Método | Rota                  | Descrição                         |
| ------ | --------------------- | --------------------------------- |
| POST   | `/api/auth/login`     | Login (email + senha) → JWT       |
| POST   | `/api/auth/refresh`   | Refresh token                     |
| POST   | `/api/auth/logout`    | Invalida sessão                   |
| POST   | `/api/auth/biometric` | Valida autenticação biométrica OK |

### 5.2. Turnos (Vigia)
| Método | Rota                         | Descrição                                |
| ------ | ---------------------------- | ---------------------------------------- |
| POST   | `/api/turnos/iniciar`        | Inicia turno (login em campo)            |
| POST   | `/api/turnos/checkin`        | Reafirmação de vida (senha + GPS)        |
| POST   | `/api/turnos/finalizar`      | Check-in de finalização                  |
| GET    | `/api/turnos/status`         | Status do turno atual                    |
| POST   | `/api/turnos/sabotagem`      | Reporta sabotagem (GPS/Permissões off)   |

### 5.3. Turnos (Gerencial)
| Método | Rota                         | Descrição                                |
| ------ | ---------------------------- | ---------------------------------------- |
| GET    | `/api/turnos/ativos`         | Todos os turnos em andamento da empresa  |
| GET    | `/api/turnos/{id}`           | Detalhes do turno + check-ins            |
| POST   | `/api/turnos/{id}/revogar`   | Revoga sessão + libera PIN novo aparelho |
| GET    | `/api/turnos/historico`      | Histórico de turnos com filtros          |

### 5.4. Check-ins e GPS
| Método | Rota                              | Descrição                         |
| ------ | --------------------------------- | --------------------------------- |
| POST   | `/api/checkins/lote`              | Sincronização de lote offline     |
| GET    | `/api/checkins/turno/{turno_id}`  | Check-ins de um turno específico  |

### 5.5. Escalas / Agenda
| Método | Rota                         | Descrição                             |
| ------ | ---------------------------- | ------------------------------------- |
| POST   | `/api/escalas`               | Criar escala (obra x vigia x período) |
| GET    | `/api/escalas`               | Listar escalas com filtros            |
| PUT    | `/api/escalas/{id}`          | Atualizar escala                      |
| DELETE | `/api/escalas/{id}`          | Excluir escala                        |

### 5.6. Alertas
| Método | Rota                              | Descrição                      |
| ------ | --------------------------------- | ------------------------------ |
| GET    | `/api/alertas`                    | Listar alertas da empresa      |
| PUT    | `/api/alertas/{id}/reconhecer`    | Reconhecer alerta (ack)        |
| PUT    | `/api/alertas/{id}/encerrar`      | Encerrar alerta manualmente    |
| GET    | `/api/alertas/estatisticas`       | Dashboards de alertas          |

### 5.7. Configurações
| Método | Rota                                    | Descrição                        |
| ------ | --------------------------------------- | -------------------------------- |
| GET    | `/api/config/escalonamento`             | Níveis de escalonamento          |
| PUT    | `/api/config/escalonamento`             | Atualizar níveis                 |
| GET    | `/api/config/empresa`                   | Configurações gerais da empresa  |

### 5.8. WebSocket
| Método | Rota              | Descrição                           |
| ------ | ----------------- | ----------------------------------- |
| GET    | `/ws`             | Upgrade para WebSocket (JWT query)  |

## 6. WebSocket: Hub e Eventos

### 6.1. Modelo Hub-Client
- Um **Hub** central registra e gerencia todos os clients conectados por `empresa_id`.
- Cada **Client** é identificado pelo `usuario_id` e `empresa_id` extraídos do JWT no handshake.
- O Hub faz broadcast seletivo: apenas supervisores da mesma empresa recebem eventos de seus vigias.

### 6.2. Eventos Emitidos pelo Servidor

```json
// Atualização de localização (GPS) do vigia
{ "type": "gps_update", "payload": { "turno_id": "...", "latitude": -23.5, "longitude": -46.6, "timestamp": "..." } }

// Mudança de status do turno
{ "type": "status_change", "payload": { "turno_id": "...", "status": "critico", "timestamp": "..." } }

// Novo alerta gerado
{ "type": "new_alert", "payload": { "alerta_id": "...", "tipo": "atraso_n1", "turno_id": "...", "nivel": 1 } }

// Reconciliação offline concluída
{ "type": "sync_resolved", "payload": { "turno_id": "...", "resolvido": true, "motivo": "falha_infra" } }
```

## 7. Motor de Escalonamento (Workers)

### 7.1. Timeout Checker Worker
- Goroutine que executa a cada **30 segundos** (via `time.Ticker`).
- Consulta PostgreSQL: todos os turnos `em_andamento` cujo último `checkin.timestamp_criacao` excedeu `turnos.intervalo_min` minutos.
- Para cada turno atrasado, compara o tempo decorrido com `config_escalonamento` e gera alertas N1, N2, N3 conforme apropriado.
- Enfileira alertas em um `channel` consumido pelo **Alert Dispatcher Worker**.

### 7.2. Alert Dispatcher Worker
- Consome do channel de alertas pendentes.
- Para cada alerta, monta payload e dispara **WebHook do WhatsApp** (API externa, ex: Twilio, Meta Cloud API).
- Atualiza status do alerta no banco.

### 7.3. Sync Reconciler Worker
- Executa ao receber um lote offline (`POST /api/checkins/lote`).
- Para cada check-in do lote, verifica `timestamp_criacao` contra o último check-in conhecido.
- Se o check-in retroativo prova que o vigia estava OK no horário correto, **encerra alertas abertos** como `falso_positivo` e categoriza como `falha_infra`.
- Emite evento WebSocket `sync_resolved` para os supervisores.

## 8. Regras de Negócio Críticas (Core Logic)

### 8.1. Reafirmação por Janela Deslizante
- O Worker não compara contra o horário agendado, mas sim contra `ultimo_checkin.timestamp_criacao + turno.intervalo_min`.
- Se o vigia faz check-in no minuto 25 de um intervalo de 30, o próximo deadline é 30 minutos a partir do minuto 25.

### 8.2. Senha de Coação (Emergência Silenciosa)
- Quando `tipo_senha = 'coacao'`, o endpoint `/api/turnos/checkin`:
  1. Registra o check-in normalmente (GPS incluso).
  2. Altera `turnos.status` para `'critico'`.
  3. Gera alerta do tipo `'coacao'`.
  4. Dispara WebHook WhatsApp imediatamente com a localização atual e a rota percorrida.
  5. O vigia **não recebe feedback negativo** — a API retorna resposta de sucesso normal.

### 8.3. Geofencing (Haversine)
- A cada check-in, o serviço `geofence_service.go` calcula a distância entre `(lat_checkin, lon_checkin)` e `(lat_posto, lon_posto)`.
- Se `distancia > posto.raio_m`, o campo `flag_geofence` é marcado como `'desvio_rota'`.
- O check-in **é aceito** (o vigia está vivo), mas o Angular exibe o pin amarelo no mapa.

### 8.4. Sincronização Retroativa (Offline-First)
- O endpoint `POST /api/checkins/lote` recebe um array de check-ins.
- Cada item contém `timestamp_criacao` (hora do celular, confiável para o negócio).
- O servidor processa em ordem cronológica (`timestamp_criacao ASC`) e aplica a lógica de janela deslizante com base no histórico completo.
- Se detecta que um check-in retroativo cobre um período de atraso que gerou alerta, o alerta é encerrado como falso positivo.

### 8.5. Controle de Sessão Única
- Ao iniciar turno (`/api/turnos/iniciar`), o servidor gera um `token_sessao` único vinculado ao `device_id`.
- Se a central revoga (`/api/turnos/{id}/revogar`), o `token_sessao` é invalidado e um **PIN temporário** é gerado para login em novo dispositivo.
- Tentativas de iniciar turno com token revogado → HTTP 403.

### 8.6. Multi-Tenancy Rigorosa
- **Toda query** SQL inclui `WHERE empresa_id = $1`, com o valor extraído do JWT do usuário autenticado (middleware).
- Isso impede vazamento de dados entre empresas concorrentes (clientes SaaS).

## 9. Fases de Desenvolvimento

### Fase 1 — Fundação
- [x] Scaffold do projeto Go (módulo, estrutura de diretórios)
- [x] Docker Compose com PostgreSQL
- [x] Migrations iniciais (empresas, usuarios, postos, turnos)
- [x] Config loader (.env)
- [x] Logger estruturado (zerolog ou slog)

### Fase 2 — Autenticação e Multi-Tenancy
- [x] Registro e login com JWT
- [x] Middleware de autenticação e extração de `empresa_id`
- [x] RBAC básico (role no JWT, middleware de autorização)
- [x] Seed de empresa e usuário admin para dev

### Fase 3 — Core: Turnos e Check-ins
- [x] CRUD de postos
- [x] Iniciar turno (`/api/turnos/iniciar`) com validação de escala
- [x] Check-in de reafirmação (`/api/turnos/checkin`)
- [x] Check-in de finalização (`/api/turnos/finalizar`)
- [x] Lógica de janela deslizante no service layer

### Fase 4 — Geofencing
- [ ] Implementação do cálculo Haversine
- [ ] Flag `desvio_rota` nos check-ins
- [ ] Endpoint de sabotagem

### Fase 5 — Alertas e Workers
- [ ] Timeout Checker Worker
- [ ] CRUD de configuração de escalonamento
- [ ] Alert Dispatcher Worker (stub do WhatsApp inicialmente)
- [ ] Endpoints de alertas (listar, reconhecer, encerrar)

### Fase 6 — Offline e Sincronização
- [ ] Endpoint de lote (`/api/checkins/lote`)
- [ ] Sync Reconciler Worker
- [ ] Resolução de falsos positivos

### Fase 7 — WebSockets
- [ ] Hub e Client em Go
- [ ] Handshake JWT no WebSocket
- [ ] Broadcast seletivo por empresa
- [ ] Emissão de eventos: gps_update, status_change, new_alert

### Fase 8 — Escalas e Agenda
- [ ] CRUD de escalas (obra x vigia x período)
- [ ] Validação de escala ao iniciar turno
- [ ] Tolerância de início e alerta de no-show

### Fase 9 — Integração WhatsApp
- [ ] Integração com API do WhatsApp Business (Twilio / Meta)
- [ ] Templates de mensagem por tipo de alerta
- [ ] Rate limiting e retry com backoff

### Fase 10 — Produção e Observabilidade
- [ ] Dockerfile otimizado (multi-stage build)
- [ ] Health checks (`/health`, `/ready`)
- [ ] Métricas e tracing (Prometheus, Jaeger — opcional)
- [ ] Deploy no Railway via GitHub Actions CI/CD

## 10. Requisitos Não-Funcionais

| Requisito                | Meta                    |
| ------------------------ | ----------------------- |
| Latência de API (p95)    | < 100ms                 |
| Tolerância a falhas      | Retry com backoff no Worker |
| Escalabilidade           | Stateless (API), Workers independentes |
| Segurança                | JWT, HTTPS, SQL parametrizado, sanitização |
| Logs                     | JSON estruturado, níveis: debug, info, warn, error |
| Backups                  | PostgreSQL dump diário (Railway gerencia) |

## 11. Convenções de Código

- **Nomenclatura**: português para domínio (entidades, colunas), inglês para código (funções, variáveis).
- **Tratamento de erros**: sempre retornar `error` como último valor; nunca usar `panic` em handlers.
- **Validação**: `go-playground/validator` nos DTOs de entrada.
- **Context**: todo handler e repository recebe `context.Context` como primeiro parâmetro para timeout e cancelamento.
- **Migrations**: `golang-migrate/migrate` com arquivos SQL numerados (`000001_create_empresas.up.sql`).
