package handler

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"

	"github.com/guardpoint/guardpoint-server/internal/model"
	"github.com/guardpoint/guardpoint-server/internal/service"
)

type EscalaHandler struct {
	escalaService *service.EscalaService
	validate      *validator.Validate
}

func NewEscalaHandler(escalaService *service.EscalaService) *EscalaHandler {
	return &EscalaHandler{
		escalaService: escalaService,
		validate:      validator.New(),
	}
}

// Create godoc
// @Summary      Cria uma escala semanal (admin/supervisor)
// @Tags         escalas
// @Param        request body model.CreateEscalaRequest true "Dados da escala"
// @Success      201 {object} model.Escala
// @Failure      400 {object} map[string]string
// @Failure      500 {object} map[string]string
// @Router       /escalas [post]
func (h *EscalaHandler) Create(w http.ResponseWriter, r *http.Request) {
	empresaID := GetEmpresaID(r.Context())

	parsedEmpresaID, err := uuid.Parse(empresaID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "empresa_id invalido")
		return
	}

	var req model.CreateEscalaRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "json invalido")
		return
	}

	if err := h.validate.Struct(req); err != nil {
		writeValidationError(w, err)
		return
	}

	esc, err := h.escalaService.Create(r.Context(), parsedEmpresaID, req)
	if err != nil {
		slog.Error("create escala failed", "error", err)
		writeError(w, http.StatusInternalServerError, "erro ao criar escala")
		return
	}

	writeJSON(w, http.StatusCreated, esc)
}

// CreateLote godoc
// @Summary      Cria escalas em lote (ate 7 dias) para um usuario/posto (admin/supervisor)
// @Tags         escalas
// @Param        request body model.CreateEscalaLoteRequest true "Dados do lote"
// @Success      201 {object} model.CreateEscalaLoteResponse
// @Failure      400 {object} map[string]string
// @Failure      500 {object} map[string]string
// @Router       /escalas/lote [post]
func (h *EscalaHandler) CreateLote(w http.ResponseWriter, r *http.Request) {
	empresaID := GetEmpresaID(r.Context())

	parsedEmpresaID, err := uuid.Parse(empresaID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "empresa_id invalido")
		return
	}

	var req model.CreateEscalaLoteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "json invalido")
		return
	}

	if err := h.validate.Struct(req); err != nil {
		writeValidationError(w, err)
		return
	}

	escalas, err := h.escalaService.CreateLote(r.Context(), parsedEmpresaID, req)
	if err != nil {
		slog.Error("create escalas lote failed", "error", err)
		writeError(w, http.StatusInternalServerError, "erro ao criar escalas em lote")
		return
	}

	writeJSON(w, http.StatusCreated, escalas)
}

// GetByID godoc
// @Summary      Busca uma escala pelo ID (admin/supervisor)
// @Tags         escalas
// @Param        id path string true "ID da escala (uuid)"
// @Success      200 {object} model.Escala
// @Failure      400 {object} map[string]string
// @Failure      404 {object} map[string]string
// @Router       /escalas/{id} [get]
func (h *EscalaHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	empresaID := GetEmpresaID(r.Context())
	id := chi.URLParam(r, "id")

	parsedEmpresaID, err := uuid.Parse(empresaID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "empresa_id invalido")
		return
	}
	parsedID, err := uuid.Parse(id)
	if err != nil {
		writeError(w, http.StatusBadRequest, "id invalido")
		return
	}

	esc, err := h.escalaService.GetByID(r.Context(), parsedEmpresaID, parsedID)
	if err != nil {
		writeError(w, http.StatusNotFound, "escala nao encontrada")
		return
	}

	writeJSON(w, http.StatusOK, esc)
}

