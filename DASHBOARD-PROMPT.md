# Prompt para IA do Frontend — Tela Visão Geral do Dashboard

> **Contexto**: GuardPoint Manager — Angular 21 + Tailwind CSS v4 + Zard UI (shadcn-style) + Chart.js + Leaflet
>
> **Tarefa**: Refatorar a tela atual de Dashboard (`src/app/features/dashboard/`) para uma **Visão Geral Operacional** completa, substituindo o layout atual por um novo design que integre KPIs expandidos, mapa interativo, status de cobertura de postos e feed de eventos em tempo real.

---

## 1. REQUISITOS FUNCIONAIS — O QUE DEVE SER EXIBIDO

A tela deve exibir os seguintes blocos de informação, todos escopados à empresa do usuário logado (multi-tenant, `empresa_id` extraído do JWT):

### BLOCO A — KPIs (8 cards, 2 fileiras de 4 em desktop)

| Ref | Card | Ícone (Lucide) | Cor | Descrição |
|-----|------|-----------------|-----|-----------|
| **A1** | **Turnos Ativos** | `lucideUsers` | `primary` | Total de turnos `em_andamento` + `pausado` + `critico` |
| **A3** | **Turnos Críticos** | `lucideShieldAlert` | `destructive` (novo) | Total de turnos com `status = 'critico'` (sabotagem/coação ativa). Deve ter **destaque visual forte** (ex: borda vermelha pulsando, badge com ícone de alerta) |
| **A11** | **Check-ins (1h)** | `lucideCheckCircle` | `success` (novo) | Total de checkins nos últimos 60 minutos |
| **B1** | **Alertas Abertos** | `lucideTriangleAlert` | `warn` | Total de alertas com `status = 'aberto'` |
| **NOVO** | **Turnos em Atraso** | `lucideClock` | `warning` (novo) | Turnos `em_andamento` cujo último checkin excedeu o intervalo configurado |
| **NOVO** | **Postos Cobertos** (A4) | `lucideBuilding2` | `info` | Formato: `"8 / 12"` (cobertos / total de postos ativos). Abaixo: barra de progresso `%` |
| **NOVO** | **No-Shows Hoje** | `lucideUserX` | `destructive` (novo) | Turnos agendados para hoje que nunca foram iniciados (passaram do `inicio_previsto + tolerancia` sem `inicio_real`) |
| **NOVO** | **Desvios 24h** | `lucideGitBranchPlus` | `warning` (novo) | Checkins com `flag_geofence = 'desvio_rota'` nas últimas 24h |

**Opcional**: Cada card pode mostrar um **trend indicator** (↑ 12% vs ontem) se o backend fornecer — mas isso é secundário. O essencial agora é o valor numérico.

### BLOCO B — Mapa de Turnos Ativos (C1)

**O que é**: Um mapa Leaflet embutido no dashboard (não em página separada), mostrando:

- **Pins dos vigias** com a mesma lógica de cores do `MapaComponent` existente (verde=normal, amarelo=desvio, vermelho=crítico, cinza=offline, azul=posto)
- **Círculos dos postos** (geofence) com tooltip de nome do posto e popup com detalhes
- **Clique no pin** → popup com nome do vigia, posto, status, último checkin, coordenadas + botão "Ver detalhes"
- **Atualização em tempo real** via WebSocket (`gps_update`, `status_change`)

**Requisitos de layout**:
- O mapa deve ocupar **50-60% da largura** em desktop (coluna esquerda de um grid 2-colunas)
- Altura fixa de **400-500px** com `border-radius: var(--radius-lg)` (8px)
- Deve ter uma **legenda sobreposta** flutuante (canto inferior esquerdo, estilo vidro/frosted glass)
- **Estado vazio**: se não houver turnos ativos, mostrar `EmptyState` centralizado no lugar do mapa

### BLOCO C — Status de Cobertura de Postos (A4 + A5)

**O que é**: Um painel que mostra visualmente quais postos estão cobertos e quais não estão.

**Layout**:
- Ocupa a **coluna direita** do grid junto com o feed de eventos (ver abaixo)
- Dividido em duas seções verticais:

