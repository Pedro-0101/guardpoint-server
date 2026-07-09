package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/guardpoint/guardpoint-server/internal/model"
	"github.com/guardpoint/guardpoint-server/internal/repository"
	"github.com/guardpoint/guardpoint-server/internal/ws"
)

var (
	ErrAlertaNaoEncontrado             = errors.New("alerta nao encontrado")
	ErrAlertaTransicaoInvalida         = errors.New("transicao de status do alerta invalida")
	ErrUsuarioNaoPertenceAEmpresa      = errors.New("usuario nao pertence a empresa")
	ErrUsuarioNaoAdminOuSupervisor     = errors.New("apenas administradores ou supervisores podem ser destinatarios de alertas")
	ErrEscalonamentoNaoEncontrado      = errors.New("config escalonamento nao encontrada")
	ErrEscalonamentoSistemaNaoEditavel = errors.New("config escalonamento do sistema nao pode ser alterada")
	ErrEscalonamentoSistemaNaoExcluivel = errors.New("config escalonamento do sistema nao pode ser excluida")
)

type AlertaService struct {
	alertaRepo   *repository.AlertaRepository
	configRepo   *repository.ConfigEscalonamentoRepository
	turnoRepo    *repository.TurnoRepository
	checkinRepo  *repository.CheckinRepository
	userRepo     *repository.UserRepository
	alertChannel chan *model.PendingAlert
	hub          *ws.Hub
}

func NewAlertaService(
	alertaRepo *repository.AlertaRepository,
	configRepo *repository.ConfigEscalonamentoRepository,
	turnoRepo *repository.TurnoRepository,
	checkinRepo *repository.CheckinRepository,
	userRepo *repository.UserRepository,
	hub *ws.Hub,
) *AlertaService {
	return &AlertaService{
		alertaRepo:   alertaRepo,
		configRepo:   configRepo,
		turnoRepo:    turnoRepo,
		checkinRepo:  checkinRepo,
		userRepo:     userRepo,
		alertChannel: make(chan *model.PendingAlert, 100),
		hub:          hub,
	}
}

func (s *AlertaService) AlertChannel() <-chan *model.PendingAlert {
	return s.alertChannel
}

func (s *AlertaService) CreateAlerta(ctx context.Context, empresaID, turnoID uuid.UUID, tipo string, mensagem string) (*model.Alerta, error) {
	count, err := s.alertaRepo.CountByTurnoETipo(ctx, turnoID, tipo)
	if err != nil {
		return nil, fmt.Errorf("verificar duplicidade: %w", err)
	}
	if count > 0 {
		return nil, nil
	}

	usuarioIDs, err := s.destinatariosPadrao(ctx, empresaID)
	if err != nil {
		return nil, fmt.Errorf("resolver destinatarios: %w", err)
	}

	return s.criarAlerta(ctx, empresaID, turnoID, tipo, mensagem, usuarioIDs)
}

func (s *AlertaService) CreateAlertaImediato(ctx context.Context, empresaID, turnoID uuid.UUID, tipo string, mensagem string, nivelEscalonamentoID *uuid.UUID) (*model.Alerta, error) {
	var usuarioIDs []uuid.UUID
	var err error

	if nivelEscalonamentoID != nil {
		cfg, cfgErr := s.configRepo.FindByIDEmpresa(ctx, *nivelEscalonamentoID, empresaID)
		if cfgErr != nil {
			return nil, fmt.Errorf("resolver destinatarios do nivel: %w", cfgErr)
		}
		if cfg != nil {
			usuarioIDs = cfg.UsuarioIDs
		}
	} else {
		usuarioIDs, err = s.destinatariosPadrao(ctx, empresaID)
		if err != nil {
			return nil, fmt.Errorf("resolver destinatarios: %w", err)
		}
	}

	return s.criarAlerta(ctx, empresaID, turnoID, tipo, mensagem, usuarioIDs)
}

