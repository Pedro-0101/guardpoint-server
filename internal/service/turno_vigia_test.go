package service

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/guardpoint/guardpoint-server/internal/model"
	"github.com/guardpoint/guardpoint-server/internal/timeutil"
)

func TestGetVigiaTurno_SemEscalasNemSubstituicoes_RetornaVazio(t *testing.T) {
	ctx := context.Background()
	empresaID := uuid.New()
	userID := uuid.New()

	turnoRepo := &fakeTurnoTurnoRepo{
		findAtivoByUsuarioFn: func(ctx context.Context, eID, uID uuid.UUID) (*model.Turno, error) {
			return nil, nil
		},
	}
	escalaRepo := &fakeTurnoEscalaRepo{
		listAtivasByEmpresaFn: func(ctx context.Context, empresaID uuid.UUID, usuarioIDs, postoIDs []string) ([]model.Escala, error) {
			return nil, nil
		},
	}
	substituicaoRepo := &fakeTurnoSubstituicaoRepo{
		listAtivasByDateRangeFn: func(ctx context.Context, empresaID uuid.UUID, dataInicio, dataFim string, usuarioIDs, postoIDs []string) ([]model.Substituicao, error) {
			return nil, nil
		},
	}

	svc := newTestTurnoServiceFull(turnoRepo, &fakeTurnoCheckinRepo{}, &fakeTurnoPostoRepo{}, &fakeTurnoUserRepo{}, escalaRepo, substituicaoRepo)

	resp, err := svc.GetVigiaTurno(ctx, userID.String(), empresaID.String())
	if err != nil {
		t.Fatalf("GetVigiaTurno() erro inesperado: %v", err)
	}
	if resp.TemTurnoAtivo {
		t.Error("TemTurnoAtivo = true, esperado false")
	}
	if resp.Turno != nil {
		t.Error("Turno deveria ser nil (sem turno ativo)")
	}
	if resp.ProximoTurno != nil {
		t.Errorf("ProximoTurno deveria ser nil (sem escalas), veio: %+v", resp.ProximoTurno)
	}
}

func TestGetVigiaTurno_SemEscalasMasComSubstituicao_RetornaVazio(t *testing.T) {
	ctx := context.Background()
	empresaID := uuid.New()
	userID := uuid.New()
	postoID := uuid.New()
	now := timeutil.NowBRT()
	amanha := now.AddDate(0, 0, 1)

	turnoRepo := &fakeTurnoTurnoRepo{
		findAtivoByUsuarioFn: func(ctx context.Context, eID, uID uuid.UUID) (*model.Turno, error) {
			return nil, nil
		},
	}
	escalaRepo := &fakeTurnoEscalaRepo{
		listAtivasByEmpresaFn: func(ctx context.Context, empresaID uuid.UUID, usuarioIDs, postoIDs []string) ([]model.Escala, error) {
			return nil, nil
		},
	}
	substituicaoRepo := &fakeTurnoSubstituicaoRepo{
		listAtivasByDateRangeFn: func(ctx context.Context, empresaID uuid.UUID, dataInicio, dataFim string, usuarioIDs, postoIDs []string) ([]model.Substituicao, error) {
			return []model.Substituicao{
				{
					ID:        uuid.New(),
					EmpresaID: empresaID,
					UsuarioID: userID,
					PostoID:   postoID,
					DataInicio: amanha,
					DataFim:    amanha,
					HoraInicio: "08:00",
					HoraFim:    "17:00",
					Ativo:     true,
				},
			}, nil
		},
	}

	svc := newTestTurnoServiceFull(turnoRepo, &fakeTurnoCheckinRepo{}, &fakeTurnoPostoRepo{}, &fakeTurnoUserRepo{}, escalaRepo, substituicaoRepo)

	resp, err := svc.GetVigiaTurno(ctx, userID.String(), empresaID.String())
	if err != nil {
		t.Fatalf("GetVigiaTurno() erro inesperado: %v", err)
	}
	if resp.TemTurnoAtivo {
		t.Error("TemTurnoAtivo = true, esperado false")
	}
	if resp.ProximoTurno != nil {
		t.Errorf("ProximoTurno deveria ser nil (substituicao sem escala nao cria turno agendado), veio: %+v", resp.ProximoTurno)
	}
}

