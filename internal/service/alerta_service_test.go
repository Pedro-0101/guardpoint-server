package service

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/guardpoint/guardpoint-server/internal/model"
)

type fakeAlertaAlertaRepo struct {
	findByIDFn       func(ctx context.Context, empresaID, id uuid.UUID) (*model.Alerta, error)
	updateStatusFn   func(ctx context.Context, id, empresaID uuid.UUID, status string, resolvidoEm *time.Time) error
	listFn           func(ctx context.Context, empresaID uuid.UUID, filter model.AlertaFilter) ([]model.Alerta, int, error)
	countPorTipoFn   func(ctx context.Context, empresaID uuid.UUID) ([]model.AlertaPorTipo, error)
	countPorHoraFn   func(ctx context.Context, empresaID uuid.UUID) ([]model.AlertaPorHora, error)
	createFn         func(ctx context.Context, a *model.Alerta) error
	countByTurnoFn   func(ctx context.Context, turnoID uuid.UUID, tipo string) (int, error)
	closeFn          func(ctx context.Context, turnoID uuid.UUID) (int64, error)
}

func (m *fakeAlertaAlertaRepo) Create(ctx context.Context, a *model.Alerta) error {
	if m.createFn != nil { return m.createFn(ctx, a) }; return nil
}
func (m *fakeAlertaAlertaRepo) FindByID(ctx context.Context, empresaID, id uuid.UUID) (*model.Alerta, error) {
	if m.findByIDFn != nil { return m.findByIDFn(ctx, empresaID, id) }; return nil, nil
}
func (m *fakeAlertaAlertaRepo) List(ctx context.Context, empresaID uuid.UUID, filter model.AlertaFilter) ([]model.Alerta, int, error) {
	if m.listFn != nil { return m.listFn(ctx, empresaID, filter) }; return nil, 0, nil
}
func (m *fakeAlertaAlertaRepo) UpdateStatus(ctx context.Context, id, empresaID uuid.UUID, status string, resolvidoEm *time.Time) error {
	if m.updateStatusFn != nil { return m.updateStatusFn(ctx, id, empresaID, status, resolvidoEm) }; return nil
}
func (m *fakeAlertaAlertaRepo) CountByTurnoETipo(ctx context.Context, turnoID uuid.UUID, tipo string) (int, error) {
	if m.countByTurnoFn != nil { return m.countByTurnoFn(ctx, turnoID, tipo) }; return 0, nil
}
func (m *fakeAlertaAlertaRepo) CountPorTipo(ctx context.Context, empresaID uuid.UUID) ([]model.AlertaPorTipo, error) {
	if m.countPorTipoFn != nil { return m.countPorTipoFn(ctx, empresaID) }; return nil, nil
}
func (m *fakeAlertaAlertaRepo) CountPorHora(ctx context.Context, empresaID uuid.UUID) ([]model.AlertaPorHora, error) {
	if m.countPorHoraFn != nil { return m.countPorHoraFn(ctx, empresaID) }; return nil, nil
}
func (m *fakeAlertaAlertaRepo) CloseAlertasResolvidoCheckin(ctx context.Context, turnoID uuid.UUID) (int64, error) {
	if m.closeFn != nil { return m.closeFn(ctx, turnoID) }; return 0, nil
}

type fakeAlertaConfigRepo struct {
	listByEmpresaFn  func(ctx context.Context, empresaID uuid.UUID) ([]model.ConfigEscalonamento, error)
	findByEmpresaFn  func(ctx context.Context, empresaID uuid.UUID) (*model.ConfigEscalonamento, error)
	findByIDEmpresaFn func(ctx context.Context, id, empresaID uuid.UUID) (*model.ConfigEscalonamento, error)
	createFn         func(ctx context.Context, c *model.ConfigEscalonamento) error
	updateFn         func(ctx context.Context, c *model.ConfigEscalonamento) error
	updateUsuariosFn func(ctx context.Context, configID uuid.UUID, usuarioIDs []uuid.UUID) error
	deleteByIDFn     func(ctx context.Context, id, empresaID uuid.UUID) error
}

