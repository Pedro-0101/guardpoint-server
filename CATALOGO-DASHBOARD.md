# Catálogo de Opções para Dashboard Gerencial — GuardPoint

> Documento de referência para planejamento de dashboards. Cada item lista: o quê mostrar, como visualizar, e o endpoint sugerido.
>
> Itens marcados com ⭐ são os selecionados para a **Tela 1 — Visão Geral** (implementação atual).

---

## BLOCO A — VISÃO GERAL OPERACIONAL (TEMPO REAL)

| # | ⭐ | Informação | Visualização | Endpoint Sugerido |
|---|---|---|---|---|
| A1 | ⭐ | **Turnos ativos agora** (em_andamento + pausado + critico) | Card numérico grande (KPI) | `GET /dashboard/summary` (campo `turnos_ativos`) |
| A2 | | **Turnos em atraso agora** | Card com badge vermelho | `GET /dashboard/summary` (campo `turnos_atrasados`) |
| A3 | ⭐ | **Turnos críticos agora** (sabotagem, coação ativa) | Card com destaque visual (borda pulsante) | `GET /dashboard/summary` (campo `turnos_criticos`) |
| A4 | ⭐ | **% de postos cobertos** (cobertos / total ativos) | Gráfico de rosca (donut) com % | `GET /dashboard/summary` (campos `postos_cobertos`, `postos_total`) |
| A5 | ⭐ | **Postos sem cobertura agora** | Lista com nome do posto + ícone alerta | `GET /dashboard/summary` (array `postos_sem_cobertura`) |
| A6 | | **Postos com cobertura dupla/tripla** (>1 turno ativo no mesmo posto) | Lista com quantidade | `GET /dashboard/postos` (enriquecer) |
| A7 | | **Próximos turnos a iniciar** (próximos 60 min) | Timeline/cronograma horizontal | `GET /dashboard/proximos-turnos?minutos=60` |
| A8 | | **Turnos previstos para hoje** (agendados + iniciados + finalizados) | Barra de progresso horizontal (iniciados / previstos) | `GET /dashboard/turnos-hoje` |
| A9 | | **Turnos finalizados hoje** | Card numérico | `GET /dashboard/turnos-hoje` (campo) |
| A10 | | **Taxa de início pontual hoje** (% iniciados dentro da tolerância) | Card com % + gauge | `GET /dashboard/pontualidade?periodo=hoje` |
| A11 | ⭐ | **Checkins na última hora** | Card numérico (KPI) | `GET /dashboard/summary` (campo `checkins_ultima_hora`) |
| A12 | | **Média de checkins por turno ativo** (checkins/h / turnos ativos) | Card com sparkline | Calcular no frontend |
| A13 | | **Feed de últimos eventos** (inícios, fins, checkins, alertas) | Timeline estilo feed ao vivo | `GET /dashboard/summary` (array `feed_eventos`) |
| A14 | | **Vigias sem checkin nos últimos N minutos** (risco iminente) | Tabela com nome, posto, minutos sem checkin | `GET /dashboard/vigias-sem-checkin?minutos=15` |

---

## BLOCO B — ALERTAS E INCIDENTES

