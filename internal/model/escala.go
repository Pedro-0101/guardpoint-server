package model

import (
	"time"

	"github.com/google/uuid"
)

type Escala struct {
	ID            uuid.UUID  `json:"id"`
	EmpresaID     uuid.UUID  `json:"empresa_id"`
	UsuarioID     uuid.UUID  `json:"usuario_id"`
	PostoID       uuid.UUID  `json:"posto_id"`
	DataInicio    time.Time  `json:"data_inicio"`
	DataFim       time.Time  `json:"data_fim"`
	HoraInicio    string     `json:"hora_inicio"`
	HoraFim       string     `json:"hora_fim"`
	DiasSemana    []int16    `json:"dias_semana"`
	ToleranciaMin int        `json:"tolerancia_min"`
	Ativo         bool       `json:"ativo"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`

	UsuarioNome   string     `json:"usuario_nome,omitempty"`
	PostoNome     string     `json:"posto_nome,omitempty"`
}

type CreateEscalaRequest struct {
	UsuarioID     string  `json:"usuario_id" validate:"required,uuid"`
	PostoID       string  `json:"posto_id" validate:"required,uuid"`
	DataInicio    string  `json:"data_inicio" validate:"required"`
	DataFim       string  `json:"data_fim" validate:"required"`
	HoraInicio    string  `json:"hora_inicio" validate:"required"`
	HoraFim       string  `json:"hora_fim" validate:"required"`
	DiasSemana    []int16 `json:"dias_semana" validate:"required,min=1,max=7,dive,min=0,max=6"`
	ToleranciaMin int     `json:"tolerancia_min" validate:"omitempty,min=0,max=120"`
}

type UpdateEscalaRequest struct {
	UsuarioID     *string `json:"usuario_id" validate:"omitempty,uuid"`
	PostoID       *string `json:"posto_id" validate:"omitempty,uuid"`
	DataInicio    *string `json:"data_inicio" validate:"omitempty"`
	DataFim       *string `json:"data_fim" validate:"omitempty"`
	HoraInicio    *string `json:"hora_inicio" validate:"omitempty"`
	HoraFim       *string `json:"hora_fim" validate:"omitempty"`
	DiasSemana    []int16 `json:"dias_semana" validate:"omitempty,min=1,max=7,dive,min=0,max=6"`
	ToleranciaMin *int    `json:"tolerancia_min" validate:"omitempty,min=0,max=120"`
	Ativo         *bool   `json:"ativo"`
}

type EscalaFilter struct {
	UsuarioID string
	PostoID   string
	Ativo     *bool
	Data      string
	Limit     int
	Offset    int
}