**Seção 1 — Barra de Cobertura** (A4):
- Gráfico de rosca (donut) do Chart.js mostrando: `postos cobertos (verde)` vs `postos descobertos (cinza claro)`
- No centro do donut: número grande com `"8/12"` e abaixo `"67%"` em fonte menor
- Tooltip ao passar o mouse

**Seção 2 — Lista de Postos Descobertos** (A5):
- Título: "Postos sem cobertura" com badge de contagem
- Lista scrollável (max-height ~200px) com cada item mostrando:
  - Nome do posto
  - Endereço/coordenadas resumidas
  - Ícone de alerta `lucideAlertCircle` em muted-foreground
- **Estado vazio**: "Todos os postos estão cobertos" com ícone `lucideCheckCircle` verde

### BLOCO D — Feed de Últimos Eventos

**O que é**: Uma timeline ao vivo dos últimos acontecimentos da operação.

**Layout**:
- Abaixo do bloco de cobertura (C), na mesma coluna direita
- Título: "Últimos Eventos" com badge "AO VIVO" pulsando

**Cada item do feed** mostra:
- Ícone do tipo de evento (colorido):
  - Início de turno: `lucidePlay` (verde)
  - Fim de turno: `lucideStopCircle` (cinza)
  - Check-in: `lucideMapPin` (azul)
  - Alerta aberto: `lucideBellRing` (vermelho)
  - Alerta reconhecido: `lucideBellOff` (amarelo)
  - Sabotagem: `lucideShieldAlert` (vermelho escuro)
- Texto descritivo (ex: "João Silva iniciou turno na Portaria Principal")
- Tempo relativo (ex: "há 2 min", "há 15 min")
- Link clicável para o detalhe (turno, alerta, etc.)
- Máximo de **15 itens**, com scroll

---

## 2. REQUISITOS DE BACKEND — ENDPOINT NECESSÁRIO

O endpoint atual `GET /api/v1/dashboard/summary` precisa ser **enriquecido** com os seguintes campos NOVOS no JSON de resposta:

```json
{
  "turnos_ativos": 12,
  "turnos_criticos": 1,
  "turnos_atrasados": 3,
  "alertas_abertos": 5,
  "checkins_ultima_hora": 48,
  "desvios_rota": 2,
  "no_shows_hoje": 1,
  "postos_cobertos": 8,
  "postos_total": 12,
  "postos_sem_cobertura": [
    { "posto_id": "...", "posto_nome": "Estacionamento" },
    { "posto_id": "...", "posto_nome": "Galpão 3" }
  ],
  "alertas_recentes": [...],
  "turnos_por_posto": [...],
  "feed_eventos": [
    {
      "tipo": "inicio_turno",
      "usuario_nome": "João Silva",
      "posto_nome": "Portaria Principal",
      "turno_id": "...",
      "timestamp": "2025-01-01T08:00:00Z"
    }
  ]
}
```

**Nota para o backend**: O feed de eventos pode ser implementado via uma **query UNION ALL** entre as tabelas `turnos` (início/fim com `inicio_real`/`fim_real`), `checkins` e `alertas`, ordenado por `timestamp DESC LIMIT 15`. Alternativamente, pode ser substituído por WebSocket se a complexidade for alta — nesse caso, o frontend monta o feed localmente acumulando eventos recebidos via WS.

---

## 3. REQUISITOS DE FRONTEND

### 3.1. Stack e Padrões

| Item | Obrigatório |
|------|-------------|
| Framework | Angular 21 (standalone components) |
| Estilização | Tailwind CSS v4 + SCSS (BEM para nomes de classe específicos) |
| Componentes | Zard UI (`src/app/shared/components/`) — usar `gp-kpi-card` existente, `z-skeleton`, `z-card`, `z-badge` |
| Ícones | `@ng-icons/lucide` (Lucide) — usar os ícones especificados em cada card/bloco |
| Gráficos | Chart.js v4.5.1 com `Chart.register(...registerables)` — **não** usar outra lib |
| Mapa | Leaflet (`import * as L from 'leaflet'`) — **reutilizar** a lógica de `MapaComponent` |
| Estado | RxJS (`BehaviorSubject` + `AsyncPipe`) — **mesmo padrão** do `DashboardService` existente |
| Change Detection | `OnPush` |
| DTO Mapping | Interface `XxxDto` com snake_case → função `mapXxxFromDto()` → camelCase |
| WebSocket | Injetar `WebSocketService`, eventos: `new_alert`, `status_change`, `gps_update` |
| Responsivo | Breakpoints: 1200px (2-col KPIs, grid 1-col), 640px (1-col KPIs) |