| # | ⭐ | Informação | Visualização | Endpoint Sugerido |
|---|---|---|---|---|
| B1 | ⭐ | **Total de alertas abertos** | Card com contagem + tendência (↑↓ vs ontem) | `GET /dashboard/summary` (campo `alertas_abertos`) |
| B2 | | **Alertas abertos por tipo** (atraso, no_show, emergência, sabotagem, desvio_rota) | Gráfico de barras horizontais ou pizza | `GET /alertas/estatisticas` (já existe) |
| B3 | | **Alertas abertos por severidade/nível** (N1, N2, N3...) | Gráfico de barras empilhadas por cor | `GET /dashboard/alertas-por-nivel` |
| B4 | | **Alertas abertos por posto** (quais postos geram mais alertas) | Gráfico de barras horizontal (top 10) | `GET /dashboard/alertas-por-posto?status=aberto` |
| B5 | | **Alertas abertos por vigia** (quais vigias geram mais alertas) | Tabela ranqueada | `GET /dashboard/alertas-por-vigia?status=aberto` |
| B6 | | **Tempo médio de reconhecimento** (criação → reconhecimento) | Card com minutos médios + gráfico de linha | `GET /dashboard/tempo-resposta-alertas?periodo=30d` |
| B7 | | **Tempo médio de encerramento** (criação → encerramento) | Card com minutos médios | `GET /dashboard/tempo-resposta-alertas?periodo=30d` |
| B8 | | **Alertas não reconhecidos há mais de X minutos** (SLA quebrado) | Tabela com destaque vermelho | `GET /dashboard/alertas-sla-quebrado?minutos=30` |
| B9 | | **Evolução diária de alertas** (últimos 7/30 dias) | Gráfico de linha com séries por tipo | `GET /dashboard/alertas-timeline?dias=30` |
| B10 | | **Taxa de falso positivo** (% encerrados como falso_positivo ou resolvido_checkin) | Card com % | `GET /dashboard/taxa-falso-positivo?periodo=30d` |
| B11 | | **Top 5 alertas mais recentes** (feed ao vivo) | Lista com scroll (tipo, posto, vigia, tempo) | `GET /dashboard/summary` (array `alertas_recentes` — já existe) |
| B12 | | **Heatmap de alertas por hora do dia** (quais horários concentram mais incidentes) | Heatmap (calendário semanal 24×7) | `GET /dashboard/alertas-heatmap?dias=30` |
| B13 | | **Alertas por dia da semana** (padrão semanal de incidentes) | Gráfico de barras (Dom-Sáb) | `GET /dashboard/alertas-por-dia-semana?periodo=90d` |

---

## BLOCO C — GEOLOCALIZAÇÃO E RASTREAMENTO

| # | ⭐ | Informação | Visualização | Endpoint Sugerido |
|---|---|---|---|---|
| C1 | ⭐ | **Mapa de turnos ativos** (posição de cada vigia + geofence dos postos) | Mapa Leaflet com pins coloridos (verde=ok, amarelo=desvio, vermelho=crítico, cinza=offline, azul=posto) | `GET /turnos/mapa` (já existe) |
| C2 | | **Desvios de rota nas últimas 24h** | Card numérico + lista | `GET /dashboard/summary` (campo `desvios_rota` — já existe) |
| C3 | | **Desvios de rota por posto** (top postos com mais desvios) | Gráfico de barras | `GET /dashboard/desvios-por-posto?periodo=30d` |
| C4 | | **Desvios de rota por vigia** (top vigias com mais desvios) | Tabela ranqueada | `GET /dashboard/desvios-por-vigia?periodo=30d` |
| C5 | | **% de checkins dentro da geofence** (taxa de conformidade geográfica) | Card com % + gauge circular | `GET /dashboard/conformidade-geofence?periodo=30d` |
| C6 | | **Distância média do posto nos checkins** (quão longe os vigias estão em média) | Card com metros | `GET /dashboard/distancia-media-posto?periodo=30d` |
| C7 | | **Rastro de um turno específico** (todos os checkins GPS em ordem) | Linha no mapa com waypoints numerados | `GET /turnos/{id}` (já existe com checkins) |
| C8 | | **Vigias parados (sem movimento)** — N checkins consecutivos no mesmo local | Lista de alerta | `GET /dashboard/vigias-estaticos?checkins=5` |
| C9 | | **Mapa de calor de presença** — onde os vigias mais fazem check-in por posto | Heatmap no mapa por posto | `GET /dashboard/heatmap-presenca?periodo=7d` |

---

## BLOCO D — PRODUTIVIDADE E DESEMPENHO DOS VIGIAS

