package service

import (
	"context"

	"github.com/google/uuid"

	"github.com/guardpoint/guardpoint-server/internal/model"
	"github.com/guardpoint/guardpoint-server/internal/repository"
)

type EmpresaService struct {
	empresaRepo             *repository.EmpresaRepository
	configEscalonamentoRepo *repository.ConfigEscalonamentoRepository
}

func NewEmpresaService(empresaRepo *repository.EmpresaRepository, configEscalonamentoRepo *repository.ConfigEscalonamentoRepository) *EmpresaService {
	return &EmpresaService{empresaRepo: empresaRepo, configEscalonamentoRepo: configEscalonamentoRepo}
}

func (s *EmpresaService) Get(ctx context.Context, empresaID uuid.UUID) (*model.Empresa, error) {
	return s.empresaRepo.FindByID(ctx, empresaID)
}

func (s *EmpresaService) Update(ctx context.Context, empresaID uuid.UUID, req model.UpdateEmpresaRequest) (*model.Empresa, error) {
	return s.empresaRepo.Update(ctx, empresaID, req.Nome, req.AlertaSonoro)
}

func (s *EmpresaService) ProvisionarPadrao(ctx context.Context, empresaID, adminID uuid.UUID) error {
	existing, err := s.configEscalonamentoRepo.FindByEmpresa(ctx, empresaID)
	if err != nil {
		return err
	}
	if existing != nil {
		return nil
	}

	cfg := &model.ConfigEscalonamento{
		EmpresaID:     empresaID,
		AtrasoMinutos: 0,
		Descricao:     "Emergencia nao justificada",
		Sistema:       true,
		UsuarioIDs:    []uuid.UUID{adminID},
	}
	return s.configEscalonamentoRepo.Create(ctx, cfg)
}
