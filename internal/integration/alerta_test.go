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

// Alerta de senha de emergencia (PIN "emergencia") deve enfileirar PendingAlert
// com os usuario_ids do nivel de escalonamento padrao do sistema (sistema=true)
// ao qual a senha de emergencia esta vinculada. Diferente da senha customizada,
// a senha de emergencia sempre aponta para o escalonamento padrao criado pelo
// ProvisionarPadrao.
func TestAlertaSenhaEmergenciaUsaNivelPadraoDoSistema(t *testing.T) {
	c := novoCenario(t)
	turno := c.iniciarTurno()

	supervisor := c.e.criarUsuario(c.empresa.ID, "Supervisor N1", "sup.n1.emerg@a.com", "senha123", "supervisor", true)
	c.e.criarUsuario(c.empresa.ID, "Diretor N2", "diretor.n2.emerg@a.com", "senha123", "admin", true)
	c.e.reqJSON(http.MethodPut, "/api/v1/config/escalonamento", c.adminToken, []map[string]any{
		{"nivel": 1, "atraso_minutos": 5, "usuario_ids": []string{supervisor.ID.String()}},
	}, http.StatusOK, nil)

	c.e.reqJSON(http.MethodPost, "/api/v1/turnos/checkin", c.vigiaToken,
		c.checkinBody(turno.ID, SenhaEmergencia, time.Now()), http.StatusOK, nil)

	select {
	case pending := <-c.e.app.AlertaService.AlertChannel():
		if pending.Alerta.Tipo != "senha_emergencia" {
			t.Fatalf("tipo do alerta = %s, esperado senha_emergencia", pending.Alerta.Tipo)
		}
		if len(pending.UsuarioIDs) != 1 || pending.UsuarioIDs[0] != c.admin.ID {
			t.Fatalf("usuario_ids = %v, esperado [%s] (nivel padrao do sistema)", pending.UsuarioIDs, c.admin.ID)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("nenhum PendingAlert recebido no canal em 2s")
	}
}

// PIN customizado vinculado a um nivel de escalonamento ESPECIFICO deve
// notificar apenas os destinatarios daquele nivel, mesmo quando um nivel
// diferente (maior) existe e seria escolhido pela resolucao dinamica default.
func TestAlertaSenhaCustomizadaUsaNivelEspecificoNaoOMaximo(t *testing.T) {
	c := novoCenario(t)
	turno := c.iniciarTurno()

	nivel1User := c.e.criarUsuario(c.empresa.ID, "Nivel1 User", "nivel1.custom@a.com", "senha123", "supervisor", true)
	nivelMaxUser := c.e.criarUsuario(c.empresa.ID, "NivelMax User", "nivelmax.custom@a.com", "senha123", "admin", true)

	var nivel1, nivelMax model.ConfigEscalonamento
	c.e.reqJSON(http.MethodPost, "/api/v1/config/escalonamento", c.adminToken, map[string]any{
		"nivel": 1, "atraso_minutos": 5, "usuario_ids": []string{nivel1User.ID.String()},
	}, http.StatusCreated, &nivel1)
	c.e.reqJSON(http.MethodPost, "/api/v1/config/escalonamento", c.adminToken, map[string]any{
		"nivel": 2, "atraso_minutos": 15, "usuario_ids": []string{nivelMaxUser.ID.String()},
	}, http.StatusCreated, &nivelMax)

	senhaCustom := c.criarSenhaVigia(c.vigia.ID, "customizada", "5555", &nivel1.ID)

	c.e.reqJSON(http.MethodPost, "/api/v1/turnos/checkin", c.vigiaToken,
		c.checkinBody(turno.ID, senhaCustom.Codigo, time.Now()), http.StatusOK, nil)

	select {
	case pending := <-c.e.app.AlertaService.AlertChannel():
		if pending.Alerta.Tipo != "senha_customizada" {
			t.Fatalf("tipo do alerta = %s, esperado senha_customizada", pending.Alerta.Tipo)
		}
		if len(pending.UsuarioIDs) != 1 || pending.UsuarioIDs[0] != nivel1User.ID {
			t.Fatalf("usuario_ids = %v, esperado [%s] (nivel especifico do PIN, nao o maximo)", pending.UsuarioIDs, nivel1User.ID)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("nenhum PendingAlert recebido no canal em 2s")
	}

	det := c.getTurno(turno.ID)
	if det.Status != "critico" {
		t.Errorf("status do turno apos senha customizada = %q, esperado critico", det.Status)
	}
}

// Vigia sem nenhum PIN cadastrado: iniciar/checkin/finalizar devem prosseguir
// normalmente (o vigia continua trabalhando mesmo sem PINs configurados pelo
// admin), sem gerar nenhum alerta e sem marcar o turno como critico.
func TestTurnoSemPinsConfigurados(t *testing.T) {
	c := novoCenario(t)

	// vigem novo, sem nenhuma senha cadastrada (novoCenario so cadastra ok/emergencia
	// para c.vigia, nao para usuarios criados a parte)
	vigiaSemPin := c.e.criarUsuario(c.empresa.ID, "Vigia Sem Pin", "sempin@a.com", "senha123", "vigia", true)
	tokenSemPin := c.e.login(vigiaSemPin.Email, "senha123").AccessToken
	deviceSemPin := "device-sem-pin-01"

	var reg model.BiometricRegisterResponse
	c.e.reqJSON(http.MethodPost, "/api/v1/auth/biometric/register", tokenSemPin,
		map[string]string{"device_id": deviceSemPin}, http.StatusCreated, &reg)

	c.criarEscala(vigiaSemPin.ID, c.posto.ID, time.Now(), 60)

	var turno model.Turno
	c.e.reqJSON(http.MethodPost, "/api/v1/turnos/iniciar", tokenSemPin, map[string]any{
		"posto_id": c.posto.ID.String(), "device_id": deviceSemPin, "intervalo_min": 30,
		"latitude": postoLat, "longitude": postoLon, "senha": "0000",
	}, http.StatusCreated, &turno)
	if turno.Status != "em_andamento" {
		t.Fatalf("status apos iniciar sem pin = %q, esperado em_andamento", turno.Status)
	}

	var checkinResp model.CheckinResponse
	c.e.reqJSON(http.MethodPost, "/api/v1/turnos/checkin", tokenSemPin, map[string]any{
		"turno_id": turno.ID.String(), "device_id": deviceSemPin,
		"latitude": postoLat, "longitude": postoLon, "senha": "0000",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}, http.StatusOK, &checkinResp)
	if checkinResp.Status != "em_andamento" {
		t.Errorf("status apos checkin sem pin = %q, esperado em_andamento", checkinResp.Status)
	}

	var fin model.Turno
	c.e.reqJSON(http.MethodPost, "/api/v1/turnos/finalizar", tokenSemPin, map[string]any{
		"turno_id": turno.ID.String(), "device_id": deviceSemPin,
		"latitude": postoLat, "longitude": postoLon, "senha": "0000",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}, http.StatusOK, &fin)
	if fin.Status != "finalizado" {
		t.Errorf("status apos finalizar sem pin = %q, esperado finalizado", fin.Status)
	}

	select {
	case pending := <-c.e.app.AlertaService.AlertChannel():
		t.Fatalf("alerta inesperado gerado para vigia sem PINs: %+v", pending.Alerta)
	case <-time.After(300 * time.Millisecond):
		// esperado: nenhum alerta em fila
	}
}

// Nao e possivel deletar um nivel de escalonamento referenciado pelo
// nivel_escalonamento_id de um PIN customizado (FK protege a integridade da
// resolucao de destinatarios em runtime).
func TestDeleteEscalonamentoEmUsoPorSenhaCustomizadaRetorna409(t *testing.T) {
	c := novoCenario(t)

	var nivel model.ConfigEscalonamento
	c.e.reqJSON(http.MethodPost, "/api/v1/config/escalonamento", c.adminToken, map[string]any{
		"nivel": 1, "atraso_minutos": 5, "usuario_ids": []string{c.admin.ID.String()},
	}, http.StatusCreated, &nivel)

	c.criarSenhaVigia(c.vigia.ID, "customizada", "7777", &nivel.ID)

	status, _ := c.e.request(http.MethodDelete, "/api/v1/config/escalonamento/"+nivel.ID.String(), c.adminToken, nil)
	if status != http.StatusConflict {
		t.Fatalf("status = %d, esperado 409 (nivel em uso por senha customizada)", status)
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

	// turno comecou ha 50 min sem nenhuma atividade desde entao; intervalo de 30
	// min => atraso de ~20 min, dispara nivel 1
	c.backdatarCheckinInicio(turno.ID, 50*time.Minute)

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
		c.checkinBody(turno.ID, SenhaOK, time.Now()), http.StatusOK, nil)

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
