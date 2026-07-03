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
	"github.com/guardpoint/guardpoint-server/internal/worker"
)

type TurnoHandler struct {
	turnoService   *service.TurnoService
	syncReconciler *worker.SyncReconciler
	validate       *validator.Validate
}

func NewTurnoHandler(turnoService *service.TurnoService, syncReconciler *worker.SyncReconciler) *TurnoHandler {
	return &TurnoHandler{
		turnoService:   turnoService,
		syncReconciler: syncReconciler,
		validate:       validator.New(),
	}
}

func (h *TurnoHandler) Iniciar(w http.ResponseWriter, r *http.Request) {
	userID := GetUserID(r.Context())
	empresaID := GetEmpresaID(r.Context())

	var req model.IniciarTurnoRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "json invalido")
		return
	}

	if err := h.validate.Struct(req); err != nil {
		writeValidationError(w, err)
		return
	}

	turno, err := h.turnoService.Iniciar(r.Context(), userID, empresaID, req)
	if err != nil {
		if errors.Is(err, service.ErrTurnoJaAtivo) {
			writeError(w, http.StatusConflict, "usuario ja possui um turno em andamento")
			return
		}
		if errors.Is(err, service.ErrPostoNaoEncontrado) {
			writeError(w, http.StatusNotFound, "posto nao encontrado ou inativo")
			return
		}
		if errors.Is(err, service.ErrDeviceNaoRegistrado) {
			writeError(w, http.StatusForbidden, "dispositivo nao registrado - faca login biometrico primeiro")
			return
		}
		if errors.Is(err, service.ErrEscalaSemEscala) {
			writeError(w, http.StatusForbidden, "nenhuma escala ativa encontrada para este usuario, posto e horario")
			return
		}
		if errors.Is(err, service.ErrEscalaForaTolerancia) {
			writeError(w, http.StatusForbidden, "horario de inicio fora da tolerancia da escala")
			return
		}
		slog.Error("iniciar turno failed", "error", err)
		writeError(w, http.StatusInternalServerError, "erro ao iniciar turno")
		return
	}

	writeJSON(w, http.StatusCreated, turno)
}

func (h *TurnoHandler) Checkin(w http.ResponseWriter, r *http.Request) {
	userID := GetUserID(r.Context())
	empresaID := GetEmpresaID(r.Context())

	var req model.CheckinRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "json invalido")
		return
	}

	if err := h.validate.Struct(req); err != nil {
		writeValidationError(w, err)
		return
	}

	resp, err := h.turnoService.Checkin(r.Context(), userID, empresaID, req)
	if err != nil {
		if errors.Is(err, service.ErrTurnoNaoEncontrado) {
			writeError(w, http.StatusNotFound, "turno nao encontrado")
			return
		}
		if errors.Is(err, service.ErrTurnoJaFinalizado) {
			writeError(w, http.StatusConflict, "turno ja finalizado")
			return
		}
		if errors.Is(err, service.ErrTurnoNaoPertenceAoUsuario) {
			writeError(w, http.StatusForbidden, "turno nao pertence a este usuario")
			return
		}
		slog.Error("checkin failed", "error", err)
		writeError(w, http.StatusInternalServerError, "erro ao registrar check-in")
		return
	}

	writeJSON(w, http.StatusOK, resp)
}

func (h *TurnoHandler) Finalizar(w http.ResponseWriter, r *http.Request) {
	userID := GetUserID(r.Context())
	empresaID := GetEmpresaID(r.Context())

	var req model.FinalizarTurnoRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "json invalido")
		return
	}

	if err := h.validate.Struct(req); err != nil {
		writeValidationError(w, err)
		return
	}

	turno, err := h.turnoService.Finalizar(r.Context(), userID, empresaID, req)
	if err != nil {
		if errors.Is(err, service.ErrTurnoNaoEncontrado) {
			writeError(w, http.StatusNotFound, "turno nao encontrado")
			return
		}
		if errors.Is(err, service.ErrTurnoJaFinalizado) {
			writeError(w, http.StatusConflict, "turno ja finalizado")
			return
		}
		if errors.Is(err, service.ErrTurnoNaoPertenceAoUsuario) {
			writeError(w, http.StatusForbidden, "turno nao pertence a este usuario")
			return
		}
		slog.Error("finalizar turno failed", "error", err)
		writeError(w, http.StatusInternalServerError, "erro ao finalizar turno")
		return
	}

	writeJSON(w, http.StatusOK, turno)
}

func (h *TurnoHandler) Status(w http.ResponseWriter, r *http.Request) {
	userID := GetUserID(r.Context())
	empresaID := GetEmpresaID(r.Context())

	status, err := h.turnoService.GetStatus(r.Context(), userID, empresaID)
	if err != nil {
		if errors.Is(err, service.ErrTurnoNaoEncontrado) {
			writeError(w, http.StatusNotFound, "nenhum turno ativo encontrado")
			return
		}
		slog.Error("status turno failed", "error", err)
		writeError(w, http.StatusInternalServerError, "erro ao buscar status do turno")
		return
	}

	writeJSON(w, http.StatusOK, status)
}

func (h *TurnoHandler) Ativos(w http.ResponseWriter, r *http.Request) {
	empresaID := GetEmpresaID(r.Context())

	turnos, err := h.turnoService.GetAtivos(r.Context(), empresaID)
	if err != nil {
		slog.Error("listar ativos failed", "error", err)
		writeError(w, http.StatusInternalServerError, "erro ao listar turnos ativos")
		return
	}

	if turnos == nil {
		turnos = []model.TurnoDetalhe{}
	}

	writeJSON(w, http.StatusOK, turnos)
}

