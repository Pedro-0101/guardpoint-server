package service

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/guardpoint/guardpoint-server/internal/model"
)

func ptrTime(t time.Time) *time.Time { return &t }

type fakeTurnoTurnoRepo struct {
	findAtivoByUsuarioFn func(ctx context.Context, empresaID, usuarioID uuid.UUID) (*model.Turno, error)
	findByIDFn           func(ctx context.Context, empresaID, id uuid.UUID) (*model.Turno, error)
	listAtivosFn         func(ctx context.Context, empresaID uuid.UUID) ([]model.Turno, error)
	listHistoricoFn      func(ctx context.Context, empresaID uuid.UUID, filter model.HistoricoFilter) ([]model.Turno, int, error)
}

func (m *fakeTurnoTurnoRepo) Create(ctx context.Context, t *model.Turno) error { return nil }
func (m *fakeTurnoTurnoRepo) FindAtivoByUsuario(ctx context.Context, empresaID, usuarioID uuid.UUID) (*model.Turno, error) {
	if m.findAtivoByUsuarioFn != nil {
		return m.findAtivoByUsuarioFn(ctx, empresaID, usuarioID)
	}
	return nil, nil
}
func (m *fakeTurnoTurnoRepo) FindByID(ctx context.Context, empresaID, id uuid.UUID) (*model.Turno, error) {
	if m.findByIDFn != nil {
		return m.findByIDFn(ctx, empresaID, id)
	}
	return nil, nil
}
func (m *fakeTurnoTurnoRepo) UpdateStatus(ctx context.Context, id, empresaID uuid.UUID, status string, fimReal *time.Time) error {
	return nil
}
func (m *fakeTurnoTurnoRepo) ListAtivos(ctx context.Context, empresaID uuid.UUID) ([]model.Turno, error) {
	if m.listAtivosFn != nil {
		return m.listAtivosFn(ctx, empresaID)
	}
	return nil, nil
}
func (m *fakeTurnoTurnoRepo) ListHistorico(ctx context.Context, empresaID uuid.UUID, filter model.HistoricoFilter) ([]model.Turno, int, error) {
	if m.listHistoricoFn != nil {
		return m.listHistoricoFn(ctx, empresaID, filter)
	}
	return nil, 0, nil
}
func (m *fakeTurnoTurnoRepo) ListTurnos(ctx context.Context, empresaID uuid.UUID, filter model.TurnoFilter) ([]model.Turno, error) {
	return nil, nil
}
func (m *fakeTurnoTurnoRepo) ListTurnosByDateRange(ctx context.Context, empresaID uuid.UUID, dataInicio, dataFim string, usuarioIDs, postoIDs []string) ([]model.Turno, error) {
	return nil, nil
}
func (m *fakeTurnoTurnoRepo) RevogarToken(ctx context.Context, id, empresaID uuid.UUID, pin string, pinValidoAte time.Time) error {
	return nil
}
func (m *fakeTurnoTurnoRepo) FindAtivoComPinByUsuario(ctx context.Context, empresaID, usuarioID uuid.UUID) (*model.Turno, error) {
	return nil, nil
}
func (m *fakeTurnoTurnoRepo) Reassociar(ctx context.Context, id, empresaID uuid.UUID, deviceID, tokenSessao string) error {
	return nil
}

type fakeTurnoCheckinRepo struct {
	findUltimoByTurnoNoErrorFn func(ctx context.Context, turnoID uuid.UUID) *model.Checkin
	countByTurnoHojeFn         func(ctx context.Context, turnoID uuid.UUID) (int, error)
	listByTurnoFn              func(ctx context.Context, turnoID uuid.UUID) ([]model.Checkin, error)
}

