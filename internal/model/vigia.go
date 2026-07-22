package model

import (
	"time"

	"github.com/google/uuid"
)

type VigiaTurnoResponse struct {
	TemTurnoAtivo bool            `json:"tem_turno_ativo"`
	Mensagem      string          `json:"mensagem"`
	Turno         *VigiaTurnoInfo `json:"turno,omitempty"`
	ProximoTurno  *VigiaProximoTurno `json:"proximo_turno,omitempty"`
}

type VigiaTurnoInfo struct {
	ID                  uuid.UUID  `json:"id"`
	Status              string     `json:"status"`
	Posto               *Posto     `json:"posto,omitempty"`
	PostoNome           string     `json:"posto_nome,omitempty"`
	TokenSessao         *string    `json:"token_sessao,omitempty"`
	InicioPrevisto      time.Time  `json:"inicio_previsto"`
	FimPrevisto         time.Time  `json:"fim_previsto"`
	InicioReal          *time.Time `json:"inicio_real,omitempty"`
	IntervaloMin        int        `json:"intervalo_min"`
	ProximoDeadline     *time.Time `json:"proximo_deadline,omitempty"`
	TipoProximoDeadline string     `json:"tipo_proximo_deadline,omitempty"`
	Atrasado            bool       `json:"atrasado"`
	CheckinsHoje        int        `json:"checkins_hoje"`
	UltimoCheckin       *Checkin   `json:"ultimo_checkin,omitempty"`
}

type VigiaProximoTurno struct {
	Posto         *Posto    `json:"posto,omitempty"`
	InicioPrevisto time.Time `json:"inicio_previsto"`
	FimPrevisto    time.Time `json:"fim_previsto"`
	Data           string    `json:"data"`
	HoraInicio     string    `json:"hora_inicio"`
	HoraFim        string    `json:"hora_fim"`
}
