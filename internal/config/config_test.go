package config

import (
	"os"
	"strings"
	"testing"
)

func setBaseEnv(t *testing.T) {
	t.Helper()
	t.Setenv("DATABASE_URL", "postgres://x:y@localhost:5432/db")
	t.Setenv("JWT_SECRET", "um-segredo-forte-com-mais-de-32-caracteres")
	t.Setenv("CORS_ORIGINS", "https://app.example.com")
	t.Setenv("METRICS_TOKEN", "")
}

func TestLoadDevelopmentAceitaDefaults(t *testing.T) {
	setBaseEnv(t)
	t.Setenv("ENV", "development")
	t.Setenv("JWT_SECRET", "change-me-in-production")
	t.Setenv("CORS_ORIGINS", "*")

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
	t.Setenv("CORS_ORIGINS", "*")

	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "CORS_ORIGINS") {
		t.Fatalf("Load() deveria rejeitar CORS_ORIGINS=*, retornou: %v", err)
	}
}

func TestLoadProductionValida(t *testing.T) {
	setBaseEnv(t)
	t.Setenv("ENV", "production")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() com config valida falhou: %v", err)
	}
	if len(cfg.CORSOrigins) != 1 || cfg.CORSOrigins[0] != "https://app.example.com" {
		t.Errorf("CORSOrigins = %q", cfg.CORSOrigins)
	}
}

func TestLoadCORSOriginsCSV(t *testing.T) {
	setBaseEnv(t)
	t.Setenv("CORS_ORIGINS", "http://localhost:4200, https://app.example.com ,")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() falhou: %v", err)
	}
	want := []string{"http://localhost:4200", "https://app.example.com"}
	if len(cfg.CORSOrigins) != len(want) {
		t.Fatalf("CORSOrigins = %q, esperado %q", cfg.CORSOrigins, want)
	}
	for i := range want {
		if cfg.CORSOrigins[i] != want[i] {
			t.Errorf("CORSOrigins[%d] = %q, esperado %q", i, cfg.CORSOrigins[i], want[i])
		}
	}
}

func TestLoadCORSOriginLegadoComoFallback(t *testing.T) {
	setBaseEnv(t)
	os.Unsetenv("CORS_ORIGINS")
	t.Setenv("CORS_ORIGIN", "https://legado.example.com")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() falhou: %v", err)
	}
	if len(cfg.CORSOrigins) != 1 || cfg.CORSOrigins[0] != "https://legado.example.com" {
		t.Errorf("CORSOrigins = %q", cfg.CORSOrigins)
	}
}
