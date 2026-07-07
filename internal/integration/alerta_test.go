//go:build integration

package integration

import (
	"net/http"
	"testing"
	"time"

	"github.com/guardpoint/guardpoint-server/internal/model"
)

// Destinatarios de escalonamento devem ser usuarios reais da mesma empresa.
func TestConfigEscalonamentoDestinatarios(t *testing.T) {
	c := novoCenario(t)

	supervisor := c.e.criarUsuario(c.empresa.ID, "Supervisor B", "sup.b@a.com", "senha123", "supervisor", true)

	t.Run("cria nivel com destinatarios", func(t *testing.T) {
		var config model.ConfigEscalonamento
		c.e.reqJSON(http.MethodPost, "/api/v1/config/escalonamento", c.adminToken, map[string]any{
			"nivel":          1,
			"atraso_minutos": 10,
			"usuario_ids":    []string{supervisor.ID.String()},
		}, http.StatusCreated, &config)

		if len(config.UsuarioIDs) != 1 || config.UsuarioIDs[0] != supervisor.ID {
			t.Fatalf("usuario_ids = %v, esperado [%s]", config.UsuarioIDs, supervisor.ID)
		}
	})

	t.Run("rejeita usuario de outra empresa", func(t *testing.T) {
		outraEmpresa := c.e.criarEmpresa("Empresa Outra", "22222222000191")
		usuarioOutraEmpresa := c.e.criarUsuario(outraEmpresa.ID, "Estranho", "estranho@b.com", "senha123", "supervisor", true)

		c.e.reqJSON(http.MethodPost, "/api/v1/config/escalonamento", c.adminToken, map[string]any{
			"nivel":          2,
			"atraso_minutos": 20,
			"usuario_ids":    []string{usuarioOutraEmpresa.ID.String()},
		}, http.StatusBadRequest, nil)
	})

	t.Run("rejeita lista vazia de destinatarios", func(t *testing.T) {
		c.e.reqJSON(http.MethodPost, "/api/v1/config/escalonamento", c.adminToken, map[string]any{
			"nivel":          3,
			"atraso_minutos": 30,
			"usuario_ids":    []string{},
		}, http.StatusUnprocessableEntity, nil)
	})

	t.Run("GET retorna destinatarios salvos", func(t *testing.T) {
		var configs []model.ConfigEscalonamento
		c.e.reqJSON(http.MethodGet, "/api/v1/config/escalonamento", c.adminToken, nil, http.StatusOK, &configs)

		if len(configs) != 1 {
			t.Fatalf("configs = %d, esperado 1 (apenas o nivel 1 criado com sucesso)", len(configs))
		}
		if len(configs[0].UsuarioIDs) != 1 || configs[0].UsuarioIDs[0] != supervisor.ID {
			t.Fatalf("usuario_ids = %v, esperado [%s]", configs[0].UsuarioIDs, supervisor.ID)
		}
	})
}

// Configuracao de destinatarios por tipo de alerta de emergencia.
func TestConfigAlertaEmergencia(t *testing.T) {
	c := novoCenario(t)

	t.Run("GET inicial retorna os 3 tipos vazios", func(t *testing.T) {
		var configs []model.ConfigAlertaEmergencia
		c.e.reqJSON(http.MethodGet, "/api/v1/config/alertas-emergencia", c.adminToken, nil, http.StatusOK, &configs)

		if len(configs) != 3 {
			t.Fatalf("configs = %d, esperado 3 (coacao, sabotagem, no_show)", len(configs))
		}
		for _, cfg := range configs {
			if len(cfg.UsuarioIDs) != 0 {
				t.Errorf("tipo %s: usuario_ids = %v, esperado vazio", cfg.Tipo, cfg.UsuarioIDs)
			}
		}
	})

	t.Run("PUT define destinatarios de coacao", func(t *testing.T) {
		gerente := c.e.criarUsuario(c.empresa.ID, "Gerente Emergencia", "gerente.emerg@a.com", "senha123", "admin", true)

		var config model.ConfigAlertaEmergencia
		c.e.reqJSON(http.MethodPut, "/api/v1/config/alertas-emergencia/coacao", c.adminToken, map[string]any{
			"usuario_ids": []string{gerente.ID.String()},
		}, http.StatusOK, &config)

		if len(config.UsuarioIDs) != 1 || config.UsuarioIDs[0] != gerente.ID {
			t.Fatalf("usuario_ids = %v, esperado [%s]", config.UsuarioIDs, gerente.ID)
		}
	})

	t.Run("PUT com tipo invalido retorna 400", func(t *testing.T) {
		outro := c.e.criarUsuario(c.empresa.ID, "Outro", "outro.tipo@a.com", "senha123", "admin", true)
		c.e.reqJSON(http.MethodPut, "/api/v1/config/alertas-emergencia/tipo_invalido", c.adminToken, map[string]any{
			"usuario_ids": []string{outro.ID.String()},
		}, http.StatusBadRequest, nil)
	})
}

