package model

import (
	"time"

	"github.com/google/uuid"
)

type Substituicao struct {
	ID            uuid.UUID `json:"id"`
	EmpresaID     uuid.UUID `json:"empresa_id"`
	UsuarioID     uuid.UUID `json:"usuario_id"`
	PostoID       uuid.UUID `json:"posto_id"`
	DataInicio    time.Time `json:"data_inicio"`
	DataFim       time.Time `json:"data_fim"`
	HoraInicio    string    `json:"hora_inicio"`
	HoraFim       string    `json:"hora_fim"`
	ToleranciaMin int       `json:"tolerancia_min"`
	Motivo        string    `json:"motivo,omitempty"`
	Ativo         bool      `json:"ativo"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`

	UsuarioNome string `json:"usuario_nome,omitempty"`
	PostoNome   string `json:"posto_nome,omitempty"`
}

type CreateSubstituicaoRequest struct {
	UsuarioID     string `json:"usuario_id" validate:"required,uuid"`
	PostoID       string `json:"posto_id" validate:"required,uuid"`
	DataInicio    string `json:"data_inicio" validate:"required"`
	DataFim       string `json:"data_fim" validate:"required"`
	HoraInicio    string `json:"hora_inicio" validate:"required"`
	HoraFim       string `json:"hora_fim" validate:"required"`
	ToleranciaMin int    `json:"tolerancia_min" validate:"omitempty,min=0,max=120"`
	Motivo        string `json:"motivo" validate:"omitempty,max=255"`
}

type UpdateSubstituicaoRequest struct {
	UsuarioID     *string `json:"usuario_id" validate:"omitempty,uuid"`
	PostoID       *string `json:"posto_id" validate:"omitempty,uuid"`
	DataInicio    *string `json:"data_inicio" validate:"omitempty"`
	DataFim       *string `json:"data_fim" validate:"omitempty"`
	HoraInicio    *string `json:"hora_inicio" validate:"omitempty"`
	HoraFim       *string `json:"hora_fim" validate:"omitempty"`
	ToleranciaMin *int    `json:"tolerancia_min" validate:"omitempty,min=0,max=120"`
	Motivo        *string `json:"motivo" validate:"omitempty,max=255"`
	Ativo         *bool   `json:"ativo"`
}

type SubstituicaoListResponse struct {
	Data  []Substituicao `json:"data"`
	Total int            `json:"total"`
}

type SubstituicaoFilter struct {
	UsuarioID string
	PostoID   string
	Data      string
	Ativo     *bool
	Limit     int
	Offset    int
}
