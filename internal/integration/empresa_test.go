//go:build integration

package integration

import (
	"net/http"
	"testing"

	"github.com/guardpoint/guardpoint-server/internal/model"
)

func TestGetEmpresa(t *testing.T) {
	c := novoCenario(t)

	t.Run("admin recebe os dados da propria empresa", func(t *testing.T) {
		var emp model.Empresa
		c.e.reqJSON(http.MethodGet, "/api/v1/empresa", c.adminToken, nil, http.StatusOK, &emp)

		if emp.ID != c.empresa.ID {
			t.Errorf("id = %v, esperado %v", emp.ID, c.empresa.ID)
		}
		if emp.Nome != c.empresa.Nome {
			t.Errorf("nome = %q, esperado %q", emp.Nome, c.empresa.Nome)
		}
		if !emp.AlertaSonoro {
			t.Error("alerta_sonoro deveria comecar true (default da migration)")
		}
	})

	t.Run("vigia nao pode acessar", func(t *testing.T) {
		status, _ := c.e.request(http.MethodGet, "/api/v1/empresa", c.vigiaToken, nil)
		if status != http.StatusForbidden {
			t.Errorf("status = %d, esperado 403", status)
		}
	})
}

func TestUpdateEmpresa(t *testing.T) {
	c := novoCenario(t)

	t.Run("admin atualiza nome e alerta_sonoro", func(t *testing.T) {
		var emp model.Empresa
		c.e.reqJSON(http.MethodPut, "/api/v1/empresa", c.adminToken, map[string]any{
			"nome":          "Empresa Renomeada Ltda",
			"alerta_sonoro": false,
		}, http.StatusOK, &emp)

		if emp.Nome != "Empresa Renomeada Ltda" {
			t.Errorf("nome = %q, esperado atualizado", emp.Nome)
		}
		if emp.AlertaSonoro {
			t.Error("alerta_sonoro deveria ser false apos update")
		}
		if emp.CNPJ != c.empresa.CNPJ {
			t.Errorf("cnpj mudou: %q, esperado %q (nao editavel por esta rota)", emp.CNPJ, c.empresa.CNPJ)
		}
	})

	t.Run("nome vazio e rejeitado", func(t *testing.T) {
		status, _ := c.e.request(http.MethodPut, "/api/v1/empresa", c.adminToken, map[string]any{
			"nome":          "",
			"alerta_sonoro": true,
		})
		if status != http.StatusUnprocessableEntity {
			t.Errorf("status = %d, esperado 422", status)
		}
	})

	t.Run("vigia nao pode atualizar", func(t *testing.T) {
		status, _ := c.e.request(http.MethodPut, "/api/v1/empresa", c.vigiaToken, map[string]any{
			"nome":          "Tentativa Vigia",
			"alerta_sonoro": true,
		})
		if status != http.StatusForbidden {
			t.Errorf("status = %d, esperado 403", status)
		}
	})
}