func (m *fakeTurnoCheckinRepo) Create(ctx context.Context, c *model.Checkin) error { return nil }
func (m *fakeTurnoCheckinRepo) FindUltimoByTurnoNoError(ctx context.Context, turnoID uuid.UUID) *model.Checkin {
	if m.findUltimoByTurnoNoErrorFn != nil {
		return m.findUltimoByTurnoNoErrorFn(ctx, turnoID)
	}
	return nil
}
func (m *fakeTurnoCheckinRepo) CountByTurnoHoje(ctx context.Context, turnoID uuid.UUID) (int, error) {
	if m.countByTurnoHojeFn != nil {
		return m.countByTurnoHojeFn(ctx, turnoID)
	}
	return 0, nil
}
func (m *fakeTurnoCheckinRepo) CreateIdempotent(ctx context.Context, c *model.Checkin) (bool, error) {
	return false, nil
}
func (m *fakeTurnoCheckinRepo) ListByTurno(ctx context.Context, turnoID uuid.UUID) ([]model.Checkin, error) {
	if m.listByTurnoFn != nil {
		return m.listByTurnoFn(ctx, turnoID)
	}
	return nil, nil
}

type fakeTurnoPostoRepo struct {
	findByIDFn func(ctx context.Context, empresaID, id uuid.UUID) (*model.Posto, error)
}

func (m *fakeTurnoPostoRepo) FindByID(ctx context.Context, empresaID, id uuid.UUID) (*model.Posto, error) {
	if m.findByIDFn != nil {
		return m.findByIDFn(ctx, empresaID, id)
	}
	return nil, nil
}

type fakeTurnoUserRepo struct {
	findByIDFn func(ctx context.Context, id uuid.UUID) (*model.User, error)
}

func (m *fakeTurnoUserRepo) FindByID(ctx context.Context, id uuid.UUID) (*model.User, error) {
	if m.findByIDFn != nil {
		return m.findByIDFn(ctx, id)
	}
	return nil, nil
}

type fakeTurnoSessaoDispositivoRepo struct{}

func (m *fakeTurnoSessaoDispositivoRepo) FindByDeviceID(ctx context.Context, empresaID, deviceID string) (*model.SessaoDispositivo, error) {
	return nil, nil
}

type fakeTurnoEscalaRepo struct{}

func (m *fakeTurnoEscalaRepo) FindAtivaByUsuarioPostoDia(ctx context.Context, empresaID, usuarioID, postoID uuid.UUID, diaSemana int16) (*model.Escala, error) {
	return nil, nil
}
func (m *fakeTurnoEscalaRepo) ListAtivasByEmpresa(ctx context.Context, empresaID uuid.UUID, usuarioIDs, postoIDs []string) ([]model.Escala, error) {
	return nil, nil
}

type fakeTurnoSubstituicaoRepo struct{}

func (m *fakeTurnoSubstituicaoRepo) FindAtivaByUsuarioPostoData(ctx context.Context, empresaID, usuarioID, postoID uuid.UUID, data time.Time) (*model.Substituicao, error) {
	return nil, nil
}
func (m *fakeTurnoSubstituicaoRepo) ListAtivasByDateRange(ctx context.Context, empresaID uuid.UUID, dataInicio, dataFim string, usuarioIDs, postoIDs []string) ([]model.Substituicao, error) {
	return nil, nil
}

type fakeTurnoSenhaVigiaRepo struct{}

func (m *fakeTurnoSenhaVigiaRepo) CountByUsuario(ctx context.Context, empresaID, usuarioID uuid.UUID) (int, error) {
	return 0, nil
}
func (m *fakeTurnoSenhaVigiaRepo) FindByUsuarioECodigo(ctx context.Context, empresaID, usuarioID uuid.UUID, codigo string) (*model.SenhaVigia, error) {
	return nil, nil
}

func newTestTurnoService(
	turnoRepo *fakeTurnoTurnoRepo,
	checkinRepo *fakeTurnoCheckinRepo,
	postoRepo *fakeTurnoPostoRepo,
	userRepo *fakeTurnoUserRepo,
) *TurnoService {
	return NewTurnoService(
		turnoRepo,
		checkinRepo,
		postoRepo,
		userRepo,
		&fakeTurnoSessaoDispositivoRepo{},
		&fakeTurnoEscalaRepo{},
		&fakeTurnoSubstituicaoRepo{},
		&fakeTurnoSenhaVigiaRepo{},
		nil,
		nil,
	)
}

