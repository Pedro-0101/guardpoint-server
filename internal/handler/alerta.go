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

type AlertaHandler struct {
	alertaService *service.AlertaService
	validate      *validator.Validate
}

func NewAlertaHandler(alertaService *service.AlertaService) *AlertaHandler {
	return &AlertaHandler{
		alertaService: alertaService,
		validate:      validator.New(),
	}
}

// List godoc
// @Summary      Lista alertas com filtros e paginacao (admin/supervisor)
// @Description  Para supervisores, os alertas sao automaticamente filtrados pelos postos aos quais estao vinculados (via posto_supervisores). Admins veem todos os alertas. Os filtros de query sao adicionais — posto_id pode ser usado para refinar a busca.
// @Tags         alertas
// @Param        status query string false "Status do alerta (aberto, reconhecido, encerrado)"
// @Param        tipo query string false "Tipo do alerta (atraso, no_show, senha_emergencia, senha_customizada, sabotagem)"
// @Param        turno_id query string false "ID do turno (uuid)"
// @Param        posto_id query string false "ID do posto (uuid)"
// @Param        limit query int false "Limite de itens (max 100)"
// @Param        offset query int false "Offset da paginacao"
// @Success      200 {object} model.AlertaListResponse
// @Failure      500 {object} model.ErrorResponse
// @Router       /alertas [get]
func (h *AlertaHandler) List(w http.ResponseWriter, r *http.Request) {
	empresaID := middleware.GetEmpresaID(r.Context())

	limit, offset := parsePagination(r)

	filter := model.AlertaFilter{
		Status:  r.URL.Query().Get("status"),
		Tipo:    r.URL.Query().Get("tipo"),
		TurnoID: r.URL.Query().Get("turno_id"),
		PostoID: r.URL.Query().Get("posto_id"),
		Limit:   limit,
		Offset:  offset,
	}

	alertas, total, err := h.alertaService.List(r.Context(), empresaID, filter)
	if err != nil {
		slog.Error("listar alertas failed", "error", err)
		writeError(w, http.StatusInternalServerError, "erro ao listar alertas")
		return
	}

	if alertas == nil {
		alertas = []model.Alerta{}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"data":  alertas,
		"total": total,
	})
}

// Reconhecer godoc
// @Summary      Reconhece um alerta aberto (admin/supervisor)
// @Tags         alertas
// @Param        id path string true "ID do alerta (uuid)"
// @Success      200 {object} model.StatusResponse
// @Failure      404 {object} model.ErrorResponse
// @Failure      409 {object} model.ErrorResponse
// @Router       /alertas/{id}/reconhecer [put]
func (h *AlertaHandler) Reconhecer(w http.ResponseWriter, r *http.Request) {
	empresaID := middleware.GetEmpresaID(r.Context())
	alertaID := chi.URLParam(r, "id")

	if err := h.alertaService.Reconhecer(r.Context(), empresaID, alertaID); err != nil {
		if errors.Is(err, service.ErrAlertaNaoEncontrado) {
			writeError(w, http.StatusNotFound, "alerta nao encontrado")
			return
		}
		if errors.Is(err, service.ErrAlertaTransicaoInvalida) {
			writeError(w, http.StatusConflict, "alerta nao esta aberto")
			return
		}
		slog.Error("reconhecer alerta failed", "error", err)
		writeError(w, http.StatusInternalServerError, "erro ao reconhecer alerta")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "reconhecido"})
}

