# GuardPoint Server — Pendências, Correções e Plano de Testes

> Auditoria contra `PLANNING.md`, commit base `844ecdb`, gerada em 2026-07-03.
> Escopo: backend Go (`internal/`, `cmd/`, `migrations/`).
>
> **Resultado da compilação/execução:**
> - `go build ./...` → OK
> - `go vet ./...` → OK
> - Migrations `000001`→`000013` aplicam sem erro em Postgres 16 (testado em container limpo).
> - Servidor sobe, `/health` e `/ready` respondem, seed cria admin.
> - Smoke E2E via HTTP validou: login, CRUD posto/usuário/escala, biometric register/login,
>   iniciar turno com validação de escala, check-in (padrão / desvio de rota / coação),
>   revogar (gera PIN + finaliza turno), dashboard, estatísticas, RBAC (vigia recebe 403 em `/api/usuarios`).
>
> **Conformidade geral com o PLANNING:** alta. Todas as fases 1–8 e 10 estão presentes e
> funcionais; a Fase 9 (WhatsApp) está intencionalmente suspensa (dispatcher é stub, conforme planejado).
> As pendências abaixo são falhas de lógica, features incompletas, hardening e — sobretudo —
> a **ausência total de testes automatizados**.

---

## Legenda

- **Severidade**: 🔴 Crítica · 🟠 Alta · 🟡 Média · ⚪ Baixa
- **Esforço**: P (horas) · M (~1 dia) · G (multi-dia)
- Evidência sempre em `arquivo:linha`.

---

## A. Correções de Lógica / Bugs

### A1 🟠 Escalas noturnas (que cruzam a meia-noite) são impossíveis — bloqueador de domínio
- **Evidência**:
  - `migrations/000012_create_escalas.up.sql` → `CONSTRAINT ck_escalas_horas CHECK (hora_fim > hora_inicio)`.
  - `internal/service/turno_service.go:106-124` e `internal/service/escala_service.go:167-201` — a
    tolerância é calculada como `time.Parse("15:04", horaAtual).Sub(horaInicio)` no **mesmo dia**.
- **Sintoma confirmado**: `POST /api/escalas` com `hora_inicio=22:00`, `hora_fim=06:00` retorna
  `{"error":"erro ao criar escala"}` (viola a CHECK constraint).
- **Impacto**: para uma plataforma de **vigias**, o turno noturno é o caso de uso principal.
  Hoje é impossível cadastrar uma escala 22h→06h, e mesmo que fosse, a checagem de tolerância
  no início do turno quebraria ao cruzar a meia-noite (diferença HH:MM fica errada).
- **Esforço**: M
- **Correção sugerida**:
  1. Remover/ajustar a constraint `ck_escalas_horas` (nova migration `000014`) para permitir
     `hora_fim <= hora_inicio` sinalizando turno que vira o dia; ou modelar com um flag
     `vira_dia BOOLEAN` / usar `data_fim` para o fim real.
  2. Reescrever o cálculo de tolerância para trabalhar com um `time.Time` completo (data+hora),
     tratando o wrap de meia-noite, em `TurnoService.Iniciar` e `EscalaService.VerificarTolerancia`.
  3. Ajustar `EscalaRepository.FindEscalasSemTurno` (`escala_repository.go:187-205`) que compara
     `hora_inicio + tolerancia <= $1::time` — também quebra no wrap.
- **Testes**: unitário para o cálculo de tolerância com wrap; integração criando escala noturna.

### A2 🟡 `CheckinResponse.Atrasado` é sempre `false`
- **Evidência**: `internal/service/turno_service.go:209-218` (e idêntico em `ProcessarLote`, `620-630`).
  O "último check-in" é buscado **depois** de inserir o check-in atual, ordenando por
  `timestamp_criacao DESC`. Como o check-in recém-criado costuma ser o mais recente,
  `ultimo.ID == checkin.ID`, o `if ultimo != nil && ultimo.ID != checkin.ID` falha e `atrasado`
  nunca é setado.
