package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"github.com/guardpoint/guardpoint-server/internal/auth"
	"github.com/guardpoint/guardpoint-server/internal/model"
	"github.com/guardpoint/guardpoint-server/internal/repository"
)

var (
	ErrInvalidCredentials = errors.New("email ou senha invalidos")
	ErrEmailAlreadyExists = errors.New("email ja cadastrado")
	ErrUserNotActive      = errors.New("usuario inativo")
)

type AuthService struct {
	jwtService            *auth.JWTService
	userRepo              *repository.UserRepository
	empresaRepo           *repository.EmpresaRepository
	sessaoDispositivoRepo *repository.SessaoDispositivoRepository
}

func NewAuthService(
	jwtService *auth.JWTService,
	userRepo *repository.UserRepository,
	empresaRepo *repository.EmpresaRepository,
	sessaoDispositivoRepo *repository.SessaoDispositivoRepository,
) *AuthService {
	return &AuthService{
		jwtService:            jwtService,
		userRepo:              userRepo,
		empresaRepo:           empresaRepo,
		sessaoDispositivoRepo: sessaoDispositivoRepo,
	}
}

func (s *AuthService) Login(ctx context.Context, req model.LoginRequest) (*model.LoginResponse, error) {
	user, err := s.userRepo.FindByEmail(ctx, req.Email)
	if err != nil {
		return nil, fmt.Errorf("login: %w", ErrInvalidCredentials)
	}

	if !user.Ativo {
		return nil, ErrUserNotActive
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.SenhaHash), []byte(req.Password)); err != nil {
		return nil, fmt.Errorf("login: %w", ErrInvalidCredentials)
	}

	accessToken, err := s.jwtService.GenerateAccessToken(user.ID, user.EmpresaID, user.Email, user.Role, user.Nome)
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
	existing, err := s.userRepo.FindByEmail(ctx, req.Email)
	if err == nil && existing != nil {
		return nil, ErrEmailAlreadyExists
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("hash senha: %w", err)
	}

	user := &model.User{
		Nome:      req.Nome,
		Email:     req.Email,
		SenhaHash: string(hashedPassword),
		Role:      req.Role,
	}

	if req.Telefone != "" {
		user.Telefone = &req.Telefone
	}

	parsedID, err := uuid.Parse(empresaID)
	if err != nil {
		return nil, fmt.Errorf("empresa_id invalido: %w", err)
	}
	user.EmpresaID = parsedID

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

	accessToken, err := s.jwtService.GenerateAccessToken(user.ID, user.EmpresaID, user.Email, user.Role, user.Nome)
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
		return nil, fmt.Errorf("biometric: %w", err)
	}

	user, err := s.userRepo.FindByID(ctx, sessao.UsuarioID)
	if err != nil {
		return nil, fmt.Errorf("biometric: %w", err)
	}

	if !user.Ativo {
		return nil, ErrUserNotActive
	}

	accessToken, err := s.jwtService.GenerateAccessToken(user.ID, user.EmpresaID, user.Email, user.Role, user.Nome)
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

func (s *AuthService) RegisterBiometric(ctx context.Context, userID, empresaID string, req model.BiometricRegisterRequest) (*model.SessaoDispositivo, error) {
	parsedUserID, err := uuid.Parse(userID)
	if err != nil {
		return nil, fmt.Errorf("register biometric: user_id invalido: %w", err)
	}

	parsedEmpresaID, err := uuid.Parse(empresaID)
	if err != nil {
		return nil, fmt.Errorf("register biometric: empresa_id invalido: %w", err)
	}

	sessao := &model.SessaoDispositivo{
		UsuarioID: parsedUserID,
		EmpresaID: parsedEmpresaID,
		DeviceID:  req.DeviceID,
	}

	if err := s.sessaoDispositivoRepo.Create(ctx, sessao); err != nil {
		return nil, fmt.Errorf("register biometric: %w", err)
	}

	return sessao, nil
}