// Encerrar godoc
// @Summary      Encerra um alerta (admin/supervisor)
// @Tags         alertas
// @Param        id path string true "ID do alerta (uuid)"
// @Success      200 {object} model.StatusResponse
// @Failure      404 {object} model.ErrorResponse
// @Failure      409 {object} model.ErrorResponse
// @Router       /alertas/{id}/encerrar [put]
func (h *AlertaHandler) Encerrar(w http.ResponseWriter, r *http.Request) {
	empresaID := middleware.GetEmpresaID(r.Context())
	alertaID := chi.URLParam(r, "id")

	if err := h.alertaService.Encerrar(r.Context(), empresaID, alertaID); err != nil {
		if errors.Is(err, service.ErrAlertaNaoEncontrado) {
			writeError(w, http.StatusNotFound, "alerta nao encontrado")
			return
		}
		if errors.Is(err, service.ErrAlertaTransicaoInvalida) {
			writeError(w, http.StatusConflict, "alerta ja foi encerrado")
			return
		}
		slog.Error("encerrar alerta failed", "error", err)
		writeError(w, http.StatusInternalServerError, "erro ao encerrar alerta")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "encerrado"})
}

// ReconhecerEmLote godoc
// @Summary      Reconhece alertas em lote (admin/supervisor)
// @Description  Altera o status dos alertas informados para "reconhecido". Retorna a quantidade de alertas afetados.
// @Tags         alertas
// @Accept       json
// @Produce      json
// @Param        body body model.BatchAlertaRequest true "Lista de IDs dos alertas"
// @Success      200 {object} model.MessageResponse
// @Failure      422 {object} model.ErrorResponse
// @Failure      500 {object} model.ErrorResponse
// @Router       /alertas/reconhecer [post]
func (h *AlertaHandler) ReconhecerEmLote(w http.ResponseWriter, r *http.Request) {
	empresaID := middleware.GetEmpresaID(r.Context())

	var req model.BatchAlertaRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "json invalido")
		return
	}
	if err := h.validate.Struct(req); err != nil {
		writeValidationError(w, err)
		return
	}

	affected, err := h.alertaService.ReconhecerEmLote(r.Context(), empresaID, req.IDs)
	if err != nil {
		slog.Error("reconhecer alertas em lote failed", "error", err)
		writeError(w, http.StatusInternalServerError, "erro ao reconhecer alertas em lote")
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"message":      "alertas reconhecidos com sucesso",
		"reconhecidos": affected,
	})
}

// EncerrarEmLote godoc
// @Summary      Encerra alertas em lote (admin/supervisor)
// @Description  Altera o status dos alertas informados para "encerrado" com a data/hora atual. Retorna a quantidade de alertas afetados.
// @Tags         alertas
// @Accept       json
// @Produce      json
// @Param        body body model.BatchAlertaRequest true "Lista de IDs dos alertas"
// @Success      200 {object} model.MessageResponse
// @Failure      422 {object} model.ErrorResponse
// @Failure      500 {object} model.ErrorResponse
// @Router       /alertas/encerrar [post]
func (h *AlertaHandler) EncerrarEmLote(w http.ResponseWriter, r *http.Request) {
	empresaID := middleware.GetEmpresaID(r.Context())

	var req model.BatchAlertaRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "json invalido")
		return
	}
	if err := h.validate.Struct(req); err != nil {
		writeValidationError(w, err)
		return
	}

	affected, err := h.alertaService.EncerrarEmLote(r.Context(), empresaID, req.IDs)
	if err != nil {
		slog.Error("encerrar alertas em lote failed", "error", err)
		writeError(w, http.StatusInternalServerError, "erro ao encerrar alertas em lote")
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"message":    "alertas encerrados com sucesso",
		"encerrados": affected,
	})
}

// Estatisticas godoc
// @Summary      Estatisticas agregadas de alertas (admin/supervisor)
// @Tags         alertas
// @Success      200 {object} model.AlertStatistics
// @Failure      500 {object} model.ErrorResponse
// @Router       /alertas/estatisticas [get]
func (h *AlertaHandler) Estatisticas(w http.ResponseWriter, r *http.Request) {
	empresaID := middleware.GetEmpresaID(r.Context())

	stats, err := h.alertaService.GetEstatisticas(r.Context(), empresaID)
	if err != nil {
		slog.Error("estatisticas alertas failed", "error", err)
		writeError(w, http.StatusInternalServerError, "erro ao carregar estatisticas")
		return
	}

	writeJSON(w, http.StatusOK, stats)
}
