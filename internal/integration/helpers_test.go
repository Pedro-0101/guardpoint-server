//go:build integration

// Testes de integracao (D2/D3 do plano de pendencias). Sobem o roteador real
// via internal/app contra um Postgres efemero com as migrations aplicadas.
// Rode com: make test-integration (ou go test -tags integration -p 1 ./...).
package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"

	"github.com/guardpoint/guardpoint-server/internal/app"
	"github.com/guardpoint/guardpoint-server/internal/config"
	"github.com/guardpoint/guardpoint-server/internal/model"
	"github.com/guardpoint/guardpoint-server/internal/testutil"
)

const testJWTSecret = "integration-test-secret-0123456789abcdef"

type env struct {
	t    *testing.T
	pool *pgxpool.Pool
	app  *app.App
	srv  *httptest.Server
}

func newEnv(t *testing.T) *env {
	t.Helper()
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))

	pool := testutil.SetupTestDB(t)
	cfg := &config.Config{
		Env:        "test",
		JWTSecret:  testJWTSecret,
		CORSOrigin: "*",
	}
	a := app.New(cfg, pool)
	srv := httptest.NewServer(a.Router)
	t.Cleanup(srv.Close)

	return &env{t: t, pool: pool, app: a, srv: srv}
}

func (e *env) criarEmpresa(nome, cnpj string) *model.Empresa {
	e.t.Helper()
	emp := &model.Empresa{Nome: nome, CNPJ: cnpj}
	if err := e.app.EmpresaRepo.Create(context.Background(), emp); err != nil {
		e.t.Fatalf("criar empresa: %v", err)
	}
	return emp
}

func (e *env) criarUsuario(empresaID uuid.UUID, nome, email, senha, role string, ativo bool) *model.User {
	e.t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte(senha), bcrypt.MinCost)
	if err != nil {
		e.t.Fatalf("hash senha: %v", err)
	}
	u := &model.User{EmpresaID: empresaID, Nome: nome, Email: email, SenhaHash: string(hash), Role: role}
	if err := e.app.UserRepo.Create(context.Background(), u); err != nil {
		e.t.Fatalf("criar usuario %s: %v", email, err)
	}
	if !ativo {
		if _, err := e.pool.Exec(context.Background(), `UPDATE usuarios SET ativo = false WHERE id = $1`, u.ID); err != nil {
			e.t.Fatalf("inativar usuario: %v", err)
		}
	}
	return u
}

