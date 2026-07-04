# AGENTS.md — guardpoint-server (Go)

Instruções e boas práticas para agentes de IA e desenvolvedores trabalhando no backend Go do GuardPoint.

---

## 1. Extensões e Ferramentas Recomendadas (VS Code)

| Extensão | Descrição |
|---|---|
| `golang.go` | Suporte oficial da linguagem Go (IntelliSense, debug, lint) |
| `golang.go-nightly` | Atualizações semanais do gopls |
| `ms-azuretools.vscode-docker` | Docker e Docker Compose |
| `cweijan.vscode-postgresql-client2` | Cliente PostgreSQL integrado |
| `EditorConfig.EditorConfig` | Consistência de formatação |
| `GitHub.copilot` | Assistência de código (se disponível) |

---

## 2. Configuração do Ambiente de Desenvolvimento

```bash
go install golang.org/x/tools/gopls@latest
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
go install github.com/swaggo/swag/cmd/swag@latest
go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest
go install github.com/golang-migrate/migrate/v4/cmd/migrate@latest
go install github.com/air-verse/air@latest
```

---

## 2.1 Documentação da API (Swagger/OpenAPI)

A documentação das rotas é gerada automaticamente a partir dos comentários `@Summary`/`@Router`/etc. nos handlers (`internal/handler/*.go`) e dos structs de DTO referenciados (`internal/model/*.go`). Sempre que um DTO ou uma rota mudar, rode:

```bash
make docs
```

Isso regenera `docs/docs.go`, `docs/swagger.json` e `docs/swagger.yaml` — esses arquivos devem ser commitados. O CI roda o mesmo comando e falha (`git diff --exit-code docs/`) se a documentação ficar desatualizada.

Com o servidor rodando fora de produção (`ENV != production`), a Swagger UI fica disponível em `/swagger/index.html`.

---

## 3. Estrutura de Projeto Go (Padrão de Mercado)

```
project/
├── cmd/                    # Entrypoints (um por binário)
│   ├── server/main.go
│   └── worker/main.go      # Se houver worker independente
├── internal/               # Código privado ao módulo
│   ├── config/             # .env, flags, viper
│   ├── handler/            # HTTP handlers (1 arquivo por recurso)
│   ├── middleware/         # auth, logging, cors, ratelimit
│   ├── model/              # Structs de domínio, DTOs
│   ├── repository/         # Acesso a dados (sqlc queries)
│   ├── service/            # Lógica de negócio pura
│   ├── worker/             # Goroutines de background
│   └── ws/                 # WebSocket hub/client
├── migrations/             # Arquivos SQL numerados
├── pkg/                    # Código reutilizável (se aplicável)
├── .env.example
├── docker-compose.yml
├── Dockerfile
├── go.mod
├── go.sum
├── Makefile
└── .golangci.yml
```

---

## 4. Convenções de Código Go

### 4.1. Nomenclatura
- **Pacotes**: minúsculas, sem underscore, nome curto e descritivo (`handler`, não `handlers`; `postgres`, não `postgresql`).
- **Exportados**: PascalCase (`UserService`, `NewUserService`, `GetByID`).
- **Não exportados**: camelCase (`userRepo`, `validateEmail`, `parseToken`).
- **Interfaces**: sufixo `-er` quando fizer sentido (`Reader`, `Writer`, `Checker`); ou nome descritivo (`UserRepository`).
- **DTOs**: sufixo `Request`/`Response` (`LoginRequest`, `LoginResponse`).
- **Siglas**: todas maiúsculas ou todas minúsculas, de forma consistente (`HTTPServer` ou `httpServer`, nunca `HttpServer`).

### 4.2. Organização de Arquivos
- Um arquivo por estrutura principal + seus métodos.
- Interfaces pequenas (1-3 métodos), definidas no local de consumo, não no local de implementação.
- Testes no mesmo pacote com sufixo `_test` para testes de caixa-preta, sem sufixo para caixa-branca.
- Arquivos de mock vão em `internal/mock/` ou ao lado do arquivo que define a interface.

### 4.3. Tratamento de Erros
- **NUNCA** usar `panic` em código de produção, exceto em `init()` para configs inválidas irrecuperáveis.
- Sempre retornar `error` como último valor de retorno.
- Sempre verificar erros. Nunca usar `_` para ignorar erros sem justificativa documentada.
- Usar `fmt.Errorf("contexto: %w", err)` para wrapping com contexto.
- Usar `errors.Is()` e `errors.As()` para verificação de tipo, nunca comparar strings de erro.
- Erros de domínio customizados com `var ErrNotFound = errors.New("recurso não encontrado")`.

```go
// Correto
user, err := s.repo.FindByID(ctx, id)
if err != nil {
    if errors.Is(err, ErrNotFound) {
        return nil, fmt.Errorf("usuario %s: %w", id, ErrNotFound)
    }
    return nil, fmt.Errorf("buscar usuario: %w", err)
}
```

### 4.4. Contexto
- Todo método que faz I/O (HTTP, DB, arquivos) deve receber `context.Context` como primeiro parâmetro.
- NUNCA armazenar `context.Context` em structs.
- NUNCA criar contextos com `context.Background()` dentro de handlers; usar `r.Context()`.
- Sempre usar `context.WithTimeout` em chamadas externas.