func TestTurnoService_GetStatus_ActiveTurno(t *testing.T) {
	ctx := context.Background()
	empresaID := uuid.New()
	userID := uuid.New()
	turnoID := uuid.New()
	token := "sessao-token"
	deviceID := "device-abc"
	now := time.Now()

	turno := &model.Turno{
		ID:             turnoID,
		EmpresaID:      empresaID,
		UsuarioID:      userID,
		PostoID:        uuid.New(),
		PostoNome:      "Posto Central",
		Status:         "em_andamento",
		InicioPrevisto: now.Add(-2 * time.Hour),
		FimPrevisto:    now.Add(10 * time.Hour),
		InicioReal:     ptrTime(now.Add(-2 * time.Hour)),
		TokenSessao:    &token,
		DeviceID:       &deviceID,
		IntervaloMin:   30,
	}

	ultimoCheckin := &model.Checkin{
		ID:               uuid.New(),
		TurnoID:          turnoID,
		TimestampCriacao: now.Add(-30 * time.Minute),
		Evento:           "checkin",
	}

	turnoRepo := &fakeTurnoTurnoRepo{
		findAtivoByUsuarioFn: func(ctx context.Context, eID, uID uuid.UUID) (*model.Turno, error) {
			return turno, nil
		},
	}
	checkinRepo := &fakeTurnoCheckinRepo{
		findUltimoByTurnoNoErrorFn: func(ctx context.Context, tID uuid.UUID) *model.Checkin {
			return ultimoCheckin
		},
		countByTurnoHojeFn: func(ctx context.Context, tID uuid.UUID) (int, error) {
			return 4, nil
		},
	}

	svc := newTestTurnoService(turnoRepo, checkinRepo, &fakeTurnoPostoRepo{}, &fakeTurnoUserRepo{})

	resp, err := svc.GetStatus(ctx, userID.String(), empresaID.String())
	if err != nil {
		t.Fatalf("GetStatus() erro inesperado: %v", err)
	}
	if resp.Turno.ID != turnoID {
		t.Errorf("Turno.ID = %v, esperado %v", resp.Turno.ID, turnoID)
	}
	if resp.Turno.Status != "em_andamento" {
		t.Errorf("Turno.Status = %q, esperado %q", resp.Turno.Status, "em_andamento")
	}
	if resp.UltimoCheckin == nil {
		t.Fatal("UltimoCheckin não deveria ser nil")
	}
	if resp.UltimoCheckin.ID != ultimoCheckin.ID {
		t.Errorf("UltimoCheckin.ID = %v, esperado %v", resp.UltimoCheckin.ID, ultimoCheckin.ID)
	}
	if resp.ProximoDeadline == nil {
		t.Fatal("ProximoDeadline não deveria ser nil")
	}
	expectedDL := ultimoCheckin.TimestampCriacao.Add(30 * time.Minute)
	if !resp.ProximoDeadline.Equal(expectedDL) {
		t.Errorf("ProximoDeadline = %v, esperado %v", resp.ProximoDeadline, expectedDL)
	}
	if resp.CheckinsHoje != 4 {
		t.Errorf("CheckinsHoje = %d, esperado 4", resp.CheckinsHoje)
	}
}