| # | Informação | Visualização | Endpoint Sugerido |
|---|---|---|---|
| D1 | **Ranking de checkins por vigia** (hoje/semana/mês) | Tabela ranqueada (nome, posto, checkins) | `GET /dashboard/ranking-checkins?periodo=hoje` |
| D2 | **Horas trabalhadas por vigia** (soma de duração de turnos finalizados) | Tabela com total de horas | `GET /dashboard/horas-trabalhadas?periodo=30d` |
| D3 | **Média de horas por turno por vigia** | Gráfico de barras | `GET /dashboard/horas-trabalhadas?periodo=30d` (agregado) |
| D4 | **Taxa de check-in por vigia** (realizados / esperados) | Card com % + gráfico de barras | `GET /dashboard/taxa-checkin?periodo=30d` |
| D5 | **Intervalo médio entre checkins por vigia** (real vs configurado) | Gráfico de dispersão | `GET /dashboard/intervalo-checkin?periodo=30d` |
| D6 | **Vigias com maior tempo de atraso acumulado** (top 10) | Tabela ranqueada | `GET /dashboard/ranking-atrasos?periodo=30d` |
| D7 | **Vigias que mais geram alertas** | Tabela ranqueada | `GET /dashboard/vigias-problematicos?periodo=30d` |
| D8 | **Vigias que nunca geraram alerta** (impecáveis) | Lista de reconhecimento | `GET /dashboard/vigias-impecaveis?periodo=90d` |
| D9 | **Vigias que mais usam senha de emergência** | Tabela | `GET /dashboard/uso-senha-emergencia?periodo=30d` |
| D10 | **Vigias que mais usam senha customizada** | Tabela | `GET /dashboard/uso-senha-customizada?periodo=30d` |
| D11 | **Vigias com mais substituições** (quem mais é substituído) | Tabela | `GET /dashboard/vigias-substituidos?periodo=90d` |
| D12 | **Vigia do mês** (melhor score composto: pontualidade + checkins + zero alertas) | Card com destaque (nome, foto, stats) | `GET /dashboard/vigia-do-mes` |
| D13 | **Comparativo entre vigias do mesmo posto** | Gráfico radar/spider (pontualidade, checkins, alertas, geofence) | `GET /dashboard/comparativo-vigias?posto_id=X&periodo=30d` |

---

## BLOCO E — DESEMPENHO DOS POSTOS

| # | Informação | Visualização | Endpoint Sugerido |
|---|---|---|---|
| E1 | **Turnos ativos por posto** | Gráfico de barras | `GET /dashboard/summary` (array `turnos_por_posto` — já existe) |
| E2 | **Taxa de ocupação histórica por posto** (% do tempo que o posto teve vigia) | Gráfico de barras | `GET /dashboard/ocupacao-postos?periodo=30d` |
| E3 | **Postos com mais incidentes** (alertas totais por posto) | Gráfico de barras horizontal | `GET /dashboard/incidentes-por-posto?periodo=30d` |
| E4 | **Postos com mais desvios de rota** | Gráfico de barras | `GET /dashboard/desvios-por-posto?periodo=30d` |
| E5 | **Postos com mais sabotagens** | Tabela | `GET /dashboard/sabotagens-por-posto?periodo=90d` |
| E6 | **Postos com mais no-shows** (vigia não apareceu) | Tabela | `GET /dashboard/noshows-por-posto?periodo=90d` |
| E7 | **Distribuição de turnos por posto ao longo do dia** | Gráfico de área empilhada (24h) | `GET /dashboard/distribuicao-turnos-postos` |
| E8 | **Postos sem escala cadastrada** (órfãos de planejamento) | Lista simples | `GET /dashboard/postos-sem-escala` |
| E9 | **Postos com escala mas sem vigia designado** | Lista | `GET /dashboard/postos-sem-vigia` |
| E10 | **Horas de cobertura por posto** (horas cobertas / horas contratadas) | Gráfico de barras com linha de meta | `GET /dashboard/cobertura-horas-postos?periodo=30d` |

---

## BLOCO F — ESCALAS E PLANEJAMENTO

