# guardpoint-server

API REST em Go do GuardPoint (monitoramento de vigias em postos/obras). Consumida pelo painel Angular (supervisores/gestores) e pelo app Android (vigias).

## Subir localmente (sem Go instalado)

```bash
docker compose up --build
```

Sobe Postgres, aplica as migrations e inicia a API em `http://localhost:8080` com `ENV=development`, que cria automaticamente o seed de desenvolvimento:

- **Empresa**: Empresa Demo (CNPJ `00000000000191`)
- **Admin**: `admin@guardpoint.com` / `admin123`

CORS liberado para `http://localhost:4200` (Angular dev server) — ajuste `CORS_ORIGINS` no `docker-compose.yml` se necessário.

## URLs

| URL | Descrição |
| --- | --- |
| `http://localhost:8080/api/v1` | Base da API REST (todas as rotas de negócio) |
| `http://localhost:8080/swagger/index.html` | Swagger UI (desabilitado em produção) |
| `ws://localhost:8080/ws?token=<JWT>` | WebSocket de eventos em tempo real — ver [docs/websocket.md](docs/websocket.md) |
| `http://localhost:8080/health` / `/ready` | Liveness / readiness |
| `http://localhost:8080/metrics` | Prometheus (opcionalmente protegido por `METRICS_TOKEN`) |

Login de exemplo:

```bash
curl -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"admin@guardpoint.com","senha":"admin123"}'
```

## Desenvolvimento com Go

```bash
make db-up        # só o Postgres via compose
make migrate-up   # aplica migrations (golang-migrate)
make run          # roda o servidor (carrega .env — copie de .env.example)
make test         # testes unitários (-race + cobertura)
make test-db-up   # Postgres efêmero de teste na porta 5433
make test-integration
make docs         # regenera docs/ do swagger (swag init) — verificado no CI
make lint         # golangci-lint
```

## Configuração (env)

| Variável | Default | Notas |
| --- | --- | --- |
| `ENV` | `development` | `production` ativa validações estritas e desliga o seed/swagger |
| `PORT` | `8080` | |
| `DATABASE_URL` | — | obrigatória |
| `JWT_SECRET` | — | obrigatória; em produção exige 32+ caracteres e recusa o placeholder |
| `CORS_ORIGINS` | `*` | lista separada por vírgula; em produção recusa `*` |
| `LOG_LEVEL` / `LOG_FORMAT` | `info` / `text` | `json` recomendado em produção |
| `METRICS_TOKEN` | vazio | se definido, `/metrics` exige `Authorization: Bearer <token>` |

Mais contexto de arquitetura e fases em [PLANNING.md](PLANNING.md); convenções para agentes em [AGENTS.md](AGENTS.md).
