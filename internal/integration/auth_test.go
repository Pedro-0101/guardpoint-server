//go:build integration

package integration

import (
	"net/http"
	"testing"

	"github.com/guardpoint/guardpoint-server/internal/model"
)

func TestAuthFluxos(t *testing.T) {
	e := newEnv(t)
	empresa := e.criarEmpresa("Empresa A", "11111111000191")
	e.criarUsuario(empresa.ID, "Admin", "admin@a.com", "senha123", "admin", true)
	e.criarUsuario(empresa.ID, "Inativo", "inativo@a.com", "senha123", "vigia", false)

	t.Run("login sucesso", func(t *testing.T) {
		resp := e.login("admin@a.com", "senha123")
		if resp.AccessToken == "" || resp.RefreshToken == "" {
			t.Error("login nao retornou tokens")
		}
		if resp.User.SenhaHash != "" {
			t.Error("login vazou senha_hash")
		}
	})

	t.Run("senha errada retorna 401", func(t *testing.T) {
		status, _ := e.request(http.MethodPost, "/api/v1/auth/login", "", map[string]string{
			"email": "admin@a.com", "senha": "senha-errada",
		})
		if status != http.StatusUnauthorized {
			t.Errorf("status = %d, esperado 401", status)
		}
	})

	t.Run("usuario inativo retorna 403", func(t *testing.T) {
		status, _ := e.request(http.MethodPost, "/api/v1/auth/login", "", map[string]string{
			"email": "inativo@a.com", "senha": "senha123",
		})
		if status != http.StatusForbidden {
			t.Errorf("status = %d, esperado 403", status)
		}
	})

	t.Run("refresh emite novos tokens", func(t *testing.T) {
		login := e.login("admin@a.com", "senha123")
		var renovado model.LoginResponse
		e.reqJSON(http.MethodPost, "/api/v1/auth/refresh", "", map[string]string{
			"refresh_token": login.RefreshToken,
		}, http.StatusOK, &renovado)
		if renovado.AccessToken == "" {
			t.Error("refresh nao retornou access_token")
		}
	})

	t.Run("register com email duplicado retorna 409", func(t *testing.T) {
		admin := e.login("admin@a.com", "senha123")
		status, _ := e.request(http.MethodPost, "/api/v1/auth/register", admin.AccessToken, map[string]string{
			"nome": "Duplicado", "email": "admin@a.com", "senha": "senha123", "role": "vigia",
		})
		if status != http.StatusConflict {
			t.Errorf("status = %d, esperado 409", status)
		}
	})
}

func TestBiometria(t *testing.T) {
	c := novoCenario(t)
	empresaID := c.empresa.ID.String()

	t.Run("login biometrico com device_secret correto", func(t *testing.T) {
		var resp model.LoginResponse
		c.e.reqJSON(http.MethodPost, "/api/v1/auth/biometric/login", "", map[string]string{
			"empresa_id": empresaID, "device_id": c.deviceID, "device_secret": c.deviceSecret,
		}, http.StatusOK, &resp)
		if resp.User.ID != c.vigia.ID {
			t.Errorf("login biometrico autenticou usuario errado: %s", resp.User.ID)
		}
	})

	t.Run("device_secret errado retorna 401 (B1)", func(t *testing.T) {
		status, _ := c.e.request(http.MethodPost, "/api/v1/auth/biometric/login", "", map[string]string{
			"empresa_id": empresaID, "device_id": c.deviceID, "device_secret": "segredo-forjado",
		})
		if status != http.StatusUnauthorized {
			t.Errorf("status = %d, esperado 401", status)
		}
	})

	t.Run("device_secret ausente falha na validacao", func(t *testing.T) {
		status, _ := c.e.request(http.MethodPost, "/api/v1/auth/biometric/login", "", map[string]string{
			"empresa_id": empresaID, "device_id": c.deviceID,
		})
		if status != http.StatusUnprocessableEntity {
			t.Errorf("status = %d, esperado 422", status)
		}
	})

	t.Run("device desconhecido retorna 401", func(t *testing.T) {
		status, _ := c.e.request(http.MethodPost, "/api/v1/auth/biometric/login", "", map[string]string{
			"empresa_id": empresaID, "device_id": "device-inexistente", "device_secret": "x",
		})
		if status != http.StatusUnauthorized {
			t.Errorf("status = %d, esperado 401", status)
		}
	})

	t.Run("re-registro do mesmo device nao duplica sessao (A5)", func(t *testing.T) {
		novoSecret := c.registrarBiometria(c.deviceID)

		var count int
		if err := c.e.pool.QueryRow(t.Context(),
			`SELECT COUNT(*) FROM sessoes_dispositivo WHERE empresa_id = $1 AND device_id = $2`,
			c.empresa.ID, c.deviceID).Scan(&count); err != nil {
			t.Fatalf("contar sessoes: %v", err)
		}
		if count != 1 {
			t.Errorf("sessoes para o mesmo device = %d, esperado 1", count)
		}

		// o segredo antigo foi rotacionado
		status, _ := c.e.request(http.MethodPost, "/api/v1/auth/biometric/login", "", map[string]string{
			"empresa_id": empresaID, "device_id": c.deviceID, "device_secret": c.deviceSecret,
		})
		if status != http.StatusUnauthorized {
			t.Errorf("segredo antigo ainda funciona apos re-registro (status %d)", status)
		}
		c.deviceSecret = novoSecret
	})
}

