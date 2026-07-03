package service

import (
	"testing"
	"time"

	"github.com/guardpoint/guardpoint-server/internal/model"
)

// Cobre A1: a tolerancia de inicio de turno precisa funcionar tambem para
// escalas noturnas que cruzam a meia-noite (ex.: 22:00 -> 06:00).
func TestVerificarToleranciaEscala(t *testing.T) {
	dia := func(hour, minute int) time.Time {
		return time.Date(2026, 7, 1, hour, minute, 0, 0, time.UTC)
	}

	escala := func(horaInicio, horaFim string, tolMin int) *model.Escala {
		return &model.Escala{
			HoraInicio:    horaInicio,
			HoraFim:       horaFim,
			ToleranciaMin: tolMin,
		}
	}

	tests := []struct {
		name   string
		esc    *model.Escala
		now    time.Time
		wantOK bool
	}{
		// Escala diurna comum.
		{"diurna: no horario exato", escala("08:00", "17:00", 15), dia(8, 0), true},
		{"diurna: 10 min adiantado", escala("08:00", "17:00", 15), dia(7, 50), true},
		{"diurna: 15 min atrasado (limite)", escala("08:00", "17:00", 15), dia(8, 15), true},
		{"diurna: 16 min atrasado", escala("08:00", "17:00", 15), dia(8, 16), false},
		{"diurna: horas depois", escala("08:00", "17:00", 15), dia(13, 0), false},

		// Escala noturna 22:00 -> 06:00.
		{"noturna: no horario exato", escala("22:00", "06:00", 15), dia(22, 0), true},
		{"noturna: 10 min atrasado", escala("22:00", "06:00", 15), dia(22, 10), true},
		{"noturna: 20 min atrasado", escala("22:00", "06:00", 15), dia(22, 20), false},
		{"noturna: apos meia-noite fora da tolerancia", escala("22:00", "06:00", 15), dia(0, 5), false},

		// Escala que comeca perto da meia-noite: o wrap nao pode inflar a diferenca.
		{"inicio 23:50: check as 00:10 do dia seguinte (20 min)", escala("23:50", "07:00", 30), dia(0, 10), true},
		{"inicio 23:50: adiantado as 23:30 (20 min)", escala("23:50", "07:00", 30), dia(23, 30), true},
		{"inicio 23:50: as 01:00 (70 min)", escala("23:50", "07:00", 30), dia(1, 0), false},

		// Escala que comeca a meia-noite.
		{"inicio 00:00: as 23:55 do dia anterior (5 min)", escala("00:00", "08:00", 15), dia(23, 55), true},

		// hora_inicio no formato HH:MM:SS (como retornado pelo Postgres).
		{"hora com segundos", escala("22:00:00", "06:00:00", 15), dia(22, 5), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ok, motivo := VerificarToleranciaEscala(tt.esc, tt.now)
			if ok != tt.wantOK {
				t.Errorf("VerificarToleranciaEscala() = %v (motivo: %q), esperado %v", ok, motivo, tt.wantOK)
			}
		})
	}
}

func TestVerificarToleranciaEscalaInvalida(t *testing.T) {
	now := time.Date(2026, 7, 1, 8, 0, 0, 0, time.UTC)

	if ok, _ := VerificarToleranciaEscala(nil, now); ok {
		t.Error("escala nil deveria ser rejeitada")
	}

	esc := &model.Escala{HoraInicio: "invalida", HoraFim: "06:00", ToleranciaMin: 15}
	if ok, _ := VerificarToleranciaEscala(esc, now); ok {
		t.Error("hora_inicio invalida deveria ser rejeitada")
	}
}