| # | Informação | Visualização | Endpoint Sugerido |
|---|---|---|---|
| F1 | **Carga horária semanal por vigia** (soma das escalas) | Tabela com barra de progresso (limite 44h) | `GET /dashboard/carga-horaria-vigias` |
| F2 | **Vigias sem escala atribuída** (ociosos/desalocados) | Lista | `GET /dashboard/vigias-sem-escala` |
| F3 | **Conflitos de escala** (2 vigias para mesmo posto no mesmo horário) | Tabela com destaque vermelho | `GET /dashboard/conflitos-escala` |
| F4 | **Escalas noturnas vs diurnas** (proporção) | Gráfico de rosca | `GET /dashboard/distribuicao-escalas?tipo=diurno_noturno` |
| F5 | **Substituições ativas hoje** | Tabela (quem, posto, horário) | `GET /dashboard/substituicoes-hoje` |
| F6 | **Substituições agendadas para os próximos 7 dias** | Timeline/agenda | `GET /dashboard/substituicoes-futuras?dias=7` |
| F7 | **Vigias mais substituídos** (top 10) | Gráfico de barras | `GET /dashboard/ranking-substituicoes?periodo=90d` |
| F8 | **Vigias que mais substituem** (top 10 substitutos) | Gráfico de barras | `GET /dashboard/ranking-substitutos?periodo=90d` |
| F9 | **Taxa de substituição** (% de turnos que foram substituições vs escala normal) | Card com % | `GET /dashboard/taxa-substituicao?periodo=30d` |
| F10 | **Motivos de substituição mais comuns** | Gráfico de pizza (nuvem de palavras) | `GET /dashboard/motivos-substituicao?periodo=90d` |
| F11 | **Calendário semanal de cobertura** (grade 7 colunas × postos) | Heatmap/grade colorida (verde=coberto, vermelho=descoberto, amarelo=substituição) | `GET /dashboard/grade-cobertura?data_inicio=X&data_fim=Y` |

---

## BLOCO G — PONTUALIDADE E ADERÊNCIA

| # | Informação | Visualização | Endpoint Sugerido |
|---|---|---|---|
| G1 | **Taxa de início no horário** (% iniciados dentro da tolerância) | Card com % + gauge + sparkline | `GET /dashboard/pontualidade?metrica=inicio&periodo=30d` |
| G2 | **Atraso médio de início** (minutos) | Card com minutos | `GET /dashboard/pontualidade?metrica=inicio&periodo=30d` |
| G3 | **Taxa de check-in no prazo** (% checkins dentro do intervalo configurado) | Card com % | `GET /dashboard/pontualidade?metrica=checkin&periodo=30d` |
| G4 | **Atraso médio de check-in** (minutos em relação ao deadline) | Card com minutos | `GET /dashboard/pontualidade?metrica=checkin&periodo=30d` |
| G5 | **Taxa de no-show** (% turnos agendados nunca iniciados) | Card com % + tendência | `GET /dashboard/taxa-noshow?periodo=30d` |
| G6 | **Evolução da pontualidade** (linha do tempo diária) | Gráfico de linha com meta (ex: 95%) | `GET /dashboard/pontualidade-timeline?dias=30` |
| G7 | **Pontualidade por posto** | Gráfico de barras | `GET /dashboard/pontualidade-por-posto?periodo=30d` |
| G8 | **Pontualidade por vigia** (top/bottom) | Tabela ranqueada | `GET /dashboard/pontualidade-por-vigia?periodo=30d` |
| G9 | **Pontualidade por dia da semana** (padrão de atrasos) | Gráfico de barras | `GET /dashboard/pontualidade-por-dia-semana?periodo=90d` |
| G10 | **Pontualidade por faixa horária** (madrugada, manhã, tarde, noite) | Gráfico de barras agrupadas | `GET /dashboard/pontualidade-por-faixa?periodo=30d` |

---

## BLOCO H — SEGURANÇA E INTEGRIDADE

