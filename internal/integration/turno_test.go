//go:build integration

package integration

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/guardpoint/guardpoint-server/internal/model"
)

func nowRFC3339() string {
	return time.Now().UTC().Format(time.RFC3339)
}

func TestIniciarTurno(t *testing.T) {
	c := novoCenario(t)

	t.Run("device nao registrado retorna 403", func(t *testing.T) {
		status, _ := c.e.request(http.MethodPost, "/api/v1/turnos/iniciar", c.vigiaToken, map[string]any{
			"posto_id": c.posto.ID.String(), "device_id": "device-nao-registrado",
		})
		if status != http.StatusForbidden {
			t.Errorf("status = %d, esperado 403", status)
		}
	})

	t.Run("caminho feliz retorna 201 e grava device", func(t *testing.T) {
		turno := c.iniciarTurno()
		if turno.Status != "em_andamento" {
			t.Errorf("status do turno = %q", turno.Status)
		}
		if turno.DeviceID == nil || *turno.DeviceID != c.deviceID {
			t.Errorf("turno nao gravou device_id de origem (A4): %v", turno.DeviceID)
		}
		if turno.TokenSessao == nil || *turno.TokenSessao == "" {
			t.Error("turno sem token_sessao")
		}
	})

	t.Run("segundo turno simultaneo retorna 409", func(t *testing.T) {
		status, _ := c.e.request(http.MethodPost, "/api/v1/turnos/iniciar", c.vigiaToken, map[string]any{
			"posto_id": c.posto.ID.String(), "device_id": c.deviceID,
		})
		if status != http.StatusConflict {
			t.Errorf("status = %d, esperado 409", status)
		}
	})
}

func TestIniciarTurnoSemEscalaEForaTolerancia(t *testing.T) {
	c := novoCenario(t)

	// vigia 2 sem escala nenhuma
	vigia2 := c.e.criarUsuario(c.empresa.ID, "Vigia 2", "vigia2@a.com", "senha123", "vigia", true)
	token2 := c.e.login(vigia2.Email, "senha123").AccessToken
	var resp model.BiometricRegisterResponse
	c.e.reqJSON(http.MethodPost, "/api/v1/auth/biometric/register", token2,
		map[string]string{"device_id": "device-vigia-2"}, http.StatusCreated, &resp)

	t.Run("sem escala ativa retorna 403", func(t *testing.T) {
		status, raw := c.e.request(http.MethodPost, "/api/v1/turnos/iniciar", token2, map[string]any{
			"posto_id": c.posto.ID.String(), "device_id": "device-vigia-2",
		})
		if status != http.StatusForbidden || !strings.Contains(string(raw), "escala") {
			t.Errorf("status = %d (corpo %s), esperado 403 por falta de escala", status, raw)
		}
	})

	t.Run("fora da tolerancia retorna 403", func(t *testing.T) {
		// escala comecou ha 4h com tolerancia de 15 min
		c.criarEscala(vigia2.ID, c.posto.ID, time.Now().Add(-4*time.Hour), 15)

		status, raw := c.e.request(http.MethodPost, "/api/v1/turnos/iniciar", token2, map[string]any{
			"posto_id": c.posto.ID.String(), "device_id": "device-vigia-2",
		})
		if status != http.StatusForbidden || !strings.Contains(string(raw), "tolerancia") {
			t.Errorf("status = %d (corpo %s), esperado 403 por tolerancia", status, raw)
		}
	})
}

