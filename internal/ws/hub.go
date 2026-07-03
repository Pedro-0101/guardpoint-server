package ws

import (
	"encoding/json"
	"log/slog"
	"sync"
)

type Hub struct {
	mu      sync.RWMutex
	clients map[string]map[*Client]bool
}

func NewHub() *Hub {
	return &Hub{
		clients: make(map[string]map[*Client]bool),
	}
}

func (h *Hub) Register(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.clients[client.empresaID] == nil {
		h.clients[client.empresaID] = make(map[*Client]bool)
	}
	h.clients[client.empresaID][client] = true

	slog.Info("ws client registered",
		"empresa_id", client.empresaID,
		"user_id", client.userID,
		"role", client.role,
	)
}

func (h *Hub) Unregister(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if clients, ok := h.clients[client.empresaID]; ok {
		if _, exists := clients[client]; exists {
			delete(clients, client)
			close(client.send)

			if len(clients) == 0 {
				delete(h.clients, client.empresaID)
			}
		}
	}

	slog.Info("ws client unregistered",
		"empresa_id", client.empresaID,
		"user_id", client.userID,
	)
}

func (h *Hub) Broadcast(empresaID string, event Event) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	clients, ok := h.clients[empresaID]
	if !ok {
		return
	}

	data, err := json.Marshal(event)
	if err != nil {
		slog.Error("ws marshal event", "error", err)
		return
	}

	for client := range clients {
		select {
		case client.send <- data:
		default:
			go h.Unregister(client)
		}
	}
}