func (m *fakeAlertaConfigRepo) ListByEmpresa(ctx context.Context, empresaID uuid.UUID) ([]model.ConfigEscalonamento, error) {
	if m.listByEmpresaFn != nil { return m.listByEmpresaFn(ctx, empresaID) }; return nil, nil
}
func (m *fakeAlertaConfigRepo) FindByEmpresa(ctx context.Context, empresaID uuid.UUID) (*model.ConfigEscalonamento, error) {
	if m.findByEmpresaFn != nil { return m.findByEmpresaFn(ctx, empresaID) }; return nil, nil
}
func (m *fakeAlertaConfigRepo) FindByIDEmpresa(ctx context.Context, id, empresaID uuid.UUID) (*model.ConfigEscalonamento, error) {
	if m.findByIDEmpresaFn != nil { return m.findByIDEmpresaFn(ctx, id, empresaID) }; return nil, nil
}
func (m *fakeAlertaConfigRepo) Create(ctx context.Context, c *model.ConfigEscalonamento) error {
	if m.createFn != nil { return m.createFn(ctx, c) }; return nil
}
func (m *fakeAlertaConfigRepo) Update(ctx context.Context, c *model.ConfigEscalonamento) error {
	if m.updateFn != nil { return m.updateFn(ctx, c) }; return nil
}
func (m *fakeAlertaConfigRepo) UpdateUsuarios(ctx context.Context, configID uuid.UUID, usuarioIDs []uuid.UUID) error {
	if m.updateUsuariosFn != nil { return m.updateUsuariosFn(ctx, configID, usuarioIDs) }; return nil
}
func (m *fakeAlertaConfigRepo) DeleteByID(ctx context.Context, id, empresaID uuid.UUID) error {
	if m.deleteByIDFn != nil { return m.deleteByIDFn(ctx, id, empresaID) }; return nil
}

type fakeAlertaUserRepo struct {
	findByIDEmpresaFn func(ctx context.Context, empresaID, id uuid.UUID) (*model.User, error)
}

func (m *fakeAlertaUserRepo) FindByIDEmpresa(ctx context.Context, empresaID, id uuid.UUID) (*model.User, error) {
	if m.findByIDEmpresaFn != nil { return m.findByIDEmpresaFn(ctx, empresaID, id) }
	return nil, nil
}

func makeAlertaService(alertaRepo AlertaAlertaRepository, configRepo EscalonamentoConfigRepository, userRepo EscalonamentoUserRepository) *AlertaService {
	escSvc := NewEscalonamentoService(configRepo, userRepo)
	return NewAlertaService(alertaRepo, escSvc, nil)
}

func makeEscalonamentoService(configRepo EscalonamentoConfigRepository, userRepo EscalonamentoUserRepository) *EscalonamentoService {
	return NewEscalonamentoService(configRepo, userRepo)
}

func TestAlertaService_Reconhecer_Success(t *testing.T) {
	ctx := context.Background()
	alertaID := uuid.New()
	empresaID := uuid.New()

	updateCalled := false
	svc := makeAlertaService(
		&fakeAlertaAlertaRepo{
			findByIDFn: func(ctx context.Context, eID, id uuid.UUID) (*model.Alerta, error) {
				return &model.Alerta{ID: alertaID, Status: "aberto"}, nil
			},
			updateStatusFn: func(ctx context.Context, id, eID uuid.UUID, status string, resolved *time.Time) error {
				updateCalled = true
				return nil
			},
		},
		nil,
		nil,
	)

	err := svc.Reconhecer(ctx, empresaID.String(), alertaID.String())
	if err != nil {
		t.Fatalf("Reconhecer() erro: %v", err)
	}
	if !updateCalled {
		t.Error("UpdateStatus nao foi chamado")
	}
}

func TestAlertaService_Reconhecer_AlertaNaoAberto(t *testing.T) {
	ctx := context.Background()
	alertaID := uuid.New()
	empresaID := uuid.New()

	svc := makeAlertaService(
		&fakeAlertaAlertaRepo{
			findByIDFn: func(ctx context.Context, eID, id uuid.UUID) (*model.Alerta, error) {
				return &model.Alerta{ID: alertaID, Status: "encerrado"}, nil
			},
		},
		nil, nil,
	)

	err := svc.Reconhecer(ctx, empresaID.String(), alertaID.String())
	if err == nil {
		t.Fatal("Reconhecer() deveria falhar para alerta encerrado")
	}
}

func TestAlertaService_Encerrar_Success(t *testing.T) {
	ctx := context.Background()
	alertaID := uuid.New()
	empresaID := uuid.New()

	updateCalled := false
	svc := makeAlertaService(
		&fakeAlertaAlertaRepo{
			findByIDFn: func(ctx context.Context, eID, id uuid.UUID) (*model.Alerta, error) {
				return &model.Alerta{ID: alertaID, Status: "aberto"}, nil
			},
			updateStatusFn: func(ctx context.Context, id, eID uuid.UUID, status string, resolved *time.Time) error {
				updateCalled = true
				return nil
			},
		},
		nil, nil,
	)

	err := svc.Encerrar(ctx, empresaID.String(), alertaID.String())
	if err != nil {
		t.Fatalf("Encerrar() erro: %v", err)
	}
	if !updateCalled {
		t.Error("UpdateStatus nao foi chamado")
	}
}