// Regressao A1: escala noturna que cruza a meia-noite pode ser cadastrada.
func TestEscalaNoturna(t *testing.T) {
	c := novoCenario(t)

	var esc model.Escala
	c.e.reqJSON(http.MethodPost, "/api/v1/escalas", c.adminToken, map[string]any{
		"usuario_id":  c.vigia.ID.String(),
		"posto_id":    c.posto.ID.String(),
		"data_inicio": time.Now().Format("2006-01-02"),
		"data_fim":    time.Now().AddDate(0, 1, 0).Format("2006-01-02"),
		"hora_inicio": "22:00",
		"hora_fim":    "06:00",
		"dias_semana": []int{0, 1, 2, 3, 4, 5, 6},
	}, http.StatusCreated, &esc)

	if !strings.HasPrefix(esc.HoraFim, "06:00") {
		t.Errorf("hora_fim = %q", esc.HoraFim)
	}

	t.Run("hora_fim igual a hora_inicio e rejeitada", func(t *testing.T) {
		status, _ := c.e.request(http.MethodPost, "/api/v1/escalas", c.adminToken, map[string]any{
			"usuario_id":  c.vigia.ID.String(),
			"posto_id":    c.posto.ID.String(),
			"data_inicio": time.Now().Format("2006-01-02"),
			"data_fim":    time.Now().AddDate(0, 1, 0).Format("2006-01-02"),
			"hora_inicio": "08:00",
			"hora_fim":    "08:00",
			"dias_semana": []int{0, 1, 2, 3, 4, 5, 6},
		})
		if status == http.StatusCreated {
			t.Error("escala com hora_fim == hora_inicio foi aceita")
		}
	})
}

func TestCheckin(t *testing.T) {
	c := novoCenario(t)
	turno := c.iniciarTurno()
	base := time.Now()

	t.Run("dentro do geofence", func(t *testing.T) {
		var resp model.CheckinResponse
		c.e.reqJSON(http.MethodPost, "/api/v1/turnos/checkin", c.vigiaToken,
			c.checkinBody(turno.ID, "padrao", base), http.StatusOK, &resp)
		if resp.Checkin.FlagGeofence == nil || *resp.Checkin.FlagGeofence != "ok" {
			t.Errorf("flag_geofence = %v, esperado ok", resp.Checkin.FlagGeofence)
		}
		if resp.Atrasado {
			t.Error("primeiro check-in dentro do intervalo marcado como atrasado")
		}
		if resp.ProximoDeadline == nil {
			t.Error("resposta sem proximo_deadline")
		}
	})

	t.Run("fora do raio marca desvio_rota", func(t *testing.T) {
		body := c.checkinBody(turno.ID, "padrao", base.Add(1*time.Minute))
		body["latitude"] = postoLat + 0.01 // ~1.1 km
		var resp model.CheckinResponse
		c.e.reqJSON(http.MethodPost, "/api/v1/turnos/checkin", c.vigiaToken, body, http.StatusOK, &resp)
		if resp.Checkin.FlagGeofence == nil || *resp.Checkin.FlagGeofence != "desvio_rota" {
			t.Errorf("flag_geofence = %v, esperado desvio_rota", resp.Checkin.FlagGeofence)
		}
	})

	t.Run("check-in alem da janela marca atrasado (A2)", func(t *testing.T) {
		var resp model.CheckinResponse
		c.e.reqJSON(http.MethodPost, "/api/v1/turnos/checkin", c.vigiaToken,
			c.checkinBody(turno.ID, "padrao", base.Add(45*time.Minute)), http.StatusOK, &resp)
		if !resp.Atrasado {
			t.Error("check-in 45 min apos o anterior (intervalo 30) nao marcou atrasado")
		}
	})

	t.Run("device diferente retorna 403 (A4)", func(t *testing.T) {
		body := c.checkinBody(turno.ID, "padrao", base.Add(46*time.Minute))
		body["device_id"] = "device-clonado"
		status, _ := c.e.request(http.MethodPost, "/api/v1/turnos/checkin", c.vigiaToken, body)
		if status != http.StatusForbidden {
			t.Errorf("status = %d, esperado 403", status)
		}
	})

	t.Run("coacao gera alerta e status critico com resposta normal", func(t *testing.T) {
		var resp model.CheckinResponse
		c.e.reqJSON(http.MethodPost, "/api/v1/turnos/checkin", c.vigiaToken,
			c.checkinBody(turno.ID, "coacao", base.Add(47*time.Minute)), http.StatusOK, &resp)

		det := c.getTurno(turno.ID)
		if det.Status != "critico" {
			t.Errorf("status do turno apos coacao = %q, esperado critico", det.Status)
		}
		if c.contarAlertas("coacao") == 0 {
			t.Error("coacao nao gerou alerta")
		}
	})
}

