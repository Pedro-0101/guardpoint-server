package ws

import "time"

type EventType string

const (
	EventGPSUpdate    EventType = "gps_update"
	EventStatusChange EventType = "status_change"
	EventNewAlert     EventType = "new_alert"
	EventSyncResolved EventType = "sync_resolved"
)

type Event struct {
	Type    EventType   `json:"type"`
	Payload interface{} `json:"payload"`
}

type GPSUpdatePayload struct {
	TurnoID      string  `json:"turno_id"`
	Latitude     float64 `json:"latitude"`
	Longitude    float64 `json:"longitude"`
	Timestamp    string  `json:"timestamp"`
	FlagGeofence *string `json:"flag_geofence"`
}

type StatusChangePayload struct {
	TurnoID   string `json:"turno_id"`
	Status    string `json:"status"`
	Timestamp string `json:"timestamp"`
}

type NewAlertPayload struct {
	AlertaID string `json:"alerta_id"`
	Tipo     string `json:"tipo"`
	TurnoID  string `json:"turno_id"`
	Nivel    int    `json:"nivel"`
}

type SyncResolvedPayload struct {
	TurnoID   string `json:"turno_id"`
	Resolvido bool   `json:"resolvido"`
	Motivo    string `json:"motivo"`
}

func NewGPSUpdateEvent(turnoID, timestamp string, latitude, longitude float64, flagGeofence *string) Event {
	return Event{
		Type: EventGPSUpdate,
		Payload: GPSUpdatePayload{
			TurnoID:      turnoID,
			Latitude:     latitude,
			Longitude:    longitude,
			Timestamp:    timestamp,
			FlagGeofence: flagGeofence,
		},
	}
}

func NewStatusChangeEvent(turnoID, status string) Event {
	return Event{
		Type: EventStatusChange,
		Payload: StatusChangePayload{
			TurnoID:   turnoID,
			Status:    status,
			Timestamp: time.Now().UTC().Format(time.RFC3339),
		},
	}
}

func NewAlertEvent(alertaID, tipo, turnoID string, nivel int) Event {
	return Event{
		Type: EventNewAlert,
		Payload: NewAlertPayload{
			AlertaID: alertaID,
			Tipo:     tipo,
			TurnoID:  turnoID,
			Nivel:    nivel,
		},
	}
}

func NewSyncResolvedEvent(turnoID, motivo string) Event {
	return Event{
		Type: EventSyncResolved,
		Payload: SyncResolvedPayload{
			TurnoID:   turnoID,
			Resolvido: true,
			Motivo:    motivo,
		},
	}
}