func TestTurnoService_GetStatus_NoActiveTurno(t *testing.T) {
	ctx := context.Background()
	empresaID := uuid.New()
	userID := uuid.New()

	turnoRepo := &fakeTurnoTurnoRepo{
		findAtivoByUsuarioFn: func(ctx context.Context, eID, uID uuid.UUID) (*model.Turno, error) {
			return nil, nil
		},
	}

	svc := newTestTurnoService(turnoRepo, &fakeTurnoCheckinRepo{}, &fakeTurnoPostoRepo{}, &fakeTurnoUserRepo{})

	_, err := svc.GetStatus(ctx, userID.String(), empresaID.String())
	if err == nil {
		t.Fatal("GetStatus() deveria retornar erro para usuário sem turno ativo")
	}
	if err != ErrTurnoNaoEncontrado {
		t.Errorf("erro = %v, esperado %v", err, ErrTurnoNaoEncontrado)
	}
}

func TestTurnoService_GetStatus_ActiveWithoutCheckin(t *testing.T) {
	ctx := context.Background()
	empresaID := uuid.New()
	userID := uuid.New()
	turnoID := uuid.New()
	now := time.Now()
	token := "sessao-token"
	deviceID := "device-abc"

	turno := &model.Turno{
		ID:             turnoID,
		EmpresaID:      empresaID,
		UsuarioID:      userID,
		PostoID:        uuid.New(),
		PostoNome:      "Posto Norte",
		Status:         "em_andamento",
		InicioPrevisto: now.Add(-1 * time.Hour),
		FimPrevisto:    now.Add(11 * time.Hour),
		InicioReal:     ptrTime(now.Add(-1 * time.Hour)),
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
			return nil
		},
		countByTurnoHojeFn: func(ctx context.Context, tID uuid.UUID) (int, error) {
			return 0, nil
		},
	}

	svc := newTestTurnoService(turnoRepo, checkinRepo, &fakeTurnoPostoRepo{}, &fakeTurnoUserRepo{})

	resp, err := svc.GetStatus(ctx, userID.String(), empresaID.String())
	if err != nil {
		t.Fatalf("GetStatus() erro inesperado: %v", err)
	}
	if resp.UltimoCheckin != nil {
		t.Error("UltimoCheckin deveria ser nil sem checkins")
	}
	if resp.CheckinsHoje != 0 {
		t.Errorf("CheckinsHoje = %d, esperado 0", resp.CheckinsHoje)
	}
	if resp.ProximoDeadline == nil {
		t.Fatal("ProximoDeadline não deveria ser nil com InicioReal definido")
	}
	expectedDL := turno.InicioReal.Add(30 * time.Minute)
	if !resp.ProximoDeadline.Equal(expectedDL) {
		t.Errorf("ProximoDeadline = %v, esperado %v (baseado no inicio_real)", resp.ProximoDeadline, expectedDL)
	}
}

func TestTurnoService_GetStatus_InvalidUserID(t *testing.T) {
	ctx := context.Background()
	empresaID := uuid.New().String()

	svc := newTestTurnoService(&fakeTurnoTurnoRepo{}, &fakeTurnoCheckinRepo{}, &fakeTurnoPostoRepo{}, &fakeTurnoUserRepo{})

	_, err := svc.GetStatus(ctx, "nao-e-uuid", empresaID)
	if err == nil {
		t.Fatal("GetStatus() deveria retornar erro para user_id inválido")
	}
}