func (s *AlertaService) criarAlerta(ctx context.Context, empresaID, turnoID uuid.UUID, tipo string, mensagem string, usuarioIDs []uuid.UUID) (*model.Alerta, error) {
	msg := &mensagem
	if mensagem == "" {
		msg = nil
	}

	turnoRef, turnoStr := nullableTurno(turnoID)

	alerta := &model.Alerta{
		EmpresaID: empresaID,
		TurnoID:   turnoRef,
		Tipo:      tipo,
		Status:    "aberto",
		Mensagem:  msg,
	}

	if err := s.alertaRepo.Create(ctx, alerta); err != nil {
		return nil, fmt.Errorf("criar alerta: %w", err)
	}

	s.hub.Broadcast(empresaID.String(), ws.NewAlertEvent(alerta.ID.String(), tipo, turnoStr))

	if len(usuarioIDs) > 0 {
		select {
		case s.alertChannel <- &model.PendingAlert{Alerta: alerta, UsuarioIDs: usuarioIDs}:
		default:
		}
	}

	return alerta, nil
}

func (s *AlertaService) destinatariosPadrao(ctx context.Context, empresaID uuid.UUID) ([]uuid.UUID, error) {
	configs, err := s.configRepo.ListByEmpresa(ctx, empresaID)
	if err != nil {
		return nil, err
	}

	visto := make(map[uuid.UUID]bool)
	var todos []uuid.UUID
	for _, cfg := range configs {
		for _, uid := range cfg.UsuarioIDs {
			if !visto[uid] {
				visto[uid] = true
				todos = append(todos, uid)
			}
		}
	}
	return todos, nil
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

func (s *AlertaService) GetEscalonamento(ctx context.Context, empresaID string) (*model.ConfigEscalonamento, error) {
	parsedEmpresaID, err := uuid.Parse(empresaID)
	if err != nil {
		return nil, fmt.Errorf("empresa_id invalido: %w", err)
	}
	return s.configRepo.FindByEmpresa(ctx, parsedEmpresaID)
}

func (s *AlertaService) ListEscalonamentos(ctx context.Context, empresaID string) ([]model.ConfigEscalonamento, error) {
	parsedEmpresaID, err := uuid.Parse(empresaID)
	if err != nil {
		return nil, fmt.Errorf("empresa_id invalido: %w", err)
	}
	return s.configRepo.ListByEmpresa(ctx, parsedEmpresaID)
}

func (s *AlertaService) GetEscalonamentoByID(ctx context.Context, empresaID, configID string) (*model.ConfigEscalonamento, error) {
	parsedEmpresaID, err := uuid.Parse(empresaID)
	if err != nil {
		return nil, fmt.Errorf("empresa_id invalido: %w", err)
	}
	parsedConfigID, err := uuid.Parse(configID)
	if err != nil {
		return nil, fmt.Errorf("config_id invalido: %w", err)
	}
	cfg, err := s.configRepo.FindByIDEmpresa(ctx, parsedConfigID, parsedEmpresaID)
	if err != nil {
		return nil, err
	}
	if cfg == nil {
		return nil, ErrEscalonamentoNaoEncontrado
	}
	return cfg, nil
}

func (s *AlertaService) CreateEscalonamento(ctx context.Context, empresaID string, req model.CreateConfigEscalonamentoRequest) (*model.ConfigEscalonamento, error) {
	parsedEmpresaID, err := uuid.Parse(empresaID)
	if err != nil {
		return nil, fmt.Errorf("empresa_id invalido: %w", err)
	}

	if err := s.validarUsuariosDaEmpresa(ctx, parsedEmpresaID, req.UsuarioIDs); err != nil {
		return nil, err
	}

	c := &model.ConfigEscalonamento{
		EmpresaID:     parsedEmpresaID,
		AtrasoMinutos: req.AtrasoMinutos,
		Descricao:     req.Descricao,
		Sistema:       false,
		UsuarioIDs:    req.UsuarioIDs,
	}

	if err := s.configRepo.Create(ctx, c); err != nil {
		return nil, fmt.Errorf("criar config escalonamento: %w", err)
	}
	return c, nil
}

func (s *AlertaService) UpdateEscalonamento(ctx context.Context, empresaID, configID string, req model.UpdateConfigEscalonamentoRequest) (*model.ConfigEscalonamento, error) {
	parsedEmpresaID, err := uuid.Parse(empresaID)
	if err != nil {
		return nil, fmt.Errorf("empresa_id invalido: %w", err)
	}
	parsedConfigID, err := uuid.Parse(configID)
	if err != nil {
		return nil, fmt.Errorf("config_id invalido: %w", err)
	}

	existing, err := s.configRepo.FindByIDEmpresa(ctx, parsedConfigID, parsedEmpresaID)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, ErrEscalonamentoNaoEncontrado
	}
	if existing.Sistema {
		return nil, ErrEscalonamentoSistemaNaoEditavel
	}

	if err := s.validarUsuariosDaEmpresa(ctx, parsedEmpresaID, req.UsuarioIDs); err != nil {
		return nil, err
	}

	c := &model.ConfigEscalonamento{
		ID:            parsedConfigID,
		EmpresaID:     parsedEmpresaID,
		AtrasoMinutos: req.AtrasoMinutos,
		Descricao:     req.Descricao,
		UsuarioIDs:    req.UsuarioIDs,
	}

	if err := s.configRepo.Update(ctx, c); err != nil {
		return nil, fmt.Errorf("atualizar config escalonamento: %w", err)
	}
	return c, nil
}

