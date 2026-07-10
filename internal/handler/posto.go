package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"

	"github.com/guardpoint/guardpoint-server/internal/middleware"
	"github.com/guardpoint/guardpoint-server/internal/model"
	"github.com/guardpoint/guardpoint-server/internal/service"
)

type PostoHandler struct {
	postoService *service.PostoService
	validate     *validator.Validate
}

func NewPostoHandler(postoService *service.PostoService) *PostoHandler {
	return &PostoHandler{
		postoService: postoService,
		validate:     validator.New(),
	}
}

// Create godoc
// @Summary      Cria um posto
// @Tags         postos
// @Param        request body model.CreatePostoRequest true "Dados do posto"
// @Success      201 {object} model.Posto
// @Failure      400 {object} model.ErrorResponse
// @Router       /postos [post]
func (h *PostoHandler) Create(w http.ResponseWriter, r *http.Request) {
	empresaID := middleware.GetEmpresaID(r.Context())

	var req model.CreatePostoRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "json invalido")
		return
	}

	if err := h.validate.Struct(req); err != nil {
		writeValidationError(w, err)
		return
	}

	parsedEmpresaID, err := uuid.Parse(empresaID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "empresa_id invalido")
		return
	}

	raioM := req.RaioM
	if raioM <= 0 {
		raioM = 100
	}

	posto := &model.Posto{
		EmpresaID: parsedEmpresaID,
		Nome:      req.Nome,
		Latitude:  req.Latitude,
		Longitude: req.Longitude,
		RaioM:     raioM,
	}

	if err := h.postoService.Create(r.Context(), posto); err != nil {
		slog.Error("create posto failed", "error", err)
		writeError(w, http.StatusInternalServerError, "erro ao criar posto")
		return
	}

	writeJSON(w, http.StatusCreated, posto)
}

// GetByID godoc
// @Summary      Busca um posto pelo ID
// @Tags         postos
// @Param        id path string true "ID do posto (uuid)"
// @Success      200 {object} model.Posto
// @Failure      400 {object} model.ErrorResponse
// @Failure      404 {object} model.ErrorResponse
// @Router       /postos/{id} [get]
func (h *PostoHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	empresaID := middleware.GetEmpresaID(r.Context())
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

	posto, err := h.postoService.GetByID(r.Context(), parsedEmpresaID, parsedID)
	if err != nil {
		writeError(w, http.StatusNotFound, "posto nao encontrado")
		return
	}

	writeJSON(w, http.StatusOK, posto)
}

// List godoc
// @Summary      Lista os postos da empresa
// @Tags         postos
// @Param        ativos query string false "Filtra somente postos ativos (true/false)"
// @Success      200 {array} model.Posto
// @Failure      400 {object} model.ErrorResponse
// @Router       /postos [get]
func (h *PostoHandler) List(w http.ResponseWriter, r *http.Request) {
	empresaID := middleware.GetEmpresaID(r.Context())

	parsedEmpresaID, err := uuid.Parse(empresaID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "empresa_id invalido")
		return
	}

	apenasAtivos := r.URL.Query().Get("ativos") == "true"

	postos, err := h.postoService.List(r.Context(), parsedEmpresaID, apenasAtivos)
	if err != nil {
		slog.Error("list postos failed", "error", err)
		writeError(w, http.StatusInternalServerError, "erro ao listar postos")
		return
	}

	if postos == nil {
		postos = []model.Posto{}
	}

	writeJSON(w, http.StatusOK, postos)
}

// Update godoc
// @Summary      Atualiza um posto
// @Tags         postos
// @Param        id path string true "ID do posto (uuid)"
// @Param        request body model.UpdatePostoRequest true "Campos a atualizar"
// @Success      200 {object} model.Posto
// @Failure      400 {object} model.ErrorResponse
// @Failure      404 {object} model.ErrorResponse
// @Router       /postos/{id} [put]
func (h *PostoHandler) Update(w http.ResponseWriter, r *http.Request) {
	empresaID := middleware.GetEmpresaID(r.Context())
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

	posto, err := h.postoService.GetByID(r.Context(), parsedEmpresaID, parsedID)
	if err != nil {
		writeError(w, http.StatusNotFound, "posto nao encontrado")
		return
	}

	var req model.UpdatePostoRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "json invalido")
		return
	}

	if err := h.validate.Struct(req); err != nil {
		writeValidationError(w, err)
		return
	}

	if req.Nome != "" {
		posto.Nome = req.Nome
	}
	if req.Latitude != 0 {
		posto.Latitude = req.Latitude
	}
	if req.Longitude != 0 {
		posto.Longitude = req.Longitude
	}
	if req.RaioM > 0 {
		posto.RaioM = req.RaioM
	}
	if req.Ativo != nil {
		posto.Ativo = *req.Ativo
	}

	if err := h.postoService.Update(r.Context(), parsedEmpresaID, parsedID, posto); err != nil {
		slog.Error("update posto failed", "error", err)
		writeError(w, http.StatusInternalServerError, "erro ao atualizar posto")
		return
	}

	writeJSON(w, http.StatusOK, posto)
}

// Delete godoc
// @Summary      Desativa um posto
// @Tags         postos
// @Param        id path string true "ID do posto (uuid)"
// @Success      200 {object} model.MessageResponse
// @Failure      400 {object} model.ErrorResponse
// @Failure      500 {object} model.ErrorResponse
// @Router       /postos/{id} [delete]
func (h *PostoHandler) Delete(w http.ResponseWriter, r *http.Request) {
	empresaID := middleware.GetEmpresaID(r.Context())
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

	if err := h.postoService.Deactivate(r.Context(), parsedEmpresaID, parsedID); err != nil {
		slog.Error("delete posto failed", "error", err)
		writeError(w, http.StatusInternalServerError, "erro ao desativar posto")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "posto desativado"})
}


