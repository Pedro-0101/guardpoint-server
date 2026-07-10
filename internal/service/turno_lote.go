package service

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/guardpoint/guardpoint-server/internal/model"
	"github.com/guardpoint/guardpoint-server/internal/timeutil"
)

func (s *TurnoService) ProcessarLote(ctx context.Context, userID, empresaID string, checkins []model.CheckinRequest) ([]model.CheckinResponse, error) {
	parsedUserID, err := uuid.Parse(userID)
	if err != nil {
		return nil, fmt.Errorf("user_id invalido: %w", err)
	}
	parsedEmpresaID, err := uuid.Parse(empresaID)
	if err != nil {
		return nil, fmt.Errorf("empresa_id invalido: %w", err)
	}

	resultados := make([]model.CheckinResponse, 0, len(checkins))

	for _, req := range checkins {
		parsedTurnoID, err := uuid.Parse(req.TurnoID)
		if err != nil {
			continue
		}

		turno, err := s.turnoRepo.FindByID(ctx, parsedEmpresaID, parsedTurnoID)
		if err != nil {
			continue
		}

		if turno.UsuarioID != parsedUserID {
			continue
		}

		if turno.Status == "finalizado" {
			continue
		}

		if err := validarSessaoTurno(turno, req.DeviceID); err != nil {
			continue
		}

		timestampCriacao, err := time.Parse(time.RFC3339, req.Timestamp)
		if err != nil {
			timestampCriacao = timeutil.NowBRT()
		}

		flagGeofence := s.calcularGeofence(ctx, turno.PostoID, parsedEmpresaID, req.Latitude, req.Longitude)

		origemRede := "offline_sincronizado"

		senhaResolvida, err := s.resolverSenha(ctx, parsedEmpresaID, parsedUserID, req.Senha)
		if err != nil {
			slog.Warn("senha de vigia nao resolvida", "error", err, "turno_id", parsedTurnoID, "usuario_id", parsedUserID)
			senhaResolvida = nil
		}

		checkin := &model.Checkin{
			TurnoID:          parsedTurnoID,
			EmpresaID:        parsedEmpresaID,
			Latitude:         req.Latitude,
			Longitude:        req.Longitude,
			TimestampCriacao: timestampCriacao,
			Evento:           "checkin",
			FlagGeofence:     flagGeofence,
			OrigemRede:       origemRede,
		}
		if senhaResolvida != nil {
			tipoSenha := senhaResolvida.Tipo
			checkin.TipoSenha = &tipoSenha
			senhaID := senhaResolvida.ID
			checkin.SenhaVigiaID = &senhaID
		}

		var anterior *time.Time
		if ultimo := s.checkinRepo.FindUltimoByTurnoNoError(ctx, turno.ID); ultimo != nil {
			anterior = &ultimo.TimestampCriacao
		}

		if req.ClienteCheckinID != "" {
			cid := req.ClienteCheckinID
			checkin.ClienteCheckinID = &cid
			if _, err := s.checkinRepo.CreateIdempotent(ctx, checkin); err != nil {
				continue
			}
		} else {
			if err := s.checkinRepo.Create(ctx, checkin); err != nil {
				continue
			}
		}

		s.aplicarConsequenciaSenha(ctx, empresaID, parsedEmpresaID, parsedTurnoID, parsedUserID, req.Senha)

		s.emitirGPSUpdate(empresaID, req.TurnoID, req.Latitude, req.Longitude, timestampCriacao, flagGeofence)

		atrasado := checkinAtrasado(anterior, turno.InicioReal, turno.IntervaloMin, timestampCriacao)
		dl := timestampCriacao.Add(time.Duration(turno.IntervaloMin) * time.Minute)
		proximoDeadline := &dl

		posto, err := s.postoRepo.FindByID(ctx, parsedEmpresaID, turno.PostoID)
		postoNome := ""
		if err == nil {
			postoNome = posto.Nome
		}

		resultados = append(resultados, model.CheckinResponse{
			Checkin:         *checkin,
			Status:          turno.Status,
			PostoNome:       postoNome,
			ProximoDeadline: proximoDeadline,
			Atrasado:        atrasado,
		})
	}

	return resultados, nil
}
