package model

type TurnoPorPosto struct {
	PostoID    string `json:"posto_id" example:"550e8400-e29b-41d4-a716-446655440000"`
	PostoNome  string `json:"posto_nome" example:"Portaria Principal"`
	Quantidade int    `json:"quantidade" example:"3"`
}

type AlertaRecente struct {
	ID        string `json:"id" example:"550e8400-e29b-41d4-a716-446655440000"`
	Tipo      string `json:"tipo" example:"atraso"`
	TurnoID   string `json:"turno_id" example:"770e8400-e29b-41d4-a716-446655440000"`
	PostoID   string `json:"posto_id,omitempty" example:"880e8400-e29b-41d4-a716-446655440000"`
	Mensagem  string `json:"mensagem" example:"Atraso de 5 minutos detectado."`
	CreatedAt string `json:"created_at" example:"2025-07-10T15:04:05Z"`
}

type DashboardSummary struct {
	TurnosAtivos       int             `json:"turnos_ativos" example:"12"`
	AlertasAbertos     int             `json:"alertas_abertos" example:"5"`
	CheckinsUltimaHora int             `json:"checkins_ultima_hora" example:"48"`
	DesviosRota        int             `json:"desvios_rota" example:"2"`
	AlertasRecentes    []AlertaRecente `json:"alertas_recentes"`
	TurnosPorPosto     []TurnoPorPosto `json:"turnos_por_posto"`
}
