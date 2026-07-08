package model

import (
	"time"

	"github.com/google/uuid"
)

type SenhaVigia struct {
	ID                   uuid.UUID   `json:"id"`
	EmpresaID            uuid.UUID   `json:"empresa_id"`
	UsuarioID            uuid.UUID   `json:"usuario_id"`
	Tipo                 string      `json:"tipo"`
	Codigo               string      `json:"codigo"`
	Descricao            *string     `json:"descricao,omitempty"`
	AtrasoMinutos        int         `json:"atraso_minutos"`
	Destinatarios        []uuid.UUID `json:"destinatarios"`
	CreatedAt            time.Time   `json:"created_at"`
	UpdatedAt            time.Time   `json:"updated_at"`
}

type CreateSenhaVigiaRequest struct {
	Tipo          string      `json:"tipo" validate:"required,oneof=ok emergencia customizada"`
	Codigo        string      `json:"codigo" validate:"required,numeric,min=4,max=6"`
	Descricao     *string     `json:"descricao" validate:"required_if=Tipo customizada,omitempty,max=255"`
	AtrasoMinutos int         `json:"atraso_minutos" validate:"omitempty,min=0,max=1440"`
	Destinatarios []uuid.UUID `json:"destinatarios" validate:"omitempty,min=1"`
}

type UpdateSenhaVigiaRequest struct {
	Codigo        *string     `json:"codigo" validate:"omitempty,numeric,min=4,max=6"`
	Descricao     *string     `json:"descricao" validate:"omitempty,max=255"`
	AtrasoMinutos *int        `json:"atraso_minutos" validate:"omitempty,min=0,max=1440"`
	Destinatarios []uuid.UUID `json:"destinatarios" validate:"omitempty,min=1"`
}
