package service

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"github.com/guardpoint/guardpoint-server/internal/auth"
	"github.com/guardpoint/guardpoint-server/internal/model"
)

type fakeAuthUserRepo struct {
	findByEmailFn func(ctx context.Context, email string) (*model.User, error)
	findByIDFn    func(ctx context.Context, id uuid.UUID) (*model.User, error)
	createFn      func(ctx context.Context, u *model.User) error
}

func (m *fakeAuthUserRepo) FindByEmail(ctx context.Context, email string) (*model.User, error) {
	if m.findByEmailFn != nil { return m.findByEmailFn(ctx, email) }
	return nil, ErrInvalidCredentials
}
func (m *fakeAuthUserRepo) FindByID(ctx context.Context, id uuid.UUID) (*model.User, error) {
	if m.findByIDFn != nil { return m.findByIDFn(ctx, id) }
	return nil, nil
}
func (m *fakeAuthUserRepo) Create(ctx context.Context, u *model.User) error {
	if m.createFn != nil { return m.createFn(ctx, u) }
	return nil
}

type fakeAuthSessaoRepo struct {
	findByDeviceIDFn func(ctx context.Context, empresaID, deviceID string) (*model.SessaoDispositivo, error)
	createFn         func(ctx context.Context, s *model.SessaoDispositivo) error
	deleteByDeviceIDFn func(ctx context.Context, empresaID, deviceID string) error
}

func (m *fakeAuthSessaoRepo) FindByDeviceID(ctx context.Context, empresaID, deviceID string) (*model.SessaoDispositivo, error) {
	if m.findByDeviceIDFn != nil { return m.findByDeviceIDFn(ctx, empresaID, deviceID) }
	return nil, nil
}
func (m *fakeAuthSessaoRepo) Create(ctx context.Context, s *model.SessaoDispositivo) error {
	if m.createFn != nil { return m.createFn(ctx, s) }
	return nil
}
func (m *fakeAuthSessaoRepo) DeleteByDeviceID(ctx context.Context, empresaID, deviceID string) error {
	if m.deleteByDeviceIDFn != nil { return m.deleteByDeviceIDFn(ctx, empresaID, deviceID) }
	return nil
}

func makeAuthService(userRepo AuthUserRepository) *AuthService {
	jwtSvc := auth.NewJWTService("test-secret-with-sufficient-length-for-hs256")
	return NewAuthService(jwtSvc, userRepo, nil, nil)
}

func TestAuthService_Login_Success(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	empresaID := uuid.New()

	hashedPass, _ := bcrypt.GenerateFromPassword([]byte("senha123"), bcrypt.DefaultCost)

	svc := makeAuthService(&fakeAuthUserRepo{
		findByEmailFn: func(ctx context.Context, email string) (*model.User, error) {
			return &model.User{
				ID: userID, EmpresaID: empresaID, Nome: "Teste", Email: email,
				SenhaHash: string(hashedPass), Role: "vigia", Ativo: true,
			}, nil
		},
	})

	resp, err := svc.Login(ctx, model.LoginRequest{Email: "teste@x.com", Password: "senha123"})
	if err != nil {
		t.Fatalf("Login() erro: %v", err)
	}
	if resp.AccessToken == "" {
		t.Error("AccessToken vazio")
	}
	if resp.RefreshToken == "" {
		t.Error("RefreshToken vazio")
	}
	if resp.User.Nome != "Teste" {
		t.Errorf("Nome = %q", resp.User.Nome)
	}
}

func TestAuthService_Login_InactiveUser(t *testing.T) {
	ctx := context.Background()

	hashedPass, _ := bcrypt.GenerateFromPassword([]byte("senha123"), bcrypt.DefaultCost)

	svc := makeAuthService(&fakeAuthUserRepo{
		findByEmailFn: func(ctx context.Context, email string) (*model.User, error) {
			return &model.User{SenhaHash: string(hashedPass), Ativo: false}, nil
		},
	})

	_, err := svc.Login(ctx, model.LoginRequest{Email: "inativo@x.com", Password: "senha123"})
	if err == nil {
		t.Fatal("Login() deveria falhar para usuario inativo")
	}
}

