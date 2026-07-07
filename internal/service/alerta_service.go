package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/guardpoint/guardpoint-server/internal/model"
	"github.com/guardpoint/guardpoint-server/internal/repository"
	"github.com/guardpoint/guardpoint-server/internal/ws"
)

var (
	ErrAlertaNaoEncontrado          = errors.New("alerta nao encontrado")
	ErrAlertaTransicaoInvalida      = errors.New("transicao de status do alerta invalida")
	ErrConfigEscalonamentoDuplicado = errors.New("nivel de escalonamento ja existe para esta empresa")
	ErrUsuarioNaoPertenceAEmpresa   = errors.New("usuario nao pertence a empresa")
	ErrTipoEmergenciaInvalido       = errors.New("tipo de alerta de emergencia invalido")
)

var tiposEmergencia = []string{"coacao", "sabotagem", "no_show"}

type AlertaService struct {
	alertaRepo           *repository.AlertaRepository
	configRepo           *repository.ConfigEscalonamentoRepository
	configEmergenciaRepo *repository.ConfigAlertaEmergenciaRepository
	turnoRepo            *repository.TurnoRepository
	checkinRepo          *repository.CheckinRepository
	userRepo             *repository.UserRepository
	alertChannel         chan *model.PendingAlert
	hub                  *ws.Hub
}

func NewAlertaService(
	alertaRepo *repository.AlertaRepository,
	configRepo *repository.ConfigEscalonamentoRepository,
	configEmergenciaRepo *repository.ConfigAlertaEmergenciaRepository,
	turnoRepo *repository.TurnoRepository,
	checkinRepo *repository.CheckinRepository,
	userRepo *repository.UserRepository,
	hub *ws.Hub,
) *AlertaService {
	return &AlertaService{
		alertaRepo:           alertaRepo,
		configRepo:           configRepo,
		configEmergenciaRepo: configEmergenciaRepo,
		turnoRepo:            turnoRepo,
		checkinRepo:          checkinRepo,
		userRepo:             userRepo,
		alertChannel:         make(chan *model.PendingAlert, 100),
		hub:                  hub,
	}
}

func (s *AlertaService) AlertChannel() <-chan *model.PendingAlert {
	return s.alertChannel
}

// CreateAlerta cria um alerta de escalonamento por atraso, com deduplicacao
// por (turno, tipo). Os destinatarios vem da configuracao do nivel informado.
func (s *AlertaService) CreateAlerta(ctx context.Context, empresaID, turnoID uuid.UUID, tipo string, nivel int, mensagem string) (*model.Alerta, error) {
	count, err := s.alertaRepo.CountByTurnoETipo(ctx, turnoID, tipo)
	if err != nil {
		return nil, fmt.Errorf("verificar duplicidade: %w", err)
	}
	if count > 0 {
		return nil, nil
	}

	usuarioIDs, err := s.destinatariosPorNivel(ctx, empresaID, nivel)
	if err != nil {
		return nil, fmt.Errorf("resolver destinatarios: %w", err)
	}

	return s.criarAlerta(ctx, empresaID, turnoID, tipo, nivel, mensagem, usuarioIDs)
}

// CreateAlertaImediato cria um alerta de emergencia (coacao, sabotagem,
// no-show), sem deduplicacao. Os destinatarios vem da configuracao especifica
// do tipo de emergencia (config_alerta_emergencia), independente dos niveis
// de escalonamento por atraso.
func (s *AlertaService) CreateAlertaImediato(ctx context.Context, empresaID, turnoID uuid.UUID, tipo string, nivel int, mensagem string) (*model.Alerta, error) {
	usuarioIDs, err := s.destinatariosPorTipoEmergencia(ctx, empresaID, tipo)
	if err != nil {
		return nil, fmt.Errorf("resolver destinatarios: %w", err)
	}

	return s.criarAlerta(ctx, empresaID, turnoID, tipo, nivel, mensagem, usuarioIDs)
}

func (s *AlertaService) criarAlerta(ctx context.Context, empresaID, turnoID uuid.UUID, tipo string, nivel int, mensagem string, usuarioIDs []uuid.UUID) (*model.Alerta, error) {
	msg := &mensagem
	if mensagem == "" {
		msg = nil
	}

	turnoRef, turnoStr := nullableTurno(turnoID)

	alerta := &model.Alerta{
		EmpresaID: empresaID,
		TurnoID:   turnoRef,
		Tipo:      tipo,
		Nivel:     nivel,
		Status:    "aberto",
		Mensagem:  msg,
	}

	if err := s.alertaRepo.Create(ctx, alerta); err != nil {
		return nil, fmt.Errorf("criar alerta: %w", err)
	}

	s.hub.Broadcast(empresaID.String(), ws.NewAlertEvent(alerta.ID.String(), tipo, turnoStr, nivel))

	if len(usuarioIDs) > 0 {
		select {
		case s.alertChannel <- &model.PendingAlert{Alerta: alerta, UsuarioIDs: usuarioIDs}:
		default:
		}
	}

	return alerta, nil
}

func (s *AlertaService) destinatariosPorNivel(ctx context.Context, empresaID uuid.UUID, nivel int) ([]uuid.UUID, error) {
	cfg, err := s.configRepo.FindByEmpresaENivel(ctx, empresaID, nivel)
	if err != nil {
		return nil, err
	}
	if cfg == nil {
		return nil, nil
	}
	return cfg.UsuarioIDs, nil
}

