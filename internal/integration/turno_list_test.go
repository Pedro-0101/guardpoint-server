//go:build integration

package integration

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/guardpoint/guardpoint-server/internal/model"
)

type cenarioLista struct {
	e          *env
	empresa    *model.Empresa
	admin      *model.User
	vigia      *model.User
	adminToken string
	vigiaToken string
	posto      model.Posto
	deviceID   string
}

// novoAmbienteLista cria um ambiente minimo para testes de listagem de turnos,
// pulando a criacao de senhas (que depende de config_escalonamento, ausente no
// banco recem-migrado). Os testes de iniciarTurno que precisam de senha usam
// este setup mais o helper criarSenhaVigiaDireto que insere a config ausente.
func novoAmbienteLista(t *testing.T) *cenarioLista {
	t.Helper()
	e := newEnv(t)

	empresa := e.criarEmpresa("Empresa Lista", "22222222000192")
	admin := e.criarUsuario(empresa.ID, "Admin Lista", "admin.lista@a.com", "senha123", "admin", true)
	vigia := e.criarUsuario(empresa.ID, "Vigia Lista", "vigia.lista@a.com", "senha123", "vigia", true)

	c := &cenarioLista{
		e:          e,
		empresa:    empresa,
		admin:      admin,
		vigia:      vigia,
		adminToken: e.login(admin.Email, "senha123").AccessToken,
		vigiaToken: e.login(vigia.Email, "senha123").AccessToken,
		deviceID:   "device-lista-01",
	}

	// Cria o registro padrao de config_escalonamento que as migrations
	// deveriam criar mas nao criam quando executadas do zero.
	var nivelID uuid.UUID
	err := e.pool.QueryRow(context.Background(),
		`INSERT INTO config_escalonamento (empresa_id, atraso_minutos, descricao, sistema)
		 VALUES ($1, 15, '', true) RETURNING id`, empresa.ID).Scan(&nivelID)
	if err != nil {
		t.Fatalf("inserir config_escalonamento: %v", err)
	}

	// Cadastra os PINs obrigatorios via SQL direto.
	for _, s := range []struct {
		tipo   string
		codigo string
		nivel  *uuid.UUID
	}{
		{"ok", "1234", nil},
		{"emergencia", "9999", &nivelID},
	} {
		_, err = e.pool.Exec(context.Background(),
			`INSERT INTO senhas_vigia (empresa_id, usuario_id, tipo, codigo, nivel_escalonamento_id)
			 VALUES ($1, $2, $3, $4, $5)`,
			empresa.ID, vigia.ID, s.tipo, s.codigo, s.nivel)
		if err != nil {
			t.Fatalf("inserir senha %s: %v", s.tipo, err)
		}
	}

	e.reqJSON(http.MethodPost, "/api/v1/postos", c.adminToken, map[string]any{
		"nome": "Posto Lista", "latitude": postoLat, "longitude": postoLon, "raio_m": 100,
	}, http.StatusCreated, &c.posto)

	var reg model.BiometricRegisterResponse
	e.reqJSON(http.MethodPost, "/api/v1/auth/biometric/register", c.vigiaToken,
		map[string]string{"device_id": c.deviceID}, http.StatusCreated, &reg)

	var esc model.Escala
	e.reqJSON(http.MethodPost, "/api/v1/escalas", c.adminToken, map[string]any{
		"usuario_id":        vigia.ID.String(),
		"posto_id":          c.posto.ID.String(),
		"dia_semana_inicio": int16(time.Now().Weekday()),
		"hora_inicio":       time.Now().Add(-1 * time.Hour).Format("15:04"),
		"dia_semana_fim":    int16(time.Now().Weekday()),
		"hora_fim":          time.Now().Add(7 * time.Hour).Format("15:04"),
		"tolerancia_min":    120,
	}, http.StatusCreated, &esc)

	return c
}

func (c *cenarioLista) criarSubstituicao(usuarioID, postoID uuid.UUID, dataInicio, dataFim, horaInicio, horaFim string, toleranciaMin int) *model.Substituicao {
	c.e.t.Helper()
	var sub model.Substituicao
	c.e.reqJSON(http.MethodPost, "/api/v1/substituicoes", c.adminToken, map[string]any{
		"usuario_id":     usuarioID.String(),
		"posto_id":       postoID.String(),
		"data_inicio":    dataInicio,
		"data_fim":       dataFim,
		"hora_inicio":    horaInicio,
		"hora_fim":       horaFim,
		"tolerancia_min": toleranciaMin,
	}, http.StatusCreated, &sub)
	return &sub
}

func (c *cenarioLista) iniciarTurno() model.Turno {
	c.e.t.Helper()
	var turno model.Turno
	c.e.reqJSON(http.MethodPost, "/api/v1/turnos/iniciar", c.vigiaToken, map[string]any{
		"posto_id": c.posto.ID.String(), "device_id": c.deviceID, "intervalo_min": 30,
		"latitude": postoLat, "longitude": postoLon, "senha": "1234",
	}, http.StatusCreated, &turno)
	return turno
}

func hojeStr() string {
	return time.Now().Format("2006-01-02")
}

