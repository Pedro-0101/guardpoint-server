package service

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"golang.org/x/crypto/bcrypt"

	"github.com/guardpoint/guardpoint-server/internal/model"
)

func TestRealUsuarioService_Create_Success(t *testing.T) {
	ctx := context.Background()
	empresaID := uuid.New()

	svc := NewUsuarioService(&fakeUserRepo{
		findByEmailFn: func(ctx context.Context, email string) (*model.User, error) {
			return nil, fmt.Errorf("usuario nao encontrado")
		},
		createFn: func(ctx context.Context, u *model.User) error {
			u.ID = uuid.New()
			return nil
		},
	})

	req := model.CreateUsuarioRequest{Nome: "Joao Silva", Email: "joao@empresa.com", Senha: "123456", Cargo: "admin"}
	resp, err := svc.Create(ctx, empresaID, req)
	if err != nil {
		t.Fatalf("Create() erro: %v", err)
	}
	if resp.Nome != "Joao Silva" {
		t.Errorf("Nome = %q", resp.Nome)
	}
	if resp.Ativo != true {
		t.Errorf("Ativo = %v, esperado true", resp.Ativo)
	}
}

func TestRealUsuarioService_Create_DuplicateEmail(t *testing.T) {
	ctx := context.Background()
	empresaID := uuid.New()
	existingUser := &model.User{ID: uuid.New(), Email: "joao@empresa.com"}

	svc := NewUsuarioService(&fakeUserRepo{
		findByEmailFn: func(ctx context.Context, email string) (*model.User, error) {
			return existingUser, nil
		},
	})

	req := model.CreateUsuarioRequest{Nome: "Joao", Email: "joao@empresa.com", Senha: "123456", Cargo: "admin"}
	_, err := svc.Create(ctx, empresaID, req)
	if !errors.Is(err, ErrEmailAlreadyExists) {
		t.Errorf("erro = %v, esperado ErrEmailAlreadyExists", err)
	}
}

func TestRealUsuarioService_List(t *testing.T) {
	ctx := context.Background()
	empresaID := uuid.New()
	id1, id2 := uuid.New(), uuid.New()

	svc := NewUsuarioService(&fakeUserRepo{
		listByEmpresaFn: func(ctx context.Context, eID uuid.UUID) ([]model.User, error) {
			return []model.User{
				{ID: id1, EmpresaID: empresaID, Nome: "Alfa", Email: "alfa@x.com", Role: "admin", Ativo: true},
				{ID: id2, EmpresaID: empresaID, Nome: "Beta", Email: "beta@x.com", Role: "vigia", Ativo: true},
			}, nil
		},
	})

	resp, err := svc.List(ctx, empresaID)
	if err != nil {
		t.Fatalf("List() erro: %v", err)
	}
	if len(resp) != 2 {
		t.Fatalf("len = %d, esperado 2", len(resp))
	}
}

func TestRealUsuarioService_GetByID(t *testing.T) {
	ctx := context.Background()
	empresaID, userID := uuid.New(), uuid.New()

	svc := NewUsuarioService(&fakeUserRepo{
		findByIDEmpresaFn: func(ctx context.Context, eID, id uuid.UUID) (*model.User, error) {
			return &model.User{ID: userID, EmpresaID: empresaID, Nome: "Maria", Role: "supervisor", Ativo: true}, nil
		},
	})

	resp, err := svc.GetByID(ctx, empresaID, userID)
	if err != nil {
		t.Fatalf("GetByID() erro: %v", err)
	}
	if resp.Nome != "Maria" {
		t.Errorf("Nome = %q", resp.Nome)
	}
}

func TestRealUsuarioService_Update(t *testing.T) {
	ctx := context.Background()
	empresaID, userID := uuid.New(), uuid.New()
	novoNome := "Novo Nome"

	savedUser := &model.User{ID: userID, EmpresaID: empresaID, Nome: "Antigo", Role: "vigia", Ativo: true}
	updateCalled := false

	svc := NewUsuarioService(&fakeUserRepo{
		findByIDEmpresaFn: func(ctx context.Context, eID, id uuid.UUID) (*model.User, error) {
			return savedUser, nil
		},
		updateFn: func(ctx context.Context, eID, id uuid.UUID, u *model.User) error {
			updateCalled = true
			return nil
		},
	})

	resp, err := svc.Update(ctx, empresaID, userID, model.UpdateUsuarioRequest{Nome: &novoNome})
	if err != nil {
		t.Fatalf("Update() erro: %v", err)
	}
	if !updateCalled {
		t.Error("Update nao foi chamado")
	}
	if resp.Nome != novoNome {
		t.Errorf("Nome = %q", resp.Nome)
	}
}

