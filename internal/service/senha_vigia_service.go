package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/guardpoint/guardpoint-server/internal/model"
	"github.com/guardpoint/guardpoint-server/internal/repository"
)

var (
	ErrSenhaNaoEncontrada                = errors.New("senha nao encontrada")
	ErrSenhaCodigoDuplicado              = errors.New("codigo ja usado por outra senha deste vigia")
	ErrSenhaTipoJaExiste                 = errors.New("vigia ja possui uma senha deste tipo")
	ErrSenhaObrigatoriaNaoRemovivel      = errors.New("senha obrigatoria (ok/emergencia) nao pode ser removida")
	ErrSenhaNivelObrigatorio             = errors.New("nivel de escalonamento obrigatorio para este tipo de senha")
	ErrSenhaNivelNaoPermitido            = errors.New("senha de tipo ok nao pode ter nivel de escalonamento")
	ErrSenhaEmergenciaNivelInvalido      = errors.New("senha de emergencia deve usar o nivel padrao do sistema")
	ErrSenhaCustomizadaNivelInvalido     = errors.New("senha customizada nao pode usar o nivel padrao do sistema")
	ErrSenhaNivelInexistente             = errors.New("nivel de escalonamento nao encontrado")
	ErrSenhaNivelDuplicado               = errors.New("nivel de escalonamento ja vinculado a outra senha")
	ErrSenhaNivelNaoPertenceEmpresa      = errors.New("nivel de escalonamento nao pertence a empresa")
)

const (
	tipoSenhaOK          = "ok"
	tipoSenhaEmergencia  = "emergencia"
	tipoSenhaCustomizada = "customizada"
)

type SenhaVigiaService struct {
	senhaRepo  *repository.SenhaVigiaRepository
	userRepo   *repository.UserRepository
	configRepo *repository.ConfigEscalonamentoRepository
}

func NewSenhaVigiaService(
	senhaRepo *repository.SenhaVigiaRepository,
	userRepo *repository.UserRepository,
	configRepo *repository.ConfigEscalonamentoRepository,
) *SenhaVigiaService {
	return &SenhaVigiaService{senhaRepo: senhaRepo, userRepo: userRepo, configRepo: configRepo}
}

func (s *SenhaVigiaService) List(ctx context.Context, empresaID, usuarioID uuid.UUID) ([]model.SenhaVigia, error) {
	if err := s.validarUsuarioDaEmpresa(ctx, empresaID, usuarioID); err != nil {
		return nil, err
	}
	return s.senhaRepo.ListByUsuario(ctx, empresaID, usuarioID)
}

func (s *SenhaVigiaService) Create(ctx context.Context, empresaID, usuarioID uuid.UUID, req model.CreateSenhaVigiaRequest) (*model.SenhaVigia, error) {
	if err := s.validarUsuarioDaEmpresa(ctx, empresaID, usuarioID); err != nil {
		return nil, err
	}

	if err := s.validarNivel(ctx, empresaID, usuarioID, nil, req.Tipo, req.NivelEscalonamentoID); err != nil {
		return nil, err
	}

	existentes, err := s.senhaRepo.ListByUsuario(ctx, empresaID, usuarioID)
	if err != nil {
		return nil, err
	}

	if req.Tipo != tipoSenhaCustomizada && tipoJaExiste(existentes, req.Tipo) {
		return nil, ErrSenhaTipoJaExiste
	}
	if codigoDuplicado(existentes, req.Codigo, uuid.Nil) {
		return nil, ErrSenhaCodigoDuplicado
	}
	if req.Tipo == tipoSenhaCustomizada && req.NivelEscalonamentoID != nil {
		todasEmpresa, err := s.senhaRepo.ListByEmpresa(ctx, empresaID)
		if err != nil {
			return nil, err
		}
		if nivelDuplicado(todasEmpresa, req.NivelEscalonamentoID, uuid.Nil) {
			return nil, ErrSenhaNivelDuplicado
		}
	}

	senha := &model.SenhaVigia{
		EmpresaID:            empresaID,
		UsuarioID:            usuarioID,
		Tipo:                 req.Tipo,
		Codigo:               req.Codigo,
		NivelEscalonamentoID: req.NivelEscalonamentoID,
	}
	if err := s.senhaRepo.Create(ctx, senha); err != nil {
		return nil, err
	}
	return senha, nil
}

