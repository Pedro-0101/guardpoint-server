package model

import (
	"time"

	"github.com/google/uuid"
)

type Posto struct {
	ID        uuid.UUID `json:"id"`
	EmpresaID uuid.UUID `json:"empresa_id"`
	Nome      string    `json:"nome"`
	Latitude  float64   `json:"latitude"`
	Longitude float64   `json:"longitude"`
	RaioM     int       `json:"raio_m"`
	Ativo     bool      `json:"ativo"`
	CreatedAt time.Time `json:"created_at"`
}

type CreatePostoRequest struct {
	Nome      string  `json:"nome" validate:"required,min=2,max=255"`
	Latitude  float64 `json:"latitude" validate:"required,latitude"`
	Longitude float64 `json:"longitude" validate:"required,longitude"`
	RaioM     int     `json:"raio_m" validate:"omitempty,min=10,max=5000"`
}

type UpdatePostoRequest struct {
	Nome      string  `json:"nome" validate:"omitempty,min=2,max=255"`
	Latitude  float64 `json:"latitude" validate:"omitempty,latitude"`
	Longitude float64 `json:"longitude" validate:"omitempty,longitude"`
	RaioM     int     `json:"raio_m" validate:"omitempty,min=10,max=5000"`
	Ativo     *bool   `json:"ativo"`
}
