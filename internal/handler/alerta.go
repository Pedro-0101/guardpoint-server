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
		slog.Error("create escalonamento failed", "error", err)
		writeError(w, http.StatusInternalServerError, "erro ao criar configuracao")
		return
	}

	writeJSON(w, http.StatusCreated, config)
}

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
		slog.Error("update escalonamento failed", "error", err)
		writeError(w, http.StatusInternalServerError, "erro ao atualizar configuracao")
		return
	}

	writeJSON(w, http.StatusOK, config)
}

func (h *AlertaHandler) DeleteEscalonamento(w http.ResponseWriter, r *http.Request) {
	empresaID := GetEmpresaID(r.Context())
	configID := chi.URLParam(r, "id")

	if err := h.alertaService.DeleteEscalonamento(r.Context(), empresaID, configID); err != nil {
		slog.Error("delete escalonamento failed", "error", err)
		writeError(w, http.StatusInternalServerError, "erro ao deletar configuracao")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

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
		slog.Error("put escalonamento failed", "error", err)
		writeError(w, http.StatusInternalServerError, "erro ao salvar configuracao")
		return
	}

	writeJSON(w, http.StatusOK, configs)
}