func (s *AlertaService) UpdateEscalonamentoUsuarios(ctx context.Context, empresaID, configID string, req model.UpdateConfigEscalonamentoUsuariosRequest) (*model.ConfigEscalonamento, error) {
	parsedEmpresaID, err := uuid.Parse(empresaID)
	if err != nil {
		return nil, fmt.Errorf("empresa_id invalido: %w", err)
	}
	parsedConfigID, err := uuid.Parse(configID)
	if err != nil {
		return nil, fmt.Errorf("config_id invalido: %w", err)
	}

	existing, err := s.configRepo.FindByIDEmpresa(ctx, parsedConfigID, parsedEmpresaID)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, ErrEscalonamentoNaoEncontrado
	}

	if err := s.validarUsuariosDaEmpresa(ctx, parsedEmpresaID, req.UsuarioIDs); err != nil {
		return nil, err
	}

	if err := s.configRepo.UpdateUsuarios(ctx, parsedConfigID, req.UsuarioIDs); err != nil {
		return nil, fmt.Errorf("atualizar destinatarios: %w", err)
	}

	existing.UsuarioIDs = req.UsuarioIDs
	return existing, nil
}

func (s *AlertaService) DeleteEscalonamento(ctx context.Context, empresaID, configID string) error {
	parsedEmpresaID, err := uuid.Parse(empresaID)
	if err != nil {
		return fmt.Errorf("empresa_id invalido: %w", err)
	}
	parsedConfigID, err := uuid.Parse(configID)
	if err != nil {
		return fmt.Errorf("config_id invalido: %w", err)
	}

	existing, err := s.configRepo.FindByIDEmpresa(ctx, parsedConfigID, parsedEmpresaID)
	if err != nil {
		return err
	}
	if existing == nil {
		return ErrEscalonamentoNaoEncontrado
	}
	if existing.Sistema {
		return ErrEscalonamentoSistemaNaoExcluivel
	}

	return s.configRepo.DeleteByID(ctx, parsedConfigID, parsedEmpresaID)
}

func (s *AlertaService) validarUsuariosDaEmpresa(ctx context.Context, empresaID uuid.UUID, usuarioIDs []uuid.UUID) error {
	for _, usuarioID := range usuarioIDs {
		user, err := s.userRepo.FindByIDEmpresa(ctx, empresaID, usuarioID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return fmt.Errorf("%w: %s", ErrUsuarioNaoPertenceAEmpresa, usuarioID)
			}
			return fmt.Errorf("verificar usuario %s: %w", usuarioID, err)
		}
		if user.Role != "admin" && user.Role != "supervisor" {
			return fmt.Errorf("%w: %s", ErrUsuarioNaoAdminOuSupervisor, usuarioID)
		}
	}
	return nil
}

func nullableTurno(turnoID uuid.UUID) (*uuid.UUID, string) {
	if turnoID == uuid.Nil {
		return nil, ""
	}
	id := turnoID
	return &id, id.String()
}
