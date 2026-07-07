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
	ErrEscalaNaoEncontrada  = errors.New("escala nao encontrada")
	ErrEscalaSemEscala      = errors.New("nenhuma escala ativa para este usuario, posto e horario")
	ErrEscalaForaTolerancia = errors.New("inicio fora da tolerancia da escala")
)

type EscalaService struct {
	escalaRepo *repository.EscalaRepository
}

func NewEscalaService(escalaRepo *repository.EscalaRepository) *EscalaService {
	return &EscalaService{escalaRepo: escalaRepo}
}

func (s *EscalaService) Create(ctx context.Context, empresaID uuid.UUID, req model.CreateEscalaRequest) (*model.Escala, error) {
	usuarioID, err := uuid.Parse(req.UsuarioID)
	if err != nil {
		return nil, fmt.Errorf("usuario_id invalido: %w", err)
	}
	postoID, err := uuid.Parse(req.PostoID)
	if err != nil {
		return nil, fmt.Errorf("posto_id invalido: %w", err)
	}

	if err := validarHorasEscala(req.HoraInicio, req.HoraFim); err != nil {
		return nil, err
	}

	toleranciaMin := req.ToleranciaMin
	if toleranciaMin <= 0 {
		toleranciaMin = 15
	}

	esc := &model.Escala{
		EmpresaID:       empresaID,
		UsuarioID:       usuarioID,
		PostoID:         postoID,
		DiaSemanaInicio: req.DiaSemanaInicio,
		HoraInicio:      req.HoraInicio,
		DiaSemanaFim:    req.DiaSemanaFim,
		HoraFim:         req.HoraFim,
		ToleranciaMin:   toleranciaMin,
	}

	if err := s.escalaRepo.Create(ctx, esc); err != nil {
		return nil, fmt.Errorf("criar escala: %w", err)
	}

	return esc, nil
}

func (s *EscalaService) CreateLote(ctx context.Context, empresaID uuid.UUID, req model.CreateEscalaLoteRequest) ([]model.Escala, error) {
	usuarioID, err := uuid.Parse(req.UsuarioID)
	if err != nil {
		return nil, fmt.Errorf("usuario_id invalido: %w", err)
	}
	postoID, err := uuid.Parse(req.PostoID)
	if err != nil {
		return nil, fmt.Errorf("posto_id invalido: %w", err)
	}

	toleranciaMin := req.ToleranciaMin
	if toleranciaMin <= 0 {
		toleranciaMin = 15
	}

	result := make([]model.Escala, 0, len(req.Dias))
	for _, dia := range req.Dias {
		if err := validarHorasEscala(dia.HoraInicio, dia.HoraFim); err != nil {
			return nil, fmt.Errorf("dia %d (inicio %s): %w", dia.DiaSemanaInicio, dia.HoraInicio, err)
		}

		esc := &model.Escala{
			EmpresaID:       empresaID,
			UsuarioID:       usuarioID,
			PostoID:         postoID,
			DiaSemanaInicio: dia.DiaSemanaInicio,
			HoraInicio:      dia.HoraInicio,
			DiaSemanaFim:    dia.DiaSemanaFim,
			HoraFim:         dia.HoraFim,
			ToleranciaMin:   toleranciaMin,
		}

		if err := s.escalaRepo.Create(ctx, esc); err != nil {
			return nil, fmt.Errorf("criar escala dia %d: %w", dia.DiaSemanaInicio, err)
		}
		result = append(result, *esc)
	}

	return result, nil
}

func (s *EscalaService) GetByID(ctx context.Context, empresaID, id uuid.UUID) (*model.Escala, error) {
	return s.escalaRepo.FindByID(ctx, empresaID, id)
}

func (s *EscalaService) List(ctx context.Context, empresaID uuid.UUID, filter model.EscalaFilter) ([]model.Escala, int, error) {
	return s.escalaRepo.List(ctx, empresaID, filter)
}