func TestFinalizarESabotagem(t *testing.T) {
	c := novoCenario(t)
	turno := c.iniciarTurno()

	t.Run("sabotagem registra alerta e status critico", func(t *testing.T) {
		var resp model.SabotagemResponse
		c.e.reqJSON(http.MethodPost, "/api/v1/turnos/sabotagem", c.vigiaToken, map[string]any{
			"turno_id": turno.ID.String(), "device_id": c.deviceID,
			"latitude": postoLat, "longitude": postoLon,
			"motivo": "camera cortada", "timestamp": nowRFC3339(),
		}, http.StatusAccepted, &resp)

		if c.contarAlertas("sabotagem") == 0 {
			t.Error("sabotagem nao gerou alerta")
		}
		if det := c.getTurno(turno.ID); det.Status != "critico" {
			t.Errorf("status = %q, esperado critico", det.Status)
		}
	})

	t.Run("finalizar grava checkin de finalizacao e encerra", func(t *testing.T) {
		var fin model.Turno
		c.e.reqJSON(http.MethodPost, "/api/v1/turnos/finalizar", c.vigiaToken, map[string]any{
			"turno_id": turno.ID.String(), "device_id": c.deviceID,
			"latitude": postoLat, "longitude": postoLon, "timestamp": nowRFC3339(),
		}, http.StatusOK, &fin)
		if fin.Status != "finalizado" || fin.FimReal == nil {
			t.Errorf("turno nao finalizado corretamente: status=%q fim_real=%v", fin.Status, fin.FimReal)
		}

		det := c.getTurno(turno.ID)
		var temFinalizacao bool
		for _, ck := range det.Checkins {
			if ck.TipoSenha == "finalizacao" {
				temFinalizacao = true
			}
		}
		if !temFinalizacao {
			t.Error("finalizacao nao gravou check-in tipo finalizacao")
		}
	})

	t.Run("checkin em turno finalizado retorna 409", func(t *testing.T) {
		status, _ := c.e.request(http.MethodPost, "/api/v1/turnos/checkin", c.vigiaToken,
			c.checkinBody(turno.ID, "padrao", time.Now()))
		if status != http.StatusConflict {
			t.Errorf("status = %d, esperado 409", status)
		}
	})
}

