package worker

import (
	"context"
	"log/slog"

	"github.com/guardpoint/guardpoint-server/internal/model"
)

type AlertDispatcher struct {
	channel <-chan *model.PendingAlert
}

func NewAlertDispatcher(channel <-chan *model.PendingAlert) *AlertDispatcher {
	return &AlertDispatcher{
		channel: channel,
	}
}

func (w *AlertDispatcher) Run(ctx context.Context) {
	slog.Info("alert dispatcher worker started")

	for {
		select {
		case <-ctx.Done():
			slog.Info("alert dispatcher worker stopped")
			return
		case alert := <-w.channel:
			w.dispatch(ctx, alert)
		}
	}
}

func (w *AlertDispatcher) dispatch(_ context.Context, alert *model.PendingAlert) {
	slog.Info("alert dispatcher: stub notificacao",
		"alerta_id", alert.Alerta.ID.String(),
		"tipo", alert.Alerta.Tipo,
		"usuario_ids", alert.UsuarioIDs,
	)
}
