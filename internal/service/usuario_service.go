package service

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"github.com/guardpoint/guardpoint-server/internal/model"
	"github.com/guardpoint/guardpoint-server/internal/repository"
)

type UsuarioService struct {
	userRepo *repository.UserRepository
}

func NewUsuarioService(userRepo *repository.UserRepository) *UsuarioService {
	return &UsuarioService{userRepo: userRepo}
}

func (s *UsuarioService) List(ctx context.Context, empresaID uuid.UUID) ([]model.UsuarioResponse, error) {
	usuarios, err := s.userRepo.ListByEmpresa(ctx, empresaID)
	if err != nil {
		return nil, fmt.Errorf("listar usuarios: %w", err)
	}

	result := make([]model.UsuarioResponse, 0, len(usuarios))
	for i := range usuarios {
		result = append(result, model.ToUsuarioResponse(&usuarios[i]))
	}
	return result, nil
}

func (s *UsuarioService) GetByID(ctx context.Context, empresaID, id uuid.UUID) (*model.UsuarioResponse, error) {
	user, err := s.userRepo.FindByIDEmpresa(ctx, empresaID, id)
	if err != nil {
		return nil, fmt.Errorf("buscar usuario: %w", err)
	}

	resp := model.ToUsuarioResponse(user)
	return &resp, nil
}

func (s *UsuarioService) Create(ctx context.Context, empresaID uuid.UUID, req model.CreateUsuarioRequest) (*model.UsuarioResponse, error) {
	existing, err := s.userRepo.FindByEmail(ctx, req.Email)
	if err == nil && existing != nil {
		return nil, ErrEmailAlreadyExists
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Senha), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("hash senha: %w", err)
	}

	ativo := true
	if req.Ativo != nil {
		ativo = *req.Ativo
	}

	user := &model.User{
		EmpresaID: empresaID,
		Nome:      req.Nome,
		Email:     req.Email,
		SenhaHash: string(hashedPassword),
		Role:      req.Cargo,
		Ativo:     ativo,
	}

	if err := s.userRepo.Create(ctx, user); err != nil {
		return nil, fmt.Errorf("criar usuario: %w", err)
	}

	resp := model.ToUsuarioResponse(user)
	return &resp, nil
}

func (s *UsuarioService) Update(ctx context.Context, empresaID, id uuid.UUID, req model.UpdateUsuarioRequest) (*model.UsuarioResponse, error) {
	user, err := s.userRepo.FindByIDEmpresa(ctx, empresaID, id)
	if err != nil {
		return nil, fmt.Errorf("buscar usuario: %w", err)
	}

	if req.Nome != nil {
		user.Nome = *req.Nome
	}
	if req.Email != nil {
		user.Email = *req.Email
	}
	if req.Cargo != nil {
		user.Role = *req.Cargo
	}
	if req.Ativo != nil {
		user.Ativo = *req.Ativo
	}
	if req.Senha != nil && *req.Senha != "" {
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(*req.Senha), bcrypt.DefaultCost)
		if err != nil {
			return nil, fmt.Errorf("hash senha: %w", err)
		}
		user.SenhaHash = string(hashedPassword)
	}

	if err := s.userRepo.Update(ctx, empresaID, id, user); err != nil {
		return nil, fmt.Errorf("atualizar usuario: %w", err)
	}

	resp := model.ToUsuarioResponse(user)
	return &resp, nil
}

func (s *UsuarioService) Deactivate(ctx context.Context, empresaID, id uuid.UUID) error {
	user, err := s.userRepo.FindByIDEmpresa(ctx, empresaID, id)
	if err != nil {
		return fmt.Errorf("buscar usuario: %w", err)
	}

	user.Ativo = false
	return s.userRepo.Update(ctx, empresaID, id, user)
}
