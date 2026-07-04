# WebSocket — eventos em tempo real

O Swagger não cobre WebSocket; este documento é o contrato dos eventos para o painel Angular.

## Conexão

```
GET /ws?token=<access_token_JWT>
```

- O token é o mesmo access token do login (`POST /api/v1/auth/login`), passado por query string. Token ausente/inválido → HTTP 401 antes do upgrade.
- O escopo é extraído do JWT: o cliente só recebe eventos da **sua empresa** (broadcast seletivo por `empresa_id`).
- Navegadores enviam `Origin`, validado contra `CORS_ORIGINS`; clientes sem `Origin` (app Android) são aceitos.
- O canal é **somente servidor → cliente**; mensagens enviadas pelo cliente são descartadas. O servidor faz ping a cada ~54s; responda com pong (automático nos browsers) ou a conexão cai em 60s.
- O access token expira em 15 min, mas a conexão já estabelecida **não** é derrubada na expiração — reconecte com um token novo (do refresh) quando a conexão cair.

## Envelope

Todo evento é um JSON:

```json
{ "type": "<tipo>", "payload": { ... } }
```

## Eventos

### `gps_update` — posição registrada em um check-in

Emitido a cada check-in aceito (padrão, coação, finalização, sabotagem e itens de lote).

```json
{
  "type": "gps_update",
  "payload": {
    "turno_id": "uuid",
    "latitude": -23.5505,
    "longitude": -46.6333,
    "timestamp": "2026-07-04T12:30:00Z",
    "flag_geofence": "ok"
  }
}
```

`flag_geofence`: `"ok"` | `"desvio_rota"` | `null`. `"desvio_rota"` = fora do raio do posto (pin amarelo no mapa; o check-in é aceito mesmo assim).

### `status_change` — mudança de status do turno

```json
{
  "type": "status_change",
  "payload": {
    "turno_id": "uuid",
    "status": "critico",
    "timestamp": "2026-07-04T12:30:00Z"
  }
}
```

`status`: `em_andamento` (turno iniciado), `critico` (coação ou sabotagem — tratar como emergência), `finalizado`.

### `new_alert` — novo alerta gerado

Emitido pelo motor de escalonamento (atrasos `atraso_n1..n3`, `no_show`) e pelos fluxos imediatos (`coacao`, `sabotagem`).

```json
{
  "type": "new_alert",
  "payload": {
    "alerta_id": "uuid",
    "tipo": "atraso_n1",
    "turno_id": "uuid",
    "nivel": 1
  }
}
```

Detalhes do alerta (mensagem, status) via `GET /api/v1/alertas`.

### `sync_resolved` — reconciliação offline concluída

Emitido após um lote offline (`POST /api/v1/checkins/lote`) provar que o vigia estava OK: os alertas de atraso do turno foram encerrados como falso positivo.

```json
{
  "type": "sync_resolved",
  "payload": {
    "turno_id": "uuid",
    "resolvido": true,
    "motivo": "falha_infra"
  }
}
```

## Fonte

Tipos e payloads definidos em [`internal/ws/event.go`](../internal/ws/event.go); emissões em `internal/service/turno_service.go`, `internal/service/alerta_service.go` e `internal/worker/sync_reconciler.go`.
