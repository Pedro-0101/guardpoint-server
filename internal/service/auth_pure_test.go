package service

import (
	"encoding/hex"
	"testing"
)

func TestGerarDeviceSecret(t *testing.T) {
	for range 100 {
		secret, err := gerarDeviceSecret()
		if err != nil {
			t.Fatalf("gerarDeviceSecret() erro: %v", err)
		}
		decoded, err := hex.DecodeString(secret)
		if err != nil {
			t.Fatalf("secret nao e hex valido: %v", err)
		}
		if len(decoded) != 32 {
			t.Errorf("len(decoded) = %d, esperado 32", len(decoded))
		}
	}
}

func TestHashDeviceSecret(t *testing.T) {
	hash := hashDeviceSecret("meu-secret")
	if hash == "" {
		t.Error("hash vazio")
	}
	if hash == "meu-secret" {
		t.Error("hash nao pode ser o texto puro")
	}

	decoded, err := hex.DecodeString(hash)
	if err != nil {
		t.Fatalf("hash nao e hex valido: %v", err)
	}
	if len(decoded) != 32 {
		t.Errorf("len(decoded) = %d, esperado 32 (SHA-256)", len(decoded))
	}
}

func TestDeviceSecretConfere(t *testing.T) {
	secret := "abc123secret"
	hash := hashDeviceSecret(secret)

	t.Run("confere corretamente", func(t *testing.T) {
		if !deviceSecretConfere(secret, hash) {
			t.Error("deviceSecretConfere deveria retornar true")
		}
	})

	t.Run("rejeita secret errado", func(t *testing.T) {
		if deviceSecretConfere("secret-errado", hash) {
			t.Error("deviceSecretConfere deveria retornar false")
		}
	})

	t.Run("hash deterministico", func(t *testing.T) {
		hash2 := hashDeviceSecret(secret)
		if hash != hash2 {
			t.Error("hash nao e deterministico")
		}
	})
}
