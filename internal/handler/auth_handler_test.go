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

func TestWriteJSON(t *testing.T) {
	rec := httptest.NewRecorder()
	writeJSON(rec, http.StatusOK, map[string]string{"message": "ok"})

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, esperado %d", rec.Code, http.StatusOK)
	}

	contentType := rec.Header().Get("Content-Type")
	if contentType != "application/json; charset=utf-8" {
		t.Errorf("Content-Type = %q, esperado application/json; charset=utf-8", contentType)
	}

	var body map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body["message"] != "ok" {
		t.Errorf("message = %q, esperado ok", body["message"])
	}
}

func TestWriteError(t *testing.T) {
	rec := httptest.NewRecorder()
	writeError(rec, http.StatusBadRequest, "json invalido")

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, esperado %d", rec.Code, http.StatusBadRequest)
	}

	var body map[string]string
	json.NewDecoder(rec.Body).Decode(&body)
	if body["error"] != "json invalido" {
		t.Errorf("error = %q, esperado json invalido", body["error"])
	}
}

func TestWriteValidationError(t *testing.T) {
	validate := validator.New()

	type testRequest struct {
		Nome string `json:"nome" validate:"required"`
	}

	err := validate.Struct(testRequest{})
	if err == nil {
		t.Fatal("validacao deveria falhar")
	}

	rec := httptest.NewRecorder()
	writeValidationError(rec, err)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, esperado %d", rec.Code, http.StatusUnprocessableEntity)
	}
}

func TestWriteError_InternalServerError(t *testing.T) {
	rec := httptest.NewRecorder()
	writeError(rec, http.StatusInternalServerError, "erro interno")

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, esperado %d", rec.Code, http.StatusInternalServerError)
	}
}

func TestWriteError_NotFound(t *testing.T) {
	rec := httptest.NewRecorder()
	writeError(rec, http.StatusNotFound, "nao encontrado")

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, esperado %d", rec.Code, http.StatusNotFound)
	}
}

func TestAuthHandler_Login_JsonInvalido(t *testing.T) {
	h := &AuthHandler{validate: validator.New()}

	req := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewReader([]byte("invalid json")))
	rec := httptest.NewRecorder()

	h.Login(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, esperado %d", rec.Code, http.StatusBadRequest)
	}
}

func TestAuthHandler_Login_ValidacaoFalha(t *testing.T) {
	h := &AuthHandler{validate: validator.New()}

	type loginReq struct {
		Email    string `json:"email" validate:"required,email"`
		Password string `json:"senha" validate:"required,min=6"`
	}

	body, _ := json.Marshal(loginReq{Email: "nao-e-email"})
	req := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	h.Login(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, esperado %d (422)", rec.Code, http.StatusUnprocessableEntity)
	}
}

func TestAuthHandler_Register_SemAutenticacao(t *testing.T) {
	h := &AuthHandler{validate: validator.New()}

	req := httptest.NewRequest(http.MethodPost, "/auth/register", bytes.NewReader([]byte(`{}`)))
	rec := httptest.NewRecorder()

	h.Register(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, esperado %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestAuthHandler_Register_JsonInvalido(t *testing.T) {
	h := &AuthHandler{validate: validator.New()}

	req := httptest.NewRequest(http.MethodPost, "/auth/register", bytes.NewReader([]byte("{")))
	ctx := context.WithValue(req.Context(), CtxKeyEmpresaID, "550e8400-e29b-41d4-a716-446655440000")
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	h.Register(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, esperado %d", rec.Code, http.StatusBadRequest)
	}
}

func TestAuthHandler_Refresh_JsonInvalido(t *testing.T) {
	h := &AuthHandler{validate: validator.New()}

	req := httptest.NewRequest(http.MethodPost, "/auth/refresh", bytes.NewReader([]byte("{")))
	rec := httptest.NewRecorder()

	h.Refresh(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, esperado %d", rec.Code, http.StatusBadRequest)
	}
}

func TestAuthHandler_Refresh_Validacao(t *testing.T) {
	h := &AuthHandler{validate: validator.New()}

	body, _ := json.Marshal(map[string]string{})
	req := httptest.NewRequest(http.MethodPost, "/auth/refresh", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	h.Refresh(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, esperado %d (422)", rec.Code, http.StatusUnprocessableEntity)
	}
}

func TestAuthHandler_BiometricLogin_Validacao(t *testing.T) {
	h := &AuthHandler{validate: validator.New()}

	body, _ := json.Marshal(map[string]interface{}{})
	req := httptest.NewRequest(http.MethodPost, "/auth/biometric/login", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	h.BiometricLogin(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, esperado %d (422)", rec.Code, http.StatusUnprocessableEntity)
	}
}

func TestAuthHandler_BiometricLogin_JsonInvalido(t *testing.T) {
	h := &AuthHandler{validate: validator.New()}

	req := httptest.NewRequest(http.MethodPost, "/auth/biometric/login", bytes.NewReader([]byte("{")))
	rec := httptest.NewRecorder()

	h.BiometricLogin(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, esperado %d", rec.Code, http.StatusBadRequest)
	}
}

func TestAuthHandler_BiometricRegister_SemAutenticacao(t *testing.T) {
	h := &AuthHandler{validate: validator.New()}

	req := httptest.NewRequest(http.MethodPost, "/auth/biometric/register", bytes.NewReader([]byte(`{}`)))
	rec := httptest.NewRecorder()

	h.BiometricRegister(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, esperado %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestAuthHandler_BiometricRegister_JsonInvalido(t *testing.T) {
	h := &AuthHandler{validate: validator.New()}

	req := httptest.NewRequest(http.MethodPost, "/auth/biometric/register", bytes.NewReader([]byte("{")))
	ctx := context.WithValue(req.Context(), CtxKeyEmpresaID, "550e8400-e29b-41d4-a716-446655440000")
	ctx = context.WithValue(ctx, CtxKeyUserID, "550e8400-e29b-41d4-a716-446655440001")
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	h.BiometricRegister(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, esperado %d", rec.Code, http.StatusBadRequest)
	}
}
