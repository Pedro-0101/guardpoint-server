package model

import (
	"time"

	"github.com/google/uuid"
)

type Empresa struct {
	ID           uuid.UUID `json:"id"`
	Nome         string    `json:"nome"`
	CNPJ         string    `json:"cnpj"`
	Ativa        bool      `json:"ativa"`
	AlertaSonoro bool      `json:"alerta_sonoro"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type UpdateEmpresaRequest struct {
	Nome         string `json:"nome" validate:"required,max=255"`
	AlertaSonoro bool   `json:"alerta_sonoro"`
}
