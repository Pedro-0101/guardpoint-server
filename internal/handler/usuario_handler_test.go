package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-playground/validator/v10"
)

func TestUsuarioHandler_List_EmpresaIDInvalido(t *testing.T) {
	h := &UsuarioHandler{validate: validator.New()}

	req := httptest.NewRequest(http.MethodGet, "/usuarios", nil)
	rec := httptest.NewRecorder()

	h.List(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, esperado %d", rec.Code, http.StatusBadRequest)
	}
}

func TestUsuarioHandler_List_SemEmpresaID(t *testing.T) {
	h := &UsuarioHandler{validate: validator.New()}

	req := httptest.NewRequest(http.MethodGet, "/usuarios", nil)
	ctx := context.WithValue(req.Context(), CtxKeyEmpresaID, "")
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	h.List(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, esperado %d", rec.Code, http.StatusBadRequest)
	}
}

func TestUsuarioHandler_GetByID_IDInvalido(t *testing.T) {
	h := &UsuarioHandler{validate: validator.New()}

	req := httptest.NewRequest(http.MethodGet, "/usuarios/xyz", nil)
	ctx := context.WithValue(req.Context(), CtxKeyEmpresaID, "550e8400-e29b-41d4-a716-446655440000")
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	h.GetByID(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, esperado %d", rec.Code, http.StatusBadRequest)
	}
}

func TestUsuarioHandler_GetByID_EmpresaIDInvalido(t *testing.T) {
	h := &UsuarioHandler{validate: validator.New()}

	req := httptest.NewRequest(http.MethodGet, "/usuarios/550e8400-e29b-41d4-a716-446655440000", nil)
	ctx := context.WithValue(req.Context(), CtxKeyEmpresaID, "invalido")
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	h.GetByID(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, esperado %d", rec.Code, http.StatusBadRequest)
	}
}

func TestUsuarioHandler_Create_JsonInvalido(t *testing.T) {
	h := &UsuarioHandler{validate: validator.New()}

	req := httptest.NewRequest(http.MethodPost, "/usuarios", bytes.NewReader([]byte("{")))
	ctx := context.WithValue(req.Context(), CtxKeyEmpresaID, "550e8400-e29b-41d4-a716-446655440000")
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	h.Create(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, esperado %d", rec.Code, http.StatusBadRequest)
	}
}

func TestUsuarioHandler_Create_ValidacaoFalha(t *testing.T) {
	h := &UsuarioHandler{validate: validator.New()}

	body, _ := json.Marshal(map[string]string{"email": "nao-e-email"})
	req := httptest.NewRequest(http.MethodPost, "/usuarios", bytes.NewReader(body))
	ctx := context.WithValue(req.Context(), CtxKeyEmpresaID, "550e8400-e29b-41d4-a716-446655440000")
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	h.Create(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, esperado %d", rec.Code, http.StatusUnprocessableEntity)
	}
}

func TestUsuarioHandler_Create_EmpresaIDInvalido(t *testing.T) {
	h := &UsuarioHandler{validate: validator.New()}

	req := httptest.NewRequest(http.MethodPost, "/usuarios", bytes.NewReader([]byte(`{}`)))
	ctx := context.WithValue(req.Context(), CtxKeyEmpresaID, "invalido")
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	h.Create(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, esperado %d", rec.Code, http.StatusBadRequest)
	}
}

func TestUsuarioHandler_Update_IDInvalido(t *testing.T) {
	h := &UsuarioHandler{validate: validator.New()}

	body, _ := json.Marshal(map[string]string{})
	req := httptest.NewRequest(http.MethodPut, "/usuarios/xyz", bytes.NewReader(body))
	ctx := context.WithValue(req.Context(), CtxKeyEmpresaID, "550e8400-e29b-41d4-a716-446655440000")
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	h.Update(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, esperado %d", rec.Code, http.StatusBadRequest)
	}
}

func TestUsuarioHandler_Update_EmpresaIDInvalido(t *testing.T) {
	h := &UsuarioHandler{validate: validator.New()}

	body, _ := json.Marshal(map[string]string{})
	req := httptest.NewRequest(http.MethodPut, "/usuarios/550e8400-e29b-41d4-a716-446655440000", bytes.NewReader(body))
	ctx := context.WithValue(req.Context(), CtxKeyEmpresaID, "invalido")
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	h.Update(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, esperado %d", rec.Code, http.StatusBadRequest)
	}
}

func TestUsuarioHandler_Update_JsonInvalido(t *testing.T) {
	h := &UsuarioHandler{validate: validator.New()}

	req := httptest.NewRequest(http.MethodPut, "/usuarios/550e8400-e29b-41d4-a716-446655440000", bytes.NewReader([]byte("{")))
	ctx := context.WithValue(req.Context(), CtxKeyEmpresaID, "550e8400-e29b-41d4-a716-446655440000")
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	h.Update(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, esperado %d", rec.Code, http.StatusBadRequest)
	}
}

func TestUsuarioHandler_Delete_IDInvalido(t *testing.T) {
	h := &UsuarioHandler{validate: validator.New()}

	req := httptest.NewRequest(http.MethodDelete, "/usuarios/xyz", nil)
	ctx := context.WithValue(req.Context(), CtxKeyEmpresaID, "550e8400-e29b-41d4-a716-446655440000")
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	h.Delete(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, esperado %d", rec.Code, http.StatusBadRequest)
	}
}

func TestUsuarioHandler_Delete_EmpresaIDInvalido(t *testing.T) {
	h := &UsuarioHandler{validate: validator.New()}

	req := httptest.NewRequest(http.MethodDelete, "/usuarios/550e8400-e29b-41d4-a716-446655440000", nil)
	ctx := context.WithValue(req.Context(), CtxKeyEmpresaID, "invalido")
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	h.Delete(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, esperado %d", rec.Code, http.StatusBadRequest)
	}
}
