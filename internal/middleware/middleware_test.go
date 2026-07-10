package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRequireRole_SemRole(t *testing.T) {
	handler := RequireRole("admin")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, esperado %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestRequireRole_RoleNaoAutorizada(t *testing.T) {
	handler := RequireRole("admin")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	ctx := context.WithValue(req.Context(), CtxKeyRole, "vigia")
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, esperado %d", rec.Code, http.StatusForbidden)
	}
}

func TestRequireRole_Autorizada(t *testing.T) {
	handler := RequireRole("admin", "supervisor")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	ctx := context.WithValue(req.Context(), CtxKeyRole, "admin")
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, esperado %d", rec.Code, http.StatusOK)
	}
}

func TestGetEmpresaID_Vazio(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	if id := GetEmpresaID(req.Context()); id != "" {
		t.Errorf("empresa_id = %q, esperado vazio", id)
	}
}

func TestGetEmpresaID_Presente(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	ctx := context.WithValue(req.Context(), CtxKeyEmpresaID, "empresa-123")
	req = req.WithContext(ctx)
	if id := GetEmpresaID(req.Context()); id != "empresa-123" {
		t.Errorf("empresa_id = %q, esperado empresa-123", id)
	}
}

func TestGetUserID_Vazio(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	if id := GetUserID(req.Context()); id != "" {
		t.Errorf("user_id = %q, esperado vazio", id)
	}
}

func TestGetRole_Vazio(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	if role := GetRole(req.Context()); role != "" {
		t.Errorf("role = %q, esperado vazio", role)
	}
}

func TestGetNome_Vazio(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	if nome := GetNome(req.Context()); nome != "" {
		t.Errorf("nome = %q, esperado vazio", nome)
	}
}
