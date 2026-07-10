package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/guardpoint/guardpoint-server/internal/model"
	"github.com/guardpoint/guardpoint-server/internal/ws"
)

var (
	ErrAlertaNaoEncontrado     = errors.New("alerta nao encontrado")
	ErrAlertaTransicaoInvalida = errors.New("transicao de status do alerta invalida")
)

type AlertaAlertaRepository interface {
	Create(ctx context.Context, a *model.Alerta) error
	FindByID(ctx context.Context, empresaID, id uuid.UUID) (*model.Alerta, error)
	List(ctx context.Context, empresaID uuid.UUID, filter model.AlertaFilter) ([]model.Alerta, int, error)
	UpdateStatus(ctx context.Context, id, empresaID uuid.UUID, status string, resolvidoEm *time.Time) error
	CountByTurnoETipo(ctx context.Context, turnoID uuid.UUID, tipo string) (int, error)
	CountPorTipo(ctx context.Context, empresaID uuid.UUID) ([]model.AlertaPorTipo, error)
	CountPorHora(ctx context.Context, empresaID uuid.UUID) ([]model.AlertaPorHora, error)
	CloseAlertasResolvidoCheckin(ctx context.Context, turnoID uuid.UUID) (int64, error)
}

type AlertaService struct {
	alertaRepo          AlertaAlertaRepository
	escalonamentoSvc    *EscalonamentoService
	alertChannel        chan *model.PendingAlert
	hub                 *ws.Hub
}

func NewAlertaService(
	alertaRepo AlertaAlertaRepository,
	escalonamentoSvc *EscalonamentoService,
	hub *ws.Hub,
) *AlertaService {
	return &AlertaService{
		alertaRepo:       alertaRepo,
		escalonamentoSvc: escalonamentoSvc,
		alertChannel:     make(chan *model.PendingAlert, 100),
		hub:              hub,
	}
}

func (s *AlertaService) AlertChannel() <-chan *model.PendingAlert {
	return s.alertChannel
}

func (s *AlertaService) CreateAlerta(ctx context.Context, empresaID, turnoID, postoID uuid.UUID, tipo string, mensagem string) (*model.Alerta, error) {
	count, err := s.alertaRepo.CountByTurnoETipo(ctx, turnoID, tipo)
	if err != nil {
		return nil, fmt.Errorf("verificar duplicidade: %w", err)
	}
	if count > 0 {
		return nil, nil
	}

	usuarioIDs, err := s.resolveComPosto(ctx, empresaID, postoID)
	if err != nil {
		return nil, fmt.Errorf("resolver destinatarios: %w", err)
	}

	return s.criarAlerta(ctx, empresaID, turnoID, postoID, tipo, mensagem, usuarioIDs)
}

func (s *AlertaService) CreateAlertaImediato(ctx context.Context, empresaID, turnoID, postoID uuid.UUID, tipo string, mensagem string, nivelEscalonamentoID *uuid.UUID) (*model.Alerta, error) {
	usuarioIDs, err := s.escalonamentoSvc.ResolveDestinatariosPorNivelEPosto(ctx, empresaID, nivelEscalonamentoID, postoID)
	if err != nil {
		return nil, fmt.Errorf("resolver destinatarios do nivel: %w", err)
	}

	return s.criarAlerta(ctx, empresaID, turnoID, postoID, tipo, mensagem, usuarioIDs)
}

func (s *AlertaService) resolveComPosto(ctx context.Context, empresaID, postoID uuid.UUID) ([]uuid.UUID, error) {
	if postoID == uuid.Nil {
		return s.escalonamentoSvc.ResolveDestinatarios(ctx, empresaID)
	}
	return s.escalonamentoSvc.ResolveDestinatariosPorPosto(ctx, empresaID, postoID)
}

func (s *AlertaService) criarAlerta(ctx context.Context, empresaID, turnoID, postoID uuid.UUID, tipo string, mensagem string, usuarioIDs []uuid.UUID) (*model.Alerta, error) {
	msg := &mensagem
	if mensagem == "" {
		msg = nil
	}

	turnoRef, turnoStr := nullableTurno(turnoID)
	postoRef, postoStr := nullableUUID(postoID)

	alerta := &model.Alerta{
		EmpresaID: empresaID,
		TurnoID:   turnoRef,
		PostoID:   postoRef,
		Tipo:      tipo,
		Status:    "aberto",
		Mensagem:  msg,
	}

	if err := s.alertaRepo.Create(ctx, alerta); err != nil {
		return nil, fmt.Errorf("criar alerta: %w", err)
	}

	s.hub.Broadcast(empresaID.String(), ws.NewAlertEvent(alerta.ID.String(), tipo, turnoStr, postoStr))

	if len(usuarioIDs) > 0 {
		select {
		case s.alertChannel <- &model.PendingAlert{Alerta: alerta, UsuarioIDs: usuarioIDs, PostoID: postoRef}:
		default:
		}
	}

	return alerta, nil
}

