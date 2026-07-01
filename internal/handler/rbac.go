package handler

import (
	"net/http"
	"slices"
)

func RequireRole(roles ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userRole := GetRole(r.Context())
			if userRole == "" {
				writeError(w, http.StatusUnauthorized, "autenticacao necessaria")
				return
			}

			if !slices.Contains(roles, userRole) {
				writeError(w, http.StatusForbidden, "acesso negado")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
