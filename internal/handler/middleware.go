package handler

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/guardpoint/guardpoint-server/internal/auth"
)

type contextKey string

const (
	CtxKeyUserID    contextKey = "user_id"
	CtxKeyEmpresaID contextKey = "empresa_id"
	CtxKeyEmail     contextKey = "email"
	CtxKeyRole      contextKey = "role"
	CtxKeyNome      contextKey = "nome"
)

func AuthMiddleware(jwtService *auth.JWTService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				writeError(w, http.StatusUnauthorized, "token nao fornecido")
				return
			}

			tokenString := authHeader
			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
				tokenString = parts[1]
			}

			claims, err := jwtService.ValidateToken(tokenString)
			if err != nil {
				if errors.Is(err, auth.ErrTokenExpired) {
					writeError(w, http.StatusUnauthorized, "token expirado")
					return
				}
				writeError(w, http.StatusUnauthorized, "token invalido")
				return
			}

			ctx := context.WithValue(r.Context(), CtxKeyUserID, claims.UserID)
			ctx = context.WithValue(ctx, CtxKeyEmpresaID, claims.EmpresaID)
			ctx = context.WithValue(ctx, CtxKeyEmail, claims.Email)
			ctx = context.WithValue(ctx, CtxKeyRole, claims.Role)
			ctx = context.WithValue(ctx, CtxKeyNome, claims.Nome)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func GetEmpresaID(ctx context.Context) string {
	if v, ok := ctx.Value(CtxKeyEmpresaID).(string); ok {
		return v
	}
	return ""
}

func GetUserID(ctx context.Context) string {
	if v, ok := ctx.Value(CtxKeyUserID).(string); ok {
		return v
	}
	return ""
}

func GetRole(ctx context.Context) string {
	if v, ok := ctx.Value(CtxKeyRole).(string); ok {
		return v
	}
	return ""
}

func GetNome(ctx context.Context) string {
	if v, ok := ctx.Value(CtxKeyNome).(string); ok {
		return v
	}
	return ""
}
