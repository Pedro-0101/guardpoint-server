package service

import (
	"testing"

	"github.com/google/uuid"

	"github.com/guardpoint/guardpoint-server/internal/model"
	"github.com/guardpoint/guardpoint-server/internal/timeutil"
)

func TestHoraEmMinutos(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    int
		wantErr bool
	}{
		{"meia-noite", "00:00", 0, false},
		{"08:00", "08:00", 480, false},
		{"12:00", "12:00", 720, false},
		{"23:59", "23:59", 1439, false},
		{"24:00 vira 0", "24:00", 0, false},
		{"com segundos", "22:00:00", 1320, false},
		{"invalido abc", "abc", 0, true},
		{"invalido 25:00", "25:00", 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := horaEmMinutos(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("horaEmMinutos(%q) erro = %v, wantErr = %v", tt.input, err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("horaEmMinutos(%q) = %d, esperado %d", tt.input, got, tt.want)
			}
		})
	}
}

func TestValidarHorasEscala(t *testing.T) {
	tests := []struct {
		name      string
		inicio    string
		fim       string
		wantErr   bool
	}{
		{"valida diurna", "08:00", "17:00", false},
		{"valida noturna (fim < inicio)", "22:00", "06:00", false},
		{"00:00 e 24:00 iguais em minutos", "00:00", "24:00", true},
		{"iguais rejeitadas", "08:00", "08:00", true},
		{"inicio invalido", "abc", "17:00", true},
		{"fim invalido", "08:00", "abc", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validarHorasEscala(tt.inicio, tt.fim)
			if (err != nil) != tt.wantErr {
				t.Errorf("validarHorasEscala(%q, %q) erro = %v, wantErr = %v", tt.inicio, tt.fim, err, tt.wantErr)
			}
		})
	}
}

func TestParseHoraData(t *testing.T) {
	tests := []struct {
		name     string
		dateStr  string
		hora     string
		wantHour int
		wantErr  bool
	}{
		{"valida HH:MM", "2026-07-10", "08:00", 8, false},
		{"valida HH:MM:SS", "2026-07-10", "22:00:00", 22, false},
		{"hora invalida", "2026-07-10", "abc", 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseHoraData(tt.dateStr, tt.hora)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseHoraData(%q, %q) erro = %v, wantErr = %v", tt.dateStr, tt.hora, err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if got.Hour() != tt.wantHour {
					t.Errorf("parseHoraData(%q, %q).Hour() = %d, esperado %d", tt.dateStr, tt.hora, got.Hour(), tt.wantHour)
				}
				if got.Location().String() != timeutil.BRT.String() {
					t.Errorf("timezone = %q, esperado %q", got.Location().String(), timeutil.BRT.String())
				}
			}
		})
	}
}

func TestValidarSessaoTurno(t *testing.T) {
	device := "device-abc"
	outro := "device-xyz"
	token := "token-123"

	tests := []struct {
		name     string
		turno    *model.Turno
		deviceID string
		wantErr  error
	}{
		{"token nulo -> sessao revogada", &model.Turno{TokenSessao: nil}, device, ErrSessaoRevogada},
		{"device diferente -> outro dispositivo", &model.Turno{TokenSessao: &token, DeviceID: &device}, outro, ErrSessaoOutroDispositivo},
		{"device igual -> ok", &model.Turno{TokenSessao: &token, DeviceID: &device}, device, nil},
		{"device nulo no turno -> ok (legado)", &model.Turno{TokenSessao: &token, DeviceID: nil}, device, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validarSessaoTurno(tt.turno, tt.deviceID)
			if err != tt.wantErr {
				t.Errorf("validarSessaoTurno() = %v, esperado %v", err, tt.wantErr)
			}
		})
	}
}

func TestNullableTurno(t *testing.T) {
	t.Run("uuid nil retorna nil", func(t *testing.T) {
		id, str := nullableTurno(uuid.Nil)
		if id != nil {
			t.Errorf("expected nil, got %v", id)
		}
		if str != "" {
			t.Errorf("expected empty string, got %q", str)
		}
	})

	t.Run("uuid valido retorna ponteiro", func(t *testing.T) {
		id := uuid.New()
		ptr, str := nullableTurno(id)
		if ptr == nil {
			t.Fatal("expected non-nil pointer")
		}
		if *ptr != id {
			t.Errorf("expected %v, got %v", id, *ptr)
		}
		if str != id.String() {
			t.Errorf("expected %q, got %q", id.String(), str)
		}
	})
}
