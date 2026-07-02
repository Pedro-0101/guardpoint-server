# Prompt de Correção — guardpoint-server — Fase 5 (Alertas e Workers)

> **Para o executor:** você trabalha SOMENTE no repositório `guardpoint-server` (Go). Não altere `guardpoint-android` nem `guardpoint-manager`. Você não tem o contexto da conversa que gerou este prompt — tudo que precisa está aqui.

## Contexto e veredicto

A **Fase 5** (ver `PLANNING.md` §9 — "Alertas e Workers") está **funcionalmente implementada e correta**, apesar dos checkboxes `[ ]` no `PLANNING.md` estarem desatualizados. Já existem e funcionam:

- `internal/worker/timeout_checker.go` — varredura a cada 30s, janela deslizante correta (compara `ultimo_checkin.timestamp_criacao + intervalo_min`), gera alertas N1/N2/N3 conforme `config_escalonamento`.
- `internal/worker/alert_dispatcher.go` — consome o channel e faz o stub do WhatsApp (esperado nesta fase).
- CRUD de escalonamento e endpoints de alertas em `internal/handler/alerta.go`, acoplados em `cmd/server/main.go`.
- Deduplicação de alertas via `AlertaRepository.CountByTurnoETipo` (`internal/service/alerta_service.go`), evitando spam a cada ciclo de 30s.

Portanto **este prompt NÃO é uma reescrita** — são 4 correções pontuais de robustez/contrato. Faça-as como commits pequenos e independentes.

## Correções

### C1 (P1) — `POST /api/turnos/{id}/revogar` deve gerar e retornar o PIN

O contrato (`AGENTS.md` §7.4 e `PLANNING.md` §5.3/§8.5) diz que revogar a sessão deve **liberar um PIN temporário** para login em novo dispositivo. Hoje o service apenas invalida o token e o handler responde `{ "message": "..." }`, sem PIN:

`internal/service/turno_service.go`
```go
func (s *TurnoService) Revogar(ctx context.Context, empresaID, turnoID string) error {
    ...
    return s.turnoRepo.RevogarToken(ctx, parsedTurnoID, parsedEmpresaID)
}
```
`internal/handler/turno.go`
```go
writeJSON(w, http.StatusOK, map[string]string{"message": "turno revogado com sucesso"})
```

**Fazer:**
- Gerar um PIN numérico curto (ex.: 6 dígitos) com validade (ex.: 15 min), persistido de forma que o fluxo de login/registro de novo dispositivo possa validá-lo. Reutilize a tabela `sessoes_dispositivo` ou adicione um campo/tabela de PIN — **inspecione `internal/repository/sessao_dispositivo_repository.go` e as migrations existentes antes de decidir**; siga o padrão de migrations numeradas (`migrations/0000NN_*.up.sql` + `.down.sql`).
- `Revogar` passa a retornar `(pin string, validadeMinutos int, err error)` (ou uma struct de resultado).
- O handler responde conforme o contrato: `{ "pin_novo_dispositivo": "123456", "validade_minutos": 15 }`.
- Se o PIN exigir mudança no fluxo de `iniciar turno`/login para aceitá-lo, e isso extrapolar o esforço razoável, **implemente a geração/retorno do PIN agora e registre um TODO** para o consumo do PIB no login — mas não deixe o endpoint mentindo sobre o contrato.

> Nota: o `guardpoint-manager` depende deste retorno (ver `plans/002-manager-contrato-turnos.md`). Não altere o Manager.

### C2 (P2) — RBAC nos endpoints de configuração e alertas

Em `cmd/server/main.go`, os grupos `/config` e `/alertas` estão apenas atrás de `AuthMiddleware`, sem restrição de papel — qualquer usuário autenticado (inclusive `vigia`) pode ler/editar níveis de escalonamento e encerrar alertas:

```go
r.Route("/alertas", func(r chi.Router) { ... })      // sem RequireRole
r.Route("/config", func(r chi.Router) { ... })       // sem RequireRole
```

Compare com `/usuarios`, que já faz `r.Use(handler.RequireRole("admin"))`.