func TestTurnoService_GetByID_Success(t *testing.T) {
	ctx := context.Background()
	empresaID := uuid.New()
	turnoID := uuid.New()
	postoID := uuid.New()
	userID := uuid.New()
	now := time.Now()
	token := "sessao-token"
	deviceID := "device-xyz"

	turno := &model.Turno{
		ID:             turnoID,
		EmpresaID:      empresaID,
		UsuarioID:      userID,
		PostoID:        postoID,
		PostoNome:      "Posto Leste",
		UsuarioNome:    "Vigia Silva",
		Status:         "em_andamento",
		InicioPrevisto: now.Add(-3 * time.Hour),
		FimPrevisto:    now.Add(9 * time.Hour),
		InicioReal:     ptrTime(now.Add(-3 * time.Hour)),
		TokenSessao:    &token,
		DeviceID:       &deviceID,
		IntervaloMin:   30,
	}

	checkins := []model.Checkin{
		{
			ID:               uuid.New(),
			TurnoID:          turnoID,
			TimestampCriacao: now.Add(-2 * time.Hour),
			Evento:           "checkin",
			FlagGeofence:     stringPtr("ok"),
		},
		{
			ID:               uuid.New(),
			TurnoID:          turnoID,
			TimestampCriacao: now.Add(-1 * time.Hour),
			Evento:           "checkin",
			FlagGeofence:     stringPtr("ok"),
		},
	}

	posto := &model.Posto{
		ID:        postoID,
		EmpresaID: empresaID,
		Nome:      "Posto Leste",
		Latitude:  -23.5505,
		Longitude: -46.6333,
		RaioM:     100,
		Ativo:     true,
	}

	usuario := &model.User{
		ID:        userID,
		EmpresaID: empresaID,
		Nome:      "Vigia Silva",
		Email:     "silva@guardpoint.com",
		Role:      "vigia",
		Ativo:     true,
	}

	turnoRepo := &fakeTurnoTurnoRepo{
		findByIDFn: func(ctx context.Context, eID, id uuid.UUID) (*model.Turno, error) {
			return turno, nil
		},
	}
	checkinRepo := &fakeTurnoCheckinRepo{
		listByTurnoFn: func(ctx context.Context, tID uuid.UUID) ([]model.Checkin, error) {
			return checkins, nil
		},
	}
	postoRepo := &fakeTurnoPostoRepo{
		findByIDFn: func(ctx context.Context, eID, id uuid.UUID) (*model.Posto, error) {
			return posto, nil
		},
	}
	userRepo := &fakeTurnoUserRepo{
		findByIDFn: func(ctx context.Context, id uuid.UUID) (*model.User, error) {
			return usuario, nil
		},
	}

	svc := newTestTurnoService(turnoRepo, checkinRepo, postoRepo, userRepo)

	resp, err := svc.GetByID(ctx, empresaID.String(), turnoID.String())
	if err != nil {
		t.Fatalf("GetByID() erro inesperado: %v", err)
	}
	if resp.Turno.ID != turnoID {
		t.Errorf("Turno.ID = %v, esperado %v", resp.Turno.ID, turnoID)
	}
	if resp.Posto == nil {
		t.Fatal("Posto não deveria ser nil")
	}
	if resp.Posto.Nome != "Posto Leste" {
		t.Errorf("Posto.Nome = %q, esperado %q", resp.Posto.Nome, "Posto Leste")
	}
	if resp.Usuario == nil {
		t.Fatal("Usuario não deveria ser nil")
	}
	if resp.Usuario.Nome != "Vigia Silva" {
		t.Errorf("Usuario.Nome = %q, esperado %q", resp.Usuario.Nome, "Vigia Silva")
	}
	if resp.Usuario.SenhaHash != "" {
		t.Error("Usuario.SenhaHash deveria ser sanitizada para string vazia")
	}
	if resp.Usuario.Telefone != nil {
		t.Error("Usuario.Telefone deveria ser nil após sanitização")
	}
	if len(resp.Checkins) != 2 {
		t.Errorf("len(Checkins) = %d, esperado 2", len(resp.Checkins))
	}
}

func TestTurnoService_GetByID_NotFound(t *testing.T) {
	ctx := context.Background()
	empresaID := uuid.New()
	turnoID := uuid.New()

	turnoRepo := &fakeTurnoTurnoRepo{
		findByIDFn: func(ctx context.Context, eID, id uuid.UUID) (*model.Turno, error) {
			return nil, ErrTurnoNaoEncontrado
		},
	}

	svc := newTestTurnoService(turnoRepo, &fakeTurnoCheckinRepo{}, &fakeTurnoPostoRepo{}, &fakeTurnoUserRepo{})

	_, err := svc.GetByID(ctx, empresaID.String(), turnoID.String())
	if err == nil {
		t.Fatal("GetByID() deveria retornar erro para turno não encontrado")
	}
	if err != ErrTurnoNaoEncontrado {
		t.Errorf("erro = %v, esperado %v", err, ErrTurnoNaoEncontrado)
	}
}

