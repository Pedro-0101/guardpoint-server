package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"github.com/guardpoint/guardpoint-server/internal/auth"
)

func TestAuthMiddleware(t *testing.T) {
	jwtService := auth.NewJWTService("unit-test-secret-with-enough-length")
	userID := uuid.New()
	empresaID := uuid.New()

	validToken, err := jwtService.GenerateAccessToken(userID, empresaID, "v@e.com", "vigia", "Vigia")
	if err != nil {
		t.Fatalf("gerar token: %v", err)
	}

	var gotUserID, gotEmpresaID string
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUserID = GetUserID(r.Context())
		gotEmpresaID = GetEmpresaID(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	mw := AuthMiddleware(jwtService)(next)

	tests := []struct {
		name       string
		authHeader string
		wantStatus int
	}{
		{"sem header retorna 401", "", http.StatusUnauthorized},
		{"formato invalido retorna 401", "Token abc", http.StatusUnauthorized},
		{"bearer sem token retorna 401", "Bearer", http.StatusUnauthorized},
		{"token invalido retorna 401", "Bearer nao-e-um-jwt", http.StatusUnauthorized},
		{"token valido segue", "Bearer " + validToken, http.StatusOK},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotUserID, gotEmpresaID = "", ""
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}
			rec := httptest.NewRecorder()

			mw.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, esperado %d", rec.Code, tt.wantStatus)
			}
			if tt.wantStatus == http.StatusOK {
				if gotUserID != userID.String() {
					t.Errorf("user_id no contexto = %q, esperado %q", gotUserID, userID)
				}
				if gotEmpresaID != empresaID.String() {
					t.Errorf("empresa_id no contexto = %q, esperado %q", gotEmpresaID, empresaID)
				}
			}
		})
	}
}