**Fazer:**
- `/config/escalonamento` (GET/PUT/POST/PUT{id}/DELETE): restringir a `admin` (é configuração da empresa).
- `/alertas`: restringir leitura/ação a `admin` e `supervisor` (RBAC do domínio — supervisores operam alertas). Verifique se `RequireRole` aceita múltiplos papéis; se não, ajuste o middleware ou aplique a checagem adequada. **Não** exponha alertas a `vigia`.

### C3 (P2) — `PUT /api/config/escalonamento` (bulk) não remove níveis excluídos

`internal/handler/alerta.go` → `PutEscalonamento` itera o array e faz upsert de cada nível, mas **nunca apaga** níveis que sumiram do payload. A UI de escalonamento (que envia o conjunto completo) espera semântica de "substituir o conjunto": se o admin remove o N3, o backend mantém o N3 antigo.

**Fazer:**
- Tornar o `PUT` idempotente sobre o conjunto: dentro de uma transação, **substituir** os níveis da empresa pelo array recebido (ex.: `DELETE FROM config_escalonamento WHERE empresa_id = $1` seguido de inserts, ou reconciliar diff). Use `pgx` transaction. Garanta isolamento por `empresa_id`.
- Mantenha a validação existente por item.

### C4 (P3) — Dedup de alerta ignora o status

`AlertaService.CreateAlerta` chama `CountByTurnoETipo(ctx, turnoID, tipo)`; confira em `internal/repository/alerta_repository.go` que a query conta **todos** os alertas daquele `(turno, tipo)` independente de `status`. Efeito: depois que um alerta `atraso_n1` é `encerrado`, um novo atraso do mesmo nível no mesmo turno **nunca** é recriado.

**Fazer (avalie a regra de negócio antes):**
- Se a intenção é "um alerta aberto por (turno, tipo) por vez", altere o count para considerar apenas `status IN ('aberto','reconhecido')`. Assim, após encerrar, um novo atraso pode reabrir alerta.
- Se a intenção é "um único alerta por turno+tipo para sempre", documente isso e deixe como está.
- Escolha e justifique no PR. Baixa prioridade — só faça se C1–C3 estiverem prontos.

## Escopo

- **Em escopo:** `internal/handler/{alerta,turno}.go`, `internal/service/{alerta_service,turno_service}.go`, `internal/repository/{alerta_repository,config_escalonamento_repository,sessao_dispositivo_repository}.go`, `cmd/server/main.go` (rotas/RBAC), novas migrations se C1 exigir.
- **Fora de escopo:** não implemente a integração real do WhatsApp (Fase 9 — o stub do dispatcher é o esperado agora). Não mexa em geofencing/offline/websocket. Não altere Manager nem Android.

## Verificação

1. `cd guardpoint-server && go build ./...` — compila.
2. `gofmt -l .` — vazio (sem arquivos mal formatados).
3. `golangci-lint run ./...` — sem novos problemas.
4. `go test ./... -race` — passa. **Adicione testes** para:
   - `PutEscalonamento`: enviar `[N1,N2,N3]`, depois `[N1,N2]` → o repositório deve conter apenas N1,N2 (cobre C3).
   - `Revogar`: retorna PIN não vazio e com validade (cobre C1).
   - Siga o estilo dos testes existentes no repo (procure `*_test.go`; se não houver, crie seguindo as convenções de `PLANNING.md` §11 e use um Postgres de teste ou mocks conforme o padrão já adotado).
5. Se possível, teste de fumaça com `docker compose up -d` + `air`:
   - Como `vigia`, `PUT /api/config/escalonamento` deve retornar 403 (cobre C2).
   - `POST /api/turnos/{id}/revogar` retorna `{ pin_novo_dispositivo, validade_minutos }` (cobre C1).

## Após concluir

Atualize os checkboxes da **Fase 5** em `guardpoint-server/PLANNING.md` de `[ ]` para `[x]` (Timeout Checker, CRUD escalonamento, Alert Dispatcher stub, endpoints de alertas), já que estavam desatualizados — refletindo o estado real do código.

## Escape hatch

Se ao inspecionar o repositório de sessão/migrations você concluir que gerar o PIN (C1) exige mudança estrutural grande no fluxo de autenticação/registro de dispositivo, **pare após C2–C4 e reporte C1** com uma proposta de migration/fluxo, em vez de improvisar um esquema de PIN frágil.
