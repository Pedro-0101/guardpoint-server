package model

import (
	"time"

	"github.com/google/uuid"
)

type Alerta struct {
	ID          uuid.UUID  `json:"id"`
	EmpresaID   uuid.UUID  `json:"empresa_id"`
	TurnoID     uuid.UUID  `json:"turno_id"`
	Tipo        string     `json:"tipo"`
	Nivel       int        `json:"nivel"`
	Status      string     `json:"status"`
	Mensagem    *string    `json:"mensagem,omitempty"`
	ResolvidoEm *time.Time `json:"resolvido_em,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
}

type ConfigEscalonamento struct {
	ID            uuid.UUID `json:"id"`
	EmpresaID     uuid.UUID `json:"empresa_id"`
	Nivel         int       `json:"nivel"`
	AtrasoMinutos int       `json:"atraso_minutos"`
	WhatsappPara  string    `json:"whatsapp_para"`
	CargoAlvo     *string   `json:"cargo_alvo,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
}

type AlertaFilter struct {
	Status  string `json:"status"`
	Tipo    string `json:"tipo"`
	TurnoID string `json:"turno_id"`
	Limit   int    `json:"limit"`
	Offset  int    `json:"offset"`
}

type AlertStatistics struct {
	TotalAbertos      int             `json:"total_abertos"`
	TotalReconhecidos int             `json:"total_reconhecidos"`
	TotalEncerrados   int             `json:"total_encerrados"`
	PorTipo           []AlertaPorTipo `json:"por_tipo"`
	PorHora           []AlertaPorHora `json:"por_hora"`
}

type AlertaPorTipo struct {
	Tipo       string `json:"tipo"`
	Quantidade int    `json:"quantidade"`
}

type AlertaPorHora struct {
	Hora       string `json:"hora"`
	Quantidade int    `json:"quantidade"`
}

type CreateConfigEscalonamentoRequest struct {
	Nivel         int    `json:"nivel" validate:"required,min=1,max=5"`
	AtrasoMinutos int    `json:"atraso_minutos" validate:"required,min=1,max=1440"`
	WhatsappPara  string `json:"whatsapp_para" validate:"required,max=20"`
	CargoAlvo     string `json:"cargo_alvo,omitempty" validate:"omitempty,max=50"`
}

type UpdateConfigEscalonamentoRequest struct {
	AtrasoMinutos int    `json:"atraso_minutos" validate:"required,min=1,max=1440"`
	WhatsappPara  string `json:"whatsapp_para" validate:"required,max=20"`
	CargoAlvo     string `json:"cargo_alvo,omitempty" validate:"omitempty,max=50"`
}

type PendingAlert struct {
	Alerta       *Alerta `json:"alerta"`
	WhatsappPara string  `json:"whatsapp_para"`
}
