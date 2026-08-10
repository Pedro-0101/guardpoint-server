package service

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/guardpoint/guardpoint-server/internal/model"
	"github.com/guardpoint/guardpoint-server/internal/repository"
)

type DashboardService struct {
	dashboardRepo *repository.DashboardRepository
}

func NewDashboardService(dashboardRepo *repository.DashboardRepository) *DashboardService {
	return &DashboardService{dashboardRepo: dashboardRepo}
}

func (s *DashboardService) Table(ctx context.Context, empresaID string, limit, offset int) (*model.DashboardTableResponse, error) {
	parsedEmpresaID, err := uuid.Parse(empresaID)
	if err != nil {
		return nil, fmt.Errorf("empresa_id invalido: %w", err)
	}

	if limit <= 0 || limit > 100 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}

	linhas, total, err := s.dashboardRepo.ListTurnosDetalhados(ctx, parsedEmpresaID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("listar turnos detalhados: %w", err)
	}

	if linhas == nil {
		linhas = []model.DashboardLinha{}
	}

	return &model.DashboardTableResponse{
		Linhas: linhas,
		Total:  total,
		Limit:  limit,
		Offset: offset,
	}, nil
}
