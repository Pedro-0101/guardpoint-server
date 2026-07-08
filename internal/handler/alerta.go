package handler

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-playground/validator/v10"

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
// @Tags         alertas
// @Param        status query string false "Status do alerta"
// @Param        tipo query string false "Tipo do alerta"
// @Param        turno_id query string false "ID do turno (uuid)"
// @Param        limit query int false "Limite de itens (max 100)"
// @Param        offset query int false "Offset da paginacao"
// @Success      200 {object} map[string]interface{}
// @Failure      500 {object} map[string]string
// @Router       /alertas [get]
func (h *AlertaHandler) List(w http.ResponseWriter, r *http.Request) {
	empresaID := GetEmpresaID(r.Context())

	limit, offset := parsePagination(r)

	filter := model.AlertaFilter{
		Status:  r.URL.Query().Get("status"),
		Tipo:    r.URL.Query().Get("tipo"),
		TurnoID: r.URL.Query().Get("turno_id"),
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
// @Success      200 {object} map[string]string
// @Failure      404 {object} map[string]string
// @Failure      409 {object} map[string]string
// @Router       /alertas/{id}/reconhecer [put]
func (h *AlertaHandler) Reconhecer(w http.ResponseWriter, r *http.Request) {
	empresaID := GetEmpresaID(r.Context())
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
// @Success      200 {object} map[string]string
// @Failure      404 {object} map[string]string
// @Failure      409 {object} map[string]string
// @Router       /alertas/{id}/encerrar [put]
func (h *AlertaHandler) Encerrar(w http.ResponseWriter, r *http.Request) {
	empresaID := GetEmpresaID(r.Context())
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

// Estatisticas godoc
// @Summary      Estatisticas agregadas de alertas (admin/supervisor)
// @Tags         alertas
// @Success      200 {object} model.AlertStatistics
// @Failure      500 {object} map[string]string
// @Router       /alertas/estatisticas [get]
func (h *AlertaHandler) Estatisticas(w http.ResponseWriter, r *http.Request) {
	empresaID := GetEmpresaID(r.Context())

	stats, err := h.alertaService.GetEstatisticas(r.Context(), empresaID)
	if err != nil {
		slog.Error("estatisticas alertas failed", "error", err)
		writeError(w, http.StatusInternalServerError, "erro ao carregar estatisticas")
		return
	}

	writeJSON(w, http.StatusOK, stats)
}

// GetAlertasEmergencia godoc
// @Summary      Lista os destinatarios configurados por tipo de alerta de emergencia (somente admin)
// @Tags         config
// @Success      200 {array} model.ConfigAlertaEmergencia
// @Failure      500 {object} map[string]string
// @Router       /config/alertas-emergencia [get]
func (h *AlertaHandler) GetAlertasEmergencia(w http.ResponseWriter, r *http.Request) {
	empresaID := GetEmpresaID(r.Context())

	configs, err := h.alertaService.GetAlertasEmergencia(r.Context(), empresaID)
	if err != nil {
		slog.Error("get alertas emergencia failed", "error", err)
		writeError(w, http.StatusInternalServerError, "erro ao carregar configuracao")
		return
	}

	writeJSON(w, http.StatusOK, configs)
}

// PutAlertaEmergencia godoc
// @Summary      Define os destinatarios de um tipo de alerta de emergencia (somente admin)
// @Tags         config
// @Param        tipo path string true "Tipo de emergencia (sabotagem, no_show)"
// @Param        request body model.UpdateConfigAlertaEmergenciaRequest true "Lista de usuarios destinatarios"
// @Success      200 {object} model.ConfigAlertaEmergencia
// @Failure      400 {object} map[string]string
// @Failure      500 {object} map[string]string
// @Router       /config/alertas-emergencia/{tipo} [put]
func (h *AlertaHandler) PutAlertaEmergencia(w http.ResponseWriter, r *http.Request) {
	empresaID := GetEmpresaID(r.Context())
	tipo := chi.URLParam(r, "tipo")

	var req model.UpdateConfigAlertaEmergenciaRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "json invalido")
		return
	}

	if err := h.validate.Struct(req); err != nil {
		writeValidationError(w, err)
		return
	}

	config, err := h.alertaService.UpdateAlertaEmergencia(r.Context(), empresaID, tipo, req)
	if err != nil {
		if errors.Is(err, service.ErrTipoEmergenciaInvalido) {
			writeError(w, http.StatusBadRequest, "tipo de emergencia invalido")
			return
		}
		if errors.Is(err, service.ErrUsuarioNaoPertenceAEmpresa) {
			writeError(w, http.StatusBadRequest, "usuario_id invalido para esta empresa")
			return
		}
		if errors.Is(err, service.ErrUsuarioNaoAdmin) {
			writeError(w, http.StatusBadRequest, "apenas administradores podem ser destinatarios de alertas")
			return
		}
		slog.Error("put alerta emergencia failed", "error", err)
		writeError(w, http.StatusInternalServerError, "erro ao salvar configuracao")
		return
	}

	writeJSON(w, http.StatusOK, config)
}
