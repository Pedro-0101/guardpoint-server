package handler

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-playground/validator/v10"
)

func TestAlertaHandler_CreateEscalonamento_JsonInvalido(t *testing.T) {
	h := &AlertaHandler{validate: validator.New()}

	req := httptest.NewRequest(http.MethodPost, "/config/escalonamento", bytes.NewReader([]byte("{")))
	ctx := context.WithValue(req.Context(), CtxKeyEmpresaID, "550e8400-e29b-41d4-a716-446655440000")
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	h.CreateEscalonamento(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, esperado %d", rec.Code, http.StatusBadRequest)
	}
}

func TestPostoHandler_GetByID_IDInvalido(t *testing.T) {
	h := &PostoHandler{validate: validator.New()}

	req := httptest.NewRequest(http.MethodGet, "/postos/xyz", nil)
	ctx := context.WithValue(req.Context(), CtxKeyEmpresaID, "550e8400-e29b-41d4-a716-446655440000")
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	h.GetByID(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, esperado %d", rec.Code, http.StatusBadRequest)
	}
}

func TestPostoHandler_Create_JsonInvalido(t *testing.T) {
	h := &PostoHandler{validate: validator.New()}

	req := httptest.NewRequest(http.MethodPost, "/postos", bytes.NewReader([]byte("{")))
	ctx := context.WithValue(req.Context(), CtxKeyEmpresaID, "550e8400-e29b-41d4-a716-446655440000")
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	h.Create(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, esperado %d", rec.Code, http.StatusBadRequest)
	}
}