func TestGetTurnos_SubstituicaoId_RealTurno(t *testing.T) {
	c := novoAmbienteLista(t)
	h := hojeStr()
	agora := time.Now()
	hInicio := agora.Add(-2 * time.Hour).Format("15:04")
	hFim := agora.Add(6 * time.Hour).Format("15:04")

	sub := c.criarSubstituicao(c.vigia.ID, c.posto.ID, h, h, hInicio, hFim, 120)

	turno := c.iniciarTurno()

	if turno.SubstituicaoID == nil {
		t.Fatal("iniciarTurno response: substituicao_id is nil")
	}
	if *turno.SubstituicaoID != sub.ID {
		t.Fatalf("iniciarTurno response: substituicao_id = %v, esperado %v", *turno.SubstituicaoID, sub.ID)
	}

	t.Run("GET /turnos lista o turno real com substituicao_id", func(t *testing.T) {
		var listResp struct {
			Data  []model.TurnoDetalhe `json:"data"`
			Total int                  `json:"total"`
		}
		c.e.reqJSON(http.MethodGet, "/api/v1/turnos", c.adminToken, nil, http.StatusOK, &listResp)

		var found bool
		for _, td := range listResp.Data {
			if td.Turno.ID == turno.ID {
				found = true
				if td.Turno.SubstituicaoID == nil {
					t.Error("GET /turnos: substituicao_id is nil")
				} else if *td.Turno.SubstituicaoID != sub.ID {
					t.Errorf("GET /turnos: substituicao_id = %v, esperado %v", *td.Turno.SubstituicaoID, sub.ID)
				}
				break
			}
		}
		if !found {
			t.Errorf("turno %v nao encontrado na lista", turno.ID)
		}
	})

	t.Run("GET /turnos?status=agendado nao inclui o turno real", func(t *testing.T) {
		var listResp struct {
			Data  []model.TurnoDetalhe `json:"data"`
			Total int                  `json:"total"`
		}
		c.e.reqJSON(http.MethodGet, "/api/v1/turnos?status=agendado&data_inicio="+h+"&data_fim="+h, c.adminToken, nil, http.StatusOK, &listResp)

		for _, td := range listResp.Data {
			if td.Turno.ID == turno.ID {
				t.Error("turno real apareceu na lista agendado")
			}
		}
	})
}

func TestGetTurnos_SubstituicaoId_Agendado_MesmoUsuario(t *testing.T) {
	c := novoAmbienteLista(t)
	h := hojeStr()

	_ = c.criarSubstituicao(c.vigia.ID, c.posto.ID, h, h, "10:00", "18:00", 120)

	t.Run("GET /turnos?status=agendado retorna turno com substituicao_id", func(t *testing.T) {
		var listResp struct {
			Data  []model.TurnoDetalhe `json:"data"`
			Total int                  `json:"total"`
		}
		c.e.reqJSON(http.MethodGet, "/api/v1/turnos?status=agendado&data_inicio="+h+"&data_fim="+h, c.adminToken, nil, http.StatusOK, &listResp)

		if listResp.Total == 0 {
			t.Fatal("nenhum turno agendado retornado")
		}

		var found bool
		for _, td := range listResp.Data {
			if td.Turno.UsuarioID == c.vigia.ID && td.Turno.PostoID == c.posto.ID {
				found = true
				if td.Turno.SubstituicaoID == nil {
					t.Error("turno agendado com substituicao: substituicao_id is nil")
				}
				break
			}
		}
		if !found {
			t.Error("turno agendado para o vigia nao encontrado")
		}
	})
}

func TestGetTurnos_SubstituicaoId_Agendado_DiferenteUsuario(t *testing.T) {
	c := novoAmbienteLista(t)
	h := hojeStr()

	vigiaB := c.e.criarUsuario(c.empresa.ID, "Vigia B", "vigiaB@a.com", "senha123", "vigia", true)

	_ = c.criarSubstituicao(vigiaB.ID, c.posto.ID, h, h, "10:00", "18:00", 120)

	t.Run("GET /turnos?status=agendado retorna turno do substituto com substituicao_id", func(t *testing.T) {
		var listResp struct {
			Data  []model.TurnoDetalhe `json:"data"`
			Total int                  `json:"total"`
		}
		c.e.reqJSON(http.MethodGet, "/api/v1/turnos?status=agendado&data_inicio="+h+"&data_fim="+h, c.adminToken, nil, http.StatusOK, &listResp)

		if listResp.Total == 0 {
			t.Fatal("nenhum turno agendado retornado")
		}

		var foundSubstituto bool
		var foundOriginal bool
		for _, td := range listResp.Data {
			if td.Turno.UsuarioID == vigiaB.ID && td.Turno.PostoID == c.posto.ID {
				foundSubstituto = true
				if td.Turno.SubstituicaoID == nil {
					t.Error("turno agendado do substituto: substituicao_id is nil")
				}
			}
			if td.Turno.UsuarioID == c.vigia.ID && td.Turno.PostoID == c.posto.ID {
				foundOriginal = true
			}
		}
		if !foundSubstituto {
			t.Error("turno agendado para o substituto (vigiaB) nao encontrado")
		}
		if foundOriginal {
			t.Error("turno agendado do vigia original nao deveria aparecer pois foi substituido")
		}
	})
}