func (s *AlertaService) destinatariosPorTipoEmergencia(ctx context.Context, empresaID uuid.UUID, tipo string) ([]uuid.UUID, error) {
	cfg, err := s.configEmergenciaRepo.FindByEmpresaETipo(ctx, empresaID, tipo)
	if err != nil {
		return nil, err
	}
	if cfg == nil {
		return nil, nil
	}
	return cfg.UsuarioIDs, nil
}

// ResolverAlertasAtraso fecha os alertas de atraso abertos do turno quando um
// check-in chega (online ou via lote offline), resetando o deadman's switch.
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

	if err := s.validarUsuariosDaEmpresa(ctx, parsedEmpresaID, req.UsuarioIDs); err != nil {
		return nil, err
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
			UsuarioIDs:    req.UsuarioIDs,
		}
		if err := s.configRepo.Upsert(ctx, c); err != nil {
			return nil, fmt.Errorf("atualizar config escalonamento: %w", err)
		}
		return c, nil
	}

	c := &model.ConfigEscalonamento{
		EmpresaID:     parsedEmpresaID,
		Nivel:         req.Nivel,
		AtrasoMinutos: req.AtrasoMinutos,
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

	if err := s.validarUsuariosDaEmpresa(ctx, parsedEmpresaID, req.UsuarioIDs); err != nil {
		return nil, err
	}

	c := &model.ConfigEscalonamento{
		AtrasoMinutos: req.AtrasoMinutos,
		UsuarioIDs:    req.UsuarioIDs,
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
		if err := s.validarUsuariosDaEmpresa(ctx, parsedEmpresaID, req.UsuarioIDs); err != nil {
			return nil, err
		}
		configs = append(configs, model.ConfigEscalonamento{
			Nivel:         req.Nivel,
			AtrasoMinutos: req.AtrasoMinutos,
			UsuarioIDs:    req.UsuarioIDs,
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

// GetAlertasEmergencia sempre retorna os 3 tipos fixos (coacao, sabotagem,
// no_show), com lista de usuarios vazia para os que ainda nao tem configuracao
// salva.
func (s *AlertaService) GetAlertasEmergencia(ctx context.Context, empresaID string) ([]model.ConfigAlertaEmergencia, error) {
	parsedEmpresaID, err := uuid.Parse(empresaID)
	if err != nil {
		return nil, fmt.Errorf("empresa_id invalido: %w", err)
	}

	existentes, err := s.configEmergenciaRepo.FindByEmpresa(ctx, parsedEmpresaID)
	if err != nil {
		return nil, fmt.Errorf("listar config alerta emergencia: %w", err)
	}

	porTipo := make(map[string]model.ConfigAlertaEmergencia, len(existentes))
	for _, c := range existentes {
		porTipo[c.Tipo] = c
	}

	resultado := make([]model.ConfigAlertaEmergencia, 0, len(tiposEmergencia))
	for _, tipo := range tiposEmergencia {
		if c, ok := porTipo[tipo]; ok {
			resultado = append(resultado, c)
			continue
		}
		resultado = append(resultado, model.ConfigAlertaEmergencia{
			EmpresaID:  parsedEmpresaID,
			Tipo:       tipo,
			UsuarioIDs: []uuid.UUID{},
		})
	}
	return resultado, nil
}

func (s *AlertaService) UpdateAlertaEmergencia(ctx context.Context, empresaID, tipo string, req model.UpdateConfigAlertaEmergenciaRequest) (*model.ConfigAlertaEmergencia, error) {
	if !tipoEmergenciaValido(tipo) {
		return nil, ErrTipoEmergenciaInvalido
	}

	parsedEmpresaID, err := uuid.Parse(empresaID)
	if err != nil {
		return nil, fmt.Errorf("empresa_id invalido: %w", err)
	}

	if err := s.validarUsuariosDaEmpresa(ctx, parsedEmpresaID, req.UsuarioIDs); err != nil {
		return nil, err
	}

	c := &model.ConfigAlertaEmergencia{
		EmpresaID:  parsedEmpresaID,
		Tipo:       tipo,
		UsuarioIDs: req.UsuarioIDs,
	}
	if err := s.configEmergenciaRepo.Upsert(ctx, c); err != nil {
		return nil, fmt.Errorf("atualizar config alerta emergencia: %w", err)
	}
	return c, nil
}

func tipoEmergenciaValido(tipo string) bool {
	for _, t := range tiposEmergencia {
		if t == tipo {
			return true
		}
	}
	return false
}

func (s *AlertaService) validarUsuariosDaEmpresa(ctx context.Context, empresaID uuid.UUID, usuarioIDs []uuid.UUID) error {
	for _, usuarioID := range usuarioIDs {
		if _, err := s.userRepo.FindByIDEmpresa(ctx, empresaID, usuarioID); err != nil {
			return fmt.Errorf("%w: %s", ErrUsuarioNaoPertenceAEmpresa, usuarioID)
		}
	}
	return nil
}

// nullableTurno converte um turnoID em ponteiro nulo quando for o UUID zero
// (caso dos alertas de no-show, que nao possuem turno associado).
// Retorna tambem a representacao string usada no evento WebSocket ("" quando nulo).
func nullableTurno(turnoID uuid.UUID) (*uuid.UUID, string) {
	if turnoID == uuid.Nil {
		return nil, ""
	}
	id := turnoID
	return &id, id.String()
}
