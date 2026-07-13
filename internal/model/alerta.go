package model

import (
	"time"

	"github.com/google/uuid"
)

type Alerta struct {
	ID          uuid.UUID  `json:"id" example:"550e8400-e29b-41d4-a716-446655440000"`
	EmpresaID   uuid.UUID  `json:"empresa_id" example:"660e8400-e29b-41d4-a716-446655440000"`
	TurnoID     *uuid.UUID `json:"turno_id" example:"770e8400-e29b-41d4-a716-446655440000"`
	PostoID     *uuid.UUID `json:"posto_id,omitempty" example:"880e8400-e29b-41d4-a716-446655440000"`
	Tipo        string     `json:"tipo" example:"atraso"`
	Nivel       int        `json:"nivel" example:"1"`
	Status      string     `json:"status" example:"aberto"`
	Mensagem    *string    `json:"mensagem,omitempty" example:"Atraso de 5 minutos detectado no turno."`
	ResolvidoEm *time.Time `json:"resolvido_em,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
}

type ConfigEscalonamento struct {
	ID            uuid.UUID   `json:"id" example:"550e8400-e29b-41d4-a716-446655440000"`
	EmpresaID     uuid.UUID   `json:"empresa_id" example:"660e8400-e29b-41d4-a716-446655440000"`
	AtrasoMinutos int         `json:"atraso_minutos" example:"15"`
	Descricao     string      `json:"descricao" example:"Nivel 1 - Imediato"`
	Sistema       bool        `json:"sistema" example:"false"`
	EmUso         bool        `json:"em_uso" example:"true"`
	UsuarioIDs    []uuid.UUID `json:"usuario_ids"`
	CreatedAt     time.Time   `json:"created_at"`
}

type AlertaListResponse struct {
	Data  []Alerta `json:"data"`
	Total int      `json:"total" example:"42"`
}

type AlertaFilter struct {
	Status  string `json:"status"`
	Tipo    string `json:"tipo"`
	TurnoID string `json:"turno_id"`
	PostoID string `json:"posto_id"`
	Limit   int    `json:"limit"`
	Offset  int    `json:"offset"`
}

type AlertStatistics struct {
	TotalAbertos      int             `json:"total_abertos" example:"5"`
	TotalReconhecidos int             `json:"total_reconhecidos" example:"3"`
	TotalEncerrados   int             `json:"total_encerrados" example:"12"`
	PorTipo           []AlertaPorTipo `json:"por_tipo"`
	PorHora           []AlertaPorHora `json:"por_hora"`
}

type AlertaPorTipo struct {
	Tipo       string `json:"tipo" example:"atraso"`
	Quantidade int    `json:"quantidade" example:"8"`
}

type AlertaPorHora struct {
	Hora       string `json:"hora" example:"08:00"`
	Quantidade int    `json:"quantidade" example:"4"`
}

type CreateConfigEscalonamentoRequest struct {
	AtrasoMinutos int         `json:"atraso_minutos" validate:"min=0,max=1440" example:"15"`
	Descricao     string      `json:"descricao" example:"Nivel 1"`
	UsuarioIDs    []uuid.UUID `json:"usuario_ids" validate:"required,min=1"`
}

type UpdateConfigEscalonamentoRequest struct {
	AtrasoMinutos int         `json:"atraso_minutos" validate:"min=0,max=1440" example:"30"`
	Descricao     string      `json:"descricao" example:"Nivel 2"`
	UsuarioIDs    []uuid.UUID `json:"usuario_ids" validate:"required,min=1"`
}

type UpdateConfigEscalonamentoUsuariosRequest struct {
	UsuarioIDs []uuid.UUID `json:"usuario_ids" validate:"required,min=1"`
}

type PendingAlert struct {
	Alerta     *Alerta     `json:"alerta"`
	UsuarioIDs []uuid.UUID `json:"usuario_ids"`
	PostoID    *uuid.UUID  `json:"posto_id,omitempty"`
}

type BatchAlertaRequest struct {
	IDs []uuid.UUID `json:"ids" validate:"required,min=1,max=100"`
}

type BatchAlertaResponse struct {
	Reconhecidos int `json:"reconhecidos" example:"3"`
	Encerrados   int `json:"encerrados" example:"5"`
	Erros        int `json:"erros" example:"0"`
}
