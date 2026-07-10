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

// AddSupervisor godoc
// @Summary      Vincula um supervisor a um posto (somente admin)
// @Description  Um supervisor vinculado a um posto recebera alertas apenas desse posto (alem de precisar estar na configuracao de escalonamento). Admins recebem alertas de todos os postos independente de vinculo.
// @Tags         postos
// @Param        id path string true "ID do posto (uuid)"
// @Param        request body model.PostoSupervisorRequest true "ID do supervisor a ser vinculado"
// @Success      200 {object} model.PostoSupervisorResponse
// @Failure      400 {object} model.ErrorResponse "posto_id ou supervisor_id invalido"
// @Failure      404 {object} model.ErrorResponse "posto ou supervisor nao encontrado"
// @Failure      500 {object} model.ErrorResponse "erro interno"
// @Router       /postos/{id}/supervisores [post]
func (h *PostoHandler) AddSupervisor(w http.ResponseWriter, r *http.Request) {
	postoID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "posto_id invalido")
		return
	}

	var req model.PostoSupervisorRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "json invalido")
		return
	}

	if err := h.validate.Struct(req); err != nil {
		writeValidationError(w, err)
		return
	}

	if err := h.postoService.AddSupervisor(r.Context(), postoID, req.SupervisorID); err != nil {
		slog.Error("add supervisor failed", "error", err)
		writeError(w, http.StatusInternalServerError, "erro ao vincular supervisor")
		return
	}

	writeJSON(w, http.StatusOK, model.PostoSupervisorResponse{PostoID: postoID, SupervisorID: req.SupervisorID})
}

// RemoveSupervisor godoc
// @Summary      Remove o vinculo de um supervisor com um posto (somente admin)
// @Description  Remove o supervisor do posto. Ele deixara de receber alertas especificos deste posto (a menos que seja admin, que continua recebendo todos).
// @Tags         postos
// @Param        id path string true "ID do posto (uuid)"
// @Param        supervisorId path string true "ID do supervisor (uuid)"
// @Success      200 {object} model.MessageResponse
// @Failure      400 {object} model.ErrorResponse "posto_id ou supervisor_id invalido"
// @Failure      404 {object} model.ErrorResponse "vinculo nao encontrado"
// @Failure      500 {object} model.ErrorResponse "erro interno"
// @Router       /postos/{id}/supervisores/{supervisorId} [delete]
func (h *PostoHandler) RemoveSupervisor(w http.ResponseWriter, r *http.Request) {
	postoID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "posto_id invalido")
		return
	}

	supervisorID, err := uuid.Parse(chi.URLParam(r, "supervisorId"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "supervisor_id invalido")
		return
	}

	if err := h.postoService.RemoveSupervisor(r.Context(), postoID, supervisorID); err != nil {
		slog.Error("remove supervisor failed", "error", err)
		writeError(w, http.StatusInternalServerError, "erro ao remover supervisor")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "supervisor removido do posto"})
}

// ListSupervisores godoc
// @Summary      Lista os supervisores vinculados a um posto (admin/supervisor)
// @Description  Retorna a lista de IDs dos supervisores que estao vinculados a este posto e portanto recebem alertas do mesmo.
// @Tags         postos
// @Param        id path string true "ID do posto (uuid)"
// @Success      200 {array} model.PostoSupervisorResponse
// @Failure      400 {object} model.ErrorResponse "posto_id invalido"
// @Failure      500 {object} model.ErrorResponse "erro interno"
// @Router       /postos/{id}/supervisores [get]
func (h *PostoHandler) ListSupervisores(w http.ResponseWriter, r *http.Request) {
	postoID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "posto_id invalido")
		return
	}

	ids, err := h.postoService.ListSupervisores(r.Context(), postoID)
	if err != nil {
		slog.Error("list supervisores failed", "error", err)
		writeError(w, http.StatusInternalServerError, "erro ao listar supervisores")
		return
	}

	result := make([]model.PostoSupervisorResponse, 0, len(ids))
	for _, id := range ids {
		result = append(result, model.PostoSupervisorResponse{PostoID: postoID, SupervisorID: id})
	}

	writeJSON(w, http.StatusOK, result)
}

// ListPostosBySupervisor godoc
// @Summary      Lista os postos vinculados a um supervisor (admin/supervisor)
// @Description  Retorna a lista de postos (com nome) aos quais o supervisor esta vinculado. Util para a UI mostrar/ocultar alertas por posto.
// @Tags         postos
// @Param        supervisorId path string true "ID do supervisor (uuid)"
// @Success      200 {array} model.SupervisorPostoResponse
// @Failure      400 {object} model.ErrorResponse "supervisor_id invalido"
// @Failure      500 {object} model.ErrorResponse "erro interno"
// @Router       /usuarios/{supervisorId}/postos [get]
func (h *PostoHandler) ListPostosBySupervisor(w http.ResponseWriter, r *http.Request) {
	supervisorID, err := uuid.Parse(chi.URLParam(r, "supervisorId"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "supervisor_id invalido")
		return
	}

	result, err := h.postoService.ListPostosBySupervisor(r.Context(), supervisorID)
	if err != nil {
		slog.Error("list postos by supervisor failed", "error", err)
		writeError(w, http.StatusInternalServerError, "erro ao listar postos")
		return
	}

	if result == nil {
		result = []model.SupervisorPostoResponse{}
	}

	writeJSON(w, http.StatusOK, result)
}


