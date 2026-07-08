package model

import (
	"time"

	"github.com/google/uuid"
)

type SenhaVigia struct {
	ID                   uuid.UUID  `json:"id"`
	EmpresaID            uuid.UUID  `json:"empresa_id"`
	UsuarioID            uuid.UUID  `json:"usuario_id"`
	Tipo                 string     `json:"tipo"` // ok | emergencia | customizada
	Codigo               string     `json:"codigo"`
	Descricao            *string    `json:"descricao,omitempty"`
	NivelEscalonamentoID *uuid.UUID `json:"nivel_escalonamento_id,omitempty"` // NULL apenas para tipo "ok"; obrigatorio para emergencia/customizada
	CreatedAt            time.Time  `json:"created_at"`
	UpdatedAt            time.Time  `json:"updated_at"`
}

type CreateSenhaVigiaRequest struct {
	Tipo                 string  `json:"tipo" validate:"required,oneof=ok emergencia customizada"`
	Codigo               string  `json:"codigo" validate:"required,numeric,min=4,max=6"`
	Descricao            *string `json:"descricao" validate:"required_if=Tipo customizada,omitempty,max=255"`
	NivelEscalonamentoID *string `json:"nivel_escalonamento_id" validate:"omitempty,uuid"` // obrigatorio para emergencia/customizada (validado no service)
}

type UpdateSenhaVigiaRequest struct {
	Codigo               *string `json:"codigo" validate:"omitempty,numeric,min=4,max=6"`
	Descricao            *string `json:"descricao" validate:"omitempty,max=255"`
	NivelEscalonamentoID *string `json:"nivel_escalonamento_id" validate:"omitempty,uuid"` // somente customizada pode alterar
}
