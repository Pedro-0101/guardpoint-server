package handler

import (
	"log/slog"
	"net/http"

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

func (h *DashboardHandler) Summary(w http.ResponseWriter, r *http.Request) {
	empresaID := GetEmpresaID(r.Context())

	summary, err := h.dashboardService.Summary(r.Context(), empresaID)
	if err != nil {
		slog.Error("dashboard summary failed", "error", err)
		writeError(w, http.StatusInternalServerError, "erro ao carregar dashboard")
		return
	}

	writeJSON(w, http.StatusOK, summary)
}
