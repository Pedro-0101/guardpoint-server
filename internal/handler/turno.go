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

// Iniciar godoc
// @Summary      Inicia um turno no posto informado
// @Tags         turnos
// @Param        request body model.IniciarTurnoRequest true "Dados de inicio"
// @Success      201 {object} model.Turno
// @Failure      400 {object} map[string]string
// @Failure      403 {object} map[string]string
// @Failure      404 {object} map[string]string
// @Failure      409 {object} map[string]string
// @Router       /turnos/iniciar [post]
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

// Checkin godoc
// @Summary      Registra um check-in de turno
// @Tags         turnos
// @Param        request body model.CheckinRequest true "Dados do check-in"
// @Success      200 {object} model.CheckinResponse
// @Failure      400 {object} map[string]string
// @Failure      403 {object} map[string]string
// @Failure      404 {object} map[string]string
// @Failure      409 {object} map[string]string
// @Router       /turnos/checkin [post]
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
		if writeSessaoError(w, err) {
			return
		}
		slog.Error("checkin failed", "error", err)
		writeError(w, http.StatusInternalServerError, "erro ao registrar check-in")
		return
	}

	writeJSON(w, http.StatusOK, resp)
}

// Finalizar godoc
// @Summary      Finaliza um turno em andamento
// @Tags         turnos
// @Param        request body model.FinalizarTurnoRequest true "Dados de finalizacao"
// @Success      200 {object} model.Turno
// @Failure      400 {object} map[string]string
// @Failure      403 {object} map[string]string
// @Failure      404 {object} map[string]string
// @Failure      409 {object} map[string]string
// @Router       /turnos/finalizar [post]
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
		if writeSessaoError(w, err) {
			return
		}
		slog.Error("finalizar turno failed", "error", err)
		writeError(w, http.StatusInternalServerError, "erro ao finalizar turno")
		return
	}

	writeJSON(w, http.StatusOK, turno)
}

// Status godoc
// @Summary      Status do turno ativo do usuario autenticado
// @Tags         turnos
// @Success      200 {object} model.TurnoStatusResponse
// @Failure      404 {object} map[string]string
// @Router       /turnos/status [get]
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

// Ativos godoc
// @Summary      Lista turnos ativos da empresa
// @Tags         turnos
// @Success      200 {array} model.TurnoDetalhe
// @Failure      500 {object} map[string]string
// @Router       /turnos/ativos [get]
// List godoc
// @Summary      Lista turnos com filtros unificados, ordenacao e paginacao
// @Description  Para vigias, retorna apenas os turnos do proprio usuario autenticado.
// @Description  Admin e supervisores podem filtrar por qualquer usuario/posto.
// @Tags         turnos
// @Param        status query string false "Status (agendado,em_andamento,pausado,critico,finalizado)"
// @Param        data_inicio query string false "Data inicial (YYYY-MM-DD)"
// @Param        data_fim query string false "Data final (YYYY-MM-DD)"
// @Param        usuario_id query string false "ID do vigia (uuid)"
// @Param        posto_id query string false "ID do posto (uuid)"
// @Param        sort_by query string false "Campo de ordenacao (inicio_previsto, created_at, status)"
// @Param        sort_order query string false "Direcao (asc, desc)"
// @Param        limit query int false "Limite (max 100)"
// @Param        offset query int false "Offset"
// @Success      200 {object} map[string]interface{}
// @Failure      500 {object} map[string]string
// @Router       /turnos [get]
func (h *TurnoHandler) List(w http.ResponseWriter, r *http.Request) {
	empresaID := GetEmpresaID(r.Context())
	userID := GetUserID(r.Context())
	role := GetRole(r.Context())

	limit, offset := parsePagination(r)

	usuarioID := r.URL.Query().Get("usuario_id")

	if role == "vigia" {
		usuarioID = userID
	}

	filter := model.TurnoFilter{
		Status:     r.URL.Query().Get("status"),
		DataInicio: r.URL.Query().Get("data_inicio"),
		DataFim:    r.URL.Query().Get("data_fim"),
		UsuarioID:  usuarioID,
		PostoID:    r.URL.Query().Get("posto_id"),
		SortBy:     r.URL.Query().Get("sort_by"),
		SortOrder:  r.URL.Query().Get("sort_order"),
		Limit:      limit,
		Offset:     offset,
	}

	turnos, total, err := h.turnoService.GetTurnos(r.Context(), empresaID, filter)
	if err != nil {
		slog.Error("listar turnos failed", "error", err)
		writeError(w, http.StatusInternalServerError, "erro ao listar turnos")
		return
	}

	if turnos == nil {
		turnos = []model.TurnoDetalhe{}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"data":  turnos,
		"total": total,
	})
}

