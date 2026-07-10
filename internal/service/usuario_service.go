package service

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"github.com/guardpoint/guardpoint-server/internal/model"
)

type UserRepository interface {
	ListByEmpresa(ctx context.Context, empresaID uuid.UUID) ([]model.User, error)
	FindByIDEmpresa(ctx context.Context, empresaID, id uuid.UUID) (*model.User, error)
	FindByEmail(ctx context.Context, email string) (*model.User, error)
	FindByEmpresaNome(ctx context.Context, empresaID uuid.UUID, nome string) (*model.User, error)
	Create(ctx context.Context, u *model.User) error
	Update(ctx context.Context, empresaID, id uuid.UUID, u *model.User) error
}

type UsuarioService struct {
	userRepo UserRepository
}

func NewUsuarioService(userRepo UserRepository) *UsuarioService {
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
	if req.Email != "" {
		if existing, err := s.userRepo.FindByEmail(ctx, req.Email); err == nil && existing != nil {
			return nil, ErrEmailAlreadyExists
		}
	}
	if existing, err := s.userRepo.FindByEmpresaNome(ctx, empresaID, req.Nome); err == nil && existing != nil {
		return nil, ErrNomeAlreadyExists
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
		SenhaHash: string(hashedPassword),
		Role:      req.Cargo,
		Ativo:     ativo,
	}
	if req.Email != "" {
		email := req.Email
		user.Email = &email
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
		user.Email = req.Email
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

	if user.Role != "vigia" && (user.Email == nil || *user.Email == "") {
		return nil, ErrEmailRequired
	}

	if user.Email != nil && *user.Email != "" {
		if existing, err := s.userRepo.FindByEmail(ctx, *user.Email); err == nil && existing != nil && existing.ID != id {
			return nil, ErrEmailAlreadyExists
		}
	}
	if existing, err := s.userRepo.FindByEmpresaNome(ctx, empresaID, user.Nome); err == nil && existing != nil && existing.ID != id {
		return nil, ErrNomeAlreadyExists
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
