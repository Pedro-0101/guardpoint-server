package config

import (
	"fmt"
	"os"
	"strings"
)

type Config struct {
	Env          string
	Port         string
	DatabaseURL  string
	LogLevel     string
	LogFormat    string
	JWTSecret    string
	CORSOrigins  []string
	MetricsToken string
}

const jwtSecretPlaceholder = "change-me-in-production"

const jwtSecretMinLen = 32

func Load() (*Config, error) {
	cfg := &Config{
		Env:          getEnv("ENV", "development"),
		Port:         getEnv("PORT", "8080"),
		DatabaseURL:  getEnv("DATABASE_URL", ""),
		LogLevel:     getEnv("LOG_LEVEL", "info"),
		LogFormat:    getEnv("LOG_FORMAT", "text"),
		JWTSecret:    getEnv("JWT_SECRET", ""),
		CORSOrigins:  splitCSV(getEnv("CORS_ORIGINS", getEnv("CORS_ORIGIN", "*"))),
		MetricsToken: getEnv("METRICS_TOKEN", ""),
	}

	var missing []string
	if cfg.DatabaseURL == "" {
		missing = append(missing, "DATABASE_URL")
	}
	if cfg.JWTSecret == "" {
		missing = append(missing, "JWT_SECRET")
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("required environment variables not set: %s", strings.Join(missing, ", "))
	}

	if cfg.Env == "production" {
		if err := validateProduction(cfg); err != nil {
			return nil, err
		}
	}

	return cfg, nil
}

// validateProduction recusa defaults inseguros quando ENV=production (B2):
// segredo JWT fraco/placeholder e CORS aberto.
func validateProduction(cfg *Config) error {
	if cfg.JWTSecret == jwtSecretPlaceholder {
		return fmt.Errorf("JWT_SECRET is still the placeholder %q; set a real secret in production", jwtSecretPlaceholder)
	}
	if len(cfg.JWTSecret) < jwtSecretMinLen {
		return fmt.Errorf("JWT_SECRET must have at least %d characters in production", jwtSecretMinLen)
	}
	if len(cfg.CORSOrigins) == 0 {
		return fmt.Errorf("CORS_ORIGINS must list explicit origins in production")
	}
	for _, o := range cfg.CORSOrigins {
		if o == "*" {
			return fmt.Errorf("CORS_ORIGINS must list explicit origins in production (got %q)", o)
		}
	}
	return nil
}

// splitCSV separa uma lista separada por virgulas, descartando entradas vazias.
func splitCSV(s string) []string {
	var out []string
	for _, part := range strings.Split(s, ",") {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}