- **Impacto**: o app Android recebe sempre `atrasado:false` na resposta do check-in. A detecção
  real de atraso ainda ocorre no `TimeoutChecker` (não afeta alertas), mas o campo da resposta é
  enganoso/morto.
- **Esforço**: P
- **Correção**: buscar o **penúltimo** check-in (o último anterior ao atual), por exemplo
  `SELECT ... WHERE turno_id=$1 AND id <> $2 ORDER BY timestamp_criacao DESC LIMIT 1`, ou capturar
  o último check-in **antes** de inserir o novo.
- **Testes**: unitário/integração cobrindo janela deslizante (check-in no minuto 25 → próximo
  deadline; check-in atrasado → `atrasado:true`).

### A3 🟠 PIN de novo dispositivo é gerado mas nunca consumido (feature incompleta — PLANNING §8.5)
- **Evidência**: `internal/service/turno_service.go:443-457` gera o PIN e
  `internal/repository/turno_repository.go:195-205` o persiste (`pin`, `pin_valido_ate`).
  Não há **nenhum** endpoint/fluxo que valide o PIN para autorizar login/turno em novo aparelho
  (busca por `pin` só encontra escrita, nunca leitura).
- **Impacto**: a regra 8.5 ("um PIN temporário é gerado para login em novo dispositivo") está pela
  metade. O supervisor revoga e recebe um PIN que não serve para nada no backend.
- **Observação relacionada**: `RevogarToken` faz `status='finalizado', fim_real=now()`, ou seja,
  **encerra o turno inteiro** em vez de apenas invalidar `token_sessao`. Confirmar se é o
  comportamento desejado (a spec fala em invalidar a sessão, não necessariamente encerrar o turno).
- **Esforço**: M
- **Correção**: definir e implementar o fluxo de resgate por PIN (ex.: `POST /api/turnos/reassociar`
  recebendo `pin` + novo `device_id`, validando `pin_valido_ate`), ou, se o produto não precisa
  disso agora, remover a geração de PIN para não deixar código/coluna órfãos.

### A4 🟡 `token_sessao` nunca é validado no check-in / finalização
- **Evidência**: `TurnoService.Iniciar` gera `token_sessao` (`turno_service.go:131`), mas
  `Checkin`/`Finalizar`/`Sabotagem` só validam que o turno pertence ao usuário do JWT
  (`turno_service.go:176-178` etc.). O `token_sessao`/`device_id` não é reconferido.
- **Impacto**: o "controle de sessão única" (8.6/8.5) na prática se apoia só no JWT de 15 min e no
  encerramento do turno. Um device revogado com JWT ainda válido não é barrado no nível do check-in
  (é barrado indiretamente porque o turno foi finalizado). Aceitável a curto prazo, mas diverge da spec.
- **Esforço**: M
- **Correção**: exigir `device_id` (ou `token_sessao`) no corpo do check-in e validar contra o turno.

### A5 ⚪ `sessoes_dispositivo` sem UNIQUE — re-registro duplica linhas
- **Evidência**: `migrations/000005_create_sessoes_dispositivo.up.sql` não tem UNIQUE em
  `(empresa_id, device_id)`. `SessaoDispositivoRepository.Create` (`:43-53`) sempre faz INSERT;
  `FindByDeviceID` (`:22-41`) faz `QueryRow` (pega uma linha arbitrária se houver várias).
- **Impacto**: registrar biometria duas vezes para o mesmo device cria linhas duplicadas.
- **Esforço**: P
- **Correção**: `UNIQUE (empresa_id, device_id)` + `INSERT ... ON CONFLICT DO UPDATE` (ou upsert no repo).

### A6 ⚪ Alertas imediatos (coação/sabotagem) despacham WhatsApp para **todos** os níveis
- **Evidência**: `internal/service/alerta_service.go:122-131` — `CreateAlertaImediato` percorre
  **todas** as configs de escalonamento e enfileira um `PendingAlert` para cada uma, sem filtrar por nível.