func (s *AlertaService) ResolverAlertasAtraso(ctx context.Context, turnoID uuid.UUID) error {
	if _, err := s.alertaRepo.CloseAlertasResolvidoCheckin(ctx, turnoID); err != nil {
		return fmt.Errorf("resolver alertas de atraso: %w", err)
	}
	return nil
}

func (s *AlertaService) List(ctx context.Context, empresaID string, filter model.AlertaFilter) ([]model.Alerta, int, error) {
	parsedEmpresaID, err := uuid.Parse(empresaID)
	if err != nil {
		return nil, 0, fmt.Errorf("empresa_id invalido: %w", err)
	}
	return s.alertaRepo.List(ctx, parsedEmpresaID, filter)
}

func (s *AlertaService) Reconhecer(ctx context.Context, empresaID, alertaID string) error {
	parsedEmpresaID, err := uuid.Parse(empresaID)
	if err != nil {
		return fmt.Errorf("empresa_id invalido: %w", err)
	}
	parsedAlertaID, err := uuid.Parse(alertaID)
	if err != nil {
		return fmt.Errorf("alerta_id invalido: %w", err)
	}

	alerta, err := s.alertaRepo.FindByID(ctx, parsedEmpresaID, parsedAlertaID)
	if err != nil {
		return ErrAlertaNaoEncontrado
	}

	if alerta.Status != "aberto" {
		return fmt.Errorf("%w: alerta nao esta aberto", ErrAlertaTransicaoInvalida)
	}

	if err := s.alertaRepo.UpdateStatus(ctx, parsedAlertaID, parsedEmpresaID, "reconhecido", nil); err != nil {
		return fmt.Errorf("reconhecer alerta: %w", err)
	}
	return nil
}

func (s *AlertaService) Encerrar(ctx context.Context, empresaID, alertaID string) error {
	parsedEmpresaID, err := uuid.Parse(empresaID)
	if err != nil {
		return fmt.Errorf("empresa_id invalido: %w", err)
	}
	parsedAlertaID, err := uuid.Parse(alertaID)
	if err != nil {
		return fmt.Errorf("alerta_id invalido: %w", err)
	}

	alerta, err := s.alertaRepo.FindByID(ctx, parsedEmpresaID, parsedAlertaID)
	if err != nil {
		return ErrAlertaNaoEncontrado
	}

	if alerta.Status == "encerrado" {
		return fmt.Errorf("%w: alerta ja esta encerrado", ErrAlertaTransicaoInvalida)
	}

	now := time.Now()
	if err := s.alertaRepo.UpdateStatus(ctx, parsedAlertaID, parsedEmpresaID, "encerrado", &now); err != nil {
		return fmt.Errorf("encerrar alerta: %w", err)
	}
	return nil
}

func (s *AlertaService) GetEstatisticas(ctx context.Context, empresaID string) (*model.AlertStatistics, error) {
	parsedEmpresaID, err := uuid.Parse(empresaID)
	if err != nil {
		return nil, fmt.Errorf("empresa_id invalido: %w", err)
	}

	stats := &model.AlertStatistics{}

	alertas, total, err := s.alertaRepo.List(ctx, parsedEmpresaID, model.AlertaFilter{Limit: 1000})
	if err != nil {
		return nil, fmt.Errorf("listar alertas: %w", err)
	}
	_ = total

	for _, a := range alertas {
		switch a.Status {
		case "aberto":
			stats.TotalAbertos++
		case "reconhecido":
			stats.TotalReconhecidos++
		case "encerrado":
			stats.TotalEncerrados++
		}
	}

	porTipo, err := s.alertaRepo.CountPorTipo(ctx, parsedEmpresaID)
	if err == nil {
		stats.PorTipo = porTipo
	}

	porHora, err := s.alertaRepo.CountPorHora(ctx, parsedEmpresaID)
	if err == nil {
		stats.PorHora = porHora
	}

	return stats, nil
}

func nullableTurno(turnoID uuid.UUID) (*uuid.UUID, string) {
	if turnoID == uuid.Nil {
		return nil, ""
	}
	id := turnoID
	return &id, id.String()
}

func nullableUUID(id uuid.UUID) (*uuid.UUID, string) {
	if id == uuid.Nil {
		return nil, ""
	}
	uid := id
	return &uid, uid.String()
}
