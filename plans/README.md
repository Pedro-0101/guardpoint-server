# Plans — GuardPoint Server

Índice de pendências e planos de melhoria. Gerado por auditoria contra `PLANNING.md` (commit `844ecdb`, 2026-07-03).

## Documentos

- [PENDENCIAS.md](PENDENCIAS.md) — **arquivo principal**: lista consolidada de correções de lógica,
  segurança, dívida técnica e o plano de testes (unitários, integração, workers, WebSocket) que ainda
  não existem no repositório.

## Resumo do estado

| Área | Situação |
| --- | --- |
| Build / vet | ✅ `go build ./...` e `go vet ./...` passam |
| Migrations | ✅ `000001`→`000013` aplicam em Postgres 16 limpo |
| Execução / smoke E2E | ✅ login, turnos, check-in, alertas, RBAC, dashboard OK |
| Conformidade com PLANNING | ✅ Alta — fases 1–8 e 10 completas; fase 9 (WhatsApp) suspensa por decisão |
| Testes automatizados | ❌ **Zero** arquivos `*_test.go` |

## Pendências por severidade

- 🟠 Alta: A1 (escalas noturnas impossíveis), A3 (PIN gerado mas nunca consumido), B1 (login biométrico só com device_id)
- 🟡 Média: A2 (`atrasado` sempre false), A4 (token_sessao não validado no check-in), B2 (defaults inseguros)
- ⚪ Baixa: A5, A6, B3, C1–C5

Detalhes, evidências (`arquivo:linha`) e correções sugeridas em [PENDENCIAS.md](PENDENCIAS.md).
