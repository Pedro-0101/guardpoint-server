package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/guardpoint/guardpoint-server/internal/model"
)

var (
	ErrEscalonamentoNaoEncontrado       = errors.New("config escalonamento nao encontrada")
	ErrEscalonamentoSistemaNaoEditavel  = errors.New("config escalonamento do sistema nao pode ser alterada")
	ErrEscalonamentoSistemaNaoExcluivel = errors.New("config escalonamento do sistema nao pode ser excluida")
	ErrUsuarioNaoPertenceAEmpresa       = errors.New("usuario nao pertence a empresa")
	ErrUsuarioNaoAdminOuSupervisor      = errors.New("apenas administradores ou supervisores podem ser destinatarios de alertas")
)

type EscalonamentoConfigRepository interface {
	ListByEmpresa(ctx context.Context, empresaID uuid.UUID) ([]model.ConfigEscalonamento, error)
	FindByEmpresa(ctx context.Context, empresaID uuid.UUID) (*model.ConfigEscalonamento, error)
	FindByIDEmpresa(ctx context.Context, id, empresaID uuid.UUID) (*model.ConfigEscalonamento, error)
	Create(ctx context.Context, c *model.ConfigEscalonamento) error
	Update(ctx context.Context, c *model.ConfigEscalonamento) error
	UpdateUsuarios(ctx context.Context, configID uuid.UUID, usuarioIDs []uuid.UUID) error
	DeleteByID(ctx context.Context, id, empresaID uuid.UUID) error
}

type EscalonamentoUserRepository interface {
	FindByIDEmpresa(ctx context.Context, empresaID, id uuid.UUID) (*model.User, error)
	FindAdminIDs(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]bool, error)
}

type PostoSupervisorRepository interface {
	ListSupervisoresByPosto(ctx context.Context, postoID uuid.UUID) ([]uuid.UUID, error)
}

type EscalonamentoService struct {
	configRepo  EscalonamentoConfigRepository
	userRepo    EscalonamentoUserRepository
	postoRepo   PostoSupervisorRepository
}

func NewEscalonamentoService(configRepo EscalonamentoConfigRepository, userRepo EscalonamentoUserRepository, postoRepo PostoSupervisorRepository) *EscalonamentoService {
	return &EscalonamentoService{
		configRepo: configRepo,
		userRepo:   userRepo,
		postoRepo:  postoRepo,
	}
}

func (s *EscalonamentoService) ResolveDestinatarios(ctx context.Context, empresaID uuid.UUID) ([]uuid.UUID, error) {
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

func (s *EscalonamentoService) ResolveDestinatariosPorNivel(ctx context.Context, empresaID uuid.UUID, nivelID *uuid.UUID) ([]uuid.UUID, error) {
	if nivelID == nil {
		return s.ResolveDestinatarios(ctx, empresaID)
	}

	cfg, err := s.configRepo.FindByIDEmpresa(ctx, *nivelID, empresaID)
	if err != nil {
		return nil, fmt.Errorf("buscar config de escalonamento por nivel: %w", err)
	}
	if cfg == nil {
		return nil, ErrEscalonamentoNaoEncontrado
	}
	return cfg.UsuarioIDs, nil
}

func (s *EscalonamentoService) ResolveDestinatariosPorPosto(ctx context.Context, empresaID, postoID uuid.UUID) ([]uuid.UUID, error) {
	todos, err := s.ResolveDestinatarios(ctx, empresaID)
	if err != nil {
		return nil, err
	}

	if len(todos) == 0 {
		return nil, nil
	}

	admins, err := s.userRepo.FindAdminIDs(ctx, todos)
	if err != nil {
		return nil, fmt.Errorf("buscar admins: %w", err)
	}

	supervisoresPosto, err := s.postoRepo.ListSupervisoresByPosto(ctx, postoID)
	if err != nil {
		return nil, fmt.Errorf("buscar supervisores do posto: %w", err)
	}

	supervisorSet := make(map[uuid.UUID]bool, len(supervisoresPosto))
	for _, sid := range supervisoresPosto {
		supervisorSet[sid] = true
	}

	var filtrados []uuid.UUID
	for _, uid := range todos {
		if admins[uid] {
			filtrados = append(filtrados, uid)
			continue
		}
		if supervisorSet[uid] {
			filtrados = append(filtrados, uid)
		}
	}

	return filtrados, nil
}

func (s *EscalonamentoService) ResolveDestinatariosPorNivelEPosto(ctx context.Context, empresaID uuid.UUID, nivelID *uuid.UUID, postoID uuid.UUID) ([]uuid.UUID, error) {
	usuarioIDs, err := s.ResolveDestinatariosPorNivel(ctx, empresaID, nivelID)
	if err != nil {
		return nil, err
	}

	if postoID == uuid.Nil || len(usuarioIDs) == 0 {
		return usuarioIDs, nil
	}

	admins, err := s.userRepo.FindAdminIDs(ctx, usuarioIDs)
	if err != nil {
		return nil, fmt.Errorf("buscar admins: %w", err)
	}

	supervisoresPosto, err := s.postoRepo.ListSupervisoresByPosto(ctx, postoID)
	if err != nil {
		return nil, fmt.Errorf("buscar supervisores do posto: %w", err)
	}

	supervisorSet := make(map[uuid.UUID]bool, len(supervisoresPosto))
	for _, sid := range supervisoresPosto {
		supervisorSet[sid] = true
	}

	var filtrados []uuid.UUID
	for _, uid := range usuarioIDs {
		if admins[uid] {
			filtrados = append(filtrados, uid)
			continue
		}
		if supervisorSet[uid] {
			filtrados = append(filtrados, uid)
		}
	}

	return filtrados, nil
}

func (s *EscalonamentoService) GetEscalonamento(ctx context.Context, empresaID string) (*model.ConfigEscalonamento, error) {
	parsedEmpresaID, err := uuid.Parse(empresaID)
	if err != nil {
		return nil, fmt.Errorf("empresa_id invalido: %w", err)
	}
	return s.configRepo.FindByEmpresa(ctx, parsedEmpresaID)
}

func (s *EscalonamentoService) ListEscalonamentos(ctx context.Context, empresaID string) ([]model.ConfigEscalonamento, error) {
	parsedEmpresaID, err := uuid.Parse(empresaID)
	if err != nil {
		return nil, fmt.Errorf("empresa_id invalido: %w", err)
	}
	return s.configRepo.ListByEmpresa(ctx, parsedEmpresaID)
}

func (s *EscalonamentoService) GetEscalonamentoByID(ctx context.Context, empresaID, configID string) (*model.ConfigEscalonamento, error) {
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

func (s *EscalonamentoService) CreateEscalonamento(ctx context.Context, empresaID string, req model.CreateConfigEscalonamentoRequest) (*model.ConfigEscalonamento, error) {
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

func (s *EscalonamentoService) UpdateEscalonamento(ctx context.Context, empresaID, configID string, req model.UpdateConfigEscalonamentoRequest) (*model.ConfigEscalonamento, error) {
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

func (s *EscalonamentoService) UpdateEscalonamentoUsuarios(ctx context.Context, empresaID, configID string, req model.UpdateConfigEscalonamentoUsuariosRequest) (*model.ConfigEscalonamento, error) {
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

func (s *EscalonamentoService) DeleteEscalonamento(ctx context.Context, empresaID, configID string) error {
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

func (s *EscalonamentoService) validarUsuariosDaEmpresa(ctx context.Context, empresaID uuid.UUID, usuarioIDs []uuid.UUID) error {
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