func TestRBAC(t *testing.T) {
	c := novoCenario(t)
	supervisor := c.e.criarUsuario(c.empresa.ID, "Supervisor", "sup@a.com", "senha123", "supervisor", true)
	supToken := c.e.login(supervisor.Email, "senha123").AccessToken

	casos := []struct {
		nome   string
		token  string
		method string
		path   string
		want   int
	}{
		{"vigia em /usuarios", c.vigiaToken, http.MethodGet, "/api/v1/usuarios", http.StatusForbidden},
		{"vigia em /config/escalonamento", c.vigiaToken, http.MethodGet, "/api/v1/config/escalonamento", http.StatusForbidden},
		{"vigia em /alertas", c.vigiaToken, http.MethodGet, "/api/v1/alertas", http.StatusForbidden},
		{"supervisor em /usuarios", supToken, http.MethodGet, "/api/v1/usuarios", http.StatusForbidden},
		{"supervisor em /config/escalonamento", supToken, http.MethodGet, "/api/v1/config/escalonamento", http.StatusForbidden},
		{"supervisor em /alertas", supToken, http.MethodGet, "/api/v1/alertas", http.StatusOK},
		{"supervisor em /escalas", supToken, http.MethodGet, "/api/v1/escalas", http.StatusOK},
		{"admin em /usuarios", c.adminToken, http.MethodGet, "/api/v1/usuarios", http.StatusOK},
		{"sem token", "", http.MethodGet, "/api/v1/usuarios", http.StatusUnauthorized},
	}

	for _, tc := range casos {
		t.Run(tc.nome, func(t *testing.T) {
			status, raw := c.e.request(tc.method, tc.path, tc.token, nil)
			if status != tc.want {
				t.Errorf("status = %d, esperado %d (corpo: %s)", status, tc.want, raw)
			}
		})
	}
}

func TestMultiTenancy(t *testing.T) {
	c := novoCenario(t)
	turnoA := c.iniciarTurno()

	// alerta na empresa A
	c.e.reqJSON(http.MethodPost, "/api/v1/turnos/sabotagem", c.vigiaToken, map[string]any{
		"turno_id": turnoA.ID.String(), "device_id": c.deviceID,
		"latitude": postoLat, "longitude": postoLon,
		"motivo": "teste multi-tenant", "timestamp": nowRFC3339(),
	}, http.StatusAccepted, nil)

	empresaB := c.e.criarEmpresa("Empresa B", "22222222000191")
	adminB := c.e.criarUsuario(empresaB.ID, "Admin B", "admin@b.com", "senha123", "admin", true)
	tokenB := c.e.login(adminB.Email, "senha123").AccessToken

	t.Run("turno de A invisivel para B", func(t *testing.T) {
		status, _ := c.e.request(http.MethodGet, "/api/v1/turnos/"+turnoA.ID.String(), tokenB, nil)
		if status != http.StatusNotFound {
			t.Errorf("status = %d, esperado 404", status)
		}
	})

	t.Run("alertas de A invisiveis para B", func(t *testing.T) {
		var resp struct {
			Total int `json:"total"`
		}
		c.e.reqJSON(http.MethodGet, "/api/v1/alertas", tokenB, nil, http.StatusOK, &resp)
		if resp.Total != 0 {
			t.Errorf("empresa B enxerga %d alertas da empresa A", resp.Total)
		}
	})

	t.Run("escalas de A invisiveis para B", func(t *testing.T) {
		var resp struct {
			Total int `json:"total"`
		}
		c.e.reqJSON(http.MethodGet, "/api/v1/escalas", tokenB, nil, http.StatusOK, &resp)
		if resp.Total != 0 {
			t.Errorf("empresa B enxerga %d escalas da empresa A", resp.Total)
		}
	})

	t.Run("turnos ativos de A invisiveis para B", func(t *testing.T) {
		var turnos []model.TurnoDetalhe
		c.e.reqJSON(http.MethodGet, "/api/v1/turnos/ativos", tokenB, nil, http.StatusOK, &turnos)
		if len(turnos) != 0 {
			t.Errorf("empresa B enxerga %d turnos ativos da empresa A", len(turnos))
		}
	})
}