### 3.2. Design Tokens (NÃO INVENTAR — USAR OS EXISTENTES)

| Token | Uso |
|-------|-----|
| `var(--primary)` / `var(--primary-foreground)` | Cor principal, cards "primary" |
| `var(--destructive)` / `var(--destructive-foreground)` | Cards críticos (turnos críticos, no-shows) |
| `var(--warning)` / `var(--warning-foreground)` | Cards de aviso (atrasos, desvios) |
| `var(--success)` | Cards positivos (check-ins, cobertura ok) |
| `var(--info)` | Cards informativos (postos cobertos) |
| `var(--background)` / `var(--card)` / `var(--border)` | Fundos e bordas |
| `var(--muted)` / `var(--muted-foreground)` | Textos secundários |
| `var(--radius-sm/md/lg/xl)` | Border radius (4px, 6px, 8px, 12px) |
| `var(--transition-fast/normal/slow)` | Transições (150ms, 250ms, 350ms) |
| Font: Geist (`font-family: 'Geist', -apple-system, ...`) | Tipografia padrão |
| `--header-height: 64px` | Altura da navbar |

### 3.3. Cores dos Cards KPI (novas variantes necessárias)

O `KpiCard` atual aceita `color: 'primary' | 'accent' | 'warn' | 'info'`. **Precisamos adicionar** as variantes:

- **`destructive`**: usa `var(--destructive)` com fundo `color-mix(in oklch, var(--destructive), transparent 88%)`
- **`success`**: usa `var(--success)` com fundo `color-mix(in oklch, var(--success), transparent 88%)`
- **`warning`**: usa `var(--warning)` com fundo `color-mix(in oklch, var(--warning), transparent 88%)`

Ou, alternativamente, refatorar o `KpiCard` para aceitar qualquer cor CSS customizada via input `[colorVar]` e `[bgMixPercent]`.

### 3.4. Estrutura de Arquivos Esperada

```
src/app/features/dashboard/
├── dashboard.component.ts          ← Refatorar (ver seção 4)
├── dashboard.component.html        ← Substituir completamente
├── dashboard.component.scss        ← Substituir completamente
├── dashboard.service.ts            ← Refatorar (novo modelo de dados)
├── dashboard.types.ts              ← Expandir com novos tipos
├── components/
│   ├── kpi-card/
│   │   ├── kpi-card.ts             ← Refatorar (novas variantes de cor)
│   │   ├── kpi-card.html           ← Atualizar
│   │   └── kpi-card.scss           ← Atualizar
│   ├── alertas-recentes/
│   │   └── ...                     ← MANTER (inalterado)
│   ├── turnos-resumo/
│   │   └── ...                     ← MANTER (inalterado)
│   ├── dashboard-mapa/             ← NOVO
│   │   ├── dashboard-mapa.ts
│   │   ├── dashboard-mapa.html
│   │   └── dashboard-mapa.scss
│   ├── cobertura-postos/           ← NOVO
│   │   ├── cobertura-postos.ts
│   │   ├── cobertura-postos.html
│   │   └── cobertura-postos.scss
│   └── feed-eventos/               ← NOVO
│       ├── feed-eventos.ts
│       ├── feed-eventos.html
│       └── feed-eventos.scss
```

---

## 4. LAYOUT DA TELA (WIREFRAME DESCRITIVO)