func TestGetVigiaTurno_ComEscalaFutura_RetornaProximoTurno(t *testing.T) {
	ctx := context.Background()
	empresaID := uuid.New()
	userID := uuid.New()
	postoID := uuid.New()
	now := timeutil.NowBRT()
	amanha := now.AddDate(0, 0, 1)
	diaSemanaAmanha := int16(amanha.Weekday())

	turnoRepo := &fakeTurnoTurnoRepo{
		findAtivoByUsuarioFn: func(ctx context.Context, eID, uID uuid.UUID) (*model.Turno, error) {
			return nil, nil
		},
	}
	escalaRepo := &fakeTurnoEscalaRepo{
		listAtivasByEmpresaFn: func(ctx context.Context, empresaID uuid.UUID, usuarioIDs, postoIDs []string) ([]model.Escala, error) {
			return []model.Escala{
				{
					ID:              uuid.New(),
					EmpresaID:       empresaID,
					UsuarioID:       userID,
					PostoID:         postoID,
					DiaSemanaInicio: diaSemanaAmanha,
					HoraInicio:      "08:00",
					DiaSemanaFim:    diaSemanaAmanha,
					HoraFim:         "17:00",
					Ativo:           true,
				},
			}, nil
		},
	}
	substituicaoRepo := &fakeTurnoSubstituicaoRepo{
		listAtivasByDateRangeFn: func(ctx context.Context, empresaID uuid.UUID, dataInicio, dataFim string, usuarioIDs, postoIDs []string) ([]model.Substituicao, error) {
			return nil, nil
		},
	}
	postoRepo := &fakeTurnoPostoRepo{
		findByIDFn: func(ctx context.Context, eID, id uuid.UUID) (*model.Posto, error) {
			return &model.Posto{
				ID:   postoID,
				Nome: "Posto Teste",
			}, nil
		},
	}

	svc := newTestTurnoServiceFull(turnoRepo, &fakeTurnoCheckinRepo{}, postoRepo, &fakeTurnoUserRepo{}, escalaRepo, substituicaoRepo)

	resp, err := svc.GetVigiaTurno(ctx, userID.String(), empresaID.String())
	if err != nil {
		t.Fatalf("GetVigiaTurno() erro inesperado: %v", err)
	}
	if resp.TemTurnoAtivo {
		t.Error("TemTurnoAtivo = true, esperado false")
	}
	if resp.ProximoTurno == nil {
		t.Fatal("ProximoTurno nao deveria ser nil (escala futura existe)")
	}
	if resp.ProximoTurno.Posto == nil || resp.ProximoTurno.Posto.Nome != "Posto Teste" {
		t.Errorf("Posto.Nome = %v, esperado 'Posto Teste'", resp.ProximoTurno.Posto)
	}
}

