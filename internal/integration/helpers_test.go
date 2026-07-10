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
		Env:         "test",
		JWTSecret:   testJWTSecret,
		CORSOrigins: []string{"*"},
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

// buscarNivelEmergenciaPadrao retorna o ID do nivel de escalonamento padrao de
// emergencia (sistema=true, nivel=1) da empresa, criado automaticamente pelas
// migrations.
func (e *env) buscarNivelEmergenciaPadrao(empresaID uuid.UUID) uuid.UUID {
	e.t.Helper()
	var id uuid.UUID
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err := e.pool.QueryRow(ctx,
		`SELECT id FROM config_escalonamento WHERE empresa_id = $1 AND sistema = true`,
		empresaID,
	).Scan(&id)
	if err != nil {
		e.t.Fatalf("buscar nivel de emergencia padrao: %v", err)
	}
	return id
}

func (e *env) criarUsuario(empresaID uuid.UUID, nome, email, senha, role string, ativo bool) *model.User {
	e.t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte(senha), bcrypt.MinCost)
	if err != nil {
		e.t.Fatalf("hash senha: %v", err)
	}
	u := &model.User{EmpresaID: empresaID, Nome: nome, SenhaHash: string(hash), Role: role}
	if email != "" {
		u.Email = &email
	}
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
	e.reqJSON(http.MethodPost, "/api/v1/auth/login", "", map[string]string{"email": email, "senha": senha}, http.StatusOK, &resp)
	return resp
}

func (e *env) loginVigiaPorNome(codigoEmpresa, nome, senha string) model.LoginResponse {
	e.t.Helper()
	var resp model.LoginResponse
	e.reqJSON(http.MethodPost, "/api/v1/auth/login", "", map[string]string{"codigo_empresa": codigoEmpresa, "nome": nome, "senha": senha}, http.StatusOK, &resp)
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

// Codigos de PIN fixos usados em toda a suite de integracao. novoCenario ja
// cadastra esses dois PINs obrigatorios (ok/emergencia) para c.vigia antes de
// qualquer chamada a iniciarTurno/checkinBody.
const (
	SenhaOK         = "1234"
	SenhaEmergencia = "9999"
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
		adminToken: e.login(admin.EmailOrEmpty(), "senha123").AccessToken,
		vigiaToken: e.login(vigia.EmailOrEmpty(), "senha123").AccessToken,
		deviceID:   "device-vigia-a-01",
	}

	emergenciaNivelID := e.buscarNivelEmergenciaPadrao(empresa.ID)

	// PINs obrigatorios do vigia de teste, cadastrados antes de qualquer
	// iniciarTurno/checkinBody usar seus codigos.
	c.criarSenhaVigia(c.vigia.ID, "ok", SenhaOK, nil)
	c.criarSenhaVigia(c.vigia.ID, "emergencia", SenhaEmergencia, &emergenciaNivelID)

	e.reqJSON(http.MethodPost, "/api/v1/postos", c.adminToken, map[string]any{
		"nome": "Posto Central", "latitude": postoLat, "longitude": postoLon, "raio_m": 100,
	}, http.StatusCreated, &c.posto)

	c.deviceSecret = c.registrarBiometria(c.deviceID)
	c.criarEscala(c.vigia.ID, c.posto.ID, time.Now(), 60)

	return c
}

// criarSenhaVigia cadastra um PIN para o vigia via POST /usuarios/{id}/senhas
// (rota admin-only). nivelID e obrigatorio para "emergencia" e "customizada", nil para "ok".
func (c *cenario) criarSenhaVigia(usuarioID uuid.UUID, tipo, codigo string, nivelID *uuid.UUID) model.SenhaVigia {
	c.e.t.Helper()
	body := map[string]any{
		"tipo":   tipo,
		"codigo": codigo,
	}
	if nivelID != nil {
		body["nivel_escalonamento_id"] = nivelID.String()
	}
	var senha model.SenhaVigia
	c.e.reqJSON(http.MethodPost, "/api/v1/usuarios/"+usuarioID.String()+"/senhas", c.adminToken, body, http.StatusCreated, &senha)
	return senha
}

func (c *cenario) registrarBiometria(deviceID string) string {
	c.e.t.Helper()
	var resp model.BiometricRegisterResponse
	c.e.reqJSON(http.MethodPost, "/api/v1/auth/biometric/register", c.vigiaToken,
		map[string]string{"device_id": deviceID}, http.StatusCreated, &resp)
	if resp.DeviceSecret == "" {
		c.e.t.Fatal("registro biometrico nao retornou device_secret")
	}
	return resp.DeviceSecret
}

