package handler

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-playground/validator/v10"

	"github.com/guardpoint/guardpoint-server/internal/middleware"
	"github.com/guardpoint/guardpoint-server/internal/model"
	"github.com/guardpoint/guardpoint-server/internal/service"
)

type AuthHandler struct {
	authService *service.AuthService
	validate    *validator.Validate
}

func NewAuthHandler(authService *service.AuthService) *AuthHandler {
	return &AuthHandler{
		authService: authService,
		validate:    validator.New(),
	}
}

// Login godoc
// @Summary      Login por email+senha (admin/supervisor) ou codigo da empresa+nome+senha (vigia)
// @Description  Admin e supervisor autenticam com "email" + "senha". Vigia autentica com "codigo_empresa" + "nome" + "senha" (nao usa email). Enviar exatamente um dos dois pares de credenciais.
// @Tags         auth
// @Security
// @Param        request body model.LoginRequest true "Credenciais (email+senha OU codigo_empresa+nome+senha)"
// @Success      200 {object} model.LoginResponse
// @Failure      400 {object} model.ErrorResponse
// @Failure      401 {object} model.ErrorResponse
// @Failure      403 {object} model.ErrorResponse
// @Failure      422 {object} model.ErrorResponse
// @Router       /auth/login [post]
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req model.LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "json invalido")
		return
	}

	if err := h.validate.Struct(req); err != nil {
		writeValidationError(w, err)
		return
	}

	resp, err := h.authService.Login(r.Context(), req)
	if err != nil {
		if errors.Is(err, service.ErrInvalidCredentials) {
			writeError(w, http.StatusUnauthorized, "email ou senha invalidos")
			return
		}
		if errors.Is(err, service.ErrUserNotActive) {
			writeError(w, http.StatusForbidden, "usuario inativo")
			return
		}
		slog.Error("login failed", "error", err)
		writeError(w, http.StatusInternalServerError, "erro interno")
		return
	}

	writeJSON(w, http.StatusOK, resp)
}

// Register godoc
// @Summary      Cria um novo usuario (somente admin)
// @Description  "email" e obrigatorio para role "admin"/"supervisor" e opcional para "vigia". "nome" precisa ser unico dentro da empresa (pode repetir entre empresas diferentes); "email", quando enviado, e unico globalmente.
// @Tags         auth
// @Param        request body model.RegisterRequest true "Dados do usuario"
// @Success      201 {object} model.User
// @Failure      400 {object} model.ErrorResponse
// @Failure      401 {object} model.ErrorResponse
// @Failure      409 {object} model.ErrorResponse
// @Failure      422 {object} model.ErrorResponse
// @Router       /auth/register [post]
func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	empresaID := middleware.GetEmpresaID(r.Context())
	if empresaID == "" {
		writeError(w, http.StatusUnauthorized, "autenticacao necessaria")
		return
	}

	var req model.RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "json invalido")
		return
	}

	if err := h.validate.Struct(req); err != nil {
		writeValidationError(w, err)
		return
	}

	user, err := h.authService.Register(r.Context(), empresaID, req)
	if err != nil {
		if errors.Is(err, service.ErrEmailAlreadyExists) {
			writeError(w, http.StatusConflict, "email ja cadastrado")
			return
		}
		if errors.Is(err, service.ErrNomeAlreadyExists) {
			writeError(w, http.StatusConflict, "nome ja cadastrado")
			return
		}
		slog.Error("register failed", "error", err)
		writeError(w, http.StatusInternalServerError, "erro interno")
		return
	}

	writeJSON(w, http.StatusCreated, user)
}

// Refresh godoc
// @Summary      Renova o access token a partir do refresh token
// @Tags         auth
// @Security
// @Param        request body model.RefreshRequest true "Refresh token"
// @Success      200 {object} model.LoginResponse
// @Failure      400 {object} model.ErrorResponse
// @Failure      401 {object} model.ErrorResponse
// @Router       /auth/refresh [post]
func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	var req model.RefreshRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "json invalido")
		return
	}

	if err := h.validate.Struct(req); err != nil {
		writeValidationError(w, err)
		return
	}

	resp, err := h.authService.Refresh(r.Context(), req)
	if err != nil {
		slog.Error("refresh failed", "error", err)
		writeError(w, http.StatusUnauthorized, "refresh token invalido ou expirado")
		return
	}

	writeJSON(w, http.StatusOK, resp)
}

// Logout godoc
// @Summary      Encerra a sessao do dispositivo atual
// @Tags         auth
// @Success      200 {object} model.MessageResponse
// @Failure      500 {object} model.ErrorResponse
// @Router       /auth/logout [post]
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	empresaID := middleware.GetEmpresaID(r.Context())
	userID := middleware.GetUserID(r.Context())

	var req struct {
		DeviceID string `json:"device_id"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)

	if err := h.authService.Logout(r.Context(), empresaID, userID, req.DeviceID); err != nil {
		slog.Error("logout failed", "error", err)
		writeError(w, http.StatusInternalServerError, "erro ao processar logout")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "logout realizado com sucesso"})
}

// BiometricLogin godoc
// @Summary      Login via dispositivo biometrico registrado
// @Tags         auth
// @Security
// @Param        request body model.BiometricLoginRequest true "Credenciais do dispositivo"
// @Success      200 {object} model.LoginResponse
// @Failure      400 {object} model.ErrorResponse
// @Failure      401 {object} model.ErrorResponse
// @Router       /auth/biometric/login [post]
func (h *AuthHandler) BiometricLogin(w http.ResponseWriter, r *http.Request) {
	var req model.BiometricLoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "json invalido")
		return
	}

	if err := h.validate.Struct(req); err != nil {
		writeValidationError(w, err)
		return
	}

	resp, err := h.authService.BiometricLogin(r.Context(), req)
	if err != nil {
		slog.Error("biometric login failed", "error", err)
		writeError(w, http.StatusUnauthorized, "dispositivo nao reconhecido")
		return
	}

	writeJSON(w, http.StatusOK, resp)
}

// BiometricRegister godoc
// @Summary      Registra um dispositivo biometrico para o usuario autenticado
// @Tags         auth
// @Param        request body model.BiometricRegisterRequest true "Dados do dispositivo"
// @Success      201 {object} model.BiometricRegisterResponse
// @Failure      400 {object} model.ErrorResponse
// @Failure      401 {object} model.ErrorResponse
// @Router       /auth/biometric/register [post]
func (h *AuthHandler) BiometricRegister(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	empresaID := middleware.GetEmpresaID(r.Context())
	if userID == "" || empresaID == "" {
		writeError(w, http.StatusUnauthorized, "autenticacao necessaria")
		return
	}

	var req model.BiometricRegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "json invalido")
		return
	}

	if err := h.validate.Struct(req); err != nil {
		writeValidationError(w, err)
		return
	}

	sessao, err := h.authService.RegisterBiometric(r.Context(), userID, empresaID, req)
	if err != nil {
		slog.Error("biometric register failed", "error", err)
		writeError(w, http.StatusInternalServerError, "erro ao registrar dispositivo")
		return
	}

	writeJSON(w, http.StatusCreated, sessao)
}


