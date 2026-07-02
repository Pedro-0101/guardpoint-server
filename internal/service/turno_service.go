package service

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/google/uuid"

	"github.com/guardpoint/guardpoint-server/internal/model"
	"github.com/guardpoint/guardpoint-server/internal/repository"
)

var (
	ErrTurnoJaAtivo              = errors.New("usuario ja possui um turno em andamento")
	ErrTurnoNaoEncontrado        = errors.New("turno nao encontrado")
	ErrTurnoJaFinalizado         = errors.New("turno ja finalizado")
	ErrPostoNaoEncontrado        = errors.New("posto nao encontrado")
	ErrTurnoNaoPertenceAoUsuario = errors.New("turno nao pertence ao usuario")
	ErrDeviceNaoRegistrado       = errors.New("device nao registrado para biometric login")
)

type TurnoService struct {
	turnoRepo             *repository.TurnoRepository
	checkinRepo           *repository.CheckinRepository
	postoRepo             *repository.PostoRepository
	userRepo              *repository.UserRepository
	sessaoDispositivoRepo *repository.SessaoDispositivoRepository
}

func NewTurnoService(
	turnoRepo *repository.TurnoRepository,
	checkinRepo *repository.CheckinRepository,
	postoRepo *repository.PostoRepository,
	userRepo *repository.UserRepository,
	sessaoDispositivoRepo *repository.SessaoDispositivoRepository,
) *TurnoService {
	return &TurnoService{
		turnoRepo:             turnoRepo,
		checkinRepo:           checkinRepo,
		postoRepo:             postoRepo,
		userRepo:              userRepo,
		sessaoDispositivoRepo: sessaoDispositivoRepo,
	}
}

func (s *TurnoService) Iniciar(ctx context.Context, userID, empresaID string, req model.IniciarTurnoRequest) (*model.Turno, error) {
	parsedUserID, err := uuid.Parse(userID)
	if err != nil {
		return nil, fmt.Errorf("user_id invalido: %w", err)
	}
	parsedEmpresaID, err := uuid.Parse(empresaID)
	if err != nil {
		return nil, fmt.Errorf("empresa_id invalido: %w", err)
	}
	parsedPostoID, err := uuid.Parse(req.PostoID)
	if err != nil {
		return nil, fmt.Errorf("posto_id invalido: %w", err)
	}

	ativo, err := s.turnoRepo.FindAtivoByUsuario(ctx, parsedEmpresaID, parsedUserID)
	if err != nil {
		return nil, fmt.Errorf("verificar turno ativo: %w", err)
	}
	if ativo != nil {
		return nil, ErrTurnoJaAtivo
	}

	_, err = s.sessaoDispositivoRepo.FindByDeviceID(ctx, empresaID, req.DeviceID)
	if err != nil {
		return nil, ErrDeviceNaoRegistrado
	}

	posto, err := s.postoRepo.FindByID(ctx, parsedEmpresaID, parsedPostoID)
	if err != nil {
		return nil, fmt.Errorf("posto: %w", ErrPostoNaoEncontrado)
	}
	if !posto.Ativo {
		return nil, fmt.Errorf("posto inativo")
	}

	intervaloMin := req.IntervaloMin
	if intervaloMin <= 0 {
		intervaloMin = 30
	}

	now := time.Now()
	tokenSessao := uuid.New().String()

	fimPrevisto := now.Add(12 * time.Hour)

	turno := &model.Turno{
		EmpresaID:      parsedEmpresaID,
		UsuarioID:      parsedUserID,
		PostoID:        parsedPostoID,
		PostoNome:      posto.Nome,
		Status:         "em_andamento",
		InicioPrevisto: now,
		FimPrevisto:    fimPrevisto,
		InicioReal:     &now,
		TokenSessao:    &tokenSessao,
		IntervaloMin:   intervaloMin,
	}

	if err := s.turnoRepo.Create(ctx, turno); err != nil {
		return nil, fmt.Errorf("criar turno: %w", err)
	}

	return turno, nil
}

