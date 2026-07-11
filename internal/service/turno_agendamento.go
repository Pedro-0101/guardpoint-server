package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/guardpoint/guardpoint-server/internal/model"
	"github.com/guardpoint/guardpoint-server/internal/timeutil"
)

func (s *TurnoService) gerarTurnosAgendados(ctx context.Context, empresaID uuid.UUID, filter model.TurnoFilter) ([]model.Turno, error) {
	if filter.DataInicio == "" {
		filter.DataInicio = time.Now().In(timeutil.BRT).Format("2006-01-02")
	}
	if filter.DataFim == "" {
		filter.DataFim = time.Now().In(timeutil.BRT).Format("2006-01-02")
	}

	dataInicio, err := time.ParseInLocation("2006-01-02", filter.DataInicio, timeutil.BRT)
	if err != nil {
		return nil, fmt.Errorf("data_inicio invalida: %w", err)
	}
	dataFim, err := time.ParseInLocation("2006-01-02", filter.DataFim, timeutil.BRT)
	if err != nil {
		return nil, fmt.Errorf("data_fim invalida: %w", err)
	}

	escalas, err := s.escalaRepo.ListAtivasByEmpresa(ctx, empresaID, filter.UsuarioID, filter.PostoID)
	if err != nil {
		return nil, fmt.Errorf("listar escalas: %w", err)
	}

	subs, err := s.substituicaoRepo.ListAtivasByDateRange(ctx, empresaID, filter.DataInicio, filter.DataFim, filter.UsuarioID, filter.PostoID)
	if err != nil {
		return nil, fmt.Errorf("listar substituicoes: %w", err)
	}

	reais, err := s.turnoRepo.ListTurnosByDateRange(ctx, empresaID, filter.DataInicio, filter.DataFim, filter.UsuarioID, filter.PostoID)
	if err != nil {
		return nil, fmt.Errorf("listar turnos reais: %w", err)
	}

	realByUserPostoDate := make(map[string]bool)
	for _, t := range reais {
		dateStr := t.InicioPrevisto.In(timeutil.BRT).Format("2006-01-02")
		key := t.UsuarioID.String() + "|" + t.PostoID.String() + "|" + dateStr
		realByUserPostoDate[key] = true
	}

	subByUserPostoDate := make(map[string]*model.Substituicao)
	for i := range subs {
		sub := &subs[i]
		current := sub.DataInicio
		for !current.After(sub.DataFim) {
			dateStr := current.Format("2006-01-02")
			key := sub.UsuarioID.String() + "|" + sub.PostoID.String() + "|" + dateStr
			subByUserPostoDate[key] = sub
			current = current.AddDate(0, 0, 1)
		}
	}

	subPostoDate := make(map[string]bool)
	for key := range subByUserPostoDate {
		parts := strings.SplitN(key, "|", 3)
		if len(parts) == 3 {
			subPostoDate[parts[1]+"|"+parts[2]] = true
		}
	}

	var turnos []model.Turno

	for d := dataInicio; !d.After(dataFim); d = d.AddDate(0, 0, 1) {
		diaSemana := int16(d.Weekday())
		dateStr := d.Format("2006-01-02")

		for _, esc := range escalas {
			if esc.DiaSemanaInicio != diaSemana {
				continue
			}

			key := esc.UsuarioID.String() + "|" + esc.PostoID.String() + "|" + dateStr
			if realByUserPostoDate[key] {
				continue
			}

			_, hasSubForSameUser := subByUserPostoDate[key]
			postoDateKey := esc.PostoID.String() + "|" + dateStr
			if !hasSubForSameUser && subPostoDate[postoDateKey] {
				continue
			}

			horaInicio := esc.HoraInicio
			horaFim := esc.HoraFim
			usuarioNome := esc.UsuarioNome
			postoNome := esc.PostoNome

			var substituicaoID *uuid.UUID
			if sub, ok := subByUserPostoDate[key]; ok {
				horaInicio = sub.HoraInicio
				horaFim = sub.HoraFim
				usuarioNome = sub.UsuarioNome
				postoNome = sub.PostoNome
				substituicaoID = &sub.ID
			}

			inicioPrevisto, err := parseHoraData(dateStr, horaInicio)
			if err != nil {
				continue
			}
			fimPrevisto, err := parseHoraData(dateStr, horaFim)
			if err != nil {
				continue
			}

			if !fimPrevisto.After(inicioPrevisto) {
				fimPrevisto = fimPrevisto.AddDate(0, 0, 1)
			}

			turno := model.Turno{
				EmpresaID:      empresaID,
				UsuarioID:      esc.UsuarioID,
				PostoID:        esc.PostoID,
				PostoNome:      postoNome,
				UsuarioNome:    usuarioNome,
				Status:         "agendado",
				InicioPrevisto: inicioPrevisto,
				FimPrevisto:    fimPrevisto,
				SubstituicaoID: substituicaoID,
			}
			turnos = append(turnos, turno)
		}
	}

	generatedKeys := make(map[string]bool)
	for _, t := range turnos {
		key := t.UsuarioID.String() + "|" + t.PostoID.String() + "|" + t.InicioPrevisto.In(timeutil.BRT).Format("2006-01-02")
		generatedKeys[key] = true
	}

	escalaPostoDate := make(map[string]bool)
	for d := dataInicio; !d.After(dataFim); d = d.AddDate(0, 0, 1) {
		diaSemana := int16(d.Weekday())
		dateStr := d.Format("2006-01-02")
		for _, esc := range escalas {
			if esc.DiaSemanaInicio == diaSemana {
				escalaPostoDate[esc.PostoID.String()+"|"+dateStr] = true
			}
		}
	}

	for _, sub := range subs {
		current := sub.DataInicio
		for !current.After(sub.DataFim) {
			dateStr := current.Format("2006-01-02")
			key := sub.UsuarioID.String() + "|" + sub.PostoID.String() + "|" + dateStr

			if realByUserPostoDate[key] {
				current = current.AddDate(0, 0, 1)
				continue
			}
			if generatedKeys[key] {
				current = current.AddDate(0, 0, 1)
				continue
			}
			if !escalaPostoDate[sub.PostoID.String()+"|"+dateStr] {
				current = current.AddDate(0, 0, 1)
				continue
			}

			inicioPrevisto, err := parseHoraData(dateStr, sub.HoraInicio)
			if err != nil {
				current = current.AddDate(0, 0, 1)
				continue
			}
			fimPrevisto, err := parseHoraData(dateStr, sub.HoraFim)
			if err != nil {
				current = current.AddDate(0, 0, 1)
				continue
			}
			if !fimPrevisto.After(inicioPrevisto) {
				fimPrevisto = fimPrevisto.AddDate(0, 0, 1)
			}

			turno := model.Turno{
				EmpresaID:      empresaID,
				UsuarioID:      sub.UsuarioID,
				PostoID:        sub.PostoID,
				PostoNome:      sub.PostoNome,
				UsuarioNome:    sub.UsuarioNome,
				Status:         "agendado",
				InicioPrevisto: inicioPrevisto,
				FimPrevisto:    fimPrevisto,
				SubstituicaoID: &sub.ID,
			}
			turnos = append(turnos, turno)
			current = current.AddDate(0, 0, 1)
		}
	}

	return turnos, nil
}