// GetByID godoc
// @Summary      Busca o detalhe de um turno pelo ID
// @Tags         turnos
// @Param        id path string true "ID do turno (uuid)"
// @Success      200 {object} model.TurnoDetalhe
// @Failure      404 {object} map[string]string
// @Router       /turnos/{id} [get]
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

// Revogar godoc
// @Summary      Revoga a sessao do dispositivo de um turno, gerando PIN de resgate
// @Tags         turnos
// @Param        id path string true "ID do turno (uuid)"
// @Success      200 {object} model.RevogarResponse
// @Failure      404 {object} map[string]string
// @Failure      409 {object} map[string]string
// @Router       /turnos/{id}/revogar [post]
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

// writeSessaoError trata os erros de sessao unica (A4); retorna true se o erro
// foi respondido.
func writeSessaoError(w http.ResponseWriter, err error) bool {
	if errors.Is(err, service.ErrSessaoRevogada) {
		writeError(w, http.StatusForbidden, "sessao revogada - reassocie o turno com o pin")
		return true
	}
	if errors.Is(err, service.ErrSessaoOutroDispositivo) {
		writeError(w, http.StatusForbidden, "turno associado a outro dispositivo")
		return true
	}
	return false
}

// Reassociar godoc
// @Summary      Reassocia um turno revogado a um novo dispositivo via PIN
// @Tags         turnos
// @Param        request body model.ReassociarRequest true "PIN e dispositivo"
// @Success      200 {object} model.Turno
// @Failure      400 {object} map[string]string
// @Failure      403 {object} map[string]string
// @Failure      404 {object} map[string]string
// @Router       /turnos/reassociar [post]
func (h *TurnoHandler) Reassociar(w http.ResponseWriter, r *http.Request) {
	userID := GetUserID(r.Context())
	empresaID := GetEmpresaID(r.Context())

	var req model.ReassociarRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "json invalido")
		return
	}

	if err := h.validate.Struct(req); err != nil {
		writeValidationError(w, err)
		return
	}

	turno, err := h.turnoService.Reassociar(r.Context(), userID, empresaID, req)
	if err != nil {
		if errors.Is(err, service.ErrDeviceNaoRegistrado) {
			writeError(w, http.StatusForbidden, "dispositivo nao registrado - faca login biometrico primeiro")
			return
		}
		if errors.Is(err, service.ErrTurnoNaoEncontrado) {
			writeError(w, http.StatusNotFound, "nenhum turno ativo encontrado")
			return
		}
		if errors.Is(err, service.ErrPinInvalido) {
			writeError(w, http.StatusForbidden, "pin invalido")
			return
		}
		if errors.Is(err, service.ErrPinExpirado) {
			writeError(w, http.StatusForbidden, "pin expirado - solicite nova revogacao")
			return
		}
		slog.Error("reassociar turno failed", "error", err)
		writeError(w, http.StatusInternalServerError, "erro ao reassociar turno")
		return
	}

	writeJSON(w, http.StatusOK, turno)
}

// Sabotagem godoc
// @Summary      Registra uma sabotagem durante um turno
// @Tags         turnos
// @Param        request body model.SabotagemRequest true "Dados da sabotagem"
// @Success      202 {object} model.SabotagemResponse
// @Failure      400 {object} map[string]string
// @Failure      403 {object} map[string]string
// @Failure      404 {object} map[string]string
// @Failure      409 {object} map[string]string
// @Router       /turnos/sabotagem [post]
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
		if writeSessaoError(w, err) {
			return
		}
		slog.Error("sabotagem failed", "error", err)
		writeError(w, http.StatusInternalServerError, "erro ao registrar sabotagem")
		return
	}

	writeJSON(w, http.StatusAccepted, resp)
}

// Lote godoc
// @Summary      Processa um lote de check-ins offline (ate 500 por requisicao)
// @Tags         turnos
// @Param        request body []model.CheckinRequest true "Lote de check-ins"
// @Success      200 {object} map[string]interface{}
// @Failure      400 {object} map[string]string
// @Failure      500 {object} map[string]string
// @Router       /checkins/lote [post]
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

func collectUniqueTurnoIDs(reqs []model.CheckinRequest) []uuid.UUID {
	seen := make(map[string]struct{})
	ids := make([]uuid.UUID, 0, len(reqs))
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