func TestTurnoService_GetHistorico_Success(t *testing.T) {
	ctx := context.Background()
	empresaID := uuid.New()
	userID := uuid.New()
	postoID := uuid.New()
	now := time.Now()

	turnos := []model.Turno{
		{
			ID:             uuid.New(),
			EmpresaID:      empresaID,
			UsuarioID:      userID,
			PostoID:        postoID,
			PostoNome:      "Posto Sul",
			UsuarioNome:    "Vigia Costa",
			Status:         "finalizado",
			InicioPrevisto: now.Add(-24 * time.Hour),
			FimPrevisto:    now.Add(-12 * time.Hour),
			InicioReal:     ptrTime(now.Add(-24 * time.Hour)),
			FimReal:        ptrTime(now.Add(-12 * time.Hour)),
			IntervaloMin:   30,
		},
	}

	turnoRepo := &fakeTurnoTurnoRepo{
		listHistoricoFn: func(ctx context.Context, eID uuid.UUID, filter model.HistoricoFilter) ([]model.Turno, int, error) {
			return turnos, 1, nil
		},
	}

	svc := newTestTurnoService(turnoRepo, &fakeTurnoCheckinRepo{}, &fakeTurnoPostoRepo{}, &fakeTurnoUserRepo{})

	filter := model.HistoricoFilter{
		DataInicio: now.Add(-48 * time.Hour).Format("2006-01-02"),
		DataFim:    now.Format("2006-01-02"),
		Limit:      20,
	}
	result, total, err := svc.GetHistorico(ctx, empresaID.String(), filter)
	if err != nil {
		t.Fatalf("GetHistorico() erro inesperado: %v", err)
	}
	if total != 1 {
		t.Errorf("total = %d, esperado 1", total)
	}
	if len(result) != 1 {
		t.Fatalf("len(result) = %d, esperado 1", len(result))
	}
	if result[0].Status != "finalizado" {
		t.Errorf("Status = %q, esperado %q", result[0].Status, "finalizado")
	}
	if result[0].UsuarioNome != "Vigia Costa" {
		t.Errorf("UsuarioNome = %q, esperado %q", result[0].UsuarioNome, "Vigia Costa")
	}
}

func TestTurnoService_GetHistorico_Empty(t *testing.T) {
	ctx := context.Background()
	empresaID := uuid.New()

	turnoRepo := &fakeTurnoTurnoRepo{
		listHistoricoFn: func(ctx context.Context, eID uuid.UUID, filter model.HistoricoFilter) ([]model.Turno, int, error) {
			return nil, 0, nil
		},
	}

	svc := newTestTurnoService(turnoRepo, &fakeTurnoCheckinRepo{}, &fakeTurnoPostoRepo{}, &fakeTurnoUserRepo{})

	result, total, err := svc.GetHistorico(ctx, empresaID.String(), model.HistoricoFilter{Limit: 10})
	if err != nil {
		t.Fatalf("GetHistorico() erro inesperado: %v", err)
	}
	if total != 0 {
		t.Errorf("total = %d, esperado 0", total)
	}
	if len(result) != 0 {
		t.Errorf("len(result) = %d, esperado 0", len(result))
	}
}

