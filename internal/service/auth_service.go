package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"github.com/guardpoint/guardpoint-server/internal/auth"
	"github.com/guardpoint/guardpoint-server/internal/model"
)

var (
	ErrInvalidCredentials        = errors.New("email ou senha invalidos")
	ErrEmailAlreadyExists        = errors.New("email ja cadastrado")
	ErrNomeAlreadyExists         = errors.New("nome ja cadastrado")
	ErrEmailRequired             = errors.New("email obrigatorio para este cargo")
	ErrUserNotActive             = errors.New("usuario inativo")
	ErrDispositivoNaoReconhecido = errors.New("dispositivo nao reconhecido")
)

type AuthUserRepository interface {
	FindByEmail(ctx context.Context, email string) (*model.User, error)
	FindByEmpresaNome(ctx context.Context, empresaID uuid.UUID, nome string) (*model.User, error)
	FindByID(ctx context.Context, id uuid.UUID) (*model.User, error)
	Create(ctx context.Context, u *model.User) error
}

type AuthEmpresaRepository interface {
	FindByCodigo(ctx context.Context, codigo string) (*model.Empresa, error)
}

type AuthSessaoDispositivoRepository interface {
	FindByDeviceID(ctx context.Context, empresaID, deviceID string) (*model.SessaoDispositivo, error)
	Create(ctx context.Context, s *model.SessaoDispositivo) error
	DeleteByDeviceID(ctx context.Context, empresaID, deviceID string) error
}

type AuthService struct {
	jwtService            *auth.JWTService
	userRepo              AuthUserRepository
	empresaRepo           AuthEmpresaRepository
	sessaoDispositivoRepo AuthSessaoDispositivoRepository
}

func NewAuthService(
	jwtService *auth.JWTService,
	userRepo AuthUserRepository,
	empresaRepo AuthEmpresaRepository,
	sessaoDispositivoRepo AuthSessaoDispositivoRepository,
) *AuthService {
	return &AuthService{
		jwtService:            jwtService,
		userRepo:              userRepo,
		empresaRepo:           empresaRepo,
		sessaoDispositivoRepo: sessaoDispositivoRepo,
	}
}

func (s *AuthService) Login(ctx context.Context, req model.LoginRequest) (*model.LoginResponse, error) {
	var user *model.User
	var err error
	if req.Email != "" {
		user, err = s.userRepo.FindByEmail(ctx, req.Email)
		if err != nil {
			return nil, fmt.Errorf("login: %w", ErrInvalidCredentials)
		}
	} else {
		empresa, eerr := s.empresaRepo.FindByCodigo(ctx, req.CodigoEmpresa)
		if eerr != nil {
			return nil, fmt.Errorf("login: %w", ErrInvalidCredentials)
		}
		user, err = s.userRepo.FindByEmpresaNome(ctx, empresa.ID, req.Nome)
		if err != nil {
			return nil, fmt.Errorf("login: %w", ErrInvalidCredentials)
		}
		if user.Role != "vigia" {
			return nil, fmt.Errorf("login: %w", ErrInvalidCredentials)
		}
	}

	if !user.Ativo {
		return nil, ErrUserNotActive
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.SenhaHash), []byte(req.Password)); err != nil {
		return nil, fmt.Errorf("login: %w", ErrInvalidCredentials)
	}

	accessToken, err := s.jwtService.GenerateAccessToken(user.ID, user.EmpresaID, user.EmailOrEmpty(), user.Role, user.Nome)
	if err != nil {
		return nil, fmt.Errorf("gerar access token: %w", err)
	}

	refreshToken, err := s.jwtService.GenerateRefreshToken(user.ID)
	if err != nil {
		return nil, fmt.Errorf("gerar refresh token: %w", err)
	}

	return &model.LoginResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    int64(auth.AccessTokenDuration.Seconds()),
		User:         *user,
	}, nil
}

func (s *AuthService) Register(ctx context.Context, empresaID string, req model.RegisterRequest) (*model.User, error) {
	parsedID, err := uuid.Parse(empresaID)
	if err != nil {
		return nil, fmt.Errorf("empresa_id invalido: %w", err)
	}

	if req.Email != "" {
		if existing, err := s.userRepo.FindByEmail(ctx, req.Email); err == nil && existing != nil {
			return nil, ErrEmailAlreadyExists
		}
	}
	if existing, err := s.userRepo.FindByEmpresaNome(ctx, parsedID, req.Nome); err == nil && existing != nil {
		return nil, ErrNomeAlreadyExists
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("hash senha: %w", err)
	}

	user := &model.User{
		EmpresaID: parsedID,
		Nome:      req.Nome,
		SenhaHash: string(hashedPassword),
		Role:      req.Role,
	}
	if req.Email != "" {
		email := req.Email
		user.Email = &email
	}

	if req.Telefone != "" {
		user.Telefone = &req.Telefone
	}

	if err := s.userRepo.Create(ctx, user); err != nil {
		return nil, fmt.Errorf("criar usuario: %w", err)
	}

	return user, nil
}

