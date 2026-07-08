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

// ProvisionarPadrao cria o nivel de escalonamento inicial (nivel=1, atraso=15min) com
// o usuario informado (o admin recem-criado da empresa) como destinatario. Chamado
// logo apos a criacao do primeiro admin de uma empresa, garantindo que "nivel maximo
// dinamico" (usado pelas senhas de emergencia/customizada) sempre tenha alguem pra
// notificar. Idempotente: se a empresa ja tem um nivel 1 configurado (ex.: chamada
// repetida), nao faz nada -- inclusive se esse nivel existente nao tiver nenhum
// destinatario (ex.: criado por uma empresa que nao tinha admin no momento da
// migration 000024). Chamadores futuros que reusarem este metodo para "completar"
// provisionamento de empresas antigas precisam checar destinatarios separadamente.
func (s *EmpresaService) ProvisionarPadrao(ctx context.Context, empresaID, adminID uuid.UUID) error {
	existing, err := s.configEscalonamentoRepo.FindByEmpresaENivel(ctx, empresaID, 1)
	if err != nil {
		return err
	}
	if existing != nil {
		return nil
	}

	cfg := &model.ConfigEscalonamento{
		EmpresaID:     empresaID,
		Nivel:         1,
		AtrasoMinutos: 15,
		UsuarioIDs:    []uuid.UUID{adminID},
	}
	return s.configEscalonamentoRepo.Create(ctx, cfg)
}