func (s *TurnoService) Checkin(ctx context.Context, userID, empresaID string, req model.CheckinRequest) (*model.CheckinResponse, error) {
	parsedUserID, err := uuid.Parse(userID)
	if err != nil {
		return nil, fmt.Errorf("user_id invalido: %w", err)
	}
	parsedEmpresaID, err := uuid.Parse(empresaID)
	if err != nil {
		return nil, fmt.Errorf("empresa_id invalido: %w", err)
	}
	parsedTurnoID, err := uuid.Parse(req.TurnoID)
	if err != nil {
		return nil, fmt.Errorf("turno_id invalido: %w", err)
	}

	turno, err := s.turnoRepo.FindByID(ctx, parsedEmpresaID, parsedTurnoID)
	if err != nil {
		return nil, fmt.Errorf("turno: %w", ErrTurnoNaoEncontrado)
	}

	if turno.UsuarioID != parsedUserID {
		return nil, ErrTurnoNaoPertenceAoUsuario
	}

	if turno.Status == "finalizado" {
		return nil, ErrTurnoJaFinalizado
	}

	timestampCriacao, err := time.Parse(time.RFC3339, req.Timestamp)
	if err != nil {
		timestampCriacao = time.Now()
	}

	flagGeofence := s.calcularGeofence(ctx, turno.PostoID, parsedEmpresaID, req.Latitude, req.Longitude)

	checkin := &model.Checkin{
		TurnoID:          parsedTurnoID,
		EmpresaID:        parsedEmpresaID,
		Latitude:         req.Latitude,
		Longitude:        req.Longitude,
		TimestampCriacao: timestampCriacao,
		TipoSenha:        req.TipoSenha,
		FlagGeofence:     flagGeofence,
		OrigemRede:       "online",
	}

	if err := s.checkinRepo.Create(ctx, checkin); err != nil {
		return nil, fmt.Errorf("criar checkin: %w", err)
	}

	atrasado := false
	var proximoDeadline *time.Time

	ultimo := s.checkinRepo.FindUltimoByTurnoNoError(ctx, turno.ID)
	dl := timestampCriacao.Add(time.Duration(turno.IntervaloMin) * time.Minute)
	proximoDeadline = &dl

	if ultimo != nil && ultimo.ID != checkin.ID {
		janelaDeadline := ultimo.TimestampCriacao.Add(time.Duration(turno.IntervaloMin) * time.Minute)
		if timestampCriacao.After(janelaDeadline) {
			atrasado = true
		}
	}

	if req.TipoSenha == "coacao" {
		_ = s.turnoRepo.UpdateStatus(ctx, parsedTurnoID, parsedEmpresaID, "critico", nil)
	}

	posto, err := s.postoRepo.FindByID(ctx, parsedEmpresaID, turno.PostoID)
	postoNome := ""
	if err == nil {
		postoNome = posto.Nome
	}

	return &model.CheckinResponse{
		Checkin:         *checkin,
		Status:          turno.Status,
		PostoNome:       postoNome,
		ProximoDeadline: proximoDeadline,
		Atrasado:        atrasado,
	}, nil
}

