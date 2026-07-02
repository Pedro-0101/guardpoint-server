package model

import (
	"time"

	"github.com/google/uuid"
)

type Checkin struct {
	ID                   uuid.UUID `json:"id"`
	TurnoID              uuid.UUID `json:"turno_id"`
	EmpresaID            uuid.UUID `json:"empresa_id"`
	Latitude             float64   `json:"latitude"`
	Longitude            float64   `json:"longitude"`
	TimestampCriacao     time.Time `json:"timestamp_criacao"`
	TimestampRecebimento time.Time `json:"timestamp_recebimento"`
	TipoSenha            string    `json:"tipo_senha"`
	FlagGeofence         *string   `json:"flag_geofence,omitempty"`
	OrigemRede           string    `json:"origem_rede"`
	ClienteCheckinID     *string   `json:"cliente_checkin_id,omitempty"`
	CreatedAt            time.Time `json:"created_at"`
}
