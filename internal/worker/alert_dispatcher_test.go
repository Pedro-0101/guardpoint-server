package worker

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/guardpoint/guardpoint-server/internal/model"
)

func TestAlertDispatcher_Run_ContextCancelado(t *testing.T) {
	ch := make(chan *model.PendingAlert)
	dispatcher := NewAlertDispatcher(ch)

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		dispatcher.Run(ctx)
		close(done)
	}()

	cancel()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Run() nao parou ao cancelar o contexto")
	}
}

func TestAlertDispatcher_Run_ProcessaAlertas(t *testing.T) {
	ch := make(chan *model.PendingAlert, 1)
	dispatcher := NewAlertDispatcher(ch)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan struct{})
	go func() {
		dispatcher.Run(ctx)
		close(done)
	}()

	alertaID := uuid.New()
	ch <- &model.PendingAlert{
		Alerta:     &model.Alerta{ID: alertaID, Tipo: "teste", Status: "aberto"},
		UsuarioIDs: []uuid.UUID{uuid.New()},
	}

	time.Sleep(50 * time.Millisecond)

	cancel()
	<-done
}

func TestAlertDispatcher_Dispatch(t *testing.T) {
	ch := make(chan *model.PendingAlert)
	dispatcher := NewAlertDispatcher(ch)

	alertaID := uuid.New()
	dispatcher.dispatch(context.Background(), &model.PendingAlert{
		Alerta:     &model.Alerta{ID: alertaID, Tipo: "atraso"},
		UsuarioIDs: []uuid.UUID{uuid.New(), uuid.New()},
	})
}