func (s *AuthService) Refresh(ctx context.Context, req model.RefreshRequest) (*model.LoginResponse, error) {
	userID, err := s.jwtService.ValidateRefreshToken(req.RefreshToken)
	if err != nil {
		return nil, fmt.Errorf("refresh: %w", err)
	}

	parsedID, err := uuid.Parse(userID)
	if err != nil {
		return nil, fmt.Errorf("refresh: user_id invalido no token: %w", err)
	}

	user, err := s.userRepo.FindByID(ctx, parsedID)
	if err != nil {
		return nil, fmt.Errorf("refresh: %w", err)
	}

	if !user.Ativo {
		return nil, ErrUserNotActive
	}

	accessToken, err := s.jwtService.GenerateAccessToken(user.ID, user.EmpresaID, user.EmailOrEmpty(), user.Role, user.Nome)
	if err != nil {
		return nil, fmt.Errorf("gerar access token: %w", err)
	}

	refreshToken, err := s.jwtService.GenerateRefreshToken(user.ID)
	if err != nil {
		return nil, fmt.Errorf("gerar refresh token: %w", err)
	}

	return &model.LoginResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    int64(auth.AccessTokenDuration.Seconds()),
		User:         *user,
	}, nil
}

func (s *AuthService) Logout(ctx context.Context, empresaID, deviceID string) error {
	if deviceID != "" {
		if err := s.sessaoDispositivoRepo.DeleteByDeviceID(ctx, empresaID, deviceID); err != nil {
			return fmt.Errorf("logout: %w", err)
		}
	}
	return nil
}

func (s *AuthService) BiometricLogin(ctx context.Context, req model.BiometricLoginRequest) (*model.LoginResponse, error) {
	sessao, err := s.sessaoDispositivoRepo.FindByDeviceID(ctx, req.EmpresaID, req.DeviceID)
	if err != nil {
		return nil, fmt.Errorf("biometric: %w", ErrDispositivoNaoReconhecido)
	}

	// device_id sozinho nao autentica: o aparelho precisa apresentar o
	// device_secret entregue no registro. Sessoes antigas (hash NULL) exigem
	// novo registro biometrico.
	if sessao.DeviceSecretHash == nil || !deviceSecretConfere(req.DeviceSecret, *sessao.DeviceSecretHash) {
		return nil, fmt.Errorf("biometric: %w", ErrDispositivoNaoReconhecido)
	}

	user, err := s.userRepo.FindByID(ctx, sessao.UsuarioID)
	if err != nil {
		return nil, fmt.Errorf("biometric: %w", err)
	}

	if !user.Ativo {
		return nil, ErrUserNotActive
	}

	accessToken, err := s.jwtService.GenerateAccessToken(user.ID, user.EmpresaID, user.EmailOrEmpty(), user.Role, user.Nome)
	if err != nil {
		return nil, fmt.Errorf("gerar access token: %w", err)
	}

	refreshToken, err := s.jwtService.GenerateRefreshToken(user.ID)
	if err != nil {
		return nil, fmt.Errorf("gerar refresh token: %w", err)
	}

	return &model.LoginResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    int64(auth.AccessTokenDuration.Seconds()),
		User:         *user,
	}, nil
}

func (s *AuthService) RegisterBiometric(ctx context.Context, userID, empresaID string, req model.BiometricRegisterRequest) (*model.BiometricRegisterResponse, error) {
	parsedUserID, err := uuid.Parse(userID)
	if err != nil {
		return nil, fmt.Errorf("register biometric: user_id invalido: %w", err)
	}

	parsedEmpresaID, err := uuid.Parse(empresaID)
	if err != nil {
		return nil, fmt.Errorf("register biometric: empresa_id invalido: %w", err)
	}

	secret, err := gerarDeviceSecret()
	if err != nil {
		return nil, fmt.Errorf("register biometric: gerar device_secret: %w", err)
	}
	secretHash := hashDeviceSecret(secret)

	sessao := &model.SessaoDispositivo{
		UsuarioID:        parsedUserID,
		EmpresaID:        parsedEmpresaID,
		DeviceID:         req.DeviceID,
		DeviceSecretHash: &secretHash,
	}

	if err := s.sessaoDispositivoRepo.Create(ctx, sessao); err != nil {
		return nil, fmt.Errorf("register biometric: %w", err)
	}

	return &model.BiometricRegisterResponse{
		SessaoDispositivo: *sessao,
		DeviceSecret:      secret,
	}, nil
}

func gerarDeviceSecret() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func hashDeviceSecret(secret string) string {
	sum := sha256.Sum256([]byte(secret))
	return hex.EncodeToString(sum[:])
}

func deviceSecretConfere(secret, hashArmazenado string) bool {
	calculado := hashDeviceSecret(secret)
	return subtle.ConstantTimeCompare([]byte(calculado), []byte(hashArmazenado)) == 1
}