func TestAlertaService_Encerrar_JaEncerrado(t *testing.T) {
	ctx := context.Background()
	alertaID := uuid.New()
	empresaID := uuid.New()

	svc := makeAlertaService(
		&fakeAlertaAlertaRepo{
			findByIDFn: func(ctx context.Context, eID, id uuid.UUID) (*model.Alerta, error) {
				return &model.Alerta{ID: alertaID, Status: "encerrado"}, nil
			},
		},
		nil, nil,
	)

	err := svc.Encerrar(ctx, empresaID.String(), alertaID.String())
	if err == nil {
		t.Fatal("Encerrar() deveria falhar para alerta ja encerrado")
	}
}

func TestAlertaService_Encerrar_AlertaNaoEncontrado(t *testing.T) {
	ctx := context.Background()
	empresaID := uuid.New()

	svc := makeAlertaService(
		&fakeAlertaAlertaRepo{
			findByIDFn: func(ctx context.Context, eID, id uuid.UUID) (*model.Alerta, error) {
				return nil, ErrAlertaNaoEncontrado
			},
		},
		nil, nil,
	)

	err := svc.Encerrar(ctx, empresaID.String(), uuid.New().String())
	if err == nil {
		t.Fatal("Encerrar() deveria falhar para alerta nao encontrado")
	}
}

func TestAlertaService_ValidarUsuariosDaEmpresa(t *testing.T) {
	ctx := context.Background()
	empresaID := uuid.New()
	adminID := uuid.New()
	vigiaID := uuid.New()

	userRepo := &fakeAlertaUserRepo{
		findByIDEmpresaFn: func(ctx context.Context, eID, id uuid.UUID) (*model.User, error) {
			if id == adminID {
				return &model.User{ID: adminID, Role: "admin"}, nil
			}
			if id == vigiaID {
				return &model.User{ID: vigiaID, Role: "vigia"}, nil
			}
			return nil, nil
		},
	}

	svc := makeEscalonamentoService(nil, userRepo)

	t.Run("supervisor valido", func(t *testing.T) {
		sv := uuid.New()
		repo := &fakeAlertaUserRepo{
			findByIDEmpresaFn: func(ctx context.Context, eID, id uuid.UUID) (*model.User, error) {
				return &model.User{ID: sv, Role: "supervisor"}, nil
			},
		}
		s := makeEscalonamentoService(nil, repo)
		err := s.validarUsuariosDaEmpresa(ctx, empresaID, []uuid.UUID{sv})
		if err != nil {
			t.Errorf("supervisor deveria ser valido: %v", err)
		}
	})

	t.Run("vigia rejeitado", func(t *testing.T) {
		err := svc.validarUsuariosDaEmpresa(ctx, empresaID, []uuid.UUID{adminID, vigiaID})
		if err == nil {
			t.Fatal("vigia deveria ser rejeitado como destinatario")
		}
	})

	t.Run("admin valido", func(t *testing.T) {
		err := svc.validarUsuariosDaEmpresa(ctx, empresaID, []uuid.UUID{adminID})
		if err != nil {
			t.Errorf("admin deveria ser valido: %v", err)
		}
	})
}

func TestAlertaService_GetEscalonamentoByID(t *testing.T) {
	ctx := context.Background()
	empresaID := uuid.New()
	configID := uuid.New()

	svc := makeEscalonamentoService(
		&fakeAlertaConfigRepo{
			findByIDEmpresaFn: func(ctx context.Context, id, eID uuid.UUID) (*model.ConfigEscalonamento, error) {
				return &model.ConfigEscalonamento{ID: configID, AtrasoMinutos: 10}, nil
			},
		},
		nil,
	)

	cfg, err := svc.GetEscalonamentoByID(ctx, empresaID.String(), configID.String())
	if err != nil {
		t.Fatalf("GetEscalonamentoByID() erro: %v", err)
	}
	if cfg == nil {
		t.Fatal("ConfigEscalonamento nil")
	}
	if cfg.AtrasoMinutos != 10 {
		t.Errorf("AtrasoMinutos = %d, esperado 10", cfg.AtrasoMinutos)
	}
}

