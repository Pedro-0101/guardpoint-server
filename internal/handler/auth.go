package handler

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-playground/validator/v10"

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
// @Summary      Login por email e senha
// @Tags         auth
// @Security
// @Param        request body model.LoginRequest true "Credenciais"
// @Success      200 {object} model.LoginResponse
// @Failure      400 {object} model.ErrorResponse
// @Failure      401 {object} model.ErrorResponse
// @Failure      403 {object} model.ErrorResponse
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
// @Tags         auth
// @Param        request body model.RegisterRequest true "Dados do usuario"
// @Success      201 {object} model.User
// @Failure      400 {object} model.ErrorResponse
// @Failure      401 {object} model.ErrorResponse
// @Failure      409 {object} model.ErrorResponse
// @Router       /auth/register [post]
func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	empresaID := GetEmpresaID(r.Context())
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
	empresaID := GetEmpresaID(r.Context())

	var req struct {
		DeviceID string `json:"device_id"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)

	if err := h.authService.Logout(r.Context(), empresaID, req.DeviceID); err != nil {
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
	userID := GetUserID(r.Context())
	empresaID := GetEmpresaID(r.Context())
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

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		slog.Error("encode json response", "error", err)
	}
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func writeValidationError(w http.ResponseWriter, err error) {
	var errMsg string
	var validationErrors validator.ValidationErrors
	if errors.As(err, &validationErrors) {
		errMsg = "validacao falhou: "
		for i, fe := range validationErrors {
			if i > 0 {
				errMsg += "; "
			}
			errMsg += fe.Field()
		}
	} else {
		errMsg = err.Error()
	}
	writeError(w, http.StatusUnprocessableEntity, errMsg)
}