### 4.5. Concorrência
- Prefira channels para comunicação entre goroutines, mutexes para proteger estado compartilhado.
- Use `sync.WaitGroup` para aguardar goroutines no shutdown.
- Use `errgroup` para execução paralela com cancelamento no primeiro erro.
- Sempre proteja maps acessados por múltiplas goroutines com `sync.RWMutex`.
- Documente quem é dono de cada channel (quem escreve, quem fecha).

```go
// Exemplo: worker com graceful shutdown
func (w *Worker) Run(ctx context.Context) error {
    for {
        select {
        case <-ctx.Done():
            return ctx.Err()
        case job := <-w.jobs:
            if err := w.process(job); err != nil {
                w.logger.Error("process job", "error", err)
            }
        }
    }
}
```

### 4.6. HTTP Handlers
- Handlers devem ser finos: extrair parâmetros, chamar service, escrever resposta.
- Toda lógica de negócio fica no `service/`.
- Usar `chi.Render` ou helpers de encode/decode JSON.
- Sempre definir Content-Type: `application/json; charset=utf-8`.
- Sempre validar entrada com `go-playground/validator/v10`.
- Usar middlewares para cross-cutting concerns (logging, auth, recovery).

```go
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
    var req LoginRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "json inválido", http.StatusBadRequest)
        return
    }
    if err := h.validate.Struct(req); err != nil {
        http.Error(w, err.Error(), http.StatusUnprocessableEntity)
        return
    }
    resp, err := h.authService.Login(r.Context(), req)
    if err != nil {
        http.Error(w, err.Error(), http.StatusUnauthorized)
        return
    }
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(resp)
}
```

### 4.7. SQL e Banco de Dados
- **NUNCA** concatenar strings para montar queries SQL. Usar `$1, $2...` do `pgx` ou queries do `sqlc`.
- Usar `sqlc` para gerar código typesafe a partir de arquivos `.sql`.
- Migrações versionadas com `golang-migrate`, arquivos nomeados `NNNNNN_descricao.up.sql` e `.down.sql`.
- Usar `sql.NullString`, `sql.NullTime` ou ponteiros para campos nullable.
- Sempre usar transações para operações que envolvem múltiplas tabelas.

```sql
-- name: GetTurnoAtivoByUsuario :one
SELECT * FROM turnos
WHERE usuario_id = $1 AND status = 'em_andamento'
LIMIT 1;
```

### 4.8. Testes
- Testes unitários com `testing` package nativo + `testify/assert` para asserts expressivos.
- Mocks gerados com `mockgen` ou `mockery` a partir de interfaces.
- Testes de integração com `testcontainers-go` para PostgreSQL.
- Nome de teste: `Test<NomeFuncao>_<Cenario>_<ResultadoEsperado>`.
- Usar table-driven tests para múltiplos casos.

```go
func TestCalculateDistance_InRange_ReturnsFalse(t *testing.T) {
    tests := []struct {
        name string
        lat1, lon1, lat2, lon2 float64
        maxDist float64
        want    bool
    }{
        {"within 100m", -23.5, -46.6, -23.5001, -46.6001, 100, true},
        {"outside 100m", -23.5, -46.6, -23.51, -46.61, 100, false},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got := isWithinDistance(tt.lat1, tt.lon1, tt.lat2, tt.lon2, tt.maxDist)
            assert.Equal(t, tt.want, got)
        })
    }
}
```

### 4.9. Logging
- Usar `log/slog` (Go 1.21+) ou `zerolog` para logs estruturados em JSON.
- Níveis: Debug, Info, Warn, Error.
- NUNCA logar senhas, tokens JWT completos, ou dados pessoais (LGPD).
- Incluir `request_id` em todo log dentro do contexto de uma requisição HTTP.

### 4.10. Configuração
- Usar variáveis de ambiente com `os.Getenv` ou biblioteca como `envconfig`/`caarlos0/env`.
- Arquivo `.env.example` versionado, `.env` no `.gitignore`.
- Valores sensíveis NUNCA hard-coded (senhas, chaves JWT, strings de conexão).

---

## 5. Comandos Úteis

```bash
# Desenvolvimento com hot reload
air

# Lint
golangci-lint run ./...

# Testes
go test ./... -v -race -coverprofile=coverage.out

# Cobertura
go tool cover -html=coverage.out

# Gerar código sqlc
sqlc generate

# Criar migration
migrate create -ext sql -dir migrations -seq nome_da_migration

# Rodar migrations
migrate -path migrations -database "$DATABASE_URL" up

# Build
go build -o bin/server ./cmd/server

# Docker Compose
docker compose up -d
```

---

## 6. Anti-Padrões (NÃO FAZER)

- Variáveis globais mutáveis.
- `init()` com efeitos colaterais além de registro.
- Receber interfaces, retornar structs concretas (aceitar interfaces, retornar structs).
- Repositórios que retornam entities com campos de infraestrutura (tags ORM).
- Ciclos de importação entre pacotes.
- `time.Sleep` como mecanismo de sincronização.
- Ignorar erros de `json.NewDecoder().Decode()` ou `rows.Scan()`.
- Fazer `defer file.Close()` sem verificar erro de close.
- Usar `string(byteSlice)` para dados que podem não ser UTF-8 válidos.

---

## 7. Referências

- [Effective Go](https://go.dev/doc/effective_go)
- [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments)
- [Standard Go Project Layout](https://github.com/golang-standards/project-layout)
- [Uber Go Style Guide](https://github.com/uber-go/guide)