func TestAlertaService_CreateEscalonamento(t *testing.T) {
	ctx := context.Background()
	empresaID := uuid.New()
	usuarioID := uuid.New()

	createCalled := false
	svc := makeEscalonamentoService(
		&fakeAlertaConfigRepo{
			createFn: func(ctx context.Context, c *model.ConfigEscalonamento) error {
				createCalled = true
				c.ID = uuid.New()
				return nil
			},
		},
		&fakeAlertaUserRepo{
			findByIDEmpresaFn: func(ctx context.Context, eID, id uuid.UUID) (*model.User, error) {
				return &model.User{ID: usuarioID, Role: "admin"}, nil
			},
		},
	)

	cfg, err := svc.CreateEscalonamento(ctx, empresaID.String(), model.CreateConfigEscalonamentoRequest{
		AtrasoMinutos: 15,
		Descricao:     "Nivel 1",
		UsuarioIDs:    []uuid.UUID{usuarioID},
	})
	if err != nil {
		t.Fatalf("CreateEscalonamento() erro: %v", err)
	}
	if !createCalled {
		t.Error("Create nao foi chamado")
	}
	if cfg.Sistema != false {
		t.Errorf("Sistema = %v, esperado false", cfg.Sistema)
	}
}

func TestAlertaService_DeleteEscalonamento_Success(t *testing.T) {
	ctx := context.Background()
	empresaID := uuid.New()
	configID := uuid.New()

	deleteCalled := false
	svc := makeEscalonamentoService(
		&fakeAlertaConfigRepo{
			findByIDEmpresaFn: func(ctx context.Context, id, eID uuid.UUID) (*model.ConfigEscalonamento, error) {
				return &model.ConfigEscalonamento{ID: configID, Sistema: false}, nil
			},
			deleteByIDFn: func(ctx context.Context, id, eID uuid.UUID) error {
				deleteCalled = true
				return nil
			},
		},
		nil,
	)

	err := svc.DeleteEscalonamento(ctx, empresaID.String(), configID.String())
	if err != nil {
		t.Fatalf("DeleteEscalonamento() erro: %v", err)
	}
	if !deleteCalled {
		t.Error("DeleteByID nao foi chamado")
	}
}

func TestAlertaService_DeleteEscalonamento_SistemaNaoExcluivel(t *testing.T) {
	ctx := context.Background()
	empresaID := uuid.New()
	configID := uuid.New()

	svc := makeEscalonamentoService(
		&fakeAlertaConfigRepo{
			findByIDEmpresaFn: func(ctx context.Context, id, eID uuid.UUID) (*model.ConfigEscalonamento, error) {
				return &model.ConfigEscalonamento{ID: configID, Sistema: true}, nil
			},
		},
		nil,
	)

	err := svc.DeleteEscalonamento(ctx, empresaID.String(), configID.String())
	if err == nil {
		t.Fatal("DeleteEscalonamento() deveria falhar para config de sistema")
	}
}

func TestAlertaService_UpdateEscalonamento_SistemaNaoEditavel(t *testing.T) {
	ctx := context.Background()
	empresaID := uuid.New()
	configID := uuid.New()

	svc := makeEscalonamentoService(
		&fakeAlertaConfigRepo{
			findByIDEmpresaFn: func(ctx context.Context, id, eID uuid.UUID) (*model.ConfigEscalonamento, error) {
				return &model.ConfigEscalonamento{ID: configID, Sistema: true}, nil
			},
		},
		&fakeAlertaUserRepo{
			findByIDEmpresaFn: func(ctx context.Context, eID, id uuid.UUID) (*model.User, error) {
				return &model.User{Role: "admin"}, nil
			},
		},
	)

	_, err := svc.UpdateEscalonamento(ctx, empresaID.String(), configID.String(), model.UpdateConfigEscalonamentoRequest{
		AtrasoMinutos: 20, UsuarioIDs: []uuid.UUID{},
	})
	if err == nil {
		t.Fatal("UpdateEscalonamento() deveria falhar para config de sistema")
	}
}