| # | Informação | Visualização | Endpoint Sugerido |
|---|---|---|---|
| H1 | **Total de sabotagens reportadas** (período) | Card com número + tendência | `GET /dashboard/sabotagens?periodo=30d` |
| H2 | **Sabotagens por posto** | Gráfico de barras horizontal | `GET /dashboard/sabotagens-por-posto?periodo=90d` |
| H3 | **Sabotagens por vigia** | Tabela | `GET /dashboard/sabotagens-por-vigia?periodo=90d` |
| H4 | **Uso de senha de emergência/coação** (frequência) | Card + gráfico de linha | `GET /dashboard/uso-senha-emergencia?periodo=30d` |
| H5 | **Senhas de emergência por posto** (quais postos têm mais ocorrências) | Gráfico de barras | `GET /dashboard/emergencia-por-posto?periodo=90d` |
| H6 | **Senhas de emergência por vigia** | Tabela | `GET /dashboard/emergencia-por-vigia?periodo=90d` |
| H7 | **Senhas de emergência por horário** (há padrão?) | Gráfico de barras por hora do dia | `GET /dashboard/emergencia-por-hora?periodo=90d` |
| H8 | **Reassociações de dispositivo** (trocas de celular por PIN) | Card com contagem + lista | `GET /dashboard/reassociacoes?periodo=30d` |
| H9 | **Vigias que mais trocaram de dispositivo** | Tabela | `GET /dashboard/reassociacoes-por-vigia?periodo=90d` |
| H10 | **Tentativas de check-in com dispositivo inválido** | Card (se houver log) | `GET /dashboard/tentativas-invalidas?periodo=30d` |
| H11 | **Uso de senhas customizadas** (quais e por quem) | Tabela analítica | `GET /dashboard/uso-senha-customizada?periodo=30d` |

---

## BLOCO I — TECNOLOGIA E INFRAESTRUTURA

| # | Informação | Visualização | Endpoint Sugerido |
|---|---|---|---|
| I1 | **Dispositivos ativos** (celulares registrados) | Card | `GET /dashboard/dispositivos-ativos` |
| I2 | **Checkins online vs offline** (proporção) | Gráfico de rosca | `GET /dashboard/checkins-online-offline?periodo=30d` |
| I3 | **Taxa de offline** (% checkins via offline_sincronizado) | Card com % + tendência | `GET /dashboard/checkins-online-offline?periodo=30d` |
| I4 | **Lotes offline pendentes** (se aplicável) | Card | `GET /dashboard/lotes-offline-pendentes` |
| I5 | **Tamanho médio de lote offline** | Card com número | `GET /dashboard/lotes-offline?periodo=30d` |
| I6 | **Tempo médio de reconciliação offline** (timestamp_recebimento − timestamp_criacao) | Card com minutos | `GET /dashboard/tempo-reconciliacao?periodo=30d` |
| I7 | **Vigias que mais operam offline** | Tabela | `GET /dashboard/vigias-offline?periodo=30d` |
| I8 | **Postos com pior conectividade** (mais checkins offline) | Gráfico de barras | `GET /dashboard/postos-offline?periodo=30d` |
| I9 | **Versão do app dos dispositivos** (se disponível) | Tabela de distribuição | `GET /dashboard/versoes-app` |
| I10 | **Conexões WebSocket ativas** | Card | `GET /dashboard/ws-connections` |

---

## BLOCO J — COMPARATIVOS E BENCHMARKS

| # | Informação | Visualização | Endpoint Sugerido |
|---|---|---|---|
| J1 | **Comparativo hoje vs ontem** (turnos, alertas, checkins, desvios) | Cards lado a lado com Δ% | `GET /dashboard/comparativo?periodo1=hoje&periodo2=ontem` |
| J2 | **Comparativo esta semana vs semana passada** | Gráfico de barras lado a lado | `GET /dashboard/comparativo?periodo1=semana&periodo2=semana_anterior` |
| J3 | **Comparativo este mês vs mês passado** | Cards com Δ% | `GET /dashboard/comparativo?periodo1=mes&periodo2=mes_anterior` |
| J4 | **Ranking geral de postos** (nota composta: pontualidade + alertas + geofence) | Tabela ranqueada com estrelas | `GET /dashboard/ranking-postos?periodo=30d` |
| J5 | **Ranking geral de vigias** (nota composta) | Tabela ranqueada com estrelas | `GET /dashboard/ranking-vigias?periodo=30d` |
| J6 | **Score de saúde operacional** (índice 0-100 composto por múltiplos KPIs) | Gauge estilo velocímetro | `GET /dashboard/score-operacional` |
| J7 | **Tendência do score operacional** (últimos 30 dias) | Gráfico de linha | `GET /dashboard/score-operacional?periodo=30d` |

---

## BLOCO K — TURNOS E JORNADA

