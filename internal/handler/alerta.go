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

// GetEscalonamento godoc
// @Summary      Lista a configuracao de escalonamento de alertas (somente admin)
// @Tags         config
// @Success      200 {array} model.ConfigEscalonamento
// @Failure      500 {object} map[string]string
// @Router       /config/escalonamento [get]
func (h *AlertaHandler) GetEscalonamento(w http.ResponseWriter, r *http.Request) {
	empresaID := GetEmpresaID(r.Context())

	configs, err := h.alertaService.GetEscalonamento(r.Context(), empresaID)
	if err != nil {
		slog.Error("get escalonamento failed", "error", err)
		writeError(w, http.StatusInternalServerError, "erro ao carregar configuracao")
		return
	}

	if configs == nil {
		configs = []model.ConfigEscalonamento{}
	}

	writeJSON(w, http.StatusOK, configs)
}

// CreateEscalonamento godoc
// @Summary      Cria um nivel de escalonamento de alertas (somente admin)
// @Tags         config
// @Param        request body model.CreateConfigEscalonamentoRequest true "Dados do nivel"
// @Success      201 {object} model.ConfigEscalonamento
// @Failure      400 {object} map[string]string
// @Failure      500 {object} map[string]string
// @Router       /config/escalonamento [post]
func (h *AlertaHandler) CreateEscalonamento(w http.ResponseWriter, r *http.Request) {
	empresaID := GetEmpresaID(r.Context())

	var req model.CreateConfigEscalonamentoRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "json invalido")
		return
	}

	if err := h.validate.Struct(req); err != nil {
		writeValidationError(w, err)
		return
	}

	config, err := h.alertaService.CreateEscalonamento(r.Context(), empresaID, req)
	if err != nil {
		if errors.Is(err, service.ErrUsuarioNaoPertenceAEmpresa) {
			writeError(w, http.StatusBadRequest, "usuario_id invalido para esta empresa")
			return
		}
		slog.Error("create escalonamento failed", "error", err)
		writeError(w, http.StatusInternalServerError, "erro ao criar configuracao")
		return
	}

	writeJSON(w, http.StatusCreated, config)
}

// UpdateEscalonamento godoc
// @Summary      Atualiza um nivel de escalonamento de alertas (somente admin)
// @Tags         config
// @Param        id path string true "ID da configuracao (uuid)"
// @Param        request body model.UpdateConfigEscalonamentoRequest true "Campos a atualizar"
// @Success      200 {object} model.ConfigEscalonamento
// @Failure      400 {object} map[string]string
// @Failure      500 {object} map[string]string
// @Router       /config/escalonamento/{id} [put]
func (h *AlertaHandler) UpdateEscalonamento(w http.ResponseWriter, r *http.Request) {
	empresaID := GetEmpresaID(r.Context())
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

	config, err := h.alertaService.UpdateEscalonamento(r.Context(), empresaID, configID, req)
	if err != nil {
		if errors.Is(err, service.ErrUsuarioNaoPertenceAEmpresa) {
			writeError(w, http.StatusBadRequest, "usuario_id invalido para esta empresa")
			return
		}
		slog.Error("update escalonamento failed", "error", err)
		writeError(w, http.StatusInternalServerError, "erro ao atualizar configuracao")
		return
	}

	writeJSON(w, http.StatusOK, config)
}

// DeleteEscalonamento godoc
// @Summary      Remove um nivel de escalonamento de alertas (somente admin)
// @Tags         config
// @Param        id path string true "ID da configuracao (uuid)"
// @Success      204 "sem conteudo"
// @Failure      409 {object} map[string]string
// @Failure      500 {object} map[string]string
// @Router       /config/escalonamento/{id} [delete]
func (h *AlertaHandler) DeleteEscalonamento(w http.ResponseWriter, r *http.Request) {
	empresaID := GetEmpresaID(r.Context())
	configID := chi.URLParam(r, "id")

	if err := h.alertaService.DeleteEscalonamento(r.Context(), empresaID, configID); err != nil {
		if errors.Is(err, service.ErrNivelEscalonamentoEmUso) {
			writeError(w, http.StatusConflict, "nivel de escalonamento em uso por uma senha de vigia")
			return
		}
		if errors.Is(err, service.ErrNivelEscalonamentoSistema) {
			writeError(w, http.StatusConflict, "nivel de escalonamento padrao do sistema nao pode ser removido")
			return
		}
		slog.Error("delete escalonamento failed", "error", err)
		writeError(w, http.StatusInternalServerError, "erro ao deletar configuracao")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// PutEscalonamento godoc
// @Summary      Substitui todos os niveis de escalonamento de alertas (somente admin)
// @Tags         config
// @Param        request body []model.CreateConfigEscalonamentoRequest true "Lista completa de niveis"
// @Success      200 {array} model.ConfigEscalonamento
// @Failure      400 {object} map[string]string
// @Failure      500 {object} map[string]string
// @Router       /config/escalonamento [put]
func (h *AlertaHandler) PutEscalonamento(w http.ResponseWriter, r *http.Request) {
	empresaID := GetEmpresaID(r.Context())

	var reqs []model.CreateConfigEscalonamentoRequest
	if err := json.NewDecoder(r.Body).Decode(&reqs); err != nil {
		writeError(w, http.StatusBadRequest, "json invalido")
		return
	}

	if len(reqs) == 0 {
		writeError(w, http.StatusBadRequest, "array vazio")
		return
	}

	for i := range reqs {
		if err := h.validate.Struct(reqs[i]); err != nil {
			writeValidationError(w, err)
			return
		}
	}

	configs, err := h.alertaService.ReplaceEscalonamento(r.Context(), empresaID, reqs)
	if err != nil {
		if errors.Is(err, service.ErrUsuarioNaoPertenceAEmpresa) {
			writeError(w, http.StatusBadRequest, "usuario_id invalido para esta empresa")
			return
		}
		slog.Error("put escalonamento failed", "error", err)
		writeError(w, http.StatusInternalServerError, "erro ao salvar configuracao")
		return
	}

	writeJSON(w, http.StatusOK, configs)
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
		slog.Error("put alerta emergencia failed", "error", err)
		writeError(w, http.StatusInternalServerError, "erro ao salvar configuracao")
		return
	}

	writeJSON(w, http.StatusOK, config)
}