- **Impacto**: com N níveis configurados, uma coação gera N despachos. Pode ser intencional
  (notificar todos numa emergência), mas hoje é implícito e não documentado.
- **Esforço**: P
- **Correção**: decidir a regra (todos vs. nível específico) e torná-la explícita.

---

## B. Segurança

### B1 🟠 Login biométrico concede JWT completo apenas com `empresa_id` + `device_id`
- **Evidência**: `internal/service/auth_service.go:157-188`; rota pública
  `POST /api/auth/biometric/login` (`cmd/server/main.go:118`). Confirmado em teste: com apenas
  `empresa_id` (visível em qualquer resposta de login) e o `device_id` escolhido pelo cliente,
  o servidor retorna um par de tokens válidos — **sem** senha nem prova de biometria.
- **Impacto**: o `device_id` é o único segredo e é definido pelo app sem exigência de entropia.
  Quem obtiver/adivinhar um `device_id` válido de uma empresa autentica como aquele vigia.
  A biometria é validada só no aparelho; o backend não tem prova disso.
- **Esforço**: M
- **Correção sugerida**: tratar `device_id` como segredo de alta entropia gerado/servido no
  registro; associar um `device_secret` retornado apenas uma vez; opcionalmente challenge-response.
  No mínimo, exigir formato/entropia e rate-limit no endpoint.

### B2 🟡 Defaults inseguros de configuração
- **Evidência**: `.env.example` → `JWT_SECRET=change-me-in-production`, `CORS_ORIGIN=*`.
  `internal/middleware/cors.go:8` reflete o valor configurado; default `*`.
  `internal/config/config.go` já exige `JWT_SECRET` não-vazio (bom), mas aceita o placeholder.
- **Impacto**: risco de subir em produção com segredo fraco / CORS aberto.
- **Esforço**: P
- **Correção**: validar em produção (`ENV=production`) que `JWT_SECRET` != placeholder e tem
  comprimento mínimo; recusar `CORS_ORIGIN=*` quando `ENV=production`. Documentar no README.

### B3 ⚪ `/metrics` (Prometheus) exposto sem autenticação
- **Evidência**: `cmd/server/main.go:217` registra `/metrics` fora do grupo autenticado.
- **Impacto**: comum e geralmente aceitável, mas expõe cardinalidade de rotas/latências publicamente.
- **Esforço**: P
- **Correção**: proteger via rede (Railway private) ou basic-auth/token se exposto publicamente.

> **Rejeitado (não é finding):** o `TimeoutChecker` consulta `turnos` sem `empresa_id`
> (`worker/timeout_checker.go:76-81`). É um worker global correto — cada alerta usa `t.EmpresaID`.
> Não há vazamento entre tenants.

---

## C. Dívida Técnica / Limpeza

### C1 ⚪ Diretório espúrio `migrations;C/` na raiz
- **Evidência**: pasta vazia `migrations;C/` no root (não rastreada pelo git). Provável artefato de
  um comando tipo `migrate -path migrations;C:\...` mal-escapado no PowerShell.
- **Correção**: remover a pasta.

### C2 ⚪ Código morto
- **Evidência**: sem uso em todo o repo —
  `EscalaService.ValidarEscala` e `VerificarTolerancia` (`escala_service.go:155-201`, a validação
  real está inline em `TurnoService.Iniciar`), `Hub.BroadcastToAll` (`ws/hub.go:81`),
  `CheckinRepository.FindUltimoTimestampByTurno` (`checkin_repository.go:127`).
- **Correção**: remover ou passar a usar (a lógica de `VerificarTolerancia` deveria ser a fonte
  única — ver A1; hoje há duplicação entre o service e o inline do `Iniciar`).

### C3 ⚪ Lote de check-ins não é transacional
- **Evidência**: `TurnoService.ProcessarLote` (`turno_service.go:544-648`) insere cada check-in em
  auto-commit; falha no meio deixa estado parcial. `TurnoHandler.Lote` chama `Reconcile` por turno
  depois, também fora de transação.
