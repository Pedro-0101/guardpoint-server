package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/guardpoint/guardpoint-server/internal/model"
	"github.com/guardpoint/guardpoint-server/internal/repository"
)

var (
	ErrAlertaNaoEncontrado          = errors.New("alerta nao encontrado")
	ErrConfigEscalonamentoDuplicado = errors.New("nivel de escalonamento ja existe para esta empresa")
)

type AlertaService struct {
	alertaRepo   *repository.AlertaRepository
	configRepo   *repository.ConfigEscalonamentoRepository
	turnoRepo    *repository.TurnoRepository
	checkinRepo  *repository.CheckinRepository
	alertChannel chan *model.PendingAlert
}

func NewAlertaService(
	alertaRepo *repository.AlertaRepository,
	configRepo *repository.ConfigEscalonamentoRepository,
	turnoRepo *repository.TurnoRepository,
	checkinRepo *repository.CheckinRepository,
) *AlertaService {
	return &AlertaService{
		alertaRepo:   alertaRepo,
		configRepo:   configRepo,
		turnoRepo:    turnoRepo,
		checkinRepo:  checkinRepo,
		alertChannel: make(chan *model.PendingAlert, 100),
	}
}

func (s *AlertaService) AlertChannel() <-chan *model.PendingAlert {
	return s.alertChannel
}

func (s *AlertaService) CreateAlerta(ctx context.Context, empresaID, turnoID uuid.UUID, tipo string, nivel int, mensagem string) (*model.Alerta, error) {
	count, err := s.alertaRepo.CountByTurnoETipo(ctx, turnoID, tipo)
	if err != nil {
		return nil, fmt.Errorf("verificar duplicidade: %w", err)
	}
	if count > 0 {
		return nil, nil
	}

	msg := &mensagem
	if mensagem == "" {
		msg = nil
	}

	alerta := &model.Alerta{
		EmpresaID: empresaID,
		TurnoID:   turnoID,
		Tipo:      tipo,
		Nivel:     nivel,
		Status:    "aberto",
		Mensagem:  msg,
	}

	if err := s.alertaRepo.Create(ctx, alerta); err != nil {
		return nil, fmt.Errorf("criar alerta: %w", err)
	}

	configs, _ := s.configRepo.FindByEmpresa(ctx, empresaID)
	for _, cfg := range configs {
		if cfg.Nivel == nivel {
			select {
			case s.alertChannel <- &model.PendingAlert{
				Alerta:       alerta,
				WhatsappPara: cfg.WhatsappPara,
			}:
			default:
			}
			break
		}
	}

	return alerta, nil
}

func (s *AlertaService) CreateAlertaImediato(ctx context.Context, empresaID, turnoID uuid.UUID, tipo string, nivel int, mensagem string) (*model.Alerta, error) {
	msg := &mensagem
	if mensagem == "" {
		msg = nil
	}

	alerta := &model.Alerta{
		EmpresaID: empresaID,
		TurnoID:   turnoID,
		Tipo:      tipo,
		Nivel:     nivel,
		Status:    "aberto",
		Mensagem:  msg,
	}

	if err := s.alertaRepo.Create(ctx, alerta); err != nil {
		return nil, fmt.Errorf("criar alerta imediato: %w", err)
	}

	configs, _ := s.configRepo.FindByEmpresa(ctx, empresaID)
	for _, cfg := range configs {
		select {
		case s.alertChannel <- &model.PendingAlert{
			Alerta:       alerta,
			WhatsappPara: cfg.WhatsappPara,
		}:
		default:
		}
	}

	return alerta, nil
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
		return fmt.Errorf("alerta nao esta aberto")
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
		return fmt.Errorf("alerta ja esta encerrado")
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

func (s *AlertaService) GetEscalonamento(ctx context.Context, empresaID string) ([]model.ConfigEscalonamento, error) {
	parsedEmpresaID, err := uuid.Parse(empresaID)
	if err != nil {
		return nil, fmt.Errorf("empresa_id invalido: %w", err)
	}
	return s.configRepo.FindByEmpresa(ctx, parsedEmpresaID)
}

func (s *AlertaService) CreateEscalonamento(ctx context.Context, empresaID string, req model.CreateConfigEscalonamentoRequest) (*model.ConfigEscalonamento, error) {
	parsedEmpresaID, err := uuid.Parse(empresaID)
	if err != nil {
		return nil, fmt.Errorf("empresa_id invalido: %w", err)
	}

	existing, err := s.configRepo.FindByEmpresaENivel(ctx, parsedEmpresaID, req.Nivel)
	if err != nil {
		return nil, fmt.Errorf("verificar nivel existente: %w", err)
	}
	if existing != nil {
		c := &model.ConfigEscalonamento{
			EmpresaID:     parsedEmpresaID,
			Nivel:         req.Nivel,
			AtrasoMinutos: req.AtrasoMinutos,
			WhatsappPara:  req.WhatsappPara,
			CargoAlvo:     strPtr(req.CargoAlvo),
		}
		if err := s.configRepo.Upsert(ctx, c); err != nil {
			return nil, fmt.Errorf("atualizar config escalonamento: %w", err)
		}
		return c, nil
	}

	var cargoAlvo *string
	if req.CargoAlvo != "" {
		cargoAlvo = &req.CargoAlvo
	}

	c := &model.ConfigEscalonamento{
		EmpresaID:     parsedEmpresaID,
		Nivel:         req.Nivel,
		AtrasoMinutos: req.AtrasoMinutos,
		WhatsappPara:  req.WhatsappPara,
		CargoAlvo:     cargoAlvo,
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

	var cargoAlvo *string
	if req.CargoAlvo != "" {
		cargoAlvo = &req.CargoAlvo
	}

	c := &model.ConfigEscalonamento{
		AtrasoMinutos: req.AtrasoMinutos,
		WhatsappPara:  req.WhatsappPara,
		CargoAlvo:     cargoAlvo,
	}

	if err := s.configRepo.Update(ctx, parsedConfigID, parsedEmpresaID, c); err != nil {
		return nil, fmt.Errorf("atualizar config escalonamento: %w", err)
	}
	return c, nil
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
	return s.configRepo.Delete(ctx, parsedConfigID, parsedEmpresaID)
}

func (s *AlertaService) ReplaceEscalonamento(ctx context.Context, empresaID string, reqs []model.CreateConfigEscalonamentoRequest) ([]model.ConfigEscalonamento, error) {
	parsedEmpresaID, err := uuid.Parse(empresaID)
	if err != nil {
		return nil, fmt.Errorf("empresa_id invalido: %w", err)
	}

	configs := make([]model.ConfigEscalonamento, 0, len(reqs))
	for _, req := range reqs {
		var cargoAlvo *string
		if req.CargoAlvo != "" {
			cargoAlvo = &req.CargoAlvo
		}
		configs = append(configs, model.ConfigEscalonamento{
			Nivel:         req.Nivel,
			AtrasoMinutos: req.AtrasoMinutos,
			WhatsappPara:  req.WhatsappPara,
			CargoAlvo:     cargoAlvo,
		})
	}

	if err := s.configRepo.ReplaceByEmpresa(ctx, parsedEmpresaID, configs); err != nil {
		return nil, fmt.Errorf("substituir configs: %w", err)
	}

	result, err := s.configRepo.FindByEmpresa(ctx, parsedEmpresaID)
	if err != nil {
		return nil, fmt.Errorf("buscar configs apos replace: %w", err)
	}
	return result, nil
}

func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