func TestGetVigiaTurno_ComEscalaRecorrenteHojePassou_RetornaProximaSemana(t *testing.T) {
	ctx := context.Background()
	empresaID := uuid.New()
	userID := uuid.New()
	postoID := uuid.New()
	now := timeutil.NowBRT()
	diaSemanaHoje := int16(now.Weekday())

	turnoRepo := &fakeTurnoTurnoRepo{
		findAtivoByUsuarioFn: func(ctx context.Context, eID, uID uuid.UUID) (*model.Turno, error) {
			return nil, nil
		},
	}

	escalaRepo := &fakeTurnoEscalaRepo{
		listAtivasByEmpresaFn: func(ctx context.Context, empresaID uuid.UUID, usuarioIDs, postoIDs []string) ([]model.Escala, error) {
			return []model.Escala{
				{
					ID:              uuid.New(),
					EmpresaID:       empresaID,
					UsuarioID:       userID,
					PostoID:         postoID,
					DiaSemanaInicio: diaSemanaHoje,
					HoraInicio:      "00:01",
					DiaSemanaFim:    diaSemanaHoje,
					HoraFim:         "00:02",
					Ativo:           true,
				},
			}, nil
		},
	}
	substituicaoRepo := &fakeTurnoSubstituicaoRepo{
		listAtivasByDateRangeFn: func(ctx context.Context, empresaID uuid.UUID, dataInicio, dataFim string, usuarioIDs, postoIDs []string) ([]model.Substituicao, error) {
			return nil, nil
		},
	}
	postoRepo := &fakeTurnoPostoRepo{
		findByIDFn: func(ctx context.Context, eID, id uuid.UUID) (*model.Posto, error) {
			return &model.Posto{ID: postoID, Nome: "Posto Noturno"}, nil
		},
	}

	svc := newTestTurnoServiceFull(turnoRepo, &fakeTurnoCheckinRepo{}, postoRepo, &fakeTurnoUserRepo{}, escalaRepo, substituicaoRepo)

	resp, err := svc.GetVigiaTurno(ctx, userID.String(), empresaID.String())
	if err != nil {
		t.Fatalf("GetVigiaTurno() erro inesperado: %v", err)
	}
	if resp.TemTurnoAtivo {
		t.Error("TemTurnoAtivo = true, esperado false")
	}
	if resp.ProximoTurno == nil {
		t.Fatal("ProximoTurno nao deveria ser nil (escala recorrente tem proxima ocorrencia)")
	}
}

func TestGetVigiaTurno_ComTurnoAtivo_RetornaTurno(t *testing.T) {
	ctx := context.Background()
	empresaID := uuid.New()
	userID := uuid.New()
	turnoID := uuid.New()
	postoID := uuid.New()
	now := timeutil.NowBRT()
	token := "sessao-token"
	deviceID := "device-abc"

	turno := &model.Turno{
		ID:             turnoID,
		EmpresaID:      empresaID,
		UsuarioID:      userID,
		PostoID:        postoID,
		PostoNome:      "Posto Central",
		Status:         "em_andamento",
		InicioPrevisto: now.Add(-2 * time.Hour),
		FimPrevisto:    now.Add(10 * time.Hour),
		InicioReal:     ptrTime(now.Add(-2 * time.Hour)),
		TokenSessao:    &token,
		DeviceID:       &deviceID,
		IntervaloMin:   30,
	}

	turnoRepo := &fakeTurnoTurnoRepo{
		findAtivoByUsuarioFn: func(ctx context.Context, eID, uID uuid.UUID) (*model.Turno, error) {
			return turno, nil
		},
	}
	checkinRepo := &fakeTurnoCheckinRepo{
		findUltimoByTurnoNoErrorFn: func(ctx context.Context, tID uuid.UUID) *model.Checkin {
			return &model.Checkin{
				ID:               uuid.New(),
				TurnoID:          turnoID,
				TimestampCriacao: now.Add(-30 * time.Minute),
				Evento:           "checkin",
			}
		},
		countByTurnoHojeFn: func(ctx context.Context, tID uuid.UUID) (int, error) {
			return 3, nil
		},
	}
	postoRepo := &fakeTurnoPostoRepo{
		findByIDFn: func(ctx context.Context, eID, id uuid.UUID) (*model.Posto, error) {
			return &model.Posto{
				ID:   postoID,
				Nome: "Posto Central",
			}, nil
		},
	}

	svc := newTestTurnoServiceFull(turnoRepo, checkinRepo, postoRepo, &fakeTurnoUserRepo{}, &fakeTurnoEscalaRepo{}, &fakeTurnoSubstituicaoRepo{})

	resp, err := svc.GetVigiaTurno(ctx, userID.String(), empresaID.String())
	if err != nil {
		t.Fatalf("GetVigiaTurno() erro inesperado: %v", err)
	}
	if !resp.TemTurnoAtivo {
		t.Fatal("TemTurnoAtivo = false, esperado true")
	}
	if resp.Turno == nil {
		t.Fatal("Turno nao deveria ser nil")
	}
	if resp.Turno.ID != turnoID {
		t.Errorf("Turno.ID = %v, esperado %v", resp.Turno.ID, turnoID)
	}
	if resp.Turno.Status != "em_andamento" {
		t.Errorf("Turno.Status = %q", resp.Turno.Status)
	}
	if resp.ProximoTurno != nil {
		t.Error("ProximoTurno deveria ser nil (turno ja ativo)")
	}
}

