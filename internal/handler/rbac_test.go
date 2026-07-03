package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRequireRole(t *testing.T) {
	okHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	tests := []struct {
		name       string
		role       string
		allowed    []string
		wantStatus int
	}{
		{"sem role no contexto retorna 401", "", []string{"admin"}, http.StatusUnauthorized},
		{"role nao permitido retorna 403", "vigia", []string{"admin"}, http.StatusForbidden},
		{"role permitido segue", "admin", []string{"admin"}, http.StatusOK},
		{"segundo role da lista permitido", "supervisor", []string{"admin", "supervisor"}, http.StatusOK},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if tt.role != "" {
				req = req.WithContext(context.WithValue(req.Context(), CtxKeyRole, tt.role))
			}
			rec := httptest.NewRecorder()

			RequireRole(tt.allowed...)(okHandler).ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, esperado %d", rec.Code, tt.wantStatus)
			}
		})
	}
}
