package service

import (
	"context"

	"github.com/google/uuid"

	"github.com/guardpoint/guardpoint-server/internal/model"
	"github.com/guardpoint/guardpoint-server/internal/repository"
)

type EmpresaService struct {
	empresaRepo *repository.EmpresaRepository
}

func NewEmpresaService(empresaRepo *repository.EmpresaRepository) *EmpresaService {
	return &EmpresaService{empresaRepo: empresaRepo}
}

func (s *EmpresaService) Get(ctx context.Context, empresaID uuid.UUID) (*model.Empresa, error) {
	return s.empresaRepo.FindByID(ctx, empresaID)
}

func (s *EmpresaService) Update(ctx context.Context, empresaID uuid.UUID, req model.UpdateEmpresaRequest) (*model.Empresa, error) {
	return s.empresaRepo.Update(ctx, empresaID, req.Nome, req.AlertaSonoro)
}