func TestRealUsuarioService_Deactivate(t *testing.T) {
	ctx := context.Background()
	empresaID, userID := uuid.New(), uuid.New()

	updateCalled := false
	svc := NewUsuarioService(&fakeUserRepo{
		findByIDEmpresaFn: func(ctx context.Context, eID, id uuid.UUID) (*model.User, error) {
			return &model.User{ID: userID, EmpresaID: empresaID, Ativo: true}, nil
		},
		updateFn: func(ctx context.Context, eID, id uuid.UUID, u *model.User) error {
			updateCalled = true
			return nil
		},
	})

	err := svc.Deactivate(ctx, empresaID, userID)
	if err != nil {
		t.Fatalf("Deactivate() erro: %v", err)
	}
	if !updateCalled {
		t.Error("Update nao foi chamado")
	}
}

func TestRealUsuarioService_SenhaHash(t *testing.T) {
	ctx := context.Background()
	empresaID := uuid.New()

	var createdUser *model.User
	svc := NewUsuarioService(&fakeUserRepo{
		findByEmailFn: func(ctx context.Context, email string) (*model.User, error) {
			return nil, fmt.Errorf("nao encontrado")
		},
		createFn: func(ctx context.Context, u *model.User) error {
			u.ID = uuid.New()
			createdUser = u
			return nil
		},
	})

	_, err := svc.Create(ctx, empresaID, model.CreateUsuarioRequest{
		Nome: "Teste", Email: "teste@hash.com", Senha: "minhaSenha123", Cargo: "vigia",
	})
	if err != nil {
		t.Fatalf("Create() erro: %v", err)
	}
	if createdUser == nil {
		t.Fatal("usuario nao criado")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(createdUser.SenhaHash), []byte("minhaSenha123")); err != nil {
		t.Errorf("bcrypt nao confere: %v", err)
	}
}

func TestRealPostoService_Create(t *testing.T) {
	ctx := context.Background()
	empresaID := uuid.New()

	createCalled := false
	svc := NewPostoService(&fakePostoRepo{
		createFn: func(ctx context.Context, p *model.Posto) error {
			createCalled = true
			p.ID = uuid.New()
			return nil
		},
	})

	err := svc.Create(ctx, &model.Posto{EmpresaID: empresaID, Nome: "Posto Central"})
	if err != nil {
		t.Fatalf("Create() erro: %v", err)
	}
	if !createCalled {
		t.Error("Create nao foi chamado")
	}
}

func TestRealPostoService_GetByID(t *testing.T) {
	ctx := context.Background()
	empresaID, postoID := uuid.New(), uuid.New()

	svc := NewPostoService(&fakePostoRepo{
		findByIDFn: func(ctx context.Context, eID, id uuid.UUID) (*model.Posto, error) {
			return &model.Posto{ID: postoID, Nome: "Posto X"}, nil
		},
	})

	resp, err := svc.GetByID(ctx, empresaID, postoID)
	if err != nil {
		t.Fatalf("GetByID() erro: %v", err)
	}
	if resp.Nome != "Posto X" {
		t.Errorf("Nome = %q", resp.Nome)
	}
}

func TestRealPostoService_Deactivate(t *testing.T) {
	ctx := context.Background()
	empresaID, postoID := uuid.New(), uuid.New()

	updateCalled := false
	svc := NewPostoService(&fakePostoRepo{
		findByIDFn: func(ctx context.Context, eID, id uuid.UUID) (*model.Posto, error) {
			return &model.Posto{ID: postoID, Ativo: true}, nil
		},
		updateFn: func(ctx context.Context, eID, id uuid.UUID, p *model.Posto) error {
			updateCalled = true
			return nil
		},
	})

	err := svc.Deactivate(ctx, empresaID, postoID)
	if err != nil {
		t.Fatalf("Deactivate() erro: %v", err)
	}
	if !updateCalled {
		t.Error("Update nao foi chamado")
	}
}