// request envia JSON e retorna o status e o corpo bruto.
func (e *env) request(method, path, token string, body any) (int, []byte) {
	e.t.Helper()

	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			e.t.Fatalf("marshal body: %v", err)
		}
		reader = bytes.NewReader(data)
	}

	req, err := http.NewRequest(method, e.srv.URL+path, reader)
	if err != nil {
		e.t.Fatalf("criar request: %v", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := e.srv.Client().Do(req)
	if err != nil {
		e.t.Fatalf("%s %s: %v", method, path, err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		e.t.Fatalf("ler resposta: %v", err)
	}
	return resp.StatusCode, raw
}

// reqJSON envia JSON, exige o status esperado e decodifica a resposta em out.
func (e *env) reqJSON(method, path, token string, body any, wantStatus int, out any) {
	e.t.Helper()

	status, raw := e.request(method, path, token, body)
	if status != wantStatus {
		e.t.Fatalf("%s %s: status = %d, esperado %d\ncorpo: %s", method, path, status, wantStatus, raw)
	}
	if out != nil {
		if err := json.Unmarshal(raw, out); err != nil {
			e.t.Fatalf("decodificar resposta de %s %s: %v\ncorpo: %s", method, path, err, raw)
		}
	}
}

func (e *env) login(email, senha string) model.LoginResponse {
	e.t.Helper()
	var resp model.LoginResponse
	e.reqJSON(http.MethodPost, "/api/auth/login", "", map[string]string{"email": email, "senha": senha}, http.StatusOK, &resp)
	return resp
}

// cenario padrao: empresa com admin e vigia logados, posto, device biometrico
// registrado e escala ativa cobrindo o horario atual.
type cenario struct {
	e            *env
	empresa      *model.Empresa
	admin        *model.User
	vigia        *model.User
	adminToken   string
	vigiaToken   string
	posto        model.Posto
	deviceID     string
	deviceSecret string
}

const (
	postoLat = -23.5505
	postoLon = -46.6333
)

func novoCenario(t *testing.T) *cenario {
	t.Helper()
	e := newEnv(t)

	empresa := e.criarEmpresa("Empresa A", "11111111000191")
	admin := e.criarUsuario(empresa.ID, "Admin A", "admin@a.com", "senha123", "admin", true)
	vigia := e.criarUsuario(empresa.ID, "Vigia A", "vigia@a.com", "senha123", "vigia", true)

	c := &cenario{
		e:          e,
		empresa:    empresa,
		admin:      admin,
		vigia:      vigia,
		adminToken: e.login(admin.Email, "senha123").AccessToken,
		vigiaToken: e.login(vigia.Email, "senha123").AccessToken,
		deviceID:   "device-vigia-a-01",
	}

	e.reqJSON(http.MethodPost, "/api/postos", c.adminToken, map[string]any{
		"nome": "Posto Central", "latitude": postoLat, "longitude": postoLon, "raio_m": 100,
	}, http.StatusCreated, &c.posto)

	c.deviceSecret = c.registrarBiometria(c.deviceID)
	c.criarEscala(c.vigia.ID, c.posto.ID, time.Now(), 60)

	return c
}

func (c *cenario) registrarBiometria(deviceID string) string {
	c.e.t.Helper()
	var resp model.BiometricRegisterResponse
	c.e.reqJSON(http.MethodPost, "/api/auth/biometric/register", c.vigiaToken,
		map[string]string{"device_id": deviceID}, http.StatusCreated, &resp)
	if resp.DeviceSecret == "" {
		c.e.t.Fatal("registro biometrico nao retornou device_secret")
	}
	return resp.DeviceSecret
}

// criarEscala cria uma escala diaria com inicio na hora de `inicio` e fim 8h depois.
func (c *cenario) criarEscala(usuarioID, postoID uuid.UUID, inicio time.Time, toleranciaMin int) model.Escala {
	c.e.t.Helper()
	var esc model.Escala
	c.e.reqJSON(http.MethodPost, "/api/escalas", c.adminToken, map[string]any{
		"usuario_id":     usuarioID.String(),
		"posto_id":       postoID.String(),
		"data_inicio":    inicio.AddDate(0, 0, -1).Format("2006-01-02"),
		"data_fim":       inicio.AddDate(0, 0, 1).Format("2006-01-02"),
		"hora_inicio":    inicio.Format("15:04"),
		"hora_fim":       inicio.Add(8 * time.Hour).Format("15:04"),
		"dias_semana":    []int{0, 1, 2, 3, 4, 5, 6},
		"tolerancia_min": toleranciaMin,
	}, http.StatusCreated, &esc)
	return esc
}

func (c *cenario) iniciarTurno() model.Turno {
	c.e.t.Helper()
	var turno model.Turno
	c.e.reqJSON(http.MethodPost, "/api/turnos/iniciar", c.vigiaToken, map[string]any{
		"posto_id": c.posto.ID.String(), "device_id": c.deviceID, "intervalo_min": 30,
	}, http.StatusCreated, &turno)
	return turno
}

func (c *cenario) checkinBody(turnoID uuid.UUID, tipo string, ts time.Time) map[string]any {
	return map[string]any{
		"turno_id":   turnoID.String(),
		"device_id":  c.deviceID,
		"latitude":   postoLat,
		"longitude":  postoLon,
		"tipo_senha": tipo,
		"timestamp":  ts.UTC().Format(time.RFC3339),
	}
}

func (c *cenario) getTurno(turnoID uuid.UUID) model.TurnoDetalhe {
	c.e.t.Helper()
	var det model.TurnoDetalhe
	c.e.reqJSON(http.MethodGet, "/api/turnos/"+turnoID.String(), c.adminToken, nil, http.StatusOK, &det)
	return det
}

func (c *cenario) contarAlertas(tipo string) int {
	c.e.t.Helper()
	var resp struct {
		Total int `json:"total"`
	}
	c.e.reqJSON(http.MethodGet, "/api/alertas?tipo="+tipo, c.adminToken, nil, http.StatusOK, &resp)
	return resp.Total
}
