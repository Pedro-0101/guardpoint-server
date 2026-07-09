package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/guardpoint/guardpoint-server/internal/model"
	"github.com/guardpoint/guardpoint-server/internal/repository"
)

var (
	ErrSenhaNaoEncontrada           = errors.New("senha nao encontrada")
	ErrSenhaCodigoDuplicado         = errors.New("codigo ja usado por outra senha deste vigia")
	ErrSenhaTipoJaExiste            = errors.New("vigia ja possui uma senha deste tipo")
	ErrSenhaObrigatoriaNaoRemovivel = errors.New("senha obrigatoria (ok/emergencia) nao pode ser removida")
	ErrSenhaCampoNaoEditavelParaTipo = errors.New("campo nao editavel para este tipo de senha")

	// ErrUsuarioNaoPertenceAEmpresa ja e declarado em alerta_service.go -- reaproveitado
)

const (
	tipoSenhaOK          = "ok"
	tipoSenhaEmergencia  = "emergencia"
	tipoSenhaCustomizada = "customizada"
)

type SenhaVigiaService struct {
	senhaRepo *repository.SenhaVigiaRepository
	userRepo  *repository.UserRepository
}

func NewSenhaVigiaService(
	senhaRepo *repository.SenhaVigiaRepository,
	userRepo *repository.UserRepository,
) *SenhaVigiaService {
	return &SenhaVigiaService{senhaRepo: senhaRepo, userRepo: userRepo}
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

	if req.Tipo == tipoSenhaCustomizada {
		if req.Descricao == nil || strings.TrimSpace(*req.Descricao) == "" {
			return nil, errors.New("descricao obrigatoria para senha customizada")
		}
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

	senha := &model.SenhaVigia{
		EmpresaID: empresaID,
		UsuarioID: usuarioID,
		Tipo:      req.Tipo,
		Codigo:    req.Codigo,
		Descricao: req.Descricao,
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

	switch existing.Tipo {
	case tipoSenhaOK, tipoSenhaEmergencia:
		if req.Descricao != nil {
			return nil, ErrSenhaCampoNaoEditavelParaTipo
		}
	case tipoSenhaCustomizada:
		if req.Descricao != nil {
			existing.Descricao = req.Descricao
		}
	}

	if req.Codigo != nil {
		outras, err := s.senhaRepo.ListByUsuario(ctx, empresaID, usuarioID)
		if err != nil {
			return nil, err
		}
		if codigoDuplicado(outras, *req.Codigo, senhaID) {
			return nil, ErrSenhaCodigoDuplicado
		}
		existing.Codigo = *req.Codigo
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