func TestLoteOffline(t *testing.T) {
	c := novoCenario(t)
	turno := c.iniciarTurno()
	base := time.Now().Add(-30 * time.Minute)

	item := func(clienteID, tipo string, ts time.Time) map[string]any {
		b := c.checkinBody(turno.ID, tipo, ts)
		b["cliente_checkin_id"] = clienteID
		return b
	}
	id1, id2 := "0d4de1a1-0000-4000-8000-000000000001", "0d4de1a1-0000-4000-8000-000000000002"
	lote := []map[string]any{
		item(id1, "padrao", base),
		item(id2, "padrao", base.Add(15*time.Minute)),
	}

	contarCheckins := func() int {
		var n int
		if err := c.e.pool.QueryRow(t.Context(),
			`SELECT COUNT(*) FROM checkins WHERE turno_id = $1`, turno.ID).Scan(&n); err != nil {
			t.Fatalf("contar checkins: %v", err)
		}
		return n
	}

	c.e.reqJSON(http.MethodPost, "/api/v1/checkins/lote", c.vigiaToken, lote, http.StatusOK, nil)
	antes := contarCheckins()
	if antes != 2 {
		t.Fatalf("lote inicial gravou %d checkins, esperado 2", antes)
	}

	t.Run("reenvio do lote e idempotente", func(t *testing.T) {
		c.e.reqJSON(http.MethodPost, "/api/v1/checkins/lote", c.vigiaToken, lote, http.StatusOK, nil)
		if depois := contarCheckins(); depois != 2 {
			t.Errorf("reenvio duplicou checkins: %d", depois)
		}
	})

	t.Run("reconciliacao fecha alertas de atraso como falso positivo", func(t *testing.T) {
		// alerta de atraso aberto, gerado enquanto o vigia estava offline
		if _, err := c.e.pool.Exec(t.Context(),
			`INSERT INTO alertas (empresa_id, turno_id, tipo, nivel, status) VALUES ($1, $2, 'atraso_n1', 1, 'aberto')`,
			c.empresa.ID, turno.ID); err != nil {
			t.Fatalf("inserir alerta: %v", err)
		}

		// lote sincronizado cobrindo o gap ate agora
		c.e.reqJSON(http.MethodPost, "/api/v1/checkins/lote", c.vigiaToken, []map[string]any{
			item("0d4de1a1-0000-4000-8000-000000000003", "padrao", time.Now()),
		}, http.StatusOK, nil)

		var status string
		if err := c.e.pool.QueryRow(t.Context(),
			`SELECT status FROM alertas WHERE turno_id = $1 AND tipo = 'atraso_n1'`, turno.ID).Scan(&status); err != nil {
			t.Fatalf("buscar alerta: %v", err)
		}
		if status != "falso_positivo" {
			t.Errorf("alerta de atraso = %q, esperado falso_positivo (Sync Reconciler)", status)
		}
	})

	t.Run("coacao em lote deixa turno critico", func(t *testing.T) {
		c.e.reqJSON(http.MethodPost, "/api/v1/checkins/lote", c.vigiaToken, []map[string]any{
			item("0d4de1a1-0000-4000-8000-000000000004", "coacao", time.Now()),
		}, http.StatusOK, nil)
		if det := c.getTurno(turno.ID); det.Status != "critico" {
			t.Errorf("status = %q, esperado critico", det.Status)
		}
	})
}