func (h *TurnoHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	empresaID := GetEmpresaID(r.Context())
	id := chi.URLParam(r, "id")

	detalhe, err := h.turnoService.GetByID(r.Context(), empresaID, id)
	if err != nil {
		writeError(w, http.StatusNotFound, "turno nao encontrado")
		return
	}

	writeJSON(w, http.StatusOK, detalhe)
}

func (h *TurnoHandler) Revogar(w http.ResponseWriter, r *http.Request) {
	empresaID := GetEmpresaID(r.Context())
	id := chi.URLParam(r, "id")

	resp, err := h.turnoService.Revogar(r.Context(), empresaID, id)
	if err != nil {
		if errors.Is(err, service.ErrTurnoNaoEncontrado) {
			writeError(w, http.StatusNotFound, "turno nao encontrado")
			return
		}
		if errors.Is(err, service.ErrTurnoJaFinalizado) {
			writeError(w, http.StatusConflict, "turno ja finalizado")
			return
		}
		slog.Error("revogar turno failed", "error", err)
		writeError(w, http.StatusInternalServerError, "erro ao revogar turno")
		return
	}

	writeJSON(w, http.StatusOK, resp)
}

func (h *TurnoHandler) Sabotagem(w http.ResponseWriter, r *http.Request) {
	userID := GetUserID(r.Context())
	empresaID := GetEmpresaID(r.Context())

	var req model.SabotagemRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "json invalido")
		return
	}

	if err := h.validate.Struct(req); err != nil {
		writeValidationError(w, err)
		return
	}

	resp, err := h.turnoService.Sabotagem(r.Context(), userID, empresaID, req)
	if err != nil {
		if errors.Is(err, service.ErrTurnoNaoEncontrado) {
			writeError(w, http.StatusNotFound, "turno nao encontrado")
			return
		}
		if errors.Is(err, service.ErrTurnoJaFinalizado) {
			writeError(w, http.StatusConflict, "turno ja finalizado")
			return
		}
		if errors.Is(err, service.ErrTurnoNaoPertenceAoUsuario) {
			writeError(w, http.StatusForbidden, "turno nao pertence a este usuario")
			return
		}
		slog.Error("sabotagem failed", "error", err)
		writeError(w, http.StatusInternalServerError, "erro ao registrar sabotagem")
		return
	}

	writeJSON(w, http.StatusAccepted, resp)
}

func (h *TurnoHandler) Lote(w http.ResponseWriter, r *http.Request) {
	userID := GetUserID(r.Context())
	empresaID := GetEmpresaID(r.Context())

	var reqs []model.CheckinRequest
	if err := json.NewDecoder(r.Body).Decode(&reqs); err != nil {
		writeError(w, http.StatusBadRequest, "json invalido")
		return
	}

	if len(reqs) == 0 {
		writeError(w, http.StatusBadRequest, "lote vazio")
		return
	}

	if len(reqs) > 500 {
		writeError(w, http.StatusBadRequest, "lote excede limite de 500 checkins")
		return
	}

	for i := range reqs {
		if err := h.validate.Struct(reqs[i]); err != nil {
			writeValidationError(w, err)
			return
		}
	}

	resultados, err := h.turnoService.ProcessarLote(r.Context(), userID, empresaID, reqs)
	if err != nil {
		slog.Error("lote checkin failed", "error", err)
		writeError(w, http.StatusInternalServerError, "erro ao processar lote")
		return
	}

	parsedEmpresaID, err := uuid.Parse(empresaID)
	if err == nil {
		turnoIDs := collectUniqueTurnoIDs(reqs)
		for _, turnoID := range turnoIDs {
			_ = h.syncReconciler.Reconcile(r.Context(), parsedEmpresaID, turnoID)
		}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"recebidos":   len(reqs),
		"processados": len(resultados),
		"checkins":    resultados,
	})
}

func (h *TurnoHandler) Historico(w http.ResponseWriter, r *http.Request) {
	empresaID := GetEmpresaID(r.Context())

	limit, offset := parsePagination(r)

	filter := model.HistoricoFilter{
		DataInicio: r.URL.Query().Get("data_inicio"),
		DataFim:    r.URL.Query().Get("data_fim"),
		UsuarioID:  r.URL.Query().Get("usuario_id"),
		PostoID:    r.URL.Query().Get("posto_id"),
		Status:     r.URL.Query().Get("status"),
		Limit:      limit,
		Offset:     offset,
	}

	turnos, total, err := h.turnoService.GetHistorico(r.Context(), empresaID, filter)
	if err != nil {
		slog.Error("historico turnos failed", "error", err)
		writeError(w, http.StatusInternalServerError, "erro ao buscar historico")
		return
	}

	if turnos == nil {
		turnos = []model.Turno{}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"data":  turnos,
		"total": total,
	})
}

func collectUniqueTurnoIDs(reqs []model.CheckinRequest) []uuid.UUID {
	seen := make(map[string]struct{})
	var ids []uuid.UUID
	for _, req := range reqs {
		if _, ok := seen[req.TurnoID]; ok {
			continue
		}
		id, err := uuid.Parse(req.TurnoID)
		if err != nil {
			continue
		}
		seen[req.TurnoID] = struct{}{}
		ids = append(ids, id)
	}
	return ids
}