func (s *TurnoService) Finalizar(ctx context.Context, userID, empresaID string, req model.FinalizarTurnoRequest) (*model.Turno, error) {
	parsedUserID, err := uuid.Parse(userID)
	if err != nil {
		return nil, fmt.Errorf("user_id invalido: %w", err)
	}
	parsedEmpresaID, err := uuid.Parse(empresaID)
	if err != nil {
		return nil, fmt.Errorf("empresa_id invalido: %w", err)
	}
	parsedTurnoID, err := uuid.Parse(req.TurnoID)
	if err != nil {
		return nil, fmt.Errorf("turno_id invalido: %w", err)
	}

	turno, err := s.turnoRepo.FindByID(ctx, parsedEmpresaID, parsedTurnoID)
	if err != nil {
		return nil, fmt.Errorf("turno: %w", ErrTurnoNaoEncontrado)
	}

	if turno.UsuarioID != parsedUserID {
		return nil, ErrTurnoNaoPertenceAoUsuario
	}

	if turno.Status == "finalizado" {
		return nil, ErrTurnoJaFinalizado
	}

	timestampCriacao, err := time.Parse(time.RFC3339, req.Timestamp)
	if err != nil {
		timestampCriacao = time.Now()
	}

	flagGeofence := s.calcularGeofence(ctx, turno.PostoID, parsedEmpresaID, req.Latitude, req.Longitude)

	checkin := &model.Checkin{
		TurnoID:          parsedTurnoID,
		EmpresaID:        parsedEmpresaID,
		Latitude:         req.Latitude,
		Longitude:        req.Longitude,
		TimestampCriacao: timestampCriacao,
		TipoSenha:        "finalizacao",
		FlagGeofence:     flagGeofence,
		OrigemRede:       "online",
	}

	if err := s.checkinRepo.Create(ctx, checkin); err != nil {
		return nil, fmt.Errorf("criar checkin finalizacao: %w", err)
	}

	now := time.Now()
	if err := s.turnoRepo.UpdateStatus(ctx, parsedTurnoID, parsedEmpresaID, "finalizado", &now); err != nil {
		return nil, fmt.Errorf("finalizar turno: %w", err)
	}

	turno.Status = "finalizado"
	turno.FimReal = &now

	return turno, nil
}

func (s *TurnoService) GetStatus(ctx context.Context, userID, empresaID string) (*model.TurnoStatusResponse, error) {
	parsedUserID, err := uuid.Parse(userID)
	if err != nil {
		return nil, fmt.Errorf("user_id invalido: %w", err)
	}
	parsedEmpresaID, err := uuid.Parse(empresaID)
	if err != nil {
		return nil, fmt.Errorf("empresa_id invalido: %w", err)
	}

	turno, err := s.turnoRepo.FindAtivoByUsuario(ctx, parsedEmpresaID, parsedUserID)
	if err != nil {
		return nil, fmt.Errorf("buscar status: %w", err)
	}
	if turno == nil {
		return nil, ErrTurnoNaoEncontrado
	}

	ultimoCheckin := s.checkinRepo.FindUltimoByTurnoNoError(ctx, turno.ID)

	var proximoDeadline *time.Time
	if ultimoCheckin != nil {
		dl := ultimoCheckin.TimestampCriacao.Add(time.Duration(turno.IntervaloMin) * time.Minute)
		proximoDeadline = &dl
	} else if turno.InicioReal != nil {
		dl := turno.InicioReal.Add(time.Duration(turno.IntervaloMin) * time.Minute)
		proximoDeadline = &dl
	}

	checkinsHoje, err := s.checkinRepo.CountByTurnoHoje(ctx, turno.ID)
	if err != nil {
		checkinsHoje = 0
	}

	return &model.TurnoStatusResponse{
		Turno:           *turno,
		UltimoCheckin:   ultimoCheckin,
		ProximoDeadline: proximoDeadline,
		CheckinsHoje:    checkinsHoje,
	}, nil
}

func (s *TurnoService) GetAtivos(ctx context.Context, empresaID string) ([]model.TurnoDetalhe, error) {
	parsedEmpresaID, err := uuid.Parse(empresaID)
	if err != nil {
		return nil, fmt.Errorf("empresa_id invalido: %w", err)
	}

	turnos, err := s.turnoRepo.ListAtivos(ctx, parsedEmpresaID)
	if err != nil {
		return nil, fmt.Errorf("listar ativos: %w", err)
	}

	result := make([]model.TurnoDetalhe, 0, len(turnos))
	for _, t := range turnos {
		detalhe := model.TurnoDetalhe{Turno: t}

		posto, err := s.postoRepo.FindByID(ctx, parsedEmpresaID, t.PostoID)
		if err == nil {
			detalhe.Posto = posto
		}

		usuario, err := s.userRepo.FindByID(ctx, t.UsuarioID)
		if err == nil {
			u := *usuario
			u.SenhaHash = ""
			u.Telefone = nil
			detalhe.Usuario = &u
		}

		result = append(result, detalhe)
	}

	return result, nil
}

