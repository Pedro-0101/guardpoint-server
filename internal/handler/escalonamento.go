package handler

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-playground/validator/v10"

	"github.com/guardpoint/guardpoint-server/internal/middleware"
	"github.com/guardpoint/guardpoint-server/internal/model"
	"github.com/guardpoint/guardpoint-server/internal/service"
)

type EscalonamentoHandler struct {
	service  *service.EscalonamentoService
	validate *validator.Validate
}

func NewEscalonamentoHandler(svc *service.EscalonamentoService) *EscalonamentoHandler {
	return &EscalonamentoHandler{
		service:  svc,
		validate: validator.New(),
	}
}

func (h *EscalonamentoHandler) List(w http.ResponseWriter, r *http.Request) {
	empresaID := middleware.GetEmpresaID(r.Context())

	configs, err := h.service.ListEscalonamentos(r.Context(), empresaID)
	if err != nil {
		slog.Error("list escalonamento failed", "error", err)
		writeError(w, http.StatusInternalServerError, "erro ao carregar configs")
		return
	}

	writeJSON(w, http.StatusOK, configs)
}

func (h *EscalonamentoHandler) Create(w http.ResponseWriter, r *http.Request) {
	empresaID := middleware.GetEmpresaID(r.Context())

	var req model.CreateConfigEscalonamentoRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "json invalido")
		return
	}

	if err := h.validate.Struct(req); err != nil {
		writeValidationError(w, err)
		return
	}

	config, err := h.service.CreateEscalonamento(r.Context(), empresaID, req)
	if err != nil {
		if errors.Is(err, service.ErrUsuarioNaoPertenceAEmpresa) {
			writeError(w, http.StatusBadRequest, "usuario_id invalido para esta empresa")
			return
		}
		if errors.Is(err, service.ErrUsuarioNaoAdminOuSupervisor) {
			writeError(w, http.StatusBadRequest, "apenas administradores ou supervisores podem ser destinatarios")
			return
		}
		slog.Error("create escalonamento failed", "error", err)
		writeError(w, http.StatusInternalServerError, "erro ao criar configuracao")
		return
	}

	writeJSON(w, http.StatusCreated, config)
}

func (h *EscalonamentoHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	empresaID := middleware.GetEmpresaID(r.Context())
	configID := chi.URLParam(r, "id")

	config, err := h.service.GetEscalonamentoByID(r.Context(), empresaID, configID)
	if err != nil {
		if errors.Is(err, service.ErrEscalonamentoNaoEncontrado) {
			writeError(w, http.StatusNotFound, "config nao encontrada")
			return
		}
		slog.Error("get escalonamento failed", "error", err)
		writeError(w, http.StatusInternalServerError, "erro ao carregar configuracao")
		return
	}

	writeJSON(w, http.StatusOK, config)
}

func (h *EscalonamentoHandler) Update(w http.ResponseWriter, r *http.Request) {
	empresaID := middleware.GetEmpresaID(r.Context())
	configID := chi.URLParam(r, "id")

	var req model.UpdateConfigEscalonamentoRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "json invalido")
		return
	}

	if err := h.validate.Struct(req); err != nil {
		writeValidationError(w, err)
		return
	}

	config, err := h.service.UpdateEscalonamento(r.Context(), empresaID, configID, req)
	if err != nil {
		if errors.Is(err, service.ErrEscalonamentoNaoEncontrado) {
			writeError(w, http.StatusNotFound, "config nao encontrada")
			return
		}
		if errors.Is(err, service.ErrEscalonamentoSistemaNaoEditavel) {
			writeError(w, http.StatusBadRequest, "config do sistema nao pode ser alterada")
			return
		}
		if errors.Is(err, service.ErrUsuarioNaoPertenceAEmpresa) {
			writeError(w, http.StatusBadRequest, "usuario_id invalido para esta empresa")
			return
		}
		if errors.Is(err, service.ErrUsuarioNaoAdminOuSupervisor) {
			writeError(w, http.StatusBadRequest, "apenas administradores ou supervisores podem ser destinatarios")
			return
		}
		slog.Error("update escalonamento failed", "error", err)
		writeError(w, http.StatusInternalServerError, "erro ao atualizar configuracao")
		return
	}

	writeJSON(w, http.StatusOK, config)
}

func (h *EscalonamentoHandler) UpdateUsuarios(w http.ResponseWriter, r *http.Request) {
	empresaID := middleware.GetEmpresaID(r.Context())
	configID := chi.URLParam(r, "id")

	var req model.UpdateConfigEscalonamentoUsuariosRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "json invalido")
		return
	}

	if err := h.validate.Struct(req); err != nil {
		writeValidationError(w, err)
		return
	}

	config, err := h.service.UpdateEscalonamentoUsuarios(r.Context(), empresaID, configID, req)
	if err != nil {
		if errors.Is(err, service.ErrEscalonamentoNaoEncontrado) {
			writeError(w, http.StatusNotFound, "config nao encontrada")
			return
		}
		if errors.Is(err, service.ErrUsuarioNaoPertenceAEmpresa) {
			writeError(w, http.StatusBadRequest, "usuario_id invalido para esta empresa")
			return
		}
		if errors.Is(err, service.ErrUsuarioNaoAdminOuSupervisor) {
			writeError(w, http.StatusBadRequest, "apenas administradores ou supervisores podem ser destinatarios")
			return
		}
		slog.Error("update escalonamento usuarios failed", "error", err)
		writeError(w, http.StatusInternalServerError, "erro ao atualizar destinatarios")
		return
	}

	writeJSON(w, http.StatusOK, config)
}

func (h *EscalonamentoHandler) Delete(w http.ResponseWriter, r *http.Request) {
	empresaID := middleware.GetEmpresaID(r.Context())
	configID := chi.URLParam(r, "id")

	if err := h.service.DeleteEscalonamento(r.Context(), empresaID, configID); err != nil {
		if errors.Is(err, service.ErrEscalonamentoNaoEncontrado) {
			writeError(w, http.StatusNotFound, "config nao encontrada")
			return
		}
		if errors.Is(err, service.ErrEscalonamentoSistemaNaoExcluivel) {
			writeError(w, http.StatusBadRequest, "config do sistema nao pode ser excluida")
			return
		}
		slog.Error("delete escalonamento failed", "error", err)
		writeError(w, http.StatusInternalServerError, "erro ao excluir configuracao")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "excluido"})
}
