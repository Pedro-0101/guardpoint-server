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

type SenhaVigiaHandler struct {
	service  *service.SenhaVigiaService
	validate *validator.Validate
}

func NewSenhaVigiaHandler(s *service.SenhaVigiaService) *SenhaVigiaHandler {
	return &SenhaVigiaHandler{
		service:  s,
		validate: validator.New(),
	}
}

// List godoc
// @Summary      Lista as senhas cadastradas para um vigia (somente admin)
// @Tags         usuarios
// @Param        id path string true "ID do usuario/vigia (uuid)"
// @Success      200 {array} model.SenhaVigia
// @Failure      400 {object} map[string]string
// @Failure      404 {object} map[string]string
// @Router       /usuarios/{id}/senhas [get]
func (h *SenhaVigiaHandler) List(w http.ResponseWriter, r *http.Request) {
	empresaID := GetEmpresaID(r.Context())
	usuarioID := chi.URLParam(r, "id")

	parsedEmpresaID, err := uuid.Parse(empresaID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "empresa_id invalido")
		return
	}
	parsedUsuarioID, err := uuid.Parse(usuarioID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "id invalido")
		return
	}

	senhas, err := h.service.List(r.Context(), parsedEmpresaID, parsedUsuarioID)
	if err != nil {
		if errors.Is(err, service.ErrUsuarioNaoPertenceAEmpresa) {
			writeError(w, http.StatusNotFound, "usuario nao encontrado")
			return
		}
		slog.Error("list senhas vigia failed", "error", err)
		writeError(w, http.StatusInternalServerError, "erro ao listar senhas")
		return
	}

	if senhas == nil {
		senhas = []model.SenhaVigia{}
	}

	writeJSON(w, http.StatusOK, senhas)
}

// Create godoc
// @Summary      Cria uma senha para um vigia (somente admin)
// @Tags         usuarios
// @Param        id path string true "ID do usuario/vigia (uuid)"
// @Param        request body model.CreateSenhaVigiaRequest true "Dados da senha"
// @Success      201 {object} model.SenhaVigia
// @Failure      400 {object} map[string]string
// @Failure      404 {object} map[string]string
// @Router       /usuarios/{id}/senhas [post]
func (h *SenhaVigiaHandler) Create(w http.ResponseWriter, r *http.Request) {
	empresaID := GetEmpresaID(r.Context())
	usuarioID := chi.URLParam(r, "id")

	parsedEmpresaID, err := uuid.Parse(empresaID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "empresa_id invalido")
		return
	}
	parsedUsuarioID, err := uuid.Parse(usuarioID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "id invalido")
		return
	}

	var req model.CreateSenhaVigiaRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "json invalido")
		return
	}

	if err := h.validate.Struct(req); err != nil {
		writeValidationError(w, err)
		return
	}

	senha, err := h.service.Create(r.Context(), parsedEmpresaID, parsedUsuarioID, req)
	if err != nil {
		if h.writeSenhaError(w, err) {
			return
		}
		slog.Error("create senha vigia failed", "error", err)
		writeError(w, http.StatusInternalServerError, "erro ao criar senha")
		return
	}

	writeJSON(w, http.StatusCreated, senha)
}

// Update godoc
// @Summary      Atualiza uma senha de um vigia (somente admin)
// @Tags         usuarios
// @Param        id path string true "ID do usuario/vigia (uuid)"
// @Param        senhaId path string true "ID da senha (uuid)"
// @Param        request body model.UpdateSenhaVigiaRequest true "Campos a atualizar"
// @Success      200 {object} model.SenhaVigia
// @Failure      400 {object} map[string]string
// @Failure      404 {object} map[string]string
// @Router       /usuarios/{id}/senhas/{senhaId} [put]
func (h *SenhaVigiaHandler) Update(w http.ResponseWriter, r *http.Request) {
	empresaID := GetEmpresaID(r.Context())
	usuarioID := chi.URLParam(r, "id")
	senhaID := chi.URLParam(r, "senhaId")

	parsedEmpresaID, err := uuid.Parse(empresaID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "empresa_id invalido")
		return
	}
	parsedUsuarioID, err := uuid.Parse(usuarioID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "id invalido")
		return
	}
	parsedSenhaID, err := uuid.Parse(senhaID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "senha id invalido")
		return
	}

	var req model.UpdateSenhaVigiaRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "json invalido")
		return
	}

	if err := h.validate.Struct(req); err != nil {
		writeValidationError(w, err)
		return
	}

	senha, err := h.service.Update(r.Context(), parsedEmpresaID, parsedUsuarioID, parsedSenhaID, req)
	if err != nil {
		if h.writeSenhaError(w, err) {
			return
		}
		slog.Error("update senha vigia failed", "error", err)
		writeError(w, http.StatusInternalServerError, "erro ao atualizar senha")
		return
	}

	writeJSON(w, http.StatusOK, senha)
}

// Delete godoc
// @Summary      Remove uma senha customizada de um vigia (somente admin)
// @Tags         usuarios
// @Param        id path string true "ID do usuario/vigia (uuid)"
// @Param        senhaId path string true "ID da senha (uuid)"
// @Success      200 {object} map[string]string
// @Failure      400 {object} map[string]string
// @Failure      404 {object} map[string]string
// @Failure      409 {object} map[string]string
// @Router       /usuarios/{id}/senhas/{senhaId} [delete]
func (h *SenhaVigiaHandler) Delete(w http.ResponseWriter, r *http.Request) {
	empresaID := GetEmpresaID(r.Context())
	usuarioID := chi.URLParam(r, "id")
	senhaID := chi.URLParam(r, "senhaId")

	parsedEmpresaID, err := uuid.Parse(empresaID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "empresa_id invalido")
		return
	}
	parsedUsuarioID, err := uuid.Parse(usuarioID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "id invalido")
		return
	}
	parsedSenhaID, err := uuid.Parse(senhaID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "senha id invalido")
		return
	}

	if err := h.service.Delete(r.Context(), parsedEmpresaID, parsedUsuarioID, parsedSenhaID); err != nil {
		if h.writeSenhaError(w, err) {
			return
		}
		slog.Error("delete senha vigia failed", "error", err)
		writeError(w, http.StatusInternalServerError, "erro ao remover senha")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "senha removida"})
}

// writeSenhaError mapeia os erros sentinela do SenhaVigiaService para o status HTTP
// correspondente. Retorna true se o erro foi tratado (resposta ja escrita).
func (h *SenhaVigiaHandler) writeSenhaError(w http.ResponseWriter, err error) bool {
	switch {
	case errors.Is(err, service.ErrUsuarioNaoPertenceAEmpresa):
		writeError(w, http.StatusNotFound, "usuario nao encontrado")
		return true
	case errors.Is(err, service.ErrSenhaNaoEncontrada):
		writeError(w, http.StatusNotFound, "senha nao encontrada")
		return true
	case errors.Is(err, service.ErrSenhaCodigoDuplicado):
		writeError(w, http.StatusBadRequest, "codigo ja usado por outra senha deste vigia")
		return true
	case errors.Is(err, service.ErrSenhaTipoJaExiste):
		writeError(w, http.StatusBadRequest, "vigia ja possui uma senha deste tipo")
		return true
	case errors.Is(err, service.ErrSenhaCampoNaoEditavelParaTipo):
		writeError(w, http.StatusBadRequest, "campo nao editavel para este tipo de senha")
		return true
	case errors.Is(err, service.ErrSenhaObrigatoriaNaoRemovivel):
		writeError(w, http.StatusConflict, "senha obrigatoria nao pode ser removida")
		return true
	}
	return false
}