func (s *TurnoService) GetByID(ctx context.Context, empresaID, turnoID string) (*model.TurnoDetalhe, error) {
	parsedEmpresaID, err := uuid.Parse(empresaID)
	if err != nil {
		return nil, fmt.Errorf("empresa_id invalido: %w", err)
	}
	parsedTurnoID, err := uuid.Parse(turnoID)
	if err != nil {
		return nil, fmt.Errorf("turno_id invalido: %w", err)
	}

	turno, err := s.turnoRepo.FindByID(ctx, parsedEmpresaID, parsedTurnoID)
	if err != nil {
		return nil, err
	}

	detalhe := &model.TurnoDetalhe{Turno: *turno}

	checkins, err := s.checkinRepo.ListByTurno(ctx, parsedTurnoID)
	if err == nil {
		detalhe.Checkins = checkins
	}

	posto, err := s.postoRepo.FindByID(ctx, parsedEmpresaID, turno.PostoID)
	if err == nil {
		detalhe.Posto = posto
	}

	usuario, err := s.userRepo.FindByID(ctx, turno.UsuarioID)
	if err == nil {
		u := *usuario
		u.SenhaHash = ""
		u.Telefone = nil
		detalhe.Usuario = &u
	}

	return detalhe, nil
}

func (s *TurnoService) Revogar(ctx context.Context, empresaID, turnoID string) error {
	parsedEmpresaID, err := uuid.Parse(empresaID)
	if err != nil {
		return fmt.Errorf("empresa_id invalido: %w", err)
	}
	parsedTurnoID, err := uuid.Parse(turnoID)
	if err != nil {
		return fmt.Errorf("turno_id invalido: %w", err)
	}

	turno, err := s.turnoRepo.FindByID(ctx, parsedEmpresaID, parsedTurnoID)
	if err != nil {
		return fmt.Errorf("turno: %w", ErrTurnoNaoEncontrado)
	}
	if turno.Status == "finalizado" {
		return ErrTurnoJaFinalizado
	}

	return s.turnoRepo.RevogarToken(ctx, parsedTurnoID, parsedEmpresaID)
}

func (s *TurnoService) GetHistorico(ctx context.Context, empresaID string, filter model.HistoricoFilter) ([]model.Turno, int, error) {
	parsedEmpresaID, err := uuid.Parse(empresaID)
	if err != nil {
		return nil, 0, fmt.Errorf("empresa_id invalido: %w", err)
	}

	return s.turnoRepo.ListHistorico(ctx, parsedEmpresaID, filter)
}