```
┌──────────────────────────────────────────────────────────────────┐
│  [Navbar - 64px]                                                 │
├──────────────────────────────────────────────────────────────────┤
│                                                         32px px │
│  ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌──────────┐              │
│  │ A1:     │ │ A3:     │ │ NOVO:   │ │ NOVO:    │              │
│  │ Turnos  │ │ Turnos  │ │ Em      │ │ Postos   │              │
│  │ Ativos  │ │ Críticos│ │ Atraso  │ │ Cobertos │              │
│  │   12    │ │   1 ⚠  │ │   3     │ │  8/12    │              │
│  └─────────┘ └─────────┘ └─────────┘ └──────────┘              │
│  ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌──────────┐              │
│  │ A11:    │ │ B1:     │ │ NOVO:   │ │ NOVO:    │              │
│  │ Checkins│ │ Alertas │ │ No-Shows│ │ Desvios  │              │
│  │  (1h)   │ │ Abertos │ │  Hoje   │ │  24h     │              │
│  │   48    │ │   5     │ │   1     │ │   2      │              │
│  └─────────┘ └─────────┘ └─────────┘ └──────────┘              │
│                                                         24px px │
│  ┌────────────────────────────┐ ┌─────────────────────┐        │
│  │                            │ │ Cobertura de Postos │        │
│  │                            │ │ ┌───────┐           │        │
│  │    MAPA (C1)               │ │ │ Donut │  8/12     │        │
│  │    Leaflet                 │ │ │ Chart │  67%      │        │
│  │                            │ │ └───────┘           │        │
│  │  ┌──────────────────┐      │ │                     │        │
│  │  │ Legenda          │      │ │ Postos sem cobertura│        │
│  │  │ 🟢 Normal        │      │ │ ⚠ Estacionamento   │        │
│  │  │ 🟡 Desvio        │      │ │ ⚠ Galpão 3         │        │
│  │  │ 🔴 Crítico       │      │ │ ⚠ Depósito         │        │
│  │  │ ⚪ Offline       │      │ │                     │        │
│  │  └──────────────────┘      │ ├─────────────────────┤        │
│  │                            │ │ Últimos Eventos     │        │
│  │                            │ │ ● João Silva iniciou│        │
│  │                            │ │   Portaria (2 min)  │        │
│  │                            │ │ ▲ Alerta atraso     │        │
│  │                            │ │   Maria - Galpão    │        │
│  └────────────────────────────┘ └─────────────────────┘        │
│                                                         32px px │
└──────────────────────────────────────────────────────────────────┘
```

**Regras de grid CSS**:
- KPIs: `grid-template-columns: repeat(4, 1fr)` desktop, `repeat(2, 1fr)` tablet, `1fr` mobile
- Área principal: `grid-template-columns: 1.2fr 0.8fr` (mapa maior, painel menor)
- Gap: 24px entre seções principais, 16px entre KPIs
- Max-width: 1440px, margin: 0 auto

---

## 5. REGRAS DE IMPLEMENTAÇÃO (NÃO QUEBRAR)

### 5.1. O que NÃO fazer
- ❌ NÃO criar novos componentes sem seguir o padrão shadcn/Zard UI (inputs via `input()`, sem `@Input` decorator)
- ❌ NÃO usar outra biblioteca de gráficos (apenas Chart.js) ou outro mapa (apenas Leaflet)
- ❌ NÃO usar Angular Material, PrimeNG, ou qualquer lib de UI externa
- ❌ NÃO hardcodar cores — sempre usar `var(--token)` ou Tailwind `bg-destructive`, `text-success`, etc.
- ❌ NÃO remover o `OnPush`, `AsyncPipe` ou `destroy$` pattern
- ❌ NÃO usar `any` nos tipos TypeScript
- ❌ NÃO criar endpoints fictícios — usar **apenas** o que o backend já provê (listado na seção 2) + WebSocket existente
- ❌ NÃO quebrar o tema escuro — testar ambas as variantes (`.dark` class no `<html>`)