func TestAuthService_Login_InvalidCredentials(t *testing.T) {
	ctx := context.Background()

	svc := makeAuthService(&fakeAuthUserRepo{
		findByEmailFn: func(ctx context.Context, email string) (*model.User, error) {
			return nil, ErrInvalidCredentials
		},
	})

	_, err := svc.Login(ctx, model.LoginRequest{Email: "nobody@x.com", Password: "senha"})
	if err == nil {
		t.Fatal("Login() deveria falhar para credenciais invalidas")
	}
}

func TestAuthService_Login_WrongPassword(t *testing.T) {
	ctx := context.Background()

	hashedPass, _ := bcrypt.GenerateFromPassword([]byte("senha123"), bcrypt.DefaultCost)

	svc := makeAuthService(&fakeAuthUserRepo{
		findByEmailFn: func(ctx context.Context, email string) (*model.User, error) {
			return &model.User{SenhaHash: string(hashedPass), Ativo: true}, nil
		},
	})

	_, err := svc.Login(ctx, model.LoginRequest{Email: "teste@x.com", Password: "senhaErrada"})
	if err == nil {
		t.Fatal("Login() deveria falhar para senha errada")
	}
}

func TestAuthService_Register_Success(t *testing.T) {
	ctx := context.Background()
	empresaID := uuid.New()

	createCalled := false
	svc := makeAuthService(&fakeAuthUserRepo{
		findByEmailFn: func(ctx context.Context, email string) (*model.User, error) {
			return nil, nil
		},
		createFn: func(ctx context.Context, u *model.User) error {
			createCalled = true
			u.ID = uuid.New()
			return nil
		},
	})

	user, err := svc.Register(ctx, empresaID.String(), model.RegisterRequest{
		Nome: "Novo", Email: "novo@x.com", Password: "123456", Role: "vigia",
	})
	if err != nil {
		t.Fatalf("Register() erro: %v", err)
	}
	if !createCalled {
		t.Error("Create nao foi chamado")
	}
	if user.Nome != "Novo" {
		t.Errorf("Nome = %q", user.Nome)
	}
	if user.SenhaHash == "" {
		t.Error("SenhaHash vazio")
	}
	if user.SenhaHash == "123456" {
		t.Error("SenhaHash nao pode ser texto puro")
	}
}

func TestAuthService_Register_DuplicateEmail(t *testing.T) {
	ctx := context.Background()
	empresaID := uuid.New()

	svc := makeAuthService(&fakeAuthUserRepo{
		findByEmailFn: func(ctx context.Context, email string) (*model.User, error) {
			return &model.User{ID: uuid.New()}, nil
		},
	})

	_, err := svc.Register(ctx, empresaID.String(), model.RegisterRequest{
		Nome: "Novo", Email: "existe@x.com", Password: "123456", Role: "vigia",
	})
	if err == nil {
		t.Fatal("Register() deveria falhar para email duplicado")
	}
}

func TestAuthService_Refresh_Success(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	empresaID := uuid.New()

	jwtSvc := auth.NewJWTService("test-secret-with-sufficient-length-for-hs256")
	refreshToken, err := jwtSvc.GenerateRefreshToken(userID)
	if err != nil {
		t.Fatalf("gerar refresh token: %v", err)
	}

	svc := NewAuthService(jwtSvc,
		&fakeAuthUserRepo{
			findByIDFn: func(ctx context.Context, id uuid.UUID) (*model.User, error) {
				return &model.User{ID: userID, EmpresaID: empresaID, Nome: "Teste", Ativo: true}, nil
			},
		},
		nil, nil,
	)

	resp, err := svc.Refresh(ctx, model.RefreshRequest{RefreshToken: refreshToken})
	if err != nil {
		t.Fatalf("Refresh() erro: %v", err)
	}
	if resp.AccessToken == "" {
		t.Error("AccessToken vazio")
	}
}
