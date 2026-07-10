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
	alertaRepo    *repository.AlertaRepository
}

func NewDashboardService(dashboardRepo *repository.DashboardRepository, alertaRepo *repository.AlertaRepository) *DashboardService {
	return &DashboardService{dashboardRepo: dashboardRepo, alertaRepo: alertaRepo}
}

func (s *DashboardService) Summary(ctx context.Context, empresaID string) (*model.DashboardSummary, error) {
	parsedEmpresaID, err := uuid.Parse(empresaID)
	if err != nil {
		return nil, fmt.Errorf("empresa_id invalido: %w", err)
	}

	summary := &model.DashboardSummary{}

	turnosAtivos, err := s.dashboardRepo.CountTurnosAtivos(ctx, parsedEmpresaID)
	if err == nil {
		summary.TurnosAtivos = turnosAtivos
	}

	checkinsUltimaHora, err := s.dashboardRepo.CountCheckinsUltimaHora(ctx, parsedEmpresaID)
	if err == nil {
		summary.CheckinsUltimaHora = checkinsUltimaHora
	}

	desviosRota, err := s.dashboardRepo.CountDesviosRota(ctx, parsedEmpresaID)
	if err == nil {
		summary.DesviosRota = desviosRota
	}

	alertasAbertos, err := s.alertaRepo.CountAbertos(ctx, parsedEmpresaID)
	if err == nil {
		summary.AlertasAbertos = alertasAbertos
	}

	recentes, err := s.alertaRepo.ListRecentes(ctx, parsedEmpresaID, 5)
	if err == nil {
		summary.AlertasRecentes = recentes
	}

	turnosPorPosto, err := s.dashboardRepo.AggregateTurnosPorPosto(ctx, parsedEmpresaID)
	if err == nil {
		summary.TurnosPorPosto = turnosPorPosto
	}

	summary.AlertasRecentes = safeSlice(summary.AlertasRecentes, func() []model.AlertaRecente { return []model.AlertaRecente{} })
	summary.TurnosPorPosto = safeSlice(summary.TurnosPorPosto, func() []model.TurnoPorPosto { return []model.TurnoPorPosto{} })

	return summary, nil
}

func safeSlice[T any](s []T, fallback func() []T) []T {
	if s == nil {
		return fallback()
	}
	return s
}
