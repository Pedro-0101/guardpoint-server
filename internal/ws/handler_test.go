package ws

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"

	"github.com/guardpoint/guardpoint-server/internal/auth"
)

const testSecret = "ws-test-secret-0123456789abcdef"

func newWSServer(t *testing.T, corsOrigin string) (*httptest.Server, *Hub, *auth.JWTService) {
	t.Helper()
	hub := NewHub()
	jwtService := auth.NewJWTService(testSecret)
	srv := httptest.NewServer(HandleWebSocket(hub, jwtService, corsOrigin))
	t.Cleanup(srv.Close)
	return srv, hub, jwtService
}

func wsURL(srv *httptest.Server, token string) string {
	url := "ws" + strings.TrimPrefix(srv.URL, "http")
	if token != "" {
		url += "?token=" + token
	}
	return url
}

func tokenPara(t *testing.T, jwtService *auth.JWTService, empresaID uuid.UUID) string {
	t.Helper()
	token, err := jwtService.GenerateAccessToken(uuid.New(), empresaID, "v@e.com", "vigia", "Vigia")
	if err != nil {
		t.Fatalf("gerar token: %v", err)
	}
	return token
}

func TestHandshake(t *testing.T) {
	srv, _, jwtService := newWSServer(t, "*")

	t.Run("sem token retorna 401", func(t *testing.T) {
		conn, resp, err := websocket.DefaultDialer.Dial(wsURL(srv, ""), nil)
		if err == nil {
			conn.Close()
			t.Fatal("handshake sem token deveria falhar")
		}
		if resp == nil || resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("status = %v, esperado 401", resp)
		}
	})

	t.Run("token invalido retorna 401", func(t *testing.T) {
		conn, resp, err := websocket.DefaultDialer.Dial(wsURL(srv, "nao-e-jwt"), nil)
		if err == nil {
			conn.Close()
			t.Fatal("handshake com token invalido deveria falhar")
		}
		if resp == nil || resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("status = %v, esperado 401", resp)
		}
	})

	t.Run("token valido faz upgrade", func(t *testing.T) {
		token := tokenPara(t, jwtService, uuid.New())
		conn, _, err := websocket.DefaultDialer.Dial(wsURL(srv, token), nil)
		if err != nil {
			t.Fatalf("handshake falhou: %v", err)
		}
		conn.Close()
	})
}

func TestCheckOrigin(t *testing.T) {
	srv, _, jwtService := newWSServer(t, "https://app.example.com")
	token := tokenPara(t, jwtService, uuid.New())

	t.Run("origin nao listado e rejeitado", func(t *testing.T) {
		header := http.Header{"Origin": []string{"https://evil.example.com"}}
		conn, resp, err := websocket.DefaultDialer.Dial(wsURL(srv, token), header)
		if err == nil {
			conn.Close()
			t.Fatal("origin nao listado deveria ser rejeitado")
		}
		if resp == nil || resp.StatusCode != http.StatusForbidden {
			t.Errorf("status = %v, esperado 403", resp)
		}
	})

	t.Run("origin listado conecta", func(t *testing.T) {
		header := http.Header{"Origin": []string{"https://app.example.com"}}
		conn, _, err := websocket.DefaultDialer.Dial(wsURL(srv, token), header)
		if err != nil {
			t.Fatalf("origin permitido rejeitado: %v", err)
		}
		conn.Close()
	})

	t.Run("cliente sem origin (app mobile) conecta", func(t *testing.T) {
		conn, _, err := websocket.DefaultDialer.Dial(wsURL(srv, token), nil)
		if err != nil {
			t.Fatalf("cliente sem Origin rejeitado: %v", err)
		}
		conn.Close()
	})
}

func TestBroadcastSeletivoPorEmpresa(t *testing.T) {
	srv, hub, jwtService := newWSServer(t, "*")

	empresaA := uuid.New()
	empresaB := uuid.New()

	dial := func(empresaID uuid.UUID) *websocket.Conn {
		conn, _, err := websocket.DefaultDialer.Dial(wsURL(srv, tokenPara(t, jwtService, empresaID)), nil)
		if err != nil {
			t.Fatalf("conectar empresa %s: %v", empresaID, err)
		}
		t.Cleanup(func() { conn.Close() })
		return conn
	}

	connA := dial(empresaA)
	connB := dial(empresaB)

	// aguarda o hub registrar os dois clientes
	deadline := time.Now().Add(2 * time.Second)
	for {
		hub.mu.RLock()
		registrados := len(hub.clients[empresaA.String()]) == 1 && len(hub.clients[empresaB.String()]) == 1
		hub.mu.RUnlock()
		if registrados {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("clientes nao registrados no hub")
		}
		time.Sleep(10 * time.Millisecond)
	}

	turnoID := uuid.New().String()
	hub.Broadcast(empresaA.String(), NewStatusChangeEvent(turnoID, "critico"))

	t.Run("empresa A recebe o evento", func(t *testing.T) {
		_ = connA.SetReadDeadline(time.Now().Add(2 * time.Second))
		_, msg, err := connA.ReadMessage()
		if err != nil {
			t.Fatalf("ler evento: %v", err)
		}

		var ev struct {
			Type    string `json:"type"`
			Payload struct {
				TurnoID string `json:"turno_id"`
				Status  string `json:"status"`
			} `json:"payload"`
		}
		if err := json.Unmarshal(msg, &ev); err != nil {
			t.Fatalf("decodificar evento: %v", err)
		}
		if ev.Type != string(EventStatusChange) || ev.Payload.TurnoID != turnoID || ev.Payload.Status != "critico" {
			t.Errorf("evento inesperado: %s", msg)
		}
	})

	t.Run("empresa B nao recebe", func(t *testing.T) {
		_ = connB.SetReadDeadline(time.Now().Add(300 * time.Millisecond))
		if _, msg, err := connB.ReadMessage(); err == nil {
			t.Errorf("empresa B recebeu evento da empresa A: %s", msg)
		}
	})
}
