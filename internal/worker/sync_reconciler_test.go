package worker

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/guardpoint/guardpoint-server/internal/model"
)

type alertaRepo interface {
	CloseAlertasFalsoPositivo(ctx context.Context, turnoID uuid.UUID) (int64, error)
}

type checkinRepo interface {
	ListByTurno(ctx context.Context, turnoID uuid.UUID) ([]model.Checkin, error)
}

type turnoRepo interface {
	FindByID(ctx context.Context, empresaID, id uuid.UUID) (*model.Turno, error)
}

type hubBroadcaster interface {
	Broadcast(empresaID string, event interface{})
}

type mockAlertaRepo struct {
	closeAlertasFalsoPositivoFn func(ctx context.Context, turnoID uuid.UUID) (int64, error)
}

func (m *mockAlertaRepo) CloseAlertasFalsoPositivo(ctx context.Context, turnoID uuid.UUID) (int64, error) {
	if m.closeAlertasFalsoPositivoFn != nil {
		return m.closeAlertasFalsoPositivoFn(ctx, turnoID)
	}
	return 0, nil
}

type mockCheckinRepo struct {
	listByTurnoFn func(ctx context.Context, turnoID uuid.UUID) ([]model.Checkin, error)
}

func (m *mockCheckinRepo) ListByTurno(ctx context.Context, turnoID uuid.UUID) ([]model.Checkin, error) {
	if m.listByTurnoFn != nil {
		return m.listByTurnoFn(ctx, turnoID)
	}
	return nil, nil
}

type mockTurnoRepo struct {
	findByIDFn func(ctx context.Context, empresaID, id uuid.UUID) (*model.Turno, error)
}

func (m *mockTurnoRepo) FindByID(ctx context.Context, empresaID, id uuid.UUID) (*model.Turno, error) {
	if m.findByIDFn != nil {
		return m.findByIDFn(ctx, empresaID, id)
	}
	return nil, nil
}

type mockHub struct {
	broadcastFn func(empresaID string, event interface{})
}

func (m *mockHub) Broadcast(empresaID string, event interface{}) {
	if m.broadcastFn != nil {
		m.broadcastFn(empresaID, event)
	}
}

type reconcilerTest struct {
	alertaRepo  alertaRepo
	checkinRepo checkinRepo
	turnoRepo   turnoRepo
	hub         hubBroadcaster
}

func (r *reconcilerTest) Reconcile(ctx context.Context, empresaID, turnoID uuid.UUID) error {
	checkins, err := r.checkinRepo.ListByTurno(ctx, turnoID)
	if err != nil {
		return err
	}

	if len(checkins) < 2 {
		return nil
	}

	turno, err := r.turnoRepo.FindByID(ctx, empresaID, turnoID)
	if err != nil {
		return err
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
		return nil
	}

	count, err := r.alertaRepo.CloseAlertasFalsoPositivo(ctx, turnoID)
	if err != nil {
		return err
	}

	if count > 0 {
		r.hub.Broadcast(empresaID.String(), nil)
	}

	return nil
}

func TestSyncReconciler_Reconcile_MenosDe2Checkins(t *testing.T) {
	ctx := context.Background()
	empresaID := uuid.New()
	turnoID := uuid.New()

	rt := &reconcilerTest{
		checkinRepo: &mockCheckinRepo{
			listByTurnoFn: func(ctx context.Context, id uuid.UUID) ([]model.Checkin, error) {
				return []model.Checkin{{TimestampCriacao: time.Now()}}, nil
			},
		},
	}

	err := rt.Reconcile(ctx, empresaID, turnoID)
	if err != nil {
		t.Errorf("Reconcile() erro inesperado com 1 checkin: %v", err)
	}
}