| # | Informação | Visualização | Endpoint Sugerido |
|---|---|---|---|
| K1 | **Duração média dos turnos** (real vs previsto) | Gráfico de barras duplas | `GET /dashboard/duracao-turnos?periodo=30d` |
| K2 | **Turnos que excederam o horário previsto** (hora extra) | Lista com vigia, posto, excedente | `GET /dashboard/turnos-excedentes?periodo=30d` |
| K3 | **Turnos encerrados antes do previsto** (saída antecipada) | Lista | `GET /dashboard/turnos-encerrados-antes?periodo=30d` |
| K4 | **Distribuição de status de turnos** (agendado, em_andamento, finalizado, etc.) | Gráfico de pizza | `GET /dashboard/distribuicao-status-turnos?periodo=hoje` |
| K5 | **Turnos pausados há mais de X minutos** (possível abandono) | Lista de alerta | `GET /dashboard/turnos-pausados?minutos=30` |
| K6 | **Intervalo entre turnos do mesmo vigia** (< 11h = risco) | Tabela de alerta (descanso insuficiente) | `GET /dashboard/intervalo-jornada?periodo=7d` |
| K7 | **Vigias com mais de X horas no dia** (risco trabalhista) | Tabela de alerta | `GET /dashboard/excesso-jornada?horas=12` |
| K8 | **Vigias que trabalharam 7+ dias consecutivos** (risco de fadiga) | Lista de alerta | `GET /dashboard/dias-consecutivos?periodo=30d` |
| K9 | **Turnos que cruzaram a virada do dia** (noturnos) | Card com contagem | `GET /dashboard/turnos-noturnos?periodo=30d` |

---

## BLOCO L — FINANCEIRO/CONTRATUAL (requer coluna `valor_hora` nos postos)

| # | Informação | Visualização | Endpoint Sugerido |
|---|---|---|---|
| L1 | **Horas contratadas vs trabalhadas por posto** | Gráfico de barras duplas | `GET /dashboard/horas-contratadas-vs-trabalhadas?periodo=30d` |
| L2 | **Horas extras acumuladas por vigia** | Tabela | `GET /dashboard/horas-extras?periodo=30d` |
| L3 | **Horas não cobertas por posto** (posto deveria ter vigia mas não teve) | Gráfico de barras | `GET /dashboard/horas-nao-cobertas?periodo=30d` |
| L4 | **Custo estimado de horas extras** (se houver valor/hora) | Card com R$ | `GET /dashboard/custo-horas-extras?periodo=30d` |
| L5 | **Custo estimado de não cobertura** | Card com R$ | `GET /dashboard/custo-nao-cobertura?periodo=30d` |
| L6 | **Faturamento estimado por posto** (horas cobertas × valor/hora) | Gráfico de barras | `GET /dashboard/faturamento-postos?periodo=30d` |

---

## BLOCO M — PESSOAS E RH

| # | Informação | Visualização | Endpoint Sugerido |
|---|---|---|---|
| M1 | **Total de vigias ativos** | Card | `GET /dashboard/total-vigias` |
| M2 | **Total de supervisores** | Card | `GET /dashboard/total-supervisores` |
| M3 | **Vigias sem escala** (não alocados) | Lista | `GET /dashboard/vigias-sem-escala` |
| M4 | **Vigias inativos há mais de X dias** (possível desligamento) | Lista | `GET /dashboard/vigias-inativos?dias=30` |
| M5 | **Taxa de rotatividade** (desativados no período / total) | Card com % | `GET /dashboard/rotatividade?periodo=90d` |
| M6 | **Novos vigias nos últimos 30 dias** | Card + lista | `GET /dashboard/novos-vigias?dias=30` |
| M7 | **Distribuição de vigias por supervisor** | Gráfico de barras | `GET /dashboard/vigias-por-supervisor` |
| M8 | **Supervisores com mais postos atribuídos** | Gráfico de barras | `GET /dashboard/supervisores-por-posto` |

---

## BLOCO N — TENDÊNCIAS E PROJEÇÕES