func TestTurnoService_GetAtivos_Empty(t *testing.T) {
	ctx := context.Background()
	empresaID := uuid.New()

	turnoRepo := &fakeTurnoTurnoRepo{
		listAtivosFn: func(ctx context.Context, eID uuid.UUID) ([]model.Turno, error) {
			return nil, nil
		},
	}

	svc := newTestTurnoService(turnoRepo, &fakeTurnoCheckinRepo{}, &fakeTurnoPostoRepo{}, &fakeTurnoUserRepo{})

	result, err := svc.GetAtivos(ctx, empresaID.String())
	if err != nil {
		t.Fatalf("GetAtivos() erro inesperado: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("len(result) = %d, esperado 0", len(result))
	}
}

func TestTurnoService_GetAtivos_WithTurnos(t *testing.T) {
	ctx := context.Background()
	empresaID := uuid.New()
	userID := uuid.New()
	postoID := uuid.New()
	turnoID := uuid.New()
	now := time.Now()
	token := "sessao-abc"
	deviceID := "device-001"

	turnos := []model.Turno{
		{
			ID:             turnoID,
			EmpresaID:      empresaID,
			UsuarioID:      userID,
			PostoID:        postoID,
			PostoNome:      "Posto Oeste",
			Status:         "em_andamento",
			InicioPrevisto: now.Add(-1 * time.Hour),
			FimPrevisto:    now.Add(11 * time.Hour),
			InicioReal:     ptrTime(now.Add(-1 * time.Hour)),
			TokenSessao:    &token,
			DeviceID:       &deviceID,
			IntervaloMin:   30,
		},
	}

	posto := &model.Posto{
		ID:        postoID,
		EmpresaID: empresaID,
		Nome:      "Posto Oeste",
		Latitude:  -23.5505,
		Longitude: -46.6333,
		RaioM:     100,
		Ativo:     true,
	}

	usuario := &model.User{
		ID:        userID,
		EmpresaID: empresaID,
		Nome:      "Vigia Santos",
		Email:     "santos@guardpoint.com",
		Role:      "vigia",
		Ativo:     true,
	}

	ultimoCheckin := &model.Checkin{
		ID:               uuid.New(),
		TurnoID:          turnoID,
		TimestampCriacao: now.Add(-30 * time.Minute),
		Evento:           "checkin",
	}

	turnoRepo := &fakeTurnoTurnoRepo{
		listAtivosFn: func(ctx context.Context, eID uuid.UUID) ([]model.Turno, error) {
			return turnos, nil
		},
	}
	checkinRepo := &fakeTurnoCheckinRepo{
		findUltimoByTurnoNoErrorFn: func(ctx context.Context, tID uuid.UUID) *model.Checkin {
			return ultimoCheckin
		},
	}
	postoRepo := &fakeTurnoPostoRepo{
		findByIDFn: func(ctx context.Context, eID, id uuid.UUID) (*model.Posto, error) {
			return posto, nil
		},
	}
	userRepo := &fakeTurnoUserRepo{
		findByIDFn: func(ctx context.Context, id uuid.UUID) (*model.User, error) {
			return usuario, nil
		},
	}

	svc := newTestTurnoService(turnoRepo, checkinRepo, postoRepo, userRepo)

	result, err := svc.GetAtivos(ctx, empresaID.String())
	if err != nil {
		t.Fatalf("GetAtivos() erro inesperado: %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("len(result) = %d, esperado 1", len(result))
	}
	if result[0].Turno.ID != turnoID {
		t.Errorf("Turno.ID = %v, esperado %v", result[0].Turno.ID, turnoID)
	}
	if result[0].Posto == nil {
		t.Fatal("Posto não deveria ser nil")
	}
	if result[0].Posto.Nome != "Posto Oeste" {
		t.Errorf("Posto.Nome = %q", result[0].Posto.Nome)
	}
	if result[0].Usuario == nil {
		t.Fatal("Usuario não deveria ser nil")
	}
	if result[0].Usuario.Nome != "Vigia Santos" {
		t.Errorf("Usuario.Nome = %q", result[0].Usuario.Nome)
	}
	if result[0].Usuario.SenhaHash != "" {
		t.Error("SenhaHash deveria ser sanitizada")
	}
	if len(result[0].Checkins) != 1 {
		t.Errorf("len(Checkins) = %d, esperado 1", len(result[0].Checkins))
	}
}

func stringPtr(s string) *string { return &s }
