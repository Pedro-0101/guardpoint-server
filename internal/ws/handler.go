package ws

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/gorilla/websocket"

	"github.com/guardpoint/guardpoint-server/internal/auth"
)

func HandleWebSocket(hub *Hub, jwtService *auth.JWTService, corsOrigin string) http.HandlerFunc {
	upgrader := websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool {
			origin := r.Header.Get("Origin")
			// Origin ausente = cliente nao-browser (app Android); a checagem
			// de origem protege apenas contra CSWSH em navegadores.
			if origin == "" || corsOrigin == "*" || corsOrigin == "" {
				return true
			}
			for _, o := range strings.Split(corsOrigin, ",") {
				if strings.TrimSpace(o) == origin {
					return true
				}
			}
			slog.Warn("ws origin rejected", "origin", origin, "allowed", corsOrigin)
			return false
		},
	}

	return func(w http.ResponseWriter, r *http.Request) {
		token := r.URL.Query().Get("token")
		if token == "" {
			http.Error(w, "token nao fornecido", http.StatusUnauthorized)
			return
		}

		claims, err := jwtService.ValidateToken(token)
		if err != nil {
			slog.Error("ws jwt validation failed", "error", err)
			http.Error(w, "token invalido ou expirado", http.StatusUnauthorized)
			return
		}

		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			slog.Error("ws upgrade failed", "error", err)
			return
		}

		client := NewClient(hub, conn, claims.EmpresaID, claims.UserID, claims.Role)
		hub.Register(client)

		go client.WritePump()
		go client.ReadPump()
	}
}