### 5.2. O que DEVE fazer
- ✅ Reutilizar `gp-kpi-card` existente (estendendo-o com novas cores)
- ✅ Extrair a lógica de Leaflet do `MapaComponent` para um **service compartilhado** (`leaflet.service.ts` em `src/app/shared/services/`) ou duplicar no `dashboard-mapa`. Preferível: service compartilhado para ambos `MapaComponent` e `DashboardMapaComponent`
- ✅ Reutilizar `gp-alertas-recentes` e `gp-turnos-resumo` (ou removê-los se achar melhor — decida)
- ✅ Manter `WebSocketService` com `onEvent()` para atualizações em tempo real
- ✅ Tratar loading state com `z-skeleton` do Zard UI
- ✅ Tratar error state com mensagem + botão "Tentar novamente" (padrão atual)
- ✅ Tratar empty states com `EmptyState` (padrão do Zard UI)
- ✅ Manter `debounceTime(3000)` nas atualizações WebSocket
- ✅ Testar responsividade nos 3 breakpoints

### 5.3. Lógica da Barra de Cobertura (A4)
- `postos_cobertos` = postos com pelo menos 1 turno `em_andamento` ou `critico`
- `postos_total` = total de postos ativos (`ativo = true`)
- `%` = `(postos_cobertos / postos_total) * 100`
- Se total = 0 → exibir "Nenhum posto cadastrado"

### 5.4. Lógica do Mapa (C1)
- Reutilizar **exatamente** as mesmas cores de pin e lógica de `MapaComponent.determinarCor()`
- Reutilizar a mesma lógica de criação de `DivIcon` com `mapa-pin` CSS
- O mapa deve ser **menor e mais contido** que o mapa full-page
- Height fixo: 420px (vs o mapa full-page que ocupa a tela quase toda)
- Legendas: manter as mesmas 5 + legenda do posto

---

## 6. TIPOS TypeScript ESPERADOS

Expandir `dashboard.types.ts`:

```typescript
// ... manter tipos existentes ...

export interface DashboardKpis {
  turnosAtivos: number;
  turnosCriticos: number;
  turnosAtrasados: number;
  alertasAbertos: number;
  checkinsUltimaHora: number;
  desviosRota: number;
  noShowsHoje: number;
  postosCobertos: number;
  postosTotal: number;
}

export interface PostoSemCobertura {
  postoId: string;
  postoNome: string;
}

export interface FeedEvento {
  tipo: 'inicio_turno' | 'fim_turno' | 'checkin' | 'alerta_aberto'
    | 'alerta_reconhecido' | 'sabotagem';
  usuarioNome: string;
  postoNome: string;
  turnoId: string;
  timestamp: string;
}

export interface DashboardSummary {
  kpis: DashboardKpis;
  postosSemCobertura: PostoSemCobertura[];
  alertasRecentes: Alerta[];
  turnosPorPosto: TurnoPorPosto[];
  feedEventos: FeedEvento[];
}

// DTOs (snake_case)
export interface DashboardSummaryDto {
  turnos_ativos: number;
  turnos_criticos: number;
  turnos_atrasados: number;
  alertas_abertos: number;
  checkins_ultima_hora: number;
  desvios_rota: number;
  no_shows_hoje: number;
  postos_cobertos: number;
  postos_total: number;
  postos_sem_cobertura: Array<{ posto_id: string; posto_nome: string }>;
  alertas_recentes: AlertaRecenteDto[];
  turnos_por_posto: TurnoPorPostoDto[];
  feed_eventos: Array<{
    tipo: string;
    usuario_nome: string;
    posto_nome: string;
    turno_id: string;
    timestamp: string;
  }>;
}
```

---

## 7. VERIFICAÇÃO FINAL

Antes de dar a tarefa como concluída, verificar:

1. `npm run lint` passa sem erros
2. `npm run build` compila sem warnings
3. Tema claro e escuro funcionam (toggle no navbar)
4. Responsivo nos 3 breakpoints (1440px+, 1200px, 640px)
5. Loading skeletons aparecem durante a carga inicial
6. Estado de erro mostra mensagem + botão retry
7. Estados vazios não quebram o layout
8. WebSocket atualiza KPIs e mapa em tempo real
9. Mapa renderiza pins corretamente com as 5 cores
10. Nenhum console.error ou console.log esquecido
