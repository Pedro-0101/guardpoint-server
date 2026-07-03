package model

import (
	"time"

	"github.com/google/uuid"
)

type User struct {
	ID        uuid.UUID  `json:"id"`
	EmpresaID uuid.UUID  `json:"empresa_id"`
	Nome      string     `json:"nome"`
	Email     string     `json:"email"`
	SenhaHash string     `json:"-"`
	Role      string     `json:"role"`
	Telefone  *string    `json:"telefone,omitempty"`
	Ativo     bool       `json:"ativo"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt *time.Time `json:"updated_at,omitempty"`
}

type CreateUsuarioRequest struct {
	Nome  string `json:"nome" validate:"required,min=2,max=255"`
	Email string `json:"email" validate:"required,email,max=255"`
	Senha string `json:"senha" validate:"required,min=6,max=72"`
	Cargo string `json:"cargo" validate:"required,oneof=admin supervisor vigia"`
	Ativo *bool  `json:"ativo"`
}

type UpdateUsuarioRequest struct {
	Nome  *string `json:"nome" validate:"omitempty,min=2,max=255"`
	Email *string `json:"email" validate:"omitempty,email,max=255"`
	Cargo *string `json:"cargo" validate:"omitempty,oneof=admin supervisor vigia"`
	Ativo *bool   `json:"ativo"`
	Senha *string `json:"senha" validate:"omitempty,min=6,max=72"`
}

type UsuarioResponse struct {
	ID        uuid.UUID  `json:"id"`
	Nome      string     `json:"nome"`
	Email     string     `json:"email"`
	Cargo     string     `json:"cargo"`
	EmpresaID uuid.UUID  `json:"empresaId"`
	Ativo     bool       `json:"ativo"`
	CreatedAt time.Time  `json:"createdAt"`
	UpdatedAt *time.Time `json:"updatedAt,omitempty"`
}

func ToUsuarioResponse(u *User) UsuarioResponse {
	return UsuarioResponse{
		ID:        u.ID,
		Nome:      u.Nome,
		Email:     u.Email,
		Cargo:     u.Role,
		EmpresaID: u.EmpresaID,
		Ativo:     u.Ativo,
		CreatedAt: u.CreatedAt,
		UpdatedAt: u.UpdatedAt,
	}
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
	EmpresaID    string `json:"empresa_id" validate:"required,uuid"`
	DeviceID     string `json:"device_id" validate:"required"`
	DeviceSecret string `json:"device_secret" validate:"required"`
}

type BiometricRegisterRequest struct {
	DeviceID string `json:"device_id" validate:"required"`
}

// BiometricRegisterResponse carrega o device_secret gerado no registro.
// Ele e entregue UMA unica vez; o servidor guarda apenas o hash.
type BiometricRegisterResponse struct {
	SessaoDispositivo
	DeviceSecret string `json:"device_secret"`
}

type TokenClaims struct {
	UserID    string `json:"user_id"`
	EmpresaID string `json:"empresa_id"`
	Email     string `json:"email"`
	Role      string `json:"role"`
	Nome      string `json:"nome"`
}

type SessaoDispositivo struct {
	ID               uuid.UUID `json:"id"`
	UsuarioID        uuid.UUID `json:"usuario_id"`
	EmpresaID        uuid.UUID `json:"empresa_id"`
	DeviceID         string    `json:"device_id"`
	DeviceSecretHash *string   `json:"-"`
	CriadoEm         time.Time `json:"criado_em"`
}
