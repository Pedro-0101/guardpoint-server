package model

import (
	"time"

	"github.com/google/uuid"
)

type Turno struct {
	ID             uuid.UUID  `json:"id"`
	EmpresaID      uuid.UUID  `json:"empresa_id"`
	UsuarioID      uuid.UUID  `json:"usuario_id"`
	PostoID        uuid.UUID  `json:"posto_id"`
	PostoNome      string     `json:"posto_nome,omitempty"`
	Status         string     `json:"status"`
	InicioPrevisto time.Time  `json:"inicio_previsto"`
	FimPrevisto    time.Time  `json:"fim_previsto"`
	InicioReal     *time.Time `json:"inicio_real,omitempty"`
	FimReal        *time.Time `json:"fim_real,omitempty"`
	TokenSessao    *string    `json:"token_sessao,omitempty"`
	DeviceID       *string    `json:"device_id,omitempty"`
	IntervaloMin   int        `json:"intervalo_min"`
	// Pin so e preenchido pela consulta dedicada do fluxo de reassociacao;
	// nunca e serializado para nao vazar o PIN ativo em listagens.
	Pin          *string    `json:"-"`
	PinValidoAte *time.Time `json:"-"`
	CreatedAt    time.Time  `json:"created_at"`
}

type TurnoDetalhe struct {
	Turno
	Posto    *Posto    `json:"posto,omitempty"`
	Usuario  *User     `json:"usuario,omitempty"`
	Checkins []Checkin `json:"checkins,omitempty"`
}

type IniciarTurnoRequest struct {
	PostoID      string `json:"posto_id" validate:"required,uuid"`
	DeviceID     string `json:"device_id" validate:"required"`
	IntervaloMin int    `json:"intervalo_min" validate:"omitempty,min=1,max=120"`
}

type CheckinRequest struct {
	TurnoID          string  `json:"turno_id" validate:"required,uuid"`
	DeviceID         string  `json:"device_id" validate:"required"`
	Latitude         float64 `json:"latitude" validate:"required,latitude"`
	Longitude        float64 `json:"longitude" validate:"required,longitude"`
	TipoSenha        string  `json:"tipo_senha" validate:"required,oneof=padrao coacao finalizacao sabotagem"`
	Timestamp        string  `json:"timestamp" validate:"required"`
	ClienteCheckinID string  `json:"cliente_checkin_id" validate:"omitempty,uuid"`
}

type FinalizarTurnoRequest struct {
	TurnoID   string  `json:"turno_id" validate:"required,uuid"`
	DeviceID  string  `json:"device_id" validate:"required"`
	Latitude  float64 `json:"latitude" validate:"required,latitude"`
	Longitude float64 `json:"longitude" validate:"required,longitude"`
	Timestamp string  `json:"timestamp" validate:"required"`
}

type TurnoStatusResponse struct {
	Turno           Turno      `json:"turno"`
	UltimoCheckin   *Checkin   `json:"ultimo_checkin,omitempty"`
	ProximoDeadline *time.Time `json:"proximo_deadline,omitempty"`
	CheckinsHoje    int        `json:"checkins_hoje"`
}

type CheckinResponse struct {
	Checkin         Checkin    `json:"checkin"`
	Status          string     `json:"status"`
	PostoNome       string     `json:"posto_nome,omitempty"`
	ProximoDeadline *time.Time `json:"proximo_deadline,omitempty"`
	Atrasado        bool       `json:"atrasado"`
}

type SabotagemRequest struct {
	TurnoID   string  `json:"turno_id" validate:"required,uuid"`
	DeviceID  string  `json:"device_id" validate:"required"`
	Latitude  float64 `json:"latitude" validate:"required,latitude"`
	Longitude float64 `json:"longitude" validate:"required,longitude"`
	Motivo    string  `json:"motivo" validate:"required,min=3,max=500"`
	Timestamp string  `json:"timestamp" validate:"required"`
}

type SabotagemResponse struct {
	AlertaID string `json:"alerta_id"`
	Status   string `json:"status"`
	Mensagem string `json:"mensagem"`
}

type RevogarResponse struct {
	PinNovoDispositivo string `json:"pin_novo_dispositivo"`
	ValidadeMinutos    int    `json:"validade_minutos"`
}

type ReassociarRequest struct {
	Pin      string `json:"pin" validate:"required,len=6,numeric"`
	DeviceID string `json:"device_id" validate:"required"`
}

type HistoricoFilter struct {
	DataInicio string `json:"data_inicio"`
	DataFim    string `json:"data_fim"`
	UsuarioID  string `json:"usuario_id"`
	PostoID    string `json:"posto_id"`
	Status     string `json:"status"`
	Limit      int    `json:"limit"`
	Offset     int    `json:"offset"`
}