| # | Informação | Visualização | Endpoint Sugerido |
|---|---|---|---|
| N1 | **Tendência de alertas** (projeção linear próximos 7 dias) | Gráfico de linha com faixa de projeção | Calcular no frontend com `GET /dashboard/alertas-timeline?dias=60` |
| N2 | **Tendência de checkins** (volume diário) | Gráfico de linha | `GET /dashboard/checkins-timeline?dias=30` |
| N3 | **Tendência de turnos** (iniciados, finalizados, no-show) | Gráfico de área empilhada | `GET /dashboard/turnos-timeline?dias=30` |
| N4 | **Sazonalidade de incidentes** (por mês, trimestre) | Gráfico de barras | `GET /dashboard/sazonalidade-alertas?periodo=12m` |
| N5 | **Dia mais crítico da semana** (média de alertas por dia) | Gráfico de barras com destaque | `GET /dashboard/alertas-por-dia-semana?periodo=90d` |

---

## RESUMO DAS 8 TELAS DE DASHBOARD

### Tela 1 — Visão Geral (Home) ⭐ ATUAL
KPIs (8 cards) + Mapa em tempo real + Cobertura de postos (donut + lista) + Feed de eventos ao vivo

### Tela 2 — Alertas e Incidentes
KPIs (abertos, SLA, falso positivo) + Evolução diária + Heatmap hora×dia + Rankings por posto/vigia

### Tela 3 — Desempenho de Vigias
Vigia do mês + Pontualidade + Ranking com score composto + Horas trabalhadas + Checkins

### Tela 4 — Desempenho de Postos
Cobertura + Ocupação histórica + Mapa de calor + Rankings + Incidentes por posto

### Tela 5 — Planejamento e Escalas
Grade de cobertura semanal + Substituições + Carga horária + Conflitos

### Tela 6 — Segurança
Sabotagens + Emergências + Reassociações + Senhas customizadas

### Tela 7 — Infraestrutura
Online vs Offline + Conectividade + Reconciliação + Dispositivos + WS connections

### Tela 8 — Compliance Trabalhista
Excesso de jornada + Dias consecutivos + Descanso entre turnos + Horas extras

---

## ESTRATÉGIA DE UNIFICAÇÃO DE ENDPOINTS

Para reduzir chamadas HTTP, agrupar endpoints em chamadas unificadas:

| Endpoint Unificado | O que entrega | Telas |
|---|---|---|
| `GET /dashboard/home` | KPIs + cobertura + feed + alertas recentes + turnos por posto + mapa | Tela 1 |
| `GET /dashboard/alertas?visao=completa` | Estatísticas + timeline + heatmap + rankings | Tela 2 |
| `GET /dashboard/vigias` | Rankings + horas + checkins + pontualidade + score | Tela 3 |
| `GET /dashboard/postos` | Ocupação + incidentes + cobertura + rankings | Tela 4 |
| `GET /dashboard/planejamento` | Escalas + substituições + carga horária + grade | Tela 5 |
| `GET /dashboard/seguranca` | Sabotagens + emergências + reassociações + senhas | Tela 6 |
| `GET /dashboard/infra` | Online/offline + conectividade + dispositivos | Tela 7 |
| `GET /dashboard/compliance` | Excesso jornada + dias consecutivos + descanso + extras | Tela 8 |

---

## PADRÕES DE QUERY SQL REUTILIZÁVEIS

A maioria dos endpoints compartilha os mesmos patterns de consulta:

1. **Contagem com filtro de período**: `SELECT COUNT(*) FROM X WHERE empresa_id = $1 AND created_at >= $2 AND created_at < $3`
2. **Agregação por dimensão**: `SELECT dimensao, COUNT(*) FROM X WHERE empresa_id = $1 GROUP BY dimensao ORDER BY COUNT(*) DESC`
3. **Timeline diária**: `SELECT DATE(created_at) as dia, COUNT(*) FROM X WHERE empresa_id = $1 GROUP BY dia ORDER BY dia`
4. **Comparação entre períodos**: CTE com duas subqueries para períodos diferentes
5. **Score composto**: Junção de múltiplas agregações em subselects correlacionados

---

**Total**: **~130 opções** de informações mapeadas em **~50 endpoints** (ou ~8 endpoints unificados), cobrindo 8 telas de dashboard.

**Status atual**: Tela 1 (Visão Geral) com itens A1, A3, A4, A5, A11, B1, C1 em implementação.
