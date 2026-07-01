package model

type TurnoPorPosto struct {
	PostoID    string `json:"posto_id"`
	PostoNome  string `json:"posto_nome"`
	Quantidade int    `json:"quantidade"`
}

type AlertaRecente struct {
	ID        string `json:"id"`
	Tipo      string `json:"tipo"`
	TurnoID   string `json:"turno_id"`
	Nivel     int    `json:"nivel"`
	Mensagem  string `json:"mensagem"`
	CreatedAt string `json:"created_at"`
}

type DashboardSummary struct {
	TurnosAtivos       int             `json:"turnos_ativos"`
	AlertasAbertos     int             `json:"alertas_abertos"`
	CheckinsUltimaHora int             `json:"checkins_ultima_hora"`
	DesviosRota        int             `json:"desvios_rota"`
	AlertasRecentes    []AlertaRecente `json:"alertas_recentes"`
	TurnosPorPosto     []TurnoPorPosto `json:"turnos_por_posto"`
}