- **Impacto**: baixo em uso normal (idempotência por `cliente_checkin_id` mitiga reenvio), mas sem
  atomicidade por lote.
- **Correção**: opcional — envolver o lote por turno em transação `pgx`.

### C4 ⚪ Duplicação `CreateAlerta` vs `CreateAlertaImediato`
- **Evidência**: `alerta_service.go:51-134` — dois métodos quase idênticos (o segundo pula o dedup
  e não filtra nível). Consolidar com parâmetros.

### C5 ⚪ CI: `golangci-lint` v1.64.8 vs `go 1.25.0`
- **Evidência**: `.github/workflows/ci.yml` fixa `golangci-lint` em `v1.64.8`; `go.mod` usa `go 1.25.0`.
- **Impacto**: versões antigas do golangci-lint podem não suportar a toolchain 1.25 e quebrar o job
  de lint no CI. **Verificar** rodando o lint localmente/CI.
- **Correção**: se falhar, subir para uma versão de golangci-lint compatível com Go 1.25.

---

## D. Plano de Testes (nenhum teste existe hoje — 0 arquivos `*_test.go`)

> Este é o item de maior alavancagem. O `Makefile`/CI já rodam `go test ./... -race`, mas não há
> nenhum teste. Comece pelas **funções puras** (rápidas, sem DB) e depois cubra os fluxos de negócio
> com testes de integração usando um Postgres efêmero.

### D1 — Testes unitários (sem banco) — prioridade máxima, esforço P–M

| Alvo | Arquivo sugerido | O que cobrir |
| --- | --- | --- |
| `haversine` | `internal/service/geofence_test.go` | distância conhecida (ex.: dois pontos a ~1 km), ponto idêntico = 0, dentro vs. fora do `raio_m` → `flag_geofence` `ok`/`desvio_rota`. Extrair `haversine`/`calcularGeofence` se necessário para testabilidade. |
| Janela deslizante / `atrasado` | `internal/service/turno_window_test.go` | check-in dentro do intervalo → não atrasado; check-in após `ultimo + intervalo_min` → atrasado; **cobre a regressão A2**. |
| Tolerância de escala | `internal/service/escala_tolerancia_test.go` | dentro da tolerância, fora da tolerância, e **turno que cruza meia-noite** (cobre A1). |
| JWT | `internal/auth/jwt_test.go` | gerar+validar access token (claims corretas), token expirado → `ErrTokenExpired`, assinatura inválida, método de assinatura trocado (`alg` none/RS256) rejeitado. |
| `generatePIN` | `internal/service/pin_test.go` | comprimento fixo (6 dígitos, zero-padded), só dígitos. |
| RBAC middleware | `internal/handler/rbac_test.go` | role ausente → 401; role não permitido → 403; role permitido → segue. Usar `httptest`. |
| Auth middleware | `internal/handler/middleware_test.go` | sem header → 401; formato inválido → 401; token válido injeta `user_id`/`empresa_id` no contexto. |

### D2 — Testes de integração (Postgres efêmero) — esforço M–G

Infra sugerida: um helper que sobe Postgres via `testcontainers-go` **ou** usa
`DATABASE_URL` de um container de teste, aplica as migrations e trunca tabelas entre casos.
Marcar com build tag `//go:build integration` para não rodar no unit padrão.

Fluxos a cobrir (todos já validados manualmente no smoke E2E — transformar em regressão automatizada):

1. **Auth**: login sucesso/senha errada/usuário inativo; refresh; register (admin) com email duplicado → 409.
2. **Iniciar turno**: sem escala ativa → 403 (`ErrEscalaSemEscala`); fora da tolerância → 403;
   sem dispositivo registrado → 403; turno já ativo → 409; caminho feliz → 201.
