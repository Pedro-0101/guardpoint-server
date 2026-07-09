package model

import (
	"time"

	"github.com/google/uuid"
)

type SenhaVigia struct {
	ID        uuid.UUID  `json:"id"`
	EmpresaID uuid.UUID  `json:"empresa_id"`
	UsuarioID uuid.UUID  `json:"usuario_id"`
	Tipo      string     `json:"tipo"` // ok | emergencia | customizada
	Codigo    string     `json:"codigo"`
	Descricao *string    `json:"descricao,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

type CreateSenhaVigiaRequest struct {
	Tipo      string  `json:"tipo" validate:"required,oneof=ok emergencia customizada"`
	Codigo    string  `json:"codigo" validate:"required,numeric,min=2,max=6"`
	Descricao *string `json:"descricao" validate:"required_if=Tipo customizada,omitempty,max=255"`
}

type UpdateSenhaVigiaRequest struct {
	Codigo    *string `json:"codigo" validate:"omitempty,numeric,min=2,max=6"`
	Descricao *string `json:"descricao" validate:"omitempty,max=255"`
}