// criarEscala cria uma escala semanal com inicio no diaDaSemana e na hora de `inicio` e fim 8h depois.
func (c *cenario) criarEscala(usuarioID, postoID uuid.UUID, inicio time.Time, toleranciaMin int) model.Escala {
	c.e.t.Helper()
	diaSemana := int16(inicio.Weekday())
	var esc model.Escala
	c.e.reqJSON(http.MethodPost, "/api/v1/escalas", c.adminToken, map[string]any{
		"usuario_id":        usuarioID.String(),
		"posto_id":          postoID.String(),
		"dia_semana_inicio": diaSemana,
		"hora_inicio":       inicio.Format("15:04"),
		"dia_semana_fim":    diaSemana,
		"hora_fim":          inicio.Add(8 * time.Hour).Format("15:04"),
		"tolerancia_min":    toleranciaMin,
	}, http.StatusCreated, &esc)
	return esc
}

func (c *cenario) iniciarTurno() model.Turno {
	c.e.t.Helper()
	var turno model.Turno
	c.e.reqJSON(http.MethodPost, "/api/v1/turnos/iniciar", c.vigiaToken, map[string]any{
		"posto_id": c.posto.ID.String(), "device_id": c.deviceID, "intervalo_min": 30,
		"latitude": postoLat, "longitude": postoLon, "senha": SenhaOK,
	}, http.StatusCreated, &turno)
	return turno
}

// checkinBody monta o corpo de um checkin/iniciar/finalizar de turno. senha e
// o codigo do PIN cadastrado (ex.: SenhaOK, SenhaEmergencia, ou o codigo de
// uma senha customizada) -- nao mais um enum fixo de "tipo_senha".
func (c *cenario) checkinBody(turnoID uuid.UUID, senha string, ts time.Time) map[string]any {
	return map[string]any{
		"turno_id":  turnoID.String(),
		"device_id": c.deviceID,
		"latitude":  postoLat,
		"longitude": postoLon,
		"senha":     senha,
		"timestamp": ts.UTC().Format(time.RFC3339),
	}
}

// backdatarCheckinInicio move o timestamp_criacao do checkin de evento
// "inicio" (gravado automaticamente por iniciarTurno) para o passado, simulando
// um turno que comecou ha `atras` sem nenhuma outra atividade desde entao. Usado
// pelos testes de atraso/timeout, que dependiam de conseguir "fabricar" o
// ultimo check-in via checkinBody com timestamp passado -- isso deixou de
// funcionar porque iniciarTurno agora sempre grava um checkin de inicio com
// timestamp real (mais recente que qualquer timestamp passado enviado depois),
// entao getUltimoCheckin (ORDER BY timestamp_criacao DESC) passou a enxergar o
// checkin de inicio como o mais recente.
func (c *cenario) backdatarCheckinInicio(turnoID uuid.UUID, atras time.Duration) {
	c.e.t.Helper()
	if _, err := c.e.pool.Exec(context.Background(),
		`UPDATE checkins SET timestamp_criacao = $1 WHERE turno_id = $2 AND evento = 'inicio'`,
		time.Now().Add(-atras), turnoID); err != nil {
		c.e.t.Fatalf("backdatar checkin de inicio: %v", err)
	}
}

func (c *cenario) getTurno(turnoID uuid.UUID) model.TurnoDetalhe {
	c.e.t.Helper()
	var det model.TurnoDetalhe
	c.e.reqJSON(http.MethodGet, "/api/v1/turnos/"+turnoID.String(), c.adminToken, nil, http.StatusOK, &det)
	return det
}

// criarSubstituicao cria uma substituicao via API e retorna o struct completo.
func (c *cenario) criarSubstituicao(usuarioID, postoID uuid.UUID, dataInicio, dataFim string, horaInicio, horaFim string) *model.Substituicao {
	c.e.t.Helper()
	var sub model.Substituicao
	c.e.reqJSON(http.MethodPost, "/api/v1/substituicoes", c.adminToken, map[string]any{
		"usuario_id":  usuarioID.String(),
		"posto_id":    postoID.String(),
		"data_inicio": dataInicio,
		"data_fim":    dataFim,
		"hora_inicio": horaInicio,
		"hora_fim":    horaFim,
	}, http.StatusCreated, &sub)
	return &sub
}

func (c *cenario) contarAlertas(tipo string) int {
	c.e.t.Helper()
	var resp struct {
		Total int `json:"total"`
	}
	c.e.reqJSON(http.MethodGet, "/api/v1/alertas?tipo="+tipo, c.adminToken, nil, http.StatusOK, &resp)
	return resp.Total
}

// criarNivel cria um nivel de escalonamento via API e retorna seu ID.
func (c *cenario) criarNivel(nivel int, atrasoMin int) uuid.UUID {
	c.e.t.Helper()
	var resp map[string]any
	c.e.reqJSON(http.MethodPost, "/api/v1/config/escalonamento", c.adminToken, map[string]any{
		"nivel": nivel, "atraso_minutos": atrasoMin, "usuario_ids": []string{c.admin.ID.String()},
	}, http.StatusCreated, &resp)
	return uuid.MustParse(resp["id"].(string))
}
