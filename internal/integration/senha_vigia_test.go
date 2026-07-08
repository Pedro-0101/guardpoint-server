//go:build integration

package integration

import (
	"net/http"
	"testing"

	"github.com/google/uuid"
)

func TestSenhaVigiaCRUD(t *testing.T) {
	c := novoCenario(t)

	t.Run("List retorna as senhas cadastradas", func(t *testing.T) {
		var senhas []map[string]any
		c.e.reqJSON(http.MethodGet, "/api/v1/usuarios/"+c.vigia.ID.String()+"/senhas", c.adminToken, nil, http.StatusOK, &senhas)

		if len(senhas) < 2 {
			t.Fatalf("senhas = %d, esperado pelo menos 2 (ok e emergencia criadas em novoCenario)", len(senhas))
		}

		temOK := false
		temEmergencia := false
		for _, s := range senhas {
			tipo, _ := s["tipo"].(string)
			if tipo == "ok" {
				temOK = true
			}
			if tipo == "emergencia" {
				temEmergencia = true
			}
		}
		if !temOK || !temEmergencia {
			t.Fatal("lista de senhas nao contem os dois tipos obrigatorios")
		}
	})

	t.Run("List para usuario de outra empresa retorna 404", func(t *testing.T) {
		outraEmpresa := c.e.criarEmpresa("Outra Empresa", "33333333000191")
		userOutra := c.e.criarUsuario(outraEmpresa.ID, "User Outra", "user@outra.com", "senha123", "vigia", true)

		status, _ := c.e.request(http.MethodGet, "/api/v1/usuarios/"+userOutra.ID.String()+"/senhas", c.adminToken, nil)
		if status != http.StatusNotFound {
			t.Errorf("status = %d, esperado 404", status)
		}
	})

	t.Run("List retorna array vazio para vigia sem senhas", func(t *testing.T) {
		vigiaSemPin := c.e.criarUsuario(c.empresa.ID, "Vigia Sem Pin CRUD", "sempin.crud@a.com", "senha123", "vigia", true)

		var senhas []map[string]any
		c.e.reqJSON(http.MethodGet, "/api/v1/usuarios/"+vigiaSemPin.ID.String()+"/senhas", c.adminToken, nil, http.StatusOK, &senhas)
		if len(senhas) != 0 {
			t.Errorf("senhas = %d, esperado 0", len(senhas))
		}
	})

	t.Run("Create cadastra nova senha customizada", func(t *testing.T) {
		nivelID := c.criarNivel(2, 10)
		desc := "pin de teste"
		var senha map[string]any
		c.e.reqJSON(http.MethodPost, "/api/v1/usuarios/"+c.vigia.ID.String()+"/senhas", c.adminToken, map[string]any{
			"tipo":                   "customizada",
			"codigo":                 "5555",
			"descricao":              desc,
			"nivel_escalonamento_id": nivelID.String(),
		}, http.StatusCreated, &senha)

		if senha["tipo"] != "customizada" {
			t.Errorf("tipo = %v, esperado customizada", senha["tipo"])
		}
		if senha["codigo"] != "5555" {
			t.Errorf("codigo = %v, esperado 5555", senha["codigo"])
		}
	})

	t.Run("Create codigo duplicado retorna 400", func(t *testing.T) {
		nivelID := c.criarNivel(2, 10)
		status, _ := c.e.request(http.MethodPost, "/api/v1/usuarios/"+c.vigia.ID.String()+"/senhas", c.adminToken, map[string]any{
			"tipo":                   "ok",
			"codigo":                 SenhaOK,
			"nivel_escalonamento_id": nivelID.String(),
		})
		if status != http.StatusBadRequest {
			t.Errorf("status = %d, esperado 400", status)
		}
	})

	t.Run("Create tipo ok ja existente retorna 400", func(t *testing.T) {
		status, _ := c.e.request(http.MethodPost, "/api/v1/usuarios/"+c.vigia.ID.String()+"/senhas", c.adminToken, map[string]any{
			"tipo":   "ok",
			"codigo": "1111",
		})
		if status != http.StatusBadRequest {
			t.Errorf("status = %d, esperado 400", status)
		}
	})

	t.Run("Create tipo emergencia ja existente retorna 400", func(t *testing.T) {
		nivelEmergenciaID := c.e.buscarNivelEmergenciaPadrao(c.empresa.ID)
		status, _ := c.e.request(http.MethodPost, "/api/v1/usuarios/"+c.vigia.ID.String()+"/senhas", c.adminToken, map[string]any{
			"tipo":                   "emergencia",
			"codigo":                 "1111",
			"nivel_escalonamento_id": nivelEmergenciaID.String(),
		})
		if status != http.StatusBadRequest {
			t.Errorf("status = %d, esperado 400", status)
		}
	})

	t.Run("Create customizada sem descricao retorna 422", func(t *testing.T) {
		nivelID := c.criarNivel(2, 10)
		status, _ := c.e.request(http.MethodPost, "/api/v1/usuarios/"+c.vigia.ID.String()+"/senhas", c.adminToken, map[string]any{
			"tipo":                   "customizada",
			"codigo":                 "6666",
			"nivel_escalonamento_id": nivelID.String(),
		})
		if status != http.StatusUnprocessableEntity {
			t.Errorf("status = %d, esperado 422", status)
		}
	})

	t.Run("Create customizada sem nivel_escalonamento_id retorna 400", func(t *testing.T) {
		status, _ := c.e.request(http.MethodPost, "/api/v1/usuarios/"+c.vigia.ID.String()+"/senhas", c.adminToken, map[string]any{
			"tipo": "customizada", "codigo": "6666", "descricao": "sem nivel",
		})
		if status != http.StatusBadRequest {
			t.Errorf("status = %d, esperado 400", status)
		}
	})

	t.Run("Create emergencia com nivel que nao e o padrao retorna 400", func(t *testing.T) {
		nivelNaoEmergencia := c.criarNivel(2, 10)
		status, _ := c.e.request(http.MethodPost, "/api/v1/usuarios/"+c.vigia.ID.String()+"/senhas", c.adminToken, map[string]any{
			"tipo":                   "emergencia",
			"codigo":                 "1111",
			"nivel_escalonamento_id": nivelNaoEmergencia.String(),
		})
		if status != http.StatusBadRequest {
			t.Errorf("status = %d, esperado 400 (emergencia so aceita nivel padrao)", status)
		}
	})

	t.Run("Create ok com nivel de escalonamento retorna 400", func(t *testing.T) {
		nivelID := c.criarNivel(2, 10)
		status, _ := c.e.request(http.MethodPost, "/api/v1/usuarios/"+c.vigia.ID.String()+"/senhas", c.adminToken, map[string]any{
			"tipo":                   "ok",
			"codigo":                 "1111",
			"nivel_escalonamento_id": nivelID.String(),
		})
		if status != http.StatusBadRequest {
			t.Errorf("status = %d, esperado 400 (ok nao aceita nivel)", status)
		}
	})

	t.Run("Create com nivel_escalonamento_id inexistente retorna 400", func(t *testing.T) {
		fakeID := uuid.New()
		status, _ := c.e.request(http.MethodPost, "/api/v1/usuarios/"+c.vigia.ID.String()+"/senhas", c.adminToken, map[string]any{
			"tipo":                   "customizada",
			"codigo":                 "7777",
			"descricao":              "teste nivel invalido",
			"nivel_escalonamento_id": fakeID.String(),
		})
		if status != http.StatusBadRequest {
			t.Errorf("status = %d, esperado 400", status)
		}
	})

	t.Run("Create customizada com nivel ja vinculado retorna 409", func(t *testing.T) {
		nivelID := c.criarNivel(2, 10)
		desc1 := "pin 1"
		c.criarSenhaVigia(c.vigia.ID, "customizada", "8100", &desc1, toUUIDPtr(nivelID.String()))

		desc2 := "pin 2"
		status, _ := c.e.request(http.MethodPost, "/api/v1/usuarios/"+c.vigia.ID.String()+"/senhas", c.adminToken, map[string]any{
			"tipo":                   "customizada",
			"codigo":                 "8101",
			"descricao":              desc2,
			"nivel_escalonamento_id": nivelID.String(),
		})
		if status != http.StatusConflict {
			t.Errorf("status = %d, esperado 409", status)
		}
	})

	t.Run("Update altera codigo de senha", func(t *testing.T) {
		nivelID := c.criarNivel(2, 10)
		desc := "pin para update"
		senha := c.criarSenhaVigia(c.vigia.ID, "customizada", "8888", &desc, toUUIDPtr(nivelID.String()))

		var atualizada map[string]any
		novoCodigo := "3333"
		c.e.reqJSON(http.MethodPut, "/api/v1/usuarios/"+c.vigia.ID.String()+"/senhas/"+senha.ID.String(), c.adminToken, map[string]any{
			"codigo": novoCodigo,
		}, http.StatusOK, &atualizada)

		if atualizada["codigo"] != novoCodigo {
			t.Errorf("codigo = %v, esperado %s", atualizada["codigo"], novoCodigo)
		}
	})

	t.Run("Update codigo duplicado retorna 400", func(t *testing.T) {
		nivel1 := c.criarNivel(2, 10)
		nivel2 := c.criarNivel(3, 15)
		desc1 := "pin 1"
		senha1 := c.criarSenhaVigia(c.vigia.ID, "customizada", "2100", &desc1, toUUIDPtr(nivel1.String()))
		desc2 := "pin 2"
		c.criarSenhaVigia(c.vigia.ID, "customizada", "2101", &desc2, toUUIDPtr(nivel2.String()))

		status, _ := c.e.request(http.MethodPut, "/api/v1/usuarios/"+c.vigia.ID.String()+"/senhas/"+senha1.ID.String(), c.adminToken, map[string]any{
			"codigo": "2101",
		})
		if status != http.StatusBadRequest {
			t.Errorf("status = %d, esperado 400", status)
		}
	})

	t.Run("Update nao altera codigo para o proprio valor (idempotente)", func(t *testing.T) {
		nivelID := c.criarNivel(2, 10)
		desc := "pin 3"
		senha := c.criarSenhaVigia(c.vigia.ID, "customizada", "3100", &desc, toUUIDPtr(nivelID.String()))

		var atualizada map[string]any
		c.e.reqJSON(http.MethodPut, "/api/v1/usuarios/"+c.vigia.ID.String()+"/senhas/"+senha.ID.String(), c.adminToken, map[string]any{
			"codigo": "3100",
		}, http.StatusOK, &atualizada)
	})

	t.Run("Update ok/emergencia rejeita campos nao editaveis", func(t *testing.T) {
		var senhas []map[string]any
		c.e.reqJSON(http.MethodGet, "/api/v1/usuarios/"+c.vigia.ID.String()+"/senhas", c.adminToken, nil, http.StatusOK, &senhas)

		var okID string
		for _, s := range senhas {
			if s["tipo"] == "ok" {
				okID = s["id"].(string)
				break
			}
		}

		desc := "tentativa"
		status, _ := c.e.request(http.MethodPut, "/api/v1/usuarios/"+c.vigia.ID.String()+"/senhas/"+okID, c.adminToken, map[string]any{
			"descricao": desc,
		})
		if status != http.StatusBadRequest {
			t.Errorf("status = %d, esperado 400", status)
		}
	})

	t.Run("Update emergencia rejeita alterar nivel_escalonamento_id", func(t *testing.T) {
		var senhas []map[string]any
		c.e.reqJSON(http.MethodGet, "/api/v1/usuarios/"+c.vigia.ID.String()+"/senhas", c.adminToken, nil, http.StatusOK, &senhas)

		var emergenciaID string
		for _, s := range senhas {
			if s["tipo"] == "emergencia" {
				emergenciaID = s["id"].(string)
				break
			}
		}

		nivelID := c.criarNivel(2, 10)
		status, _ := c.e.request(http.MethodPut, "/api/v1/usuarios/"+c.vigia.ID.String()+"/senhas/"+emergenciaID, c.adminToken, map[string]any{
			"nivel_escalonamento_id": nivelID.String(),
		})
		if status != http.StatusBadRequest {
			t.Errorf("status = %d, esperado 400", status)
		}
	})

	t.Run("Update altera descricao e nivel de senha customizada", func(t *testing.T) {
		nivelOriginal := c.criarNivel(2, 10)
		desc := "original"
		senha := c.criarSenhaVigia(c.vigia.ID, "customizada", "4100", &desc, toUUIDPtr(nivelOriginal.String()))

		nivelNovo := c.criarNivel(3, 15)
		novaDesc := "atualizada"
		var atualizada map[string]any
		c.e.reqJSON(http.MethodPut, "/api/v1/usuarios/"+c.vigia.ID.String()+"/senhas/"+senha.ID.String(), c.adminToken, map[string]any{
			"descricao":              novaDesc,
			"nivel_escalonamento_id": nivelNovo.String(),
		}, http.StatusOK, &atualizada)

		if atualizada["descricao"] != novaDesc {
			t.Errorf("descricao = %v, esperado %s", atualizada["descricao"], novaDesc)
		}
	})

	t.Run("Update customizada com nivel ja vinculado retorna 409", func(t *testing.T) {
		nivel1 := c.criarNivel(2, 10)
		nivel2 := c.criarNivel(3, 15)
		desc1 := "pin A"
		c.criarSenhaVigia(c.vigia.ID, "customizada", "6100", &desc1, toUUIDPtr(nivel1.String()))
		desc2 := "pin B"
		senhaB := c.criarSenhaVigia(c.vigia.ID, "customizada", "6101", &desc2, toUUIDPtr(nivel2.String()))

		status, _ := c.e.request(http.MethodPut, "/api/v1/usuarios/"+c.vigia.ID.String()+"/senhas/"+senhaB.ID.String(), c.adminToken, map[string]any{
			"nivel_escalonamento_id": nivel1.String(),
		})
		if status != http.StatusConflict {
			t.Errorf("status = %d, esperado 409", status)
		}
	})

	t.Run("Update senha inexistente retorna 404", func(t *testing.T) {
		fakeID := uuid.New()
		status, _ := c.e.request(http.MethodPut, "/api/v1/usuarios/"+c.vigia.ID.String()+"/senhas/"+fakeID.String(), c.adminToken, map[string]any{
			"codigo": "0000",
		})
		if status != http.StatusNotFound {
			t.Errorf("status = %d, esperado 404", status)
		}
	})

	t.Run("Delete remove senha customizada", func(t *testing.T) {
		nivelID := c.criarNivel(2, 10)
		desc := "para deletar"
		senha := c.criarSenhaVigia(c.vigia.ID, "customizada", "7100", &desc, toUUIDPtr(nivelID.String()))

		status, _ := c.e.request(http.MethodDelete, "/api/v1/usuarios/"+c.vigia.ID.String()+"/senhas/"+senha.ID.String(), c.adminToken, nil)
		if status != http.StatusOK {
			t.Errorf("status = %d, esperado 200", status)
		}

		status, _ = c.e.request(http.MethodGet, "/api/v1/usuarios/"+c.vigia.ID.String()+"/senhas", c.adminToken, nil)
		if status != http.StatusOK {
			t.Fatal("list apos delete falhou")
		}
	})

	t.Run("Delete senha ok retorna 409", func(t *testing.T) {
		var senhas []map[string]any
		c.e.reqJSON(http.MethodGet, "/api/v1/usuarios/"+c.vigia.ID.String()+"/senhas", c.adminToken, nil, http.StatusOK, &senhas)

		var okID string
		for _, s := range senhas {
			if s["tipo"] == "ok" {
				okID = s["id"].(string)
				break
			}
		}

		status, _ := c.e.request(http.MethodDelete, "/api/v1/usuarios/"+c.vigia.ID.String()+"/senhas/"+okID, c.adminToken, nil)
		if status != http.StatusConflict {
			t.Errorf("status = %d, esperado 409", status)
		}
	})

	t.Run("Delete senha emergencia retorna 409", func(t *testing.T) {
		var senhas []map[string]any
		c.e.reqJSON(http.MethodGet, "/api/v1/usuarios/"+c.vigia.ID.String()+"/senhas", c.adminToken, nil, http.StatusOK, &senhas)

		var emergenciaID string
		for _, s := range senhas {
			if s["tipo"] == "emergencia" {
				emergenciaID = s["id"].(string)
				break
			}
		}

		status, _ := c.e.request(http.MethodDelete, "/api/v1/usuarios/"+c.vigia.ID.String()+"/senhas/"+emergenciaID, c.adminToken, nil)
		if status != http.StatusConflict {
			t.Errorf("status = %d, esperado 409", status)
		}
	})

	t.Run("Delete senha inexistente retorna 404", func(t *testing.T) {
		fakeID := uuid.New()
		status, _ := c.e.request(http.MethodDelete, "/api/v1/usuarios/"+c.vigia.ID.String()+"/senhas/"+fakeID.String(), c.adminToken, nil)
		if status != http.StatusNotFound {
			t.Errorf("status = %d, esperado 404", status)
		}
	})

	t.Run("CRUD rejeita acesso de vigia (somente admin)", func(t *testing.T) {
		status, _ := c.e.request(http.MethodPost, "/api/v1/usuarios/"+c.vigia.ID.String()+"/senhas", c.vigiaToken, map[string]any{
			"tipo": "customizada", "codigo": "1111", "descricao": "tentativa vigia",
		})
		if status != http.StatusForbidden {
			t.Errorf("status = %d, esperado 403", status)
		}
	})

	t.Run("Create codigo com menos de 4 digitos retorna 422", func(t *testing.T) {
		nivelID := c.criarNivel(2, 10)
		status, _ := c.e.request(http.MethodPost, "/api/v1/usuarios/"+c.vigia.ID.String()+"/senhas", c.adminToken, map[string]any{
			"tipo": "customizada", "codigo": "12", "descricao": "curto",
			"nivel_escalonamento_id": nivelID.String(),
		})
		if status != http.StatusUnprocessableEntity {
			t.Errorf("status = %d, esperado 422", status)
		}
	})

	t.Run("Create codigo nao numerico retorna 422", func(t *testing.T) {
		nivelID := c.criarNivel(2, 10)
		status, _ := c.e.request(http.MethodPost, "/api/v1/usuarios/"+c.vigia.ID.String()+"/senhas", c.adminToken, map[string]any{
			"tipo": "customizada", "codigo": "abcd", "descricao": "nao numerico",
			"nivel_escalonamento_id": nivelID.String(),
		})
		if status != http.StatusUnprocessableEntity {
			t.Errorf("status = %d, esperado 422", status)
		}
	})
}

func toUUIDPtr(s string) *uuid.UUID {
	id := uuid.MustParse(s)
	return &id
}
