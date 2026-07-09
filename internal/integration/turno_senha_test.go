//go:build integration

package integration

import (
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
)

// Iniciar turno com PIN de emergencia: o turno deve comecar com status
// "critico" (antes mesmo do primeiro check-in) e um alerta de
// senha_emergencia deve ser gerado.
func TestIniciarTurnoComSenhaEmergencia(t *testing.T) {
	c := novoCenario(t)

	supervisor := c.e.criarUsuario(c.empresa.ID, "Supervisor Inicio Emerg", "sup.inicio.emerg@a.com", "senha123", "supervisor", true)
	c.e.reqJSON(http.MethodPut, "/api/v1/config/escalonamento", c.adminToken, []map[string]any{
		{"nivel": 1, "atraso_minutos": 5, "usuario_ids": []string{supervisor.ID.String()}},
	}, http.StatusOK, nil)

	var turno map[string]any
	c.e.reqJSON(http.MethodPost, "/api/v1/turnos/iniciar", c.vigiaToken, map[string]any{
		"posto_id": c.posto.ID.String(), "device_id": c.deviceID, "intervalo_min": 30,
		"latitude": postoLat, "longitude": postoLon, "senha": SenhaEmergencia,
	}, http.StatusCreated, &turno)

	// O turno SEMPRE comeca (acao nunca falha por causa do PIN), mas o status
	// deve vir como "critico" em memoria na resposta.
	if turno["status"] != "critico" {
		t.Errorf("status do turno = %q, esperado critico", turno["status"])
	}

	// Confirma que o alerta foi gerado
	select {
	case pending := <-c.e.app.AlertaService.AlertChannel():
		if pending.Alerta.Tipo != "senha_emergencia" {
			t.Fatalf("tipo do alerta = %s, esperado senha_emergencia", pending.Alerta.Tipo)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("nenhum PendingAlert recebido no canal em 2s")
	}
}

// Finalizar turno com PIN de emergencia: o turno deve terminar como
// "finalizado" (nunca "critico"), mas o alerta de senha_emergencia deve ser
// disparado em paralelo com os destinatarios corretos.
func TestFinalizarTurnoComSenhaEmergencia(t *testing.T) {
	c := novoCenario(t)
	turno := c.iniciarTurno()

	supervisor := c.e.criarUsuario(c.empresa.ID, "Supervisor Fin Emerg", "sup.fin.emerg@a.com", "senha123", "supervisor", true)
	c.e.reqJSON(http.MethodPut, "/api/v1/config/escalonamento", c.adminToken, []map[string]any{
		{"nivel": 1, "atraso_minutos": 5, "usuario_ids": []string{supervisor.ID.String()}},
	}, http.StatusOK, nil)

	var fin map[string]any
	c.e.reqJSON(http.MethodPost, "/api/v1/turnos/finalizar", c.vigiaToken, map[string]any{
		"turno_id": turno.ID.String(), "device_id": c.deviceID,
		"latitude": postoLat, "longitude": postoLon, "senha": SenhaEmergencia,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}, http.StatusOK, &fin)

	// O turno sempre termina como "finalizado", mesmo com PIN de emergencia
	if fin["status"] != "finalizado" {
		t.Errorf("status do turno = %q, esperado finalizado", fin["status"])
	}

	// Mas o alerta de emergencia foi disparado
	select {
	case pending := <-c.e.app.AlertaService.AlertChannel():
		if pending.Alerta.Tipo != "senha_emergencia" {
			t.Fatalf("tipo do alerta = %s, esperado senha_emergencia", pending.Alerta.Tipo)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("nenhum PendingAlert recebido no canal em 2s")
	}
}

// PIN customizado com nivel_escalonamento_id especifico deve notificar
// exatamente os destinatarios daquele nivel, nao outro nivel qualquer.
func TestAlertaSenhaCustomizadaNotificaNivelCorreto(t *testing.T) {
	c := novoCenario(t)
	turno := c.iniciarTurno()

	nivel1User := c.e.criarUsuario(c.empresa.ID, "N1 Especifico", "n1.espec@a.com", "senha123", "supervisor", true)
	c.e.criarUsuario(c.empresa.ID, "N2 Especifico", "n2.espec@a.com", "senha123", "admin", true)

	// Cria nivel 2 com o admin como destinatario
	nivelID2 := c.criarNivel(2, 10)

	// Atualiza nivel 1 com destinatario nivel1User
	c.e.reqJSON(http.MethodPut, "/api/v1/config/escalonamento", c.adminToken, []map[string]any{
		{"nivel": 1, "atraso_minutos": 5, "usuario_ids": []string{nivel1User.ID.String()}},
	}, http.StatusOK, nil)

	// PIN customizada vinculada ao nivel 2
	senhaCustom := c.criarSenhaVigia(c.vigia.ID, "customizada", "9991", toUUIDPtr(nivelID2.String()))

	c.e.reqJSON(http.MethodPost, "/api/v1/turnos/checkin", c.vigiaToken,
		c.checkinBody(turno.ID, senhaCustom.Codigo, time.Now()), http.StatusOK, nil)

	select {
	case pending := <-c.e.app.AlertaService.AlertChannel():
		if pending.Alerta.Tipo != "senha_customizada" {
			t.Fatalf("tipo do alerta = %s, esperado senha_customizada", pending.Alerta.Tipo)
		}
		// Deve notificar o admin (destinatario do nivel 2), nao o nivel1User
		if len(pending.UsuarioIDs) != 1 || pending.UsuarioIDs[0] != c.admin.ID {
			t.Fatalf("usuario_ids = %v, esperado [%s] (destinatarios do nivel 2)", pending.UsuarioIDs, c.admin.ID)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("nenhum PendingAlert recebido no canal em 2s")
	}
}

// Checkin com senha invalida: a acao deve prosseguir normalmente (200 OK),
// sem nenhum alerta gerado e sem alterar o status do turno.
func TestCheckinComSenhaInvalidaProssegueNormalmente(t *testing.T) {
	c := novoCenario(t)
	turno := c.iniciarTurno()

	var resp map[string]any
	c.e.reqJSON(http.MethodPost, "/api/v1/turnos/checkin", c.vigiaToken,
		c.checkinBody(turno.ID, "0000", time.Now().Add(1*time.Minute)), http.StatusOK, &resp)

	if resp["status"] != "em_andamento" {
		t.Errorf("status = %v, esperado em_andamento", resp["status"])
	}

	select {
	case pending := <-c.e.app.AlertaService.AlertChannel():
		t.Fatalf("alerta inesperado gerado para senha invalida: %+v", pending.Alerta)
	case <-time.After(300 * time.Millisecond):
	}
}

// Checkin com senha ok: acao prossegue normalmente, sem alerta.
func TestCheckinComSenhaOkNaoGeraAlerta(t *testing.T) {
	c := novoCenario(t)
	turno := c.iniciarTurno()

	var resp map[string]any
	c.e.reqJSON(http.MethodPost, "/api/v1/turnos/checkin", c.vigiaToken,
		c.checkinBody(turno.ID, SenhaOK, time.Now().Add(1*time.Minute)), http.StatusOK, &resp)

	if resp["status"] != "em_andamento" {
		t.Errorf("status = %v, esperado em_andamento", resp["status"])
	}

	select {
	case pending := <-c.e.app.AlertaService.AlertChannel():
		t.Fatalf("alerta inesperado gerado para senha ok: %+v", pending.Alerta)
	case <-time.After(300 * time.Millisecond):
	}
}

// Finalizar turno com senha ok nao gera alerta e turno termina finalizado.
func TestFinalizarTurnoComSenhaOk(t *testing.T) {
	c := novoCenario(t)
	turno := c.iniciarTurno()

	var fin map[string]any
	c.e.reqJSON(http.MethodPost, "/api/v1/turnos/finalizar", c.vigiaToken, map[string]any{
		"turno_id": turno.ID.String(), "device_id": c.deviceID,
		"latitude": postoLat, "longitude": postoLon, "senha": SenhaOK,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}, http.StatusOK, &fin)

	if fin["status"] != "finalizado" {
		t.Errorf("status = %v, esperado finalizado", fin["status"])
	}

	select {
	case pending := <-c.e.app.AlertaService.AlertChannel():
		t.Fatalf("alerta inesperado gerado para finalizar com senha ok: %+v", pending.Alerta)
	case <-time.After(300 * time.Millisecond):
	}
}

// CreateSenhaVigia com tipo invalido retorna 422.
func TestCreateSenhaVigiaTipoInvalido(t *testing.T) {
	c := novoCenario(t)

	status, _ := c.e.request(http.MethodPost, "/api/v1/usuarios/"+c.vigia.ID.String()+"/senhas", c.adminToken, map[string]any{
		"tipo":   "invalido",
		"codigo": "1234",
	})
	if status != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, esperado 422", status)
	}
}

// CreateSenhaVigia com codigo > 6 digitos retorna 422.
func TestCreateSenhaVigiaCodigoLongo(t *testing.T) {
	c := novoCenario(t)

	status, _ := c.e.request(http.MethodPost, "/api/v1/usuarios/"+c.vigia.ID.String()+"/senhas", c.adminToken, map[string]any{
		"tipo": "customizada", "codigo": "1234567", "descricao": "codigo longo",
	})
	if status != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, esperado 422", status)
	}
}

// Vigia nao-admin nao pode acessar CRUD de senhas (403).
func TestSenhaVigiaRBAC(t *testing.T) {
	c := novoCenario(t)

	t.Run("vigia nao pode listar senhas de outro vigia", func(t *testing.T) {
		outroVigia := c.e.criarUsuario(c.empresa.ID, "Outro Vigia", "outro.vigia.rbac@a.com", "senha123", "vigia", true)
		status, _ := c.e.request(http.MethodGet, "/api/v1/usuarios/"+outroVigia.ID.String()+"/senhas", c.vigiaToken, nil)
		if status != http.StatusForbidden {
			t.Errorf("status = %d, esperado 403", status)
		}
	})

	t.Run("vigia nao pode criar senha", func(t *testing.T) {
		status, _ := c.e.request(http.MethodPost, "/api/v1/usuarios/"+c.vigia.ID.String()+"/senhas", c.vigiaToken, map[string]any{
			"tipo": "customizada", "codigo": "1234", "descricao": "test",
		})
		if status != http.StatusForbidden {
			t.Errorf("status = %d, esperado 403", status)
		}
	})

	t.Run("sem autenticacao retorna 401", func(t *testing.T) {
		status, _ := c.e.request(http.MethodGet, "/api/v1/usuarios/"+uuid.New().String()+"/senhas", "", nil)
		if status != http.StatusUnauthorized {
			t.Errorf("status = %d, esperado 401", status)
		}
	})
}
