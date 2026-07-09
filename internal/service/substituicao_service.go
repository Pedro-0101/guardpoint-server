package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/guardpoint/guardpoint-server/internal/model"
	"github.com/guardpoint/guardpoint-server/internal/repository"
	"github.com/guardpoint/guardpoint-server/internal/timeutil"
)

var (
	ErrSubstituicaoNaoEncontrada = errors.New("substituicao nao encontrada")
)

type SubstituicaoService struct {
	repo *repository.SubstituicaoRepository
}

func NewSubstituicaoService(repo *repository.SubstituicaoRepository) *SubstituicaoService {
	return &SubstituicaoService{repo: repo}
}

func (s *SubstituicaoService) Create(ctx context.Context, empresaID uuid.UUID, req model.CreateSubstituicaoRequest) (*model.Substituicao, error) {
	usuarioID, err := uuid.Parse(req.UsuarioID)
	if err != nil {
		return nil, fmt.Errorf("usuario_id invalido: %w", err)
	}
	postoID, err := uuid.Parse(req.PostoID)
	if err != nil {
		return nil, fmt.Errorf("posto_id invalido: %w", err)
	}
	dataInicio, err := time.ParseInLocation("2006-01-02", req.DataInicio, timeutil.BRT)
	if err != nil {
		return nil, fmt.Errorf("data_inicio invalida: %w", err)
	}
	dataFim, err := time.ParseInLocation("2006-01-02", req.DataFim, timeutil.BRT)
	if err != nil {
		return nil, fmt.Errorf("data_fim invalida: %w", err)
	}
	if dataFim.Before(dataInicio) {
		return nil, fmt.Errorf("data_fim deve ser posterior a data_inicio")
	}

	if err := validarHorasEscala(req.HoraInicio, req.HoraFim); err != nil {
		return nil, err
	}

	toleranciaMin := req.ToleranciaMin
	if toleranciaMin <= 0 {
		toleranciaMin = 15
	}

	sub := &model.Substituicao{
		EmpresaID:     empresaID,
		UsuarioID:     usuarioID,
		PostoID:       postoID,
		DataInicio:    dataInicio,
		DataFim:       dataFim,
		HoraInicio:    req.HoraInicio,
		HoraFim:       req.HoraFim,
		ToleranciaMin: toleranciaMin,
		Motivo:        req.Motivo,
	}

	if err := s.repo.Create(ctx, sub); err != nil {
		return nil, fmt.Errorf("criar substituicao: %w", err)
	}

	return sub, nil
}

func (s *SubstituicaoService) GetByID(ctx context.Context, empresaID, id uuid.UUID) (*model.Substituicao, error) {
	return s.repo.FindByID(ctx, empresaID, id)
}

func (s *SubstituicaoService) List(ctx context.Context, empresaID uuid.UUID, filter model.SubstituicaoFilter) ([]model.Substituicao, int, error) {
	return s.repo.List(ctx, empresaID, filter)
}

func (s *SubstituicaoService) Update(ctx context.Context, empresaID, id uuid.UUID, req model.UpdateSubstituicaoRequest) (*model.Substituicao, error) {
	sub, err := s.repo.FindByID(ctx, empresaID, id)
	if err != nil {
		return nil, fmt.Errorf("substituicao: %w", ErrSubstituicaoNaoEncontrada)
	}

	if req.UsuarioID != nil {
		uid, err := uuid.Parse(*req.UsuarioID)
		if err != nil {
			return nil, fmt.Errorf("usuario_id invalido: %w", err)
		}
		sub.UsuarioID = uid
	}
	if req.PostoID != nil {
		pid, err := uuid.Parse(*req.PostoID)
		if err != nil {
			return nil, fmt.Errorf("posto_id invalido: %w", err)
		}
		sub.PostoID = pid
	}
	if req.DataInicio != nil {
		di, err := time.ParseInLocation("2006-01-02", *req.DataInicio, timeutil.BRT)
		if err != nil {
			return nil, fmt.Errorf("data_inicio invalida: %w", err)
		}
		sub.DataInicio = di
	}
	if req.DataFim != nil {
		df, err := time.ParseInLocation("2006-01-02", *req.DataFim, timeutil.BRT)
		if err != nil {
			return nil, fmt.Errorf("data_fim invalida: %w", err)
		}
		sub.DataFim = df
	}
	if req.HoraInicio != nil {
		sub.HoraInicio = *req.HoraInicio
	}
	if req.HoraFim != nil {
		sub.HoraFim = *req.HoraFim
	}
	if req.ToleranciaMin != nil {
		sub.ToleranciaMin = *req.ToleranciaMin
	}
	if req.Motivo != nil {
		sub.Motivo = *req.Motivo
	}
	if req.Ativo != nil {
		sub.Ativo = *req.Ativo
	}

	if err := validarHorasEscala(sub.HoraInicio, sub.HoraFim); err != nil {
		return nil, err
	}

	if err := s.repo.Update(ctx, empresaID, id, sub); err != nil {
		return nil, fmt.Errorf("atualizar substituicao: %w", err)
	}

	return sub, nil
}

func (s *SubstituicaoService) Delete(ctx context.Context, empresaID, id uuid.UUID) error {
	sub, err := s.repo.FindByID(ctx, empresaID, id)
	if err != nil {
		return fmt.Errorf("substituicao: %w", ErrSubstituicaoNaoEncontrada)
	}
	sub.Ativo = false
	return s.repo.Update(ctx, empresaID, id, sub)
}