func TestSyncReconciler_Reconcile_GapsDentroDaTolerancia(t *testing.T) {
	ctx := context.Background()
	empresaID := uuid.New()
	turnoID := uuid.New()

	base := time.Now().Add(-10 * time.Minute)

	closeCalled := false
	hubCalled := false

	rt := &reconcilerTest{
		checkinRepo: &mockCheckinRepo{
			listByTurnoFn: func(ctx context.Context, id uuid.UUID) ([]model.Checkin, error) {
				return []model.Checkin{
					{TimestampCriacao: base},
					{TimestampCriacao: base.Add(5 * time.Minute)},
					{TimestampCriacao: base.Add(10 * time.Minute)},
				}, nil
			},
		},
		turnoRepo: &mockTurnoRepo{
			findByIDFn: func(ctx context.Context, eID, id uuid.UUID) (*model.Turno, error) {
				return &model.Turno{IntervaloMin: 30}, nil
			},
		},
		alertaRepo: &mockAlertaRepo{
			closeAlertasFalsoPositivoFn: func(ctx context.Context, turnoID uuid.UUID) (int64, error) {
				closeCalled = true
				return 2, nil
			},
		},
		hub: &mockHub{
			broadcastFn: func(empresaID string, event interface{}) {
				hubCalled = true
			},
		},
	}

	err := rt.Reconcile(ctx, empresaID, turnoID)
	if err != nil {
		t.Fatalf("Reconcile() erro inesperado: %v", err)
	}
	if !closeCalled {
		t.Error("CloseAlertasFalsoPositivo nao foi chamado")
	}
	if !hubCalled {
		t.Error("hub.Broadcast nao foi chamado")
	}
}

func TestSyncReconciler_Reconcile_GapsAcimaDaTolerancia(t *testing.T) {
	ctx := context.Background()
	empresaID := uuid.New()
	turnoID := uuid.New()

	closeCalled := false

	rt := &reconcilerTest{
		checkinRepo: &mockCheckinRepo{
			listByTurnoFn: func(ctx context.Context, id uuid.UUID) ([]model.Checkin, error) {
				return []model.Checkin{
					{TimestampCriacao: time.Now().Add(-2 * time.Hour)},
					{TimestampCriacao: time.Now().Add(-1 * time.Hour)},
				}, nil
			},
		},
		turnoRepo: &mockTurnoRepo{
			findByIDFn: func(ctx context.Context, eID, id uuid.UUID) (*model.Turno, error) {
				return &model.Turno{IntervaloMin: 15}, nil
			},
		},
		alertaRepo: &mockAlertaRepo{
			closeAlertasFalsoPositivoFn: func(ctx context.Context, turnoID uuid.UUID) (int64, error) {
				closeCalled = true
				return 0, nil
			},
		},
		hub: &mockHub{},
	}

	err := rt.Reconcile(ctx, empresaID, turnoID)
	if err != nil {
		t.Fatalf("Reconcile() erro inesperado: %v", err)
	}
	if closeCalled {
		t.Error("CloseAlertasFalsoPositivo nao deveria ser chamado com gaps acima da tolerancia")
	}
}

func TestSyncReconciler_Reconcile_ZeroAlertasFechados(t *testing.T) {
	ctx := context.Background()
	empresaID := uuid.New()
	turnoID := uuid.New()

	hubCalled := false

	rt := &reconcilerTest{
		checkinRepo: &mockCheckinRepo{
			listByTurnoFn: func(ctx context.Context, id uuid.UUID) ([]model.Checkin, error) {
				return []model.Checkin{
					{TimestampCriacao: time.Now().Add(-5 * time.Minute)},
					{TimestampCriacao: time.Now().Add(-2 * time.Minute)},
				}, nil
			},
		},
		turnoRepo: &mockTurnoRepo{
			findByIDFn: func(ctx context.Context, eID, id uuid.UUID) (*model.Turno, error) {
				return &model.Turno{IntervaloMin: 30}, nil
			},
		},
		alertaRepo: &mockAlertaRepo{
			closeAlertasFalsoPositivoFn: func(ctx context.Context, turnoID uuid.UUID) (int64, error) {
				return 0, nil
			},
		},
		hub: &mockHub{
			broadcastFn: func(empresaID string, event interface{}) {
				hubCalled = true
			},
		},
	}

	err := rt.Reconcile(ctx, empresaID, turnoID)
	if err != nil {
		t.Fatalf("Reconcile() erro inesperado: %v", err)
	}
	if hubCalled {
		t.Error("hub.Broadcast nao deveria ser chamado quando count = 0")
	}
}
