package service

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/guardpoint/guardpoint-server/internal/model"
)

type PostoRepository interface {
	Create(ctx context.Context, p *model.Posto) error
	FindByID(ctx context.Context, empresaID, id uuid.UUID) (*model.Posto, error)
	List(ctx context.Context, empresaID uuid.UUID, apenasAtivos bool) ([]model.Posto, error)
	Update(ctx context.Context, empresaID, id uuid.UUID, p *model.Posto) error
}

type PostoService struct {
	postoRepo PostoRepository
}

func NewPostoService(postoRepo PostoRepository) *PostoService {
	return &PostoService{postoRepo: postoRepo}
}

func (s *PostoService) Create(ctx context.Context, p *model.Posto) error {
	return s.postoRepo.Create(ctx, p)
}

func (s *PostoService) GetByID(ctx context.Context, empresaID, id uuid.UUID) (*model.Posto, error) {
	return s.postoRepo.FindByID(ctx, empresaID, id)
}

func (s *PostoService) List(ctx context.Context, empresaID uuid.UUID, apenasAtivos bool) ([]model.Posto, error) {
	return s.postoRepo.List(ctx, empresaID, apenasAtivos)
}

func (s *PostoService) Update(ctx context.Context, empresaID, id uuid.UUID, p *model.Posto) error {
	return s.postoRepo.Update(ctx, empresaID, id, p)
}

func (s *PostoService) Deactivate(ctx context.Context, empresaID, id uuid.UUID) error {
	posto, err := s.postoRepo.FindByID(ctx, empresaID, id)
	if err != nil {
		return fmt.Errorf("posto nao encontrado: %w", err)
	}

	posto.Ativo = false
	return s.postoRepo.Update(ctx, empresaID, id, posto)
}
