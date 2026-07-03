package service

import "testing"

func TestGeneratePIN(t *testing.T) {
	for range 200 {
		pin, err := generatePIN(6)
		if err != nil {
			t.Fatalf("generatePIN() erro inesperado: %v", err)
		}
		if len(pin) != 6 {
			t.Fatalf("generatePIN() = %q, esperado 6 digitos (zero-padded)", pin)
		}
		for _, c := range pin {
			if c < '0' || c > '9' {
				t.Fatalf("generatePIN() = %q contem caractere nao numerico", pin)
			}
		}
	}
}