func TestBuscarProximoTurnoAgendado_SemEscalas_RetornaNil(t *testing.T) {
	ctx := context.Background()
	empresaID := uuid.New()
	userID := uuid.New()

	escalaRepo := &fakeTurnoEscalaRepo{
		listAtivasByEmpresaFn: func(ctx context.Context, empresaID uuid.UUID, usuarioIDs, postoIDs []string) ([]model.Escala, error) {
			return nil, nil
		},
	}
	substituicaoRepo := &fakeTurnoSubstituicaoRepo{
		listAtivasByDateRangeFn: func(ctx context.Context, empresaID uuid.UUID, dataInicio, dataFim string, usuarioIDs, postoIDs []string) ([]model.Substituicao, error) {
			return nil, nil
		},
	}

	svc := newTestTurnoServiceFull(&fakeTurnoTurnoRepo{}, &fakeTurnoCheckinRepo{}, &fakeTurnoPostoRepo{}, &fakeTurnoUserRepo{}, escalaRepo, substituicaoRepo)

	result, err := svc.buscarProximoTurnoAgendado(ctx, empresaID, userID)
	if err != nil {
		t.Fatalf("buscarProximoTurnoAgendado() erro inesperado: %v", err)
	}
	if result != nil {
		t.Errorf("resultado deveria ser nil (sem escalas), veio: %+v", result)
	}
}

func TestBuscarProximoTurnoAgendado_ComEscalaFutura_RetornaTurno(t *testing.T) {
	ctx := context.Background()
	empresaID := uuid.New()
	userID := uuid.New()
	postoID := uuid.New()
	now := timeutil.NowBRT()
	amanha := now.AddDate(0, 0, 1)
	diaSemanaAmanha := int16(amanha.Weekday())

	escalaRepo := &fakeTurnoEscalaRepo{
		listAtivasByEmpresaFn: func(ctx context.Context, empresaID uuid.UUID, usuarioIDs, postoIDs []string) ([]model.Escala, error) {
			return []model.Escala{
				{
					ID:              uuid.New(),
					EmpresaID:       empresaID,
					UsuarioID:       userID,
					PostoID:         postoID,
					DiaSemanaInicio: diaSemanaAmanha,
					HoraInicio:      "08:00",
					DiaSemanaFim:    diaSemanaAmanha,
					HoraFim:         "17:00",
					Ativo:           true,
				},
			}, nil
		},
	}
	substituicaoRepo := &fakeTurnoSubstituicaoRepo{
		listAtivasByDateRangeFn: func(ctx context.Context, empresaID uuid.UUID, dataInicio, dataFim string, usuarioIDs, postoIDs []string) ([]model.Substituicao, error) {
			return nil, nil
		},
	}
	postoRepo := &fakeTurnoPostoRepo{
		findByIDFn: func(ctx context.Context, eID, id uuid.UUID) (*model.Posto, error) {
			return &model.Posto{ID: postoID, Nome: "Posto Amanha"}, nil
		},
	}

	svc := newTestTurnoServiceFull(&fakeTurnoTurnoRepo{}, &fakeTurnoCheckinRepo{}, postoRepo, &fakeTurnoUserRepo{}, escalaRepo, substituicaoRepo)

	result, err := svc.buscarProximoTurnoAgendado(ctx, empresaID, userID)
	if err != nil {
		t.Fatalf("buscarProximoTurnoAgendado() erro inesperado: %v", err)
	}
	if result == nil {
		t.Fatal("resultado nao deveria ser nil (escala futura existe)")
	}
	if result.Posto == nil || result.Posto.ID != postoID {
		t.Errorf("Posto.ID = %v, esperado %v", result.Posto.ID, postoID)
	}
}