// Fluxo completo A3: revogar invalida a sessao sem finalizar o turno e o PIN
// reassocia o turno a um novo dispositivo.
func TestRevogarEReassociar(t *testing.T) {
	c := novoCenario(t)
	turno := c.iniciarTurno()

	var rev model.RevogarResponse
	c.e.reqJSON(http.MethodPost, "/api/v1/turnos/"+turno.ID.String()+"/revogar", c.adminToken,
		nil, http.StatusOK, &rev)
	if len(rev.PinNovoDispositivo) != 6 {
		t.Fatalf("pin = %q, esperado 6 digitos", rev.PinNovoDispositivo)
	}

	t.Run("turno continua em andamento apos revogar", func(t *testing.T) {
		if det := c.getTurno(turno.ID); det.Status != "em_andamento" {
			t.Errorf("status = %q; revogar nao deve finalizar o turno", det.Status)
		}
	})

	t.Run("checkin do device revogado retorna 403", func(t *testing.T) {
		status, raw := c.e.request(http.MethodPost, "/api/v1/turnos/checkin", c.vigiaToken,
			c.checkinBody(turno.ID, "padrao", time.Now()))
		if status != http.StatusForbidden || !strings.Contains(string(raw), "revogada") {
			t.Errorf("status = %d (corpo %s), esperado 403 sessao revogada", status, raw)
		}
	})

	// novo aparelho: vigia loga por senha e registra biometria
	novoDevice := "device-vigia-a-02"
	var reg model.BiometricRegisterResponse
	c.e.reqJSON(http.MethodPost, "/api/v1/auth/biometric/register", c.vigiaToken,
		map[string]string{"device_id": novoDevice}, http.StatusCreated, &reg)

	t.Run("pin errado retorna 403", func(t *testing.T) {
		pinErrado := "000000"
		if rev.PinNovoDispositivo == pinErrado {
			pinErrado = "999999"
		}
		status, _ := c.e.request(http.MethodPost, "/api/v1/turnos/reassociar", c.vigiaToken,
			map[string]string{"pin": pinErrado, "device_id": novoDevice})
		if status != http.StatusForbidden {
			t.Errorf("status = %d, esperado 403", status)
		}
	})

	t.Run("pin correto reassocia o turno ao novo device", func(t *testing.T) {
		var reassociado model.Turno
		c.e.reqJSON(http.MethodPost, "/api/v1/turnos/reassociar", c.vigiaToken,
			map[string]string{"pin": rev.PinNovoDispositivo, "device_id": novoDevice},
			http.StatusOK, &reassociado)
		if reassociado.DeviceID == nil || *reassociado.DeviceID != novoDevice {
			t.Errorf("device_id apos reassociacao = %v", reassociado.DeviceID)
		}

		c.deviceID = novoDevice
		var resp model.CheckinResponse
		c.e.reqJSON(http.MethodPost, "/api/v1/turnos/checkin", c.vigiaToken,
			c.checkinBody(turno.ID, "padrao", time.Now()), http.StatusOK, &resp)
	})

	t.Run("pin nao pode ser reutilizado", func(t *testing.T) {
		status, _ := c.e.request(http.MethodPost, "/api/v1/turnos/reassociar", c.vigiaToken,
			map[string]string{"pin": rev.PinNovoDispositivo, "device_id": novoDevice})
		if status != http.StatusForbidden {
			t.Errorf("status = %d, esperado 403 (pin ja consumido)", status)
		}
	})
}

func TestAlertasTransicoes(t *testing.T) {
	c := novoCenario(t)
	turno := c.iniciarTurno()

	// gera um alerta de coacao
	c.e.reqJSON(http.MethodPost, "/api/v1/turnos/checkin", c.vigiaToken,
		c.checkinBody(turno.ID, "coacao", time.Now()), http.StatusOK, nil)

	var lista struct {
		Data []model.Alerta `json:"data"`
	}
	c.e.reqJSON(http.MethodGet, "/api/v1/alertas?tipo=coacao", c.adminToken, nil, http.StatusOK, &lista)
	if len(lista.Data) == 0 {
		t.Fatal("alerta de coacao nao encontrado")
	}
	alertaID := lista.Data[0].ID.String()

	c.e.reqJSON(http.MethodPut, "/api/v1/alertas/"+alertaID+"/reconhecer", c.adminToken, nil, http.StatusOK, nil)

	t.Run("reconhecer duas vezes retorna 409", func(t *testing.T) {
		status, _ := c.e.request(http.MethodPut, "/api/v1/alertas/"+alertaID+"/reconhecer", c.adminToken, nil)
		if status != http.StatusConflict {
			t.Errorf("status = %d, esperado 409", status)
		}
	})

	c.e.reqJSON(http.MethodPut, "/api/v1/alertas/"+alertaID+"/encerrar", c.adminToken, nil, http.StatusOK, nil)

	t.Run("encerrar duas vezes retorna 409", func(t *testing.T) {
		status, _ := c.e.request(http.MethodPut, "/api/v1/alertas/"+alertaID+"/encerrar", c.adminToken, nil)
		if status != http.StatusConflict {
			t.Errorf("status = %d, esperado 409", status)
		}
	})

	t.Run("estatisticas respondem", func(t *testing.T) {
		var stats model.AlertStatistics
		c.e.reqJSON(http.MethodGet, "/api/v1/alertas/estatisticas", c.adminToken, nil, http.StatusOK, &stats)
		if stats.TotalEncerrados == 0 {
			t.Error("estatisticas nao contabilizaram o alerta encerrado")
		}
	})
}
