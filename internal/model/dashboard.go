package model

type DashboardLinha struct {
	PostoID        string  `json:"posto_id" example:"550e8400-e29b-41d4-a716-446655440000"`
	PostoNome      string  `json:"posto_nome" example:"Portaria Principal"`
	PostoLatitude  float64 `json:"posto_latitude" example:"-23.550520"`
	PostoLongitude float64 `json:"posto_longitude" example:"-46.633308"`
	PostoRaioM     int     `json:"posto_raio_m" example:"100"`

	VigiaID   string `json:"vigia_id" example:"770e8400-e29b-41d4-a716-446655440000"`
	VigiaNome string `json:"vigia_nome" example:"Carlos Silva"`

	TurnoID        string `json:"turno_id" example:"880e8400-e29b-41d4-a716-446655440000"`
	TurnoStatus    string `json:"turno_status" example:"em_andamento"`
	InicioPrevisto string `json:"inicio_previsto" example:"2025-07-10T08:00:00Z"`
	FimPrevisto    string `json:"fim_previsto" example:"2025-07-10T18:00:00Z"`
	InicioReal     string `json:"inicio_real,omitempty" example:"2025-07-10T08:02:00Z"`
	IntervaloMin   int    `json:"intervalo_min" example:"30"`

	UltimoCheckin  string `json:"ultimo_checkin,omitempty" example:"2025-07-10T15:30:00Z"`
	ProximoCheckin string `json:"proximo_checkin" example:"2025-07-10T16:00:00Z"`
	Atrasado       bool   `json:"atrasado" example:"false"`
}

type DashboardTableResponse struct {
	Linhas []DashboardLinha `json:"linhas"`
	Total  int              `json:"total" example:"12"`
	Limit  int              `json:"limit" example:"20"`
	Offset int              `json:"offset" example:"0"`
}
