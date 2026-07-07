package model

import (
	"time"

	"github.com/google/uuid"
)

type Escala struct {
	ID              uuid.UUID `json:"id"`
	EmpresaID       uuid.UUID `json:"empresa_id"`
	UsuarioID       uuid.UUID `json:"usuario_id"`
	PostoID         uuid.UUID `json:"posto_id"`
	DiaSemanaInicio int16     `json:"dia_semana_inicio"`
	HoraInicio      string    `json:"hora_inicio"`
	DiaSemanaFim    int16     `json:"dia_semana_fim"`
	HoraFim         string    `json:"hora_fim"`
	ToleranciaMin   int       `json:"tolerancia_min"`
	Ativo           bool      `json:"ativo"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`

	UsuarioNome string `json:"usuario_nome,omitempty"`
	PostoNome   string `json:"posto_nome,omitempty"`
}

type CreateEscalaRequest struct {
	UsuarioID       string `json:"usuario_id" validate:"required,uuid"`
	PostoID         string `json:"posto_id" validate:"required,uuid"`
	DiaSemanaInicio int16  `json:"dia_semana_inicio" validate:"min=0,max=6"`
	HoraInicio      string `json:"hora_inicio" validate:"required"`
	DiaSemanaFim    int16  `json:"dia_semana_fim" validate:"min=0,max=6"`
	HoraFim         string `json:"hora_fim" validate:"required"`
	ToleranciaMin   int    `json:"tolerancia_min" validate:"omitempty,min=0,max=120"`
}

type DiaEscalaEntry struct {
	DiaSemanaInicio int16  `json:"dia_semana_inicio" validate:"min=0,max=6"`
	HoraInicio      string `json:"hora_inicio" validate:"required"`
	DiaSemanaFim    int16  `json:"dia_semana_fim" validate:"min=0,max=6"`
	HoraFim         string `json:"hora_fim" validate:"required"`
}

type CreateEscalaLoteRequest struct {
	UsuarioID     string           `json:"usuario_id" validate:"required,uuid"`
	PostoID       string           `json:"posto_id" validate:"required,uuid"`
	ToleranciaMin int              `json:"tolerancia_min" validate:"omitempty,min=0,max=120"`
	Dias          []DiaEscalaEntry `json:"dias" validate:"required,min=1,max=7,dive"`
}

type CreateEscalaLoteResponse struct {
	UsuarioID     string           `json:"usuario_id"`
	PostoID       string           `json:"posto_id"`
	ToleranciaMin int              `json:"tolerancia_min"`
	Dias          []DiaEscalaEntry `json:"dias"`
}

type UpdateEscalaRequest struct {
	UsuarioID       *string `json:"usuario_id" validate:"omitempty,uuid"`
	PostoID         *string `json:"posto_id" validate:"omitempty,uuid"`
	DiaSemanaInicio *int16  `json:"dia_semana_inicio" validate:"omitempty,min=0,max=6"`
	HoraInicio      *string `json:"hora_inicio" validate:"omitempty"`
	DiaSemanaFim    *int16  `json:"dia_semana_fim" validate:"omitempty,min=0,max=6"`
	HoraFim         *string `json:"hora_fim" validate:"omitempty"`
	ToleranciaMin   *int    `json:"tolerancia_min" validate:"omitempty,min=0,max=120"`
	Ativo           *bool   `json:"ativo"`
}

type EscalaFilter struct {
	UsuarioID string
	PostoID   string
	Ativo     *bool
	Limit     int
	Offset    int
}
