package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"

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

func (h *PostoHandler) Create(w http.ResponseWriter, r *http.Request) {
	empresaID := GetEmpresaID(r.Context())

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

func (h *PostoHandler) GetByID(w http.ResponseWriter, r *http.Request) {
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

	posto, err := h.postoService.GetByID(r.Context(), parsedEmpresaID, parsedID)
	if err != nil {
		writeError(w, http.StatusNotFound, "posto nao encontrado")
		return
	}

	writeJSON(w, http.StatusOK, posto)
}

func (h *PostoHandler) List(w http.ResponseWriter, r *http.Request) {
	empresaID := GetEmpresaID(r.Context())

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

func (h *PostoHandler) Update(w http.ResponseWriter, r *http.Request) {
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

func (h *PostoHandler) Delete(w http.ResponseWriter, r *http.Request) {
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

	if err := h.postoService.Deactivate(r.Context(), parsedEmpresaID, parsedID); err != nil {
		slog.Error("delete posto failed", "error", err)
		writeError(w, http.StatusInternalServerError, "erro ao desativar posto")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "posto desativado"})
}

func parsePagination(r *http.Request) (int, int) {
	limit := 20
	offset := 0

	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 100 {
			limit = parsed
		}
	}
	if o := r.URL.Query().Get("offset"); o != "" {
		if parsed, err := strconv.Atoi(o); err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	return limit, offset
}
