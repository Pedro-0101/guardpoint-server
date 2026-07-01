package model

import (
	"time"

	"github.com/google/uuid"
)

type User struct {
	ID        uuid.UUID `json:"id"`
	EmpresaID uuid.UUID `json:"empresa_id"`
	Nome      string    `json:"nome"`
	Email     string    `json:"email"`
	SenhaHash string    `json:"-"`
	Role      string    `json:"role"`
	Telefone  *string   `json:"telefone,omitempty"`
	Ativo     bool      `json:"ativo"`
	CreatedAt time.Time `json:"created_at"`
}

type LoginRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"senha" validate:"required,min=6"`
}

type LoginResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"`
	User         User   `json:"usuario"`
}

type RegisterRequest struct {
	Nome     string `json:"nome" validate:"required,min=2,max=255"`
	Email    string `json:"email" validate:"required,email,max=255"`
	Password string `json:"senha" validate:"required,min=6,max=72"`
	Role     string `json:"role" validate:"required,oneof=admin supervisor vigia"`
	Telefone string `json:"telefone,omitempty" validate:"omitempty,max=20"`
}

type RefreshRequest struct {
	RefreshToken string `json:"refresh_token" validate:"required"`
}

type BiometricLoginRequest struct {
	EmpresaID string `json:"empresa_id" validate:"required,uuid"`
	DeviceID  string `json:"device_id" validate:"required"`
}

type BiometricRegisterRequest struct {
	DeviceID string `json:"device_id" validate:"required"`
}

type TokenClaims struct {
	UserID    string `json:"user_id"`
	EmpresaID string `json:"empresa_id"`
	Email     string `json:"email"`
	Role      string `json:"role"`
	Nome      string `json:"nome"`
}

type SessaoDispositivo struct {
	ID        uuid.UUID `json:"id"`
	UsuarioID uuid.UUID `json:"usuario_id"`
	EmpresaID uuid.UUID `json:"empresa_id"`
	DeviceID  string    `json:"device_id"`
	CriadoEm  time.Time `json:"criado_em"`
}
