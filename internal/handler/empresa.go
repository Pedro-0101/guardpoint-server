package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"

	"github.com/guardpoint/guardpoint-server/internal/model"
	"github.com/guardpoint/guardpoint-server/internal/service"
)

type EmpresaHandler struct {
	empresaService *service.EmpresaService
	validate       *validator.Validate
}

func NewEmpresaHandler(empresaService *service.EmpresaService) *EmpresaHandler {
	return &EmpresaHandler{empresaService: empresaService, validate: validator.New()}
}

// Get godoc
// @Summary      Retorna os dados/configuracoes da empresa do usuario logado (somente admin)
// @Tags         empresa
// @Success      200 {object} model.Empresa
// @Failure      400 {object} map[string]string
// @Failure      500 {object} map[string]string
// @Router       /empresa [get]
func (h *EmpresaHandler) Get(w http.ResponseWriter, r *http.Request) {
	empresaID, err := uuid.Parse(GetEmpresaID(r.Context()))
	if err != nil {
		writeError(w, http.StatusBadRequest, "empresa_id invalido")
		return
	}

	empresa, err := h.empresaService.Get(r.Context(), empresaID)
	if err != nil {
		slog.Error("get empresa failed", "error", err)
		writeError(w, http.StatusInternalServerError, "erro ao carregar empresa")
		return
	}

	writeJSON(w, http.StatusOK, empresa)
}

// Update godoc
// @Summary      Atualiza nome e preferencias da empresa do usuario logado (somente admin)
// @Tags         empresa
// @Param        request body model.UpdateEmpresaRequest true "Campos a atualizar"
// @Success      200 {object} model.Empresa
// @Failure      400 {object} map[string]string
// @Failure      500 {object} map[string]string
// @Router       /empresa [put]
func (h *EmpresaHandler) Update(w http.ResponseWriter, r *http.Request) {
	empresaID, err := uuid.Parse(GetEmpresaID(r.Context()))
	if err != nil {
		writeError(w, http.StatusBadRequest, "empresa_id invalido")
		return
	}

	var req model.UpdateEmpresaRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "json invalido")
		return
	}

	if err := h.validate.Struct(req); err != nil {
		writeValidationError(w, err)
		return
	}

	empresa, err := h.empresaService.Update(r.Context(), empresaID, req)
	if err != nil {
		slog.Error("update empresa failed", "error", err)
		writeError(w, http.StatusInternalServerError, "erro ao atualizar empresa")
		return
	}

	writeJSON(w, http.StatusOK, empresa)
}
