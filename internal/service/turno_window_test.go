package service

import (
	"testing"
	"time"
)

// Cobre a regressao A2: o campo `atrasado` da resposta de check-in deve refletir
// a janela deslizante (ultimo check-in anterior + intervalo_min).
func TestCheckinAtrasado(t *testing.T) {
	base := time.Date(2026, 7, 1, 8, 0, 0, 0, time.UTC)
	intervaloMin := 30

	ptr := func(t time.Time) *time.Time { return &t }

	tests := []struct {
		name       string
		anterior   *time.Time
		inicioReal *time.Time
		ts         time.Time
		want       bool
	}{
		{
			name:     "dentro da janela (25 min apos o anterior)",
			anterior: ptr(base),
			ts:       base.Add(25 * time.Minute),
			want:     false,
		},
		{
			name:     "exatamente no deadline nao e atraso",
			anterior: ptr(base),
			ts:       base.Add(30 * time.Minute),
			want:     false,
		},
		{
			name:     "apos o deadline e atraso",
			anterior: ptr(base),
			ts:       base.Add(31 * time.Minute),
			want:     true,
		},
		{
			name:       "primeiro check-in usa inicio real como ancora - dentro",
			anterior:   nil,
			inicioReal: ptr(base),
			ts:         base.Add(20 * time.Minute),
			want:       false,
		},
		{
			name:       "primeiro check-in usa inicio real como ancora - atrasado",
			anterior:   nil,
			inicioReal: ptr(base),
			ts:         base.Add(45 * time.Minute),
			want:       true,
		},
		{
			name:     "sem ancora nenhuma nao ha atraso",
			anterior: nil,
			ts:       base.Add(2 * time.Hour),
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := checkinAtrasado(tt.anterior, tt.inicioReal, intervaloMin, tt.ts)
			if got != tt.want {
				t.Errorf("checkinAtrasado() = %v, esperado %v", got, tt.want)
			}
		})
	}
}
