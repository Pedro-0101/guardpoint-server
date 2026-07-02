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

type UsuarioHandler struct {
	usuarioService *service.UsuarioService
	validate       *validator.Validate
}

func NewUsuarioHandler(usuarioService *service.UsuarioService) *UsuarioHandler {
	return &UsuarioHandler{
		usuarioService: usuarioService,
		validate:       validator.New(),
	}
}

func (h *UsuarioHandler) List(w http.ResponseWriter, r *http.Request) {
	empresaID := GetEmpresaID(r.Context())

	parsedEmpresaID, err := uuid.Parse(empresaID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "empresa_id invalido")
		return
	}

	usuarios, err := h.usuarioService.List(r.Context(), parsedEmpresaID)
	if err != nil {
		slog.Error("list usuarios failed", "error", err)
		writeError(w, http.StatusInternalServerError, "erro ao listar usuarios")
		return
	}

	if usuarios == nil {
		usuarios = []model.UsuarioResponse{}
	}

	writeJSON(w, http.StatusOK, usuarios)
}

func (h *UsuarioHandler) GetByID(w http.ResponseWriter, r *http.Request) {
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

	usuario, err := h.usuarioService.GetByID(r.Context(), parsedEmpresaID, parsedID)
	if err != nil {
		writeError(w, http.StatusNotFound, "usuario nao encontrado")
		return
	}

	writeJSON(w, http.StatusOK, usuario)
}

func (h *UsuarioHandler) Create(w http.ResponseWriter, r *http.Request) {
	empresaID := GetEmpresaID(r.Context())

	parsedEmpresaID, err := uuid.Parse(empresaID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "empresa_id invalido")
		return
	}

	var req model.CreateUsuarioRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "json invalido")
		return
	}

	if err := h.validate.Struct(req); err != nil {
		writeValidationError(w, err)
		return
	}

	usuario, err := h.usuarioService.Create(r.Context(), parsedEmpresaID, req)
	if err != nil {
		if errors.Is(err, service.ErrEmailAlreadyExists) {
			writeError(w, http.StatusConflict, "email ja cadastrado")
			return
		}
		slog.Error("create usuario failed", "error", err)
		writeError(w, http.StatusInternalServerError, "erro ao criar usuario")
		return
	}

	writeJSON(w, http.StatusCreated, usuario)
}

func (h *UsuarioHandler) Update(w http.ResponseWriter, r *http.Request) {
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

	var req model.UpdateUsuarioRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "json invalido")
		return
	}

	if err := h.validate.Struct(req); err != nil {
		writeValidationError(w, err)
		return
	}

	usuario, err := h.usuarioService.Update(r.Context(), parsedEmpresaID, parsedID, req)
	if err != nil {
		slog.Error("update usuario failed", "error", err)
		writeError(w, http.StatusInternalServerError, "erro ao atualizar usuario")
		return
	}

	writeJSON(w, http.StatusOK, usuario)
}

func (h *UsuarioHandler) Delete(w http.ResponseWriter, r *http.Request) {
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

	if err := h.usuarioService.Deactivate(r.Context(), parsedEmpresaID, parsedID); err != nil {
		slog.Error("delete usuario failed", "error", err)
		writeError(w, http.StatusInternalServerError, "erro ao desativar usuario")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "usuario desativado"})
}
