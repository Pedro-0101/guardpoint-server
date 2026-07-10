package model

import (
	"time"

	"github.com/google/uuid"
)

type Posto struct {
	ID        uuid.UUID `json:"id" example:"550e8400-e29b-41d4-a716-446655440000"`
	EmpresaID uuid.UUID `json:"empresa_id" example:"660e8400-e29b-41d4-a716-446655440000"`
	Nome      string    `json:"nome" example:"Portaria Principal"`
	Latitude  float64   `json:"latitude" example:"-23.550520"`
	Longitude float64   `json:"longitude" example:"-46.633308"`
	RaioM     int       `json:"raio_m" example:"100"`
	Ativo     bool      `json:"ativo" example:"true"`
	CreatedAt time.Time `json:"created_at"`
}

type CreatePostoRequest struct {
	Nome      string  `json:"nome" validate:"required,min=2,max=255" example:"Portaria Principal"`
	Latitude  float64 `json:"latitude" validate:"required,latitude" example:"-23.550520"`
	Longitude float64 `json:"longitude" validate:"required,longitude" example:"-46.633308"`
	RaioM     int     `json:"raio_m" validate:"omitempty,min=10,max=5000" example:"100"`
}

type UpdatePostoRequest struct {
	Nome      string  `json:"nome" validate:"omitempty,min=2,max=255" example:"Portaria Norte"`
	Latitude  float64 `json:"latitude" validate:"omitempty,latitude" example:"-23.551000"`
	Longitude float64 `json:"longitude" validate:"omitempty,longitude" example:"-46.634000"`
	RaioM     int     `json:"raio_m" validate:"omitempty,min=10,max=5000" example:"150"`
	Ativo     *bool   `json:"ativo"`
}

type PostoSupervisorRequest struct {
	SupervisorID uuid.UUID `json:"supervisor_id" validate:"required" example:"550e8400-e29b-41d4-a716-446655440000"`
}

type PostoSupervisorResponse struct {
	PostoID      uuid.UUID `json:"posto_id" example:"660e8400-e29b-41d4-a716-446655440000"`
	SupervisorID uuid.UUID `json:"supervisor_id" example:"550e8400-e29b-41d4-a716-446655440000"`
}

type SupervisorPostoResponse struct {
	PostoID   uuid.UUID `json:"posto_id" example:"660e8400-e29b-41d4-a716-446655440000"`
	PostoNome string    `json:"posto_nome" example:"Portaria Principal"`
}
