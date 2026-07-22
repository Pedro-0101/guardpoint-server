package handler

import (
	"log/slog"
	"net/http"

	"github.com/guardpoint/guardpoint-server/internal/middleware"
	"github.com/guardpoint/guardpoint-server/internal/service"
)

type VigiaHandler struct {
	turnoService *service.TurnoService
}

func NewVigiaHandler(turnoService *service.TurnoService) *VigiaHandler {
	return &VigiaHandler{turnoService: turnoService}
}

// Turno godoc
// @Summary      Retorna o turno ativo ou o proximo turno agendado do vigia autenticado
// @Description  Se houver turno ativo, retorna todas as informacoes (status, posto, proximo deadline, checkins, etc). Se nao houver turno ativo, retorna o proximo turno agendado mais proximo. Unifica os endpoints /turnos/status, /turnos e /postos/{id} em uma unica chamada.
// @Tags         vigia
// @Success      200 {object} model.VigiaTurnoResponse
// @Failure      500 {object} model.ErrorResponse
// @Router       /vigia/turno [get]
func (h *VigiaHandler) Turno(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	empresaID := middleware.GetEmpresaID(r.Context())

	resp, err := h.turnoService.GetVigiaTurno(r.Context(), userID, empresaID)
	if err != nil {
		slog.Error("vigia turno failed", "error", err)
		writeError(w, http.StatusInternalServerError, "erro ao buscar dados do turno")
		return
	}

	writeJSON(w, http.StatusOK, resp)
}
