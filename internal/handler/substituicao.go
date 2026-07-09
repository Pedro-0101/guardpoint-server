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

type SubstituicaoHandler struct {
	service  *service.SubstituicaoService
	validate *validator.Validate
}

func NewSubstituicaoHandler(service *service.SubstituicaoService) *SubstituicaoHandler {
	return &SubstituicaoHandler{
		service:  service,
		validate: validator.New(),
	}
}

// Create godoc
// @Summary      Cria uma substituicao de vigia (admin/supervisor)
// @Tags         substituicoes
// @Param        request body model.CreateSubstituicaoRequest true "Dados da substituicao"
// @Success      201 {object} model.Substituicao
// @Failure      400 {object} model.ErrorResponse
// @Router       /substituicoes [post]
func (h *SubstituicaoHandler) Create(w http.ResponseWriter, r *http.Request) {
	empresaID := GetEmpresaID(r.Context())

	parsedEmpresaID, err := uuid.Parse(empresaID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "empresa_id invalido")
		return
	}

	var req model.CreateSubstituicaoRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "json invalido")
		return
	}

	if err := h.validate.Struct(req); err != nil {
		writeValidationError(w, err)
		return
	}

	sub, err := h.service.Create(r.Context(), parsedEmpresaID, req)
	if err != nil {
		slog.Error("create substituicao failed", "error", err)
		writeError(w, http.StatusInternalServerError, "erro ao criar substituicao")
		return
	}

	writeJSON(w, http.StatusCreated, sub)
}

// GetByID godoc
// @Summary      Busca uma substituicao pelo ID (admin/supervisor)
// @Tags         substituicoes
// @Param        id path string true "ID da substituicao (uuid)"
// @Success      200 {object} model.Substituicao
// @Failure      404 {object} model.ErrorResponse
// @Router       /substituicoes/{id} [get]
func (h *SubstituicaoHandler) GetByID(w http.ResponseWriter, r *http.Request) {
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

	sub, err := h.service.GetByID(r.Context(), parsedEmpresaID, parsedID)
	if err != nil {
		writeError(w, http.StatusNotFound, "substituicao nao encontrada")
		return
	}

	writeJSON(w, http.StatusOK, sub)
}

// List godoc
// @Summary      Lista substituicoes com filtros e paginacao (admin/supervisor)
// @Tags         substituicoes
// @Param        usuario_id query string false "ID do usuario substituto"
// @Param        posto_id query string false "ID do posto"
// @Param        data query string false "Data (YYYY-MM-DD)"
// @Param        ativos query string false "Filtra por ativo (true/false)"
// @Param        limit query int false "Limite de itens (max 100)"
// @Param        offset query int false "Offset da paginacao"
// @Success      200 {object} model.SubstituicaoListResponse
// @Router       /substituicoes [get]
func (h *SubstituicaoHandler) List(w http.ResponseWriter, r *http.Request) {
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

	filter := model.SubstituicaoFilter{
		UsuarioID: r.URL.Query().Get("usuario_id"),
		PostoID:   r.URL.Query().Get("posto_id"),
		Data:      r.URL.Query().Get("data"),
		Ativo:     apenasAtivos,
		Limit:     limit,
		Offset:    offset,
	}

	subs, total, err := h.service.List(r.Context(), parsedEmpresaID, filter)
	if err != nil {
		slog.Error("list substituicoes failed", "error", err)
		writeError(w, http.StatusInternalServerError, "erro ao listar substituicoes")
		return
	}

	if subs == nil {
		subs = []model.Substituicao{}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"data":  subs,
		"total": total,
	})
}

// Update godoc
// @Summary      Atualiza uma substituicao (admin/supervisor)
// @Tags         substituicoes
// @Param        id path string true "ID da substituicao (uuid)"
// @Param        request body model.UpdateSubstituicaoRequest true "Campos a atualizar"
// @Success      200 {object} model.Substituicao
// @Failure      404 {object} model.ErrorResponse
// @Router       /substituicoes/{id} [put]
func (h *SubstituicaoHandler) Update(w http.ResponseWriter, r *http.Request) {
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

	var req model.UpdateSubstituicaoRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "json invalido")
		return
	}

	if err := h.validate.Struct(req); err != nil {
		writeValidationError(w, err)
		return
	}

	sub, err := h.service.Update(r.Context(), parsedEmpresaID, parsedID, req)
	if err != nil {
		if errors.Is(err, service.ErrSubstituicaoNaoEncontrada) {
			writeError(w, http.StatusNotFound, "substituicao nao encontrada")
			return
		}
		slog.Error("update substituicao failed", "error", err)
		writeError(w, http.StatusInternalServerError, "erro ao atualizar substituicao")
		return
	}

	writeJSON(w, http.StatusOK, sub)
}

// Delete godoc
// @Summary      Desativa uma substituicao (admin/supervisor)
// @Tags         substituicoes
// @Param        id path string true "ID da substituicao (uuid)"
// @Success      200 {object} model.MessageResponse
// @Failure      404 {object} model.ErrorResponse
// @Router       /substituicoes/{id} [delete]
func (h *SubstituicaoHandler) Delete(w http.ResponseWriter, r *http.Request) {
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

	if err := h.service.Delete(r.Context(), parsedEmpresaID, parsedID); err != nil {
		if errors.Is(err, service.ErrSubstituicaoNaoEncontrada) {
			writeError(w, http.StatusNotFound, "substituicao nao encontrada")
			return
		}
		slog.Error("delete substituicao failed", "error", err)
		writeError(w, http.StatusInternalServerError, "erro ao excluir substituicao")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "substituicao desativada"})
}
