package config

import (
	"strings"
	"testing"
)

func setBaseEnv(t *testing.T) {
	t.Helper()
	t.Setenv("DATABASE_URL", "postgres://x:y@localhost:5432/db")
	t.Setenv("JWT_SECRET", "um-segredo-forte-com-mais-de-32-caracteres")
	t.Setenv("CORS_ORIGIN", "https://app.example.com")
	t.Setenv("METRICS_TOKEN", "")
}

func TestLoadDevelopmentAceitaDefaults(t *testing.T) {
	setBaseEnv(t)
	t.Setenv("ENV", "development")
	t.Setenv("JWT_SECRET", "change-me-in-production")
	t.Setenv("CORS_ORIGIN", "*")

	if _, err := Load(); err != nil {
		t.Fatalf("Load() em development nao deveria falhar: %v", err)
	}
}

func TestLoadProductionRejeitaPlaceholder(t *testing.T) {
	setBaseEnv(t)
	t.Setenv("ENV", "production")
	t.Setenv("JWT_SECRET", "change-me-in-production")

	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "JWT_SECRET") {
		t.Fatalf("Load() deveria rejeitar o placeholder de JWT_SECRET, retornou: %v", err)
	}
}

func TestLoadProductionRejeitaSegredoCurto(t *testing.T) {
	setBaseEnv(t)
	t.Setenv("ENV", "production")
	t.Setenv("JWT_SECRET", "curto")

	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "JWT_SECRET") {
		t.Fatalf("Load() deveria rejeitar segredo curto, retornou: %v", err)
	}
}

func TestLoadProductionRejeitaCORSAberto(t *testing.T) {
	setBaseEnv(t)
	t.Setenv("ENV", "production")
	t.Setenv("CORS_ORIGIN", "*")

	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "CORS_ORIGIN") {
		t.Fatalf("Load() deveria rejeitar CORS_ORIGIN=*, retornou: %v", err)
	}
}

func TestLoadProductionValida(t *testing.T) {
	setBaseEnv(t)
	t.Setenv("ENV", "production")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() com config valida falhou: %v", err)
	}
	if cfg.CORSOrigin != "https://app.example.com" {
		t.Errorf("CORSOrigin = %q", cfg.CORSOrigin)
	}
}