func TestAlertaService_UpdateEscalonamentoUsuarios_Success(t *testing.T) {
	ctx := context.Background()
	empresaID := uuid.New()
	configID := uuid.New()
	usuarioID := uuid.New()

	updateCalled := false
	svc := makeEscalonamentoService(
		&fakeAlertaConfigRepo{
			findByIDEmpresaFn: func(ctx context.Context, id, eID uuid.UUID) (*model.ConfigEscalonamento, error) {
				return &model.ConfigEscalonamento{ID: configID}, nil
			},
			updateUsuariosFn: func(ctx context.Context, cID uuid.UUID, uIDs []uuid.UUID) error {
				updateCalled = true
				return nil
			},
		},
		&fakeAlertaUserRepo{
			findByIDEmpresaFn: func(ctx context.Context, eID, id uuid.UUID) (*model.User, error) {
				return &model.User{Role: "admin"}, nil
			},
		},
	)

	cfg, err := svc.UpdateEscalonamentoUsuarios(ctx, empresaID.String(), configID.String(), model.UpdateConfigEscalonamentoUsuariosRequest{
		UsuarioIDs: []uuid.UUID{usuarioID},
	})
	if err != nil {
		t.Fatalf("UpdateEscalonamentoUsuarios() erro: %v", err)
	}
	if !updateCalled {
		t.Error("UpdateUsuarios nao foi chamado")
	}
	if cfg == nil {
		t.Fatal("ConfigEscalonamento nil")
	}
}

func TestAlertaService_List(t *testing.T) {
	ctx := context.Background()
	alertaID := uuid.New()
	empresaID := uuid.New()

	svc := makeAlertaService(
		&fakeAlertaAlertaRepo{
			listFn: func(ctx context.Context, eID uuid.UUID, filter model.AlertaFilter) ([]model.Alerta, int, error) {
				return []model.Alerta{{ID: alertaID, Tipo: "atraso", Status: "aberto"}}, 1, nil
			},
		},
		nil, nil,
	)

	alertas, total, err := svc.List(ctx, empresaID.String(), model.AlertaFilter{Limit: 10})
	if err != nil {
		t.Fatalf("List() erro: %v", err)
	}
	if total != 1 {
		t.Errorf("total = %d, esperado 1", total)
	}
	if len(alertas) != 1 {
		t.Errorf("len = %d, esperado 1", len(alertas))
	}
}

func TestAlertaService_ListEscalonamentos(t *testing.T) {
	ctx := context.Background()
	empresaID := uuid.New()

	svc := makeEscalonamentoService(
		&fakeAlertaConfigRepo{
			listByEmpresaFn: func(ctx context.Context, eID uuid.UUID) ([]model.ConfigEscalonamento, error) {
				return []model.ConfigEscalonamento{
					{ID: uuid.New(), AtrasoMinutos: 10},
					{ID: uuid.New(), AtrasoMinutos: 30},
				}, nil
			},
		},
		nil,
	)

	configs, err := svc.ListEscalonamentos(ctx, empresaID.String())
	if err != nil {
		t.Fatalf("ListEscalonamentos() erro: %v", err)
	}
	if len(configs) != 2 {
		t.Errorf("len = %d, esperado 2", len(configs))
	}
}

func TestAlertaService_GetEstatisticas(t *testing.T) {
	ctx := context.Background()
	empresaID := uuid.New()

	svc := makeAlertaService(
		&fakeAlertaAlertaRepo{
			listFn: func(ctx context.Context, eID uuid.UUID, filter model.AlertaFilter) ([]model.Alerta, int, error) {
				return []model.Alerta{
					{Status: "aberto"},
					{Status: "aberto"},
					{Status: "reconhecido"},
					{Status: "encerrado"},
				}, 4, nil
			},
			countPorTipoFn: func(ctx context.Context, eID uuid.UUID) ([]model.AlertaPorTipo, error) {
				return []model.AlertaPorTipo{{Tipo: "atraso", Quantidade: 3}}, nil
			},
			countPorHoraFn: func(ctx context.Context, eID uuid.UUID) ([]model.AlertaPorHora, error) {
				return []model.AlertaPorHora{{Hora: "08:00", Quantidade: 2}}, nil
			},
		},
		nil, nil,
	)

	stats, err := svc.GetEstatisticas(ctx, empresaID.String())
	if err != nil {
		t.Fatalf("GetEstatisticas() erro: %v", err)
	}
	if stats.TotalAbertos != 2 {
		t.Errorf("TotalAbertos = %d, esperado 2", stats.TotalAbertos)
	}
	if stats.TotalReconhecidos != 1 {
		t.Errorf("TotalReconhecidos = %d, esperado 1", stats.TotalReconhecidos)
	}
	if stats.TotalEncerrados != 1 {
		t.Errorf("TotalEncerrados = %d, esperado 1", stats.TotalEncerrados)
	}
	if len(stats.PorTipo) != 1 {
		t.Errorf("len(PorTipo) = %d, esperado 1", len(stats.PorTipo))
	}
	if len(stats.PorHora) != 1 {
		t.Errorf("len(PorHora) = %d, esperado 1", len(stats.PorHora))
	}
}