3. **Check-in**: padrão dentro do geofence (`flag_geofence=ok`); fora (`desvio_rota`);
   **coação** → turno vira `critico`, alerta `coacao` criado, resposta de sucesso normal (regra 8.2);
   check-in em turno finalizado → 409.
4. **Finalização**: grava check-in `finalizacao`, status → `finalizado`.
5. **Sabotagem**: grava check-in `tipo_senha='sabotagem'`, alerta `sabotagem`, status `critico`.
6. **Lote offline**: idempotência por `cliente_checkin_id` (reenvio não duplica);
   coação em lote preserva `critico`; reconciliação fecha alertas `atraso_*` como `falso_positivo`
   quando os gaps ficam dentro da tolerância (Sync Reconciler).
7. **Revogar**: gera PIN, invalida sessão; (após A3, testar o resgate por PIN).
8. **Alertas**: listar/reconhecer/encerrar; transições de status inválidas; estatísticas.
9. **Multi-tenancy**: usuário da empresa A **não** enxerga turnos/alertas/postos/escalas da empresa B
   (repetir GET `/api/turnos/{id}`, `/api/alertas`, `/api/escalas` com JWT de outra empresa → 404/vazio).
10. **RBAC de rotas**: vigia recebe 403 em `/api/usuarios`, `/api/config/*`; supervisor vs admin.

### D3 — Testes de worker — esforço M

- **TimeoutChecker**: com um turno `em_andamento` cujo último check-in excede `intervalo_min` e
  configs N1/N2/N3, verificar que gera `atraso_n1..n3` conforme o atraso e **não duplica**
  (dedup por `(turno, tipo)`). Testar `check()` diretamente (injetar `db`/repos), sem depender do ticker.
- **No-show**: escala ativa sem turno após `hora_inicio + tolerancia` → alerta `no_show`;
  segundo ciclo **não** regenera (dedup via `[ref:usuario_id]` na mensagem).
- **SyncReconciler**: turno com gaps ≤ tolerância → fecha alertas como `falso_positivo` + emite
  evento `sync_resolved`; com gaps > tolerância → não fecha.

### D4 — Testes de WebSocket — esforço M

- Handshake: sem `token` → 401; token inválido → 401; token válido → upgrade OK.
- `CheckOrigin`: com `CORS_ORIGIN` restrito, origin não listado é rejeitado.
- Broadcast seletivo: cliente da empresa A recebe `gps_update`/`new_alert` da empresa A e **não** os da B.

---

## E. Ordem de execução recomendada

1. **D1** (testes unitários puros) — cria a rede de segurança antes de mexer em lógica. Barato.
2. **A2** (`atrasado`) e **A5** (unique de sessão) — correções pequenas, cobertas pelos testes de D1/D2.
3. **A1** (escalas noturnas) — maior impacto de domínio; exige nova migration + reescrita da
   tolerância; blindar com os testes de tolerância (D1) antes.
4. **A3/A4** (fluxo de PIN e validação de sessão) — decisão de produto primeiro.
5. **B1/B2** (hardening de auth/config) — antes de qualquer deploy real de produção.
6. **D2/D3/D4** (integração/worker/ws) — regressão dos fluxos críticos.
7. **C1–C5** (limpeza) — quando conveniente.

---

## Apêndice — Itens do PLANNING confirmados como OK

- Todos os endpoints das seções 5.1–5.8 do PLANNING existem e estão roteados (`cmd/server/main.go`).
- Multi-tenancy: todas as queries de repositório filtram por `empresa_id` (extraído do JWT no middleware).
- Geofencing Haversine, senha de coação (silenciosa), sabotagem com `tipo_senha='sabotagem'`,
  idempotência de lote (`cliente_checkin_id`), broadcast WS por empresa, dedup de escalonamento,
  health/ready/metrics, Dockerfile multi-stage não-root, CI/CD scaffolding — todos presentes.
- Fase 9 (WhatsApp) permanece stub por decisão de projeto (`worker/alert_dispatcher.go`).
