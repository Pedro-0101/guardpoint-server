package handler

import (
	"log/slog"
	"net/http"

	"github.com/guardpoint/guardpoint-server/internal/middleware"
	_ "github.com/guardpoint/guardpoint-server/internal/model"
	"github.com/guardpoint/guardpoint-server/internal/service"
)

type DashboardHandler struct {
	dashboardService *service.DashboardService
}

func NewDashboardHandler(dashboardService *service.DashboardService) *DashboardHandler {
	return &DashboardHandler{
		dashboardService: dashboardService,
	}
}

// Table godoc
// @Summary      Tabela gerencial de postos x turnos
// @Description  Lista turnos ativos com dados de posto, vigia e checkins. Ordenado por proximidade do proximo checkin (mais urgente primeiro). Suporta paginacao.
// @Tags         dashboard
// @Param        limit   query int false "Limite de registros (default 20, max 100)"
// @Param        offset  query int false "Offset para paginacao (default 0)"
// @Success      200 {object} model.DashboardTableResponse
// @Failure      500 {object} model.ErrorResponse
// @Router       /dashboard/table [get]
func (h *DashboardHandler) Table(w http.ResponseWriter, r *http.Request) {
	empresaID := middleware.GetEmpresaID(r.Context())
	limit, offset := parsePagination(r)

	resp, err := h.dashboardService.Table(r.Context(), empresaID, limit, offset)
	if err != nil {
		slog.Error("dashboard table failed", "error", err)
		writeError(w, http.StatusInternalServerError, "erro ao carregar tabela do dashboard")
		return
	}

	writeJSON(w, http.StatusOK, resp)
}