func (s *SenhaVigiaService) Update(ctx context.Context, empresaID, usuarioID, senhaID uuid.UUID, req model.UpdateSenhaVigiaRequest) (*model.SenhaVigia, error) {
	existing, err := s.senhaRepo.FindByID(ctx, empresaID, senhaID)
	if err != nil {
		return nil, err
	}
	if existing == nil || existing.UsuarioID != usuarioID {
		return nil, ErrSenhaNaoEncontrada
	}

	outras, err := s.senhaRepo.ListByUsuario(ctx, empresaID, usuarioID)
	if err != nil {
		return nil, err
	}

	if req.Codigo != nil {
		if codigoDuplicado(outras, *req.Codigo, senhaID) {
			return nil, ErrSenhaCodigoDuplicado
		}
		existing.Codigo = *req.Codigo
	}

	if req.NivelEscalonamentoID != nil {
		if err := s.validarNivel(ctx, empresaID, usuarioID, &senhaID, existing.Tipo, req.NivelEscalonamentoID); err != nil {
			return nil, err
		}
		if existing.Tipo == tipoSenhaCustomizada {
			todasEmpresa, err := s.senhaRepo.ListByEmpresa(ctx, empresaID)
			if err != nil {
				return nil, err
			}
			if nivelDuplicado(todasEmpresa, req.NivelEscalonamentoID, senhaID) {
				return nil, ErrSenhaNivelDuplicado
			}
		}
		existing.NivelEscalonamentoID = req.NivelEscalonamentoID
	}

	if err := s.senhaRepo.Update(ctx, senhaID, empresaID, existing); err != nil {
		return nil, err
	}
	return existing, nil
}

func (s *SenhaVigiaService) Delete(ctx context.Context, empresaID, usuarioID, senhaID uuid.UUID) error {
	existing, err := s.senhaRepo.FindByID(ctx, empresaID, senhaID)
	if err != nil {
		return err
	}
	if existing == nil || existing.UsuarioID != usuarioID {
		return ErrSenhaNaoEncontrada
	}
	if existing.Tipo == tipoSenhaOK || existing.Tipo == tipoSenhaEmergencia {
		return ErrSenhaObrigatoriaNaoRemovivel
	}
	return s.senhaRepo.Delete(ctx, senhaID, empresaID)
}

func (s *SenhaVigiaService) validarUsuarioDaEmpresa(ctx context.Context, empresaID, usuarioID uuid.UUID) error {
	if _, err := s.userRepo.FindByIDEmpresa(ctx, empresaID, usuarioID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrUsuarioNaoPertenceAEmpresa
		}
		return fmt.Errorf("verificar usuario %s: %w", usuarioID, err)
	}
	return nil
}

func tipoJaExiste(existentes []model.SenhaVigia, tipo string) bool {
	for _, s := range existentes {
		if s.Tipo == tipo {
			return true
		}
	}
	return false
}

func codigoDuplicado(existentes []model.SenhaVigia, codigo string, ignorarID uuid.UUID) bool {
	for _, s := range existentes {
		if s.ID == ignorarID {
			continue
		}
		if s.Codigo == codigo {
			return true
		}
	}
	return false
}

func nivelDuplicado(existentes []model.SenhaVigia, nivelID *uuid.UUID, ignorarID uuid.UUID) bool {
	if nivelID == nil {
		return false
	}
	for _, s := range existentes {
		if s.ID == ignorarID {
			continue
		}
		if s.NivelEscalonamentoID != nil && *s.NivelEscalonamentoID == *nivelID {
			return true
		}
	}
	return false
}

func (s *SenhaVigiaService) validarNivel(ctx context.Context, empresaID, usuarioID uuid.UUID, senhaID *uuid.UUID, tipo string, nivelID *uuid.UUID) error {
	if tipo == tipoSenhaOK {
		if nivelID != nil {
			return ErrSenhaNivelNaoPermitido
		}
		return nil
	}

	if nivelID == nil {
		return ErrSenhaNivelObrigatorio
	}

	cfg, err := s.configRepo.FindByIDEmpresa(ctx, *nivelID, empresaID)
	if err != nil {
		return fmt.Errorf("buscar nivel de escalonamento: %w", err)
	}
	if cfg == nil {
		return ErrSenhaNivelInexistente
	}

	if tipo == tipoSenhaEmergencia && !cfg.Sistema {
		return ErrSenhaEmergenciaNivelInvalido
	}
	if tipo == tipoSenhaCustomizada && cfg.Sistema {
		return ErrSenhaCustomizadaNivelInvalido
	}

	return nil
}
