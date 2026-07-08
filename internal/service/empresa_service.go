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

// ProvisionarPadrao cria o nivel de escalonamento padrao da empresa:
//   - nivel 1: emergencia sem atraso (0min), sistema (nao removivel)
//
// O nivel 1 e o destino padrao dos alertas disparados por senha de coacao
// (emergencia) dos vigias. Recebe o admin informado como destinatario
// inicial. Idempotente: se o nivel ja existe, pula a criacao.
func (s *EmpresaService) ProvisionarPadrao(ctx context.Context, empresaID, adminID uuid.UUID) error {
	niveis := []struct {
		nivel         int
		atrasoMinutos int
		descricao     string
	}{
		{1, 0, "Emergencia nao especificada"},
	}

	for _, n := range niveis {
		existing, err := s.configEscalonamentoRepo.FindByEmpresaENivel(ctx, empresaID, n.nivel)
		if err != nil {
			return err
		}
		if existing != nil {
			continue
		}

		cfg := &model.ConfigEscalonamento{
			EmpresaID:     empresaID,
			Nivel:         n.nivel,
			AtrasoMinutos: n.atrasoMinutos,
			Sistema:       true,
			Descricao:     n.descricao,
			UsuarioIDs:    []uuid.UUID{adminID},
		}
		if err := s.configEscalonamentoRepo.Create(ctx, cfg); err != nil {
			return err
		}
	}

	return nil
}