// Alerta imediato (coacao) deve enfileirar PendingAlert com os usuario_ids
// configurados especificamente para o tipo "coacao", nao os do escalonamento.
func TestAlertaImediatoUsaDestinatariosPorTipo(t *testing.T) {
	c := novoCenario(t)
	turno := c.iniciarTurno()

	gerente := c.e.criarUsuario(c.empresa.ID, "Gerente Coacao", "gerente.coacao@a.com", "senha123", "admin", true)
	c.e.reqJSON(http.MethodPut, "/api/v1/config/alertas-emergencia/coacao", c.adminToken, map[string]any{
		"usuario_ids": []string{gerente.ID.String()},
	}, http.StatusOK, nil)

	c.e.reqJSON(http.MethodPost, "/api/v1/turnos/checkin", c.vigiaToken,
		c.checkinBody(turno.ID, "coacao", time.Now()), http.StatusOK, nil)

	select {
	case pending := <-c.e.app.AlertaService.AlertChannel():
		if pending.Alerta.Tipo != "coacao" {
			t.Fatalf("tipo do alerta = %s, esperado coacao", pending.Alerta.Tipo)
		}
		if len(pending.UsuarioIDs) != 1 || pending.UsuarioIDs[0] != gerente.ID {
			t.Fatalf("usuario_ids = %v, esperado [%s]", pending.UsuarioIDs, gerente.ID)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("nenhum PendingAlert recebido no canal em 2s")
	}
}

// Quando o vigia finalmente da checkin, os alertas de atraso abertos daquele
// turno devem fechar sozinhos como 'resolvido_checkin'.
func TestAlertaAtrasoResolvidoNoCheckin(t *testing.T) {
	c := novoCenario(t)
	turno := c.iniciarTurno()

	supervisor := c.e.criarUsuario(c.empresa.ID, "Supervisor Resolve", "sup.resolve@a.com", "senha123", "supervisor", true)
	c.e.reqJSON(http.MethodPut, "/api/v1/config/escalonamento", c.adminToken, []map[string]any{
		{"nivel": 1, "atraso_minutos": 5, "usuario_ids": []string{supervisor.ID.String()}},
	}, http.StatusOK, nil)

	// primeiro checkin ha 50 min; intervalo de 30 min => atraso de ~20 min, dispara nivel 1
	c.e.reqJSON(http.MethodPost, "/api/v1/turnos/checkin", c.vigiaToken,
		c.checkinBody(turno.ID, "padrao", time.Now().Add(-50*time.Minute)), http.StatusOK, nil)

	c.e.app.TimeoutChecker.CheckOnce(t.Context())

	if n := c.contarAlertas("atraso_n1"); n != 1 {
		t.Fatalf("alertas atraso_n1 antes do checkin = %d, esperado 1", n)
	}

	var abertos struct {
		Total int `json:"total"`
	}
	c.e.reqJSON(http.MethodGet, "/api/v1/alertas?tipo=atraso_n1&status=aberto", c.adminToken, nil, http.StatusOK, &abertos)
	if abertos.Total != 1 {
		t.Fatalf("alertas atraso_n1 status=aberto antes do checkin = %d, esperado 1", abertos.Total)
	}

	// vigia finalmente da checkin em dia
	c.e.reqJSON(http.MethodPost, "/api/v1/turnos/checkin", c.vigiaToken,
		c.checkinBody(turno.ID, "padrao", time.Now()), http.StatusOK, nil)

	c.e.reqJSON(http.MethodGet, "/api/v1/alertas?tipo=atraso_n1&status=aberto", c.adminToken, nil, http.StatusOK, &abertos)
	if abertos.Total != 0 {
		t.Fatalf("alertas atraso_n1 status=aberto apos checkin = %d, esperado 0", abertos.Total)
	}

	var resolvidos struct {
		Total int `json:"total"`
	}
	c.e.reqJSON(http.MethodGet, "/api/v1/alertas?tipo=atraso_n1&status=resolvido_checkin", c.adminToken, nil, http.StatusOK, &resolvidos)
	if resolvidos.Total != 1 {
		t.Fatalf("alertas atraso_n1 status=resolvido_checkin apos checkin = %d, esperado 1", resolvidos.Total)
	}
}
