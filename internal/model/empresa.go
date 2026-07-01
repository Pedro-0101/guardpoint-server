package model

import (
	"time"

	"github.com/google/uuid"
)

type Empresa struct {
	ID        uuid.UUID `json:"id"`
	Nome      string    `json:"nome"`
	CNPJ      string    `json:"cnpj"`
	Ativa     bool      `json:"ativa"`
	CreatedAt time.Time `json:"created_at"`
}