func TestGetTurnos_SemEscalas_RetornaVazio(t *testing.T) {
	ctx := context.Background()
	empresaID := uuid.New()
	userID := uuid.New()

	escalaRepo := &fakeTurnoEscalaRepo{
		listAtivasByEmpresaFn: func(ctx context.Context, empresaID uuid.UUID, usuarioIDs, postoIDs []string) ([]model.Escala, error) {
			return nil, nil
		},
	}
	substituicaoRepo := &fakeTurnoSubstituicaoRepo{
		listAtivasByDateRangeFn: func(ctx context.Context, empresaID uuid.UUID, dataInicio, dataFim string, usuarioIDs, postoIDs []string) ([]model.Substituicao, error) {
			return nil, nil
		},
	}

	svc := newTestTurnoServiceFull(&fakeTurnoTurnoRepo{}, &fakeTurnoCheckinRepo{}, &fakeTurnoPostoRepo{}, &fakeTurnoUserRepo{}, escalaRepo, substituicaoRepo)

	filter := model.TurnoFilter{
		Status:    []string{"agendado"},
		UsuarioID: []string{userID.String()},
	}

	turnos, total, err := svc.GetTurnos(ctx, empresaID.String(), filter)
	if err != nil {
		t.Fatalf("GetTurnos() erro inesperado: %v", err)
	}
	if total != 0 {
		t.Errorf("total = %d, esperado 0", total)
	}
	if len(turnos) != 0 {
		t.Errorf("len(turnos) = %d, esperado 0 (sem escalas nao deveria gerar turnos agendados)", len(turnos))
	}
}

func TestGetTurnos_ComEscala_RetornaAgendados(t *testing.T) {
	ctx := context.Background()
	empresaID := uuid.New()
	userID := uuid.New()
	postoID := uuid.New()
	now := timeutil.NowBRT()
	diaSemanaHoje := int16(now.Weekday())

	escalaRepo := &fakeTurnoEscalaRepo{
		listAtivasByEmpresaFn: func(ctx context.Context, empresaID uuid.UUID, usuarioIDs, postoIDs []string) ([]model.Escala, error) {
			return []model.Escala{
				{
					ID:              uuid.New(),
					EmpresaID:       empresaID,
					UsuarioID:       userID,
					PostoID:         postoID,
					DiaSemanaInicio: diaSemanaHoje,
					HoraInicio:      "08:00",
					DiaSemanaFim:    diaSemanaHoje,
					HoraFim:         "17:00",
					Ativo:           true,
					UsuarioNome:     "Vigia Teste",
					PostoNome:       "Posto Teste",
				},
			}, nil
		},
	}
	substituicaoRepo := &fakeTurnoSubstituicaoRepo{
		listAtivasByDateRangeFn: func(ctx context.Context, empresaID uuid.UUID, dataInicio, dataFim string, usuarioIDs, postoIDs []string) ([]model.Substituicao, error) {
			return nil, nil
		},
	}

	svc := newTestTurnoServiceFull(&fakeTurnoTurnoRepo{}, &fakeTurnoCheckinRepo{}, &fakeTurnoPostoRepo{}, &fakeTurnoUserRepo{}, escalaRepo, substituicaoRepo)

	filter := model.TurnoFilter{
		Status:    []string{"agendado"},
		UsuarioID: []string{userID.String()},
		DataInicio: now.Format("2006-01-02"),
		DataFim:    now.Format("2006-01-02"),
	}

	turnos, total, err := svc.GetTurnos(ctx, empresaID.String(), filter)
	if err != nil {
		t.Fatalf("GetTurnos() erro inesperado: %v", err)
	}
	if total == 0 {
		t.Fatal("total = 0, esperado > 0 (escala existe para hoje)")
	}
	if len(turnos) == 0 {
		t.Fatal("len(turnos) = 0, esperado > 0")
	}
	if turnos[0].Turno.Status != "agendado" {
		t.Errorf("Status = %q, esperado 'agendado'", turnos[0].Turno.Status)
	}
	if turnos[0].Turno.UsuarioID != userID {
		t.Errorf("UsuarioID = %v, esperado %v", turnos[0].Turno.UsuarioID, userID)
	}
}