func TestRealEscalaService_Create(t *testing.T) {
	ctx := context.Background()
	empresaID := uuid.New()
	usuarioID, postoID := uuid.New(), uuid.New()

	createCalled := false
	svc := NewEscalaService(&fakeEscalaRepo{
		createFn: func(ctx context.Context, e *model.Escala) error {
			createCalled = true
			e.ID = uuid.New()
			return nil
		},
	})

	req := model.CreateEscalaRequest{
		UsuarioID: usuarioID.String(), PostoID: postoID.String(),
		DiaSemanaInicio: 1, HoraInicio: "08:00", DiaSemanaFim: 1, HoraFim: "17:00",
		ToleranciaMin: 30,
	}
	resp, err := svc.Create(ctx, empresaID, req)
	if err != nil {
		t.Fatalf("Create() erro: %v", err)
	}
	if !createCalled {
		t.Error("Create nao foi chamado")
	}
	if resp.ToleranciaMin != 30 {
		t.Errorf("ToleranciaMin = %d", resp.ToleranciaMin)
	}
}

func TestRealEscalaService_Create_ToleranciaDefault(t *testing.T) {
	ctx := context.Background()
	empresaID := uuid.New()
	usuarioID, postoID := uuid.New(), uuid.New()

	var created *model.Escala
	svc := NewEscalaService(&fakeEscalaRepo{
		createFn: func(ctx context.Context, e *model.Escala) error {
			created = e
			e.ID = uuid.New()
			return nil
		},
	})

	req := model.CreateEscalaRequest{
		UsuarioID: usuarioID.String(), PostoID: postoID.String(),
		DiaSemanaInicio: 1, HoraInicio: "08:00", DiaSemanaFim: 1, HoraFim: "17:00",
		ToleranciaMin: 0,
	}
	_, err := svc.Create(ctx, empresaID, req)
	if err != nil {
		t.Fatalf("Create() erro: %v", err)
	}
	if created == nil {
		t.Fatal("escala nao criada")
	}
	if created.ToleranciaMin != 15 {
		t.Errorf("ToleranciaMin = %d, esperado 15 (default)", created.ToleranciaMin)
	}
}

func TestRealEscalaService_Delete(t *testing.T) {
	ctx := context.Background()
	empresaID, escalaID := uuid.New(), uuid.New()

	updateCalled := false
	svc := NewEscalaService(&fakeEscalaRepo{
		findByIDFn: func(ctx context.Context, eID, id uuid.UUID) (*model.Escala, error) {
			return &model.Escala{ID: escalaID, Ativo: true}, nil
		},
		updateFn: func(ctx context.Context, eID, id uuid.UUID, e *model.Escala) error {
			updateCalled = true
			return nil
		},
	})

	err := svc.Delete(ctx, empresaID, escalaID)
	if err != nil {
		t.Fatalf("Delete() erro: %v", err)
	}
	if !updateCalled {
		t.Error("Update nao foi chamado")
	}
}

func TestRealEscalaService_Update(t *testing.T) {
	ctx := context.Background()
	empresaID, escalaID := uuid.New(), uuid.New()
	novaHoraInicio := "09:00"

	svc := NewEscalaService(&fakeEscalaRepo{
		findByIDFn: func(ctx context.Context, eID, id uuid.UUID) (*model.Escala, error) {
			return &model.Escala{ID: escalaID, EmpresaID: empresaID, DiaSemanaInicio: 1, HoraInicio: "08:00", DiaSemanaFim: 1, HoraFim: "17:00", ToleranciaMin: 15, Ativo: true}, nil
		},
		updateFn: func(ctx context.Context, eID, id uuid.UUID, e *model.Escala) error {
			return nil
		},
	})

	resp, err := svc.Update(ctx, empresaID, escalaID, model.UpdateEscalaRequest{HoraInicio: &novaHoraInicio})
	if err != nil {
		t.Fatalf("Update() erro: %v", err)
	}
	if resp.HoraInicio != "09:00" {
		t.Errorf("HoraInicio = %q", resp.HoraInicio)
	}
}

// =============================================================================
// fake repositories implementing the service interfaces
// =============================================================================

type fakeUserRepo struct {
	listByEmpresaFn   func(ctx context.Context, empresaID uuid.UUID) ([]model.User, error)
	findByIDEmpresaFn func(ctx context.Context, empresaID, id uuid.UUID) (*model.User, error)
	findByEmailFn     func(ctx context.Context, email string) (*model.User, error)
	createFn          func(ctx context.Context, u *model.User) error
	updateFn          func(ctx context.Context, empresaID, id uuid.UUID, u *model.User) error
}

func (m *fakeUserRepo) ListByEmpresa(ctx context.Context, empresaID uuid.UUID) ([]model.User, error) {
	if m.listByEmpresaFn != nil {
		return m.listByEmpresaFn(ctx, empresaID)
	}
	return nil, nil
}