// List godoc
// @Summary      Lista escalas com filtros e paginacao (admin/supervisor)
// @Tags         escalas
// @Param        usuario_id query string false "ID do usuario (uuid)"
// @Param        posto_id query string false "ID do posto (uuid)"
// @Param        ativos query string false "Filtra por ativo (true/false)"
// @Param        limit query int false "Limite de itens (max 100)"
// @Param        offset query int false "Offset da paginacao"
// @Success      200 {object} map[string]interface{}
// @Failure      400 {object} map[string]string
// @Failure      500 {object} map[string]string
// @Router       /escalas [get]
func (h *EscalaHandler) List(w http.ResponseWriter, r *http.Request) {
	empresaID := GetEmpresaID(r.Context())

	parsedEmpresaID, err := uuid.Parse(empresaID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "empresa_id invalido")
		return
	}

	limit, offset := parsePagination(r)

	var apenasAtivos *bool
	if v := r.URL.Query().Get("ativos"); v == "true" {
		t := true
		apenasAtivos = &t
	} else if v == "false" {
		f := false
		apenasAtivos = &f
	}

	filter := model.EscalaFilter{
		UsuarioID: r.URL.Query().Get("usuario_id"),
		PostoID:   r.URL.Query().Get("posto_id"),
		Ativo:     apenasAtivos,
		Limit:     limit,
		Offset:    offset,
	}

	escalas, total, err := h.escalaService.List(r.Context(), parsedEmpresaID, filter)
	if err != nil {
		slog.Error("list escalas failed", "error", err)
		writeError(w, http.StatusInternalServerError, "erro ao listar escalas")
		return
	}

	if escalas == nil {
		escalas = []model.Escala{}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"data":  escalas,
		"total": total,
	})
}

// Update godoc
// @Summary      Atualiza uma escala (admin/supervisor)
// @Tags         escalas
// @Param        id path string true "ID da escala (uuid)"
// @Param        request body model.UpdateEscalaRequest true "Campos a atualizar"
// @Success      200 {object} model.Escala
// @Failure      400 {object} map[string]string
// @Failure      404 {object} map[string]string
// @Failure      500 {object} map[string]string
// @Router       /escalas/{id} [put]
func (h *EscalaHandler) Update(w http.ResponseWriter, r *http.Request) {
	empresaID := GetEmpresaID(r.Context())
	id := chi.URLParam(r, "id")

	parsedEmpresaID, err := uuid.Parse(empresaID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "empresa_id invalido")
		return
	}
	parsedID, err := uuid.Parse(id)
	if err != nil {
		writeError(w, http.StatusBadRequest, "id invalido")
		return
	}

	var req model.UpdateEscalaRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "json invalido")
		return
	}

	if err := h.validate.Struct(req); err != nil {
		writeValidationError(w, err)
		return
	}

	esc, err := h.escalaService.Update(r.Context(), parsedEmpresaID, parsedID, req)
	if err != nil {
		if errors.Is(err, service.ErrEscalaNaoEncontrada) {
			writeError(w, http.StatusNotFound, "escala nao encontrada")
			return
		}
		slog.Error("update escala failed", "error", err)
		writeError(w, http.StatusInternalServerError, "erro ao atualizar escala")
		return
	}

	writeJSON(w, http.StatusOK, esc)
}

// Delete godoc
// @Summary      Desativa uma escala (admin/supervisor)
// @Tags         escalas
// @Param        id path string true "ID da escala (uuid)"
// @Success      200 {object} map[string]string
// @Failure      400 {object} map[string]string
// @Failure      404 {object} map[string]string
// @Failure      500 {object} map[string]string
// @Router       /escalas/{id} [delete]
func (h *EscalaHandler) Delete(w http.ResponseWriter, r *http.Request) {
	empresaID := GetEmpresaID(r.Context())
	id := chi.URLParam(r, "id")

	parsedEmpresaID, err := uuid.Parse(empresaID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "empresa_id invalido")
		return
	}
	parsedID, err := uuid.Parse(id)
	if err != nil {
		writeError(w, http.StatusBadRequest, "id invalido")
		return
	}

	if err := h.escalaService.Delete(r.Context(), parsedEmpresaID, parsedID); err != nil {
		if errors.Is(err, service.ErrEscalaNaoEncontrada) {
			writeError(w, http.StatusNotFound, "escala nao encontrada")
			return
		}
		slog.Error("delete escala failed", "error", err)
		writeError(w, http.StatusInternalServerError, "erro ao excluir escala")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "escala desativada"})
}