func (s *EscalaService) Update(ctx context.Context, empresaID, id uuid.UUID, req model.UpdateEscalaRequest) (*model.Escala, error) {
	esc, err := s.escalaRepo.FindByID(ctx, empresaID, id)
	if err != nil {
		return nil, fmt.Errorf("escala: %w", ErrEscalaNaoEncontrada)
	}

	if req.UsuarioID != nil {
		uid, err := uuid.Parse(*req.UsuarioID)
		if err != nil {
			return nil, fmt.Errorf("usuario_id invalido: %w", err)
		}
		esc.UsuarioID = uid
	}
	if req.PostoID != nil {
		pid, err := uuid.Parse(*req.PostoID)
		if err != nil {
			return nil, fmt.Errorf("posto_id invalido: %w", err)
		}
		esc.PostoID = pid
	}
	if req.DiaSemanaInicio != nil {
		esc.DiaSemanaInicio = *req.DiaSemanaInicio
	}
	if req.HoraInicio != nil {
		esc.HoraInicio = *req.HoraInicio
	}
	if req.DiaSemanaFim != nil {
		esc.DiaSemanaFim = *req.DiaSemanaFim
	}
	if req.HoraFim != nil {
		esc.HoraFim = *req.HoraFim
	}
	if req.ToleranciaMin != nil {
		esc.ToleranciaMin = *req.ToleranciaMin
	}
	if req.Ativo != nil {
		esc.Ativo = *req.Ativo
	}

	if err := validarHorasEscala(esc.HoraInicio, esc.HoraFim); err != nil {
		return nil, err
	}

	if err := s.escalaRepo.Update(ctx, empresaID, id, esc); err != nil {
		return nil, fmt.Errorf("atualizar escala: %w", err)
	}

	return esc, nil
}

func (s *EscalaService) Delete(ctx context.Context, empresaID, id uuid.UUID) error {
	esc, err := s.escalaRepo.FindByID(ctx, empresaID, id)
	if err != nil {
		return fmt.Errorf("escala: %w", ErrEscalaNaoEncontrada)
	}

	esc.Ativo = false
	return s.escalaRepo.Update(ctx, empresaID, id, esc)
}

// VerificarToleranciaEscala valida se `now` esta dentro da tolerancia de inicio
// da escala. A diferenca e a distancia circular no relogio de 24h, entao escalas
// noturnas que cruzam a meia-noite (ex.: 22:00 -> 06:00) funcionam: 23:55 esta a
// 5 min de uma escala que inicia 00:00, e nao a 1435.
func VerificarToleranciaEscala(esc *model.Escala, now time.Time) (bool, string) {
	if esc == nil {
		return false, "nenhuma escala ativa encontrada"
	}

	inicioMin, err := horaEmMinutos(esc.HoraInicio)
	if err != nil {
		return false, fmt.Sprintf("hora_inicio invalida na escala: %s", esc.HoraInicio)
	}

	nowMin := now.Hour()*60 + now.Minute()
	diferenca := nowMin - inicioMin
	if diferenca < 0 {
		diferenca = -diferenca
	}
	if diferenca > 720 {
		diferenca = 1440 - diferenca
	}

	if diferenca <= esc.ToleranciaMin {
		return true, ""
	}
	return false, fmt.Sprintf("fora da tolerancia: %d minutos de diferenca (max: %d)", diferenca, esc.ToleranciaMin)
}

// EscalaCruzaMeiaNoite indica se a escala termina em um dia da semana diferente
// do dia de inicio (turno noturno que vira o dia).
func EscalaCruzaMeiaNoite(esc *model.Escala) bool {
	return esc.DiaSemanaFim != esc.DiaSemanaInicio
}

// validarHorasEscala aceita escalas noturnas (hora_fim < hora_inicio significa
// que o turno vira o dia), mas rejeita inicio igual ao fim por ser ambiguo.
func validarHorasEscala(horaInicio, horaFim string) error {
	inicio, err := horaEmMinutos(horaInicio)
	if err != nil {
		return fmt.Errorf("hora_inicio invalida: %s", horaInicio)
	}
	fim, err := horaEmMinutos(horaFim)
	if err != nil {
		return fmt.Errorf("hora_fim invalida: %s", horaFim)
	}
	if inicio == fim {
		return errors.New("hora_fim deve ser diferente de hora_inicio")
	}
	return nil
}

// horaEmMinutos converte "HH:MM" ou "HH:MM:SS" em minutos desde a meia-noite.
func horaEmMinutos(hora string) (int, error) {
	if len(hora) > 5 {
		hora = hora[:5]
	}
	t, err := time.Parse("15:04", hora)
	if err != nil {
		return 0, err
	}
	return t.Hour()*60 + t.Minute(), nil
}