func (s *TurnoService) Sabotagem(ctx context.Context, userID, empresaID string, req model.SabotagemRequest) (*model.SabotagemResponse, error) {
	parsedUserID, err := uuid.Parse(userID)
	if err != nil {
		return nil, fmt.Errorf("user_id invalido: %w", err)
	}
	parsedEmpresaID, err := uuid.Parse(empresaID)
	if err != nil {
		return nil, fmt.Errorf("empresa_id invalido: %w", err)
	}
	parsedTurnoID, err := uuid.Parse(req.TurnoID)
	if err != nil {
		return nil, fmt.Errorf("turno_id invalido: %w", err)
	}

	turno, err := s.turnoRepo.FindByID(ctx, parsedEmpresaID, parsedTurnoID)
	if err != nil {
		return nil, fmt.Errorf("turno: %w", ErrTurnoNaoEncontrado)
	}

	if turno.UsuarioID != parsedUserID {
		return nil, ErrTurnoNaoPertenceAoUsuario
	}

	if turno.Status == "finalizado" {
		return nil, ErrTurnoJaFinalizado
	}

	timestampCriacao, err := time.Parse(time.RFC3339, req.Timestamp)
	if err != nil {
		timestampCriacao = time.Now()
	}

	flagGeofence := s.calcularGeofence(ctx, turno.PostoID, parsedEmpresaID, req.Latitude, req.Longitude)

	_ = s.turnoRepo.UpdateStatus(ctx, parsedTurnoID, parsedEmpresaID, "critico", nil)

	checkin := &model.Checkin{
		TurnoID:          parsedTurnoID,
		EmpresaID:        parsedEmpresaID,
		Latitude:         req.Latitude,
		Longitude:        req.Longitude,
		TimestampCriacao: timestampCriacao,
		TipoSenha:        "sabotagem",
		FlagGeofence:     flagGeofence,
		OrigemRede:       "online",
	}
	_ = s.checkinRepo.Create(ctx, checkin)

	// TODO(F5): registrar alerta real na tabela alertas (Fase 5)
	alertaID := uuid.New().String()

	return &model.SabotagemResponse{
		AlertaID: alertaID,
		Status:   "registrado",
		Mensagem: "sabotagem reportada com sucesso",
	}, nil
}

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

		timestampCriacao, err := time.Parse(time.RFC3339, req.Timestamp)
		if err != nil {
			timestampCriacao = time.Now()
		}

		flagGeofence := s.calcularGeofence(ctx, turno.PostoID, parsedEmpresaID, req.Latitude, req.Longitude)

		origemRede := "offline_sincronizado"

		checkin := &model.Checkin{
			TurnoID:          parsedTurnoID,
			EmpresaID:        parsedEmpresaID,
			Latitude:         req.Latitude,
			Longitude:        req.Longitude,
			TimestampCriacao: timestampCriacao,
			TipoSenha:        req.TipoSenha,
			FlagGeofence:     flagGeofence,
			OrigemRede:       origemRede,
		}

		if err := s.checkinRepo.Create(ctx, checkin); err != nil {
			continue
		}

		if turno.Status == "critico" && req.TipoSenha != "coacao" {
			agora := time.Now()
			_ = s.turnoRepo.UpdateStatus(ctx, parsedTurnoID, parsedEmpresaID, "em_andamento", nil)
			turno.FimReal = &agora
		}

		if req.TipoSenha == "coacao" {
			_ = s.turnoRepo.UpdateStatus(ctx, parsedTurnoID, parsedEmpresaID, "critico", nil)
		}

		ultimo := s.checkinRepo.FindUltimoByTurnoNoError(ctx, turno.ID)
		atrasado := false
		dl := timestampCriacao.Add(time.Duration(turno.IntervaloMin) * time.Minute)
		proximoDeadline := &dl

		if ultimo != nil && ultimo.ID != checkin.ID {
			janelaDeadline := ultimo.TimestampCriacao.Add(time.Duration(turno.IntervaloMin) * time.Minute)
			if timestampCriacao.After(janelaDeadline) {
				atrasado = true
			}
		}

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

func (s *TurnoService) calcularGeofence(ctx context.Context, postoID, empresaID uuid.UUID, lat, lon float64) *string {
	posto, err := s.postoRepo.FindByID(ctx, empresaID, postoID)
	if err != nil {
		geofence := "ok"
		return &geofence
	}

	distancia := haversine(lat, lon, posto.Latitude, posto.Longitude)

	if distancia > float64(posto.RaioM) {
		geofence := "desvio_rota"
		return &geofence
	}

	geofence := "ok"
	return &geofence
}

func haversine(lat1, lon1, lat2, lon2 float64) float64 {
	const earthRadius = 6371000.0

	dLat := (lat2 - lat1) * (math.Pi / 180.0)
	dLon := (lon2 - lon1) * (math.Pi / 180.0)

	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1*(math.Pi/180.0))*math.Cos(lat2*(math.Pi/180.0))*
			math.Sin(dLon/2)*math.Sin(dLon/2)

	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))

	return earthRadius * c
}