func (m *fakeUserRepo) FindByIDEmpresa(ctx context.Context, empresaID, id uuid.UUID) (*model.User, error) {
	if m.findByIDEmpresaFn != nil {
		return m.findByIDEmpresaFn(ctx, empresaID, id)
	}
	return nil, fmt.Errorf("usuario nao encontrado")
}

func (m *fakeUserRepo) FindByEmail(ctx context.Context, email string) (*model.User, error) {
	if m.findByEmailFn != nil {
		return m.findByEmailFn(ctx, email)
	}
	return nil, fmt.Errorf("usuario nao encontrado")
}

func (m *fakeUserRepo) Create(ctx context.Context, u *model.User) error {
	if m.createFn != nil {
		return m.createFn(ctx, u)
	}
	return nil
}

func (m *fakeUserRepo) Update(ctx context.Context, empresaID, id uuid.UUID, u *model.User) error {
	if m.updateFn != nil {
		return m.updateFn(ctx, empresaID, id, u)
	}
	return nil
}

type fakePostoRepo struct {
	createFn   func(ctx context.Context, p *model.Posto) error
	findByIDFn func(ctx context.Context, empresaID, id uuid.UUID) (*model.Posto, error)
	listFn     func(ctx context.Context, empresaID uuid.UUID, apenasAtivos bool) ([]model.Posto, error)
	updateFn   func(ctx context.Context, empresaID, id uuid.UUID, p *model.Posto) error
}

func (m *fakePostoRepo) Create(ctx context.Context, p *model.Posto) error {
	if m.createFn != nil {
		return m.createFn(ctx, p)
	}
	return nil
}

func (m *fakePostoRepo) FindByID(ctx context.Context, empresaID, id uuid.UUID) (*model.Posto, error) {
	if m.findByIDFn != nil {
		return m.findByIDFn(ctx, empresaID, id)
	}
	return nil, fmt.Errorf("posto nao encontrado")
}

func (m *fakePostoRepo) List(ctx context.Context, empresaID uuid.UUID, apenasAtivos bool) ([]model.Posto, error) {
	if m.listFn != nil {
		return m.listFn(ctx, empresaID, apenasAtivos)
	}
	return nil, nil
}

func (m *fakePostoRepo) Update(ctx context.Context, empresaID, id uuid.UUID, p *model.Posto) error {
	if m.updateFn != nil {
		return m.updateFn(ctx, empresaID, id, p)
	}
	return nil
}

type fakeEscalaRepo struct {
	createFn   func(ctx context.Context, e *model.Escala) error
	findByIDFn func(ctx context.Context, empresaID, id uuid.UUID) (*model.Escala, error)
	listFn     func(ctx context.Context, empresaID uuid.UUID, filter model.EscalaFilter) ([]model.Escala, int, error)
	updateFn   func(ctx context.Context, empresaID, id uuid.UUID, e *model.Escala) error
}

func (m *fakeEscalaRepo) Create(ctx context.Context, e *model.Escala) error {
	if m.createFn != nil {
		return m.createFn(ctx, e)
	}
	return nil
}

func (m *fakeEscalaRepo) FindByID(ctx context.Context, empresaID, id uuid.UUID) (*model.Escala, error) {
	if m.findByIDFn != nil {
		return m.findByIDFn(ctx, empresaID, id)
	}
	return nil, fmt.Errorf("escala nao encontrada")
}

func (m *fakeEscalaRepo) List(ctx context.Context, empresaID uuid.UUID, filter model.EscalaFilter) ([]model.Escala, int, error) {
	if m.listFn != nil {
		return m.listFn(ctx, empresaID, filter)
	}
	return nil, 0, nil
}

func (m *fakeEscalaRepo) Update(ctx context.Context, empresaID, id uuid.UUID, e *model.Escala) error {
	if m.updateFn != nil {
		return m.updateFn(ctx, empresaID, id, e)
	}
	return nil
}

func (m *fakeEscalaRepo) BeginTx(ctx context.Context) (pgx.Tx, error) {
	return nil, fmt.Errorf("nao implementado no fake")
}

func (m *fakeEscalaRepo) CreateWithTx(ctx context.Context, tx pgx.Tx, e *model.Escala) error {
	return fmt.Errorf("nao implementado no fake")
}

func (m *fakeEscalaRepo) DeleteAtivasPorUsuarioPosto(ctx context.Context, tx pgx.Tx, empresaID, usuarioID, postoID uuid.UUID) error {
	return fmt.Errorf("nao implementado no fake")
}
