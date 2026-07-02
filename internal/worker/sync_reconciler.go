package worker

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/guardpoint/guardpoint-server/internal/repository"
)

type SyncReconciler struct {
	alertaRepo  *repository.AlertaRepository
	checkinRepo *repository.CheckinRepository
	turnoRepo   *repository.TurnoRepository
}

func NewSyncReconciler(
	alertaRepo *repository.AlertaRepository,
	checkinRepo *repository.CheckinRepository,
	turnoRepo *repository.TurnoRepository,
) *SyncReconciler {
	return &SyncReconciler{
		alertaRepo:  alertaRepo,
		checkinRepo: checkinRepo,
		turnoRepo:   turnoRepo,
	}
}

func (r *SyncReconciler) Reconcile(ctx context.Context, empresaID, turnoID uuid.UUID) error {
	checkins, err := r.checkinRepo.ListByTurno(ctx, turnoID)
	if err != nil {
		return fmt.Errorf("sync reconciler: listar checkins turno %s: %w", turnoID, err)
	}

	if len(checkins) < 2 {
		return nil
	}

	turno, err := r.turnoRepo.FindByID(ctx, empresaID, turnoID)
	if err != nil {
		return fmt.Errorf("sync reconciler: buscar turno %s: %w", turnoID, err)
	}

	intervaloDuration := time.Duration(turno.IntervaloMin) * time.Minute
	tolerancia := intervaloDuration * 2

	maxGap := time.Duration(0)
	for i := 1; i < len(checkins); i++ {
		gap := checkins[i].TimestampCriacao.Sub(checkins[i-1].TimestampCriacao)
		if gap > maxGap {
			maxGap = gap
		}
	}

	agora := time.Now()
	lastCheckin := checkins[len(checkins)-1]
	gapAteAgora := agora.Sub(lastCheckin.TimestampCriacao)
	if gapAteAgora > maxGap {
		maxGap = gapAteAgora
	}

	if maxGap > tolerancia {
		slog.Info("sync reconciler: turno ainda possui gaps superiores a tolerancia",
			"turno_id", turnoID.String(),
			"max_gap_min", int(maxGap.Minutes()),
			"tolerancia_min", int(tolerancia.Minutes()),
		)
		return nil
	}

	count, err := r.alertaRepo.CloseAlertasFalsoPositivo(ctx, turnoID)
	if err != nil {
		return fmt.Errorf("sync reconciler: fechar alertas turno %s: %w", turnoID, err)
	}

	if count > 0 {
		slog.Info("sync reconciler: alertas resolvidos como falso positivo via sincronizacao offline",
			"turno_id", turnoID.String(),
			"alertas_fechados", count,
		)
	}

	return nil
}
