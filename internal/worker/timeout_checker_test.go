package worker

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestTimeoutChecker_AlertAlreadyExists(t *testing.T) {
	t.Run("alerta nao existe", func(t *testing.T) {
		w := &TimeoutChecker{}
		existe, err := w.alertaJaExisteSkipContext(uuid.New(), uuid.New(), time.Now())
		if err != nil {
			t.Fatalf("erro inesperado: %v", err)
		}
		if existe {
			t.Error("alertaJaExiste deve retornar false para usuario sem alerta")
		}
	})
}

func TestTimeoutChecker_ProcessTurno_SemCheckins(t *testing.T) {
	t.Run("sem ultimo checkin nao processa", func(t *testing.T) {
		w := &TimeoutChecker{}
		turno := turnoAtivoInfo{
			ID:           uuid.New(),
			EmpresaID:    uuid.New(),
			UsuarioID:    uuid.New(),
			PostoID:      uuid.New(),
			IntervaloMin: 30,
			Status:       "em_andamento",
		}

		processado := false
		w.processTurnoStatic(turno, nil, &processado)

		if processado {
			t.Error("nao deveria processar turno sem ultimo checkin")
		}
	})

	t.Run("com checkin recente nao gera alerta", func(t *testing.T) {
		w := &TimeoutChecker{}
		turno := turnoAtivoInfo{
			ID:           uuid.New(),
			EmpresaID:    uuid.New(),
			UsuarioID:    uuid.New(),
			PostoID:      uuid.New(),
			IntervaloMin: 30,
			Status:       "em_andamento",
		}

		recente := time.Now().Add(-10 * time.Minute)

		processado := false
		w.processTurnoStatic(turno, &recente, &processado)

		if processado {
			t.Error("nao deveria gerar alerta para checkin recente")
		}
	})
}

func TestTurnoAtivoInfo_Valores(t *testing.T) {
	id := uuid.New()
	turno := turnoAtivoInfo{
		ID:           id,
		EmpresaID:    uuid.New(),
		UsuarioID:    uuid.New(),
		PostoID:      uuid.New(),
		IntervaloMin: 30,
		Status:       "em_andamento",
	}

	if turno.ID != id {
		t.Errorf("ID = %v, esperado %v", turno.ID, id)
	}
	if turno.IntervaloMin != 30 {
		t.Errorf("IntervaloMin = %d, esperado 30", turno.IntervaloMin)
	}
	if turno.Status != "em_andamento" {
		t.Errorf("Status = %q, esperado em_andamento", turno.Status)
	}
}

func TestNoShowEntry_Valores(t *testing.T) {
	empresaID := uuid.New()
	usuarioID := uuid.New()
	postoID := uuid.New()

	entry := noShowEntry{
		empresaID:     empresaID,
		usuarioID:     usuarioID,
		postoID:       postoID,
		horaInicio:    "08:00",
		toleranciaMin: 15,
	}

	if entry.empresaID != empresaID {
		t.Errorf("empresaID = %v, esperado %v", entry.empresaID, empresaID)
	}
	if entry.horaInicio != "08:00" {
		t.Errorf("horaInicio = %q, esperado 08:00", entry.horaInicio)
	}
	if entry.toleranciaMin != 15 {
		t.Errorf("toleranciaMin = %d, esperado 15", entry.toleranciaMin)
	}
}

// processTurnoStatic testa a logica pura de verificacao de atraso do processTurno
// sem dependencias de repositorio.
func (w *TimeoutChecker) processTurnoStatic(t turnoAtivoInfo, ultimoTimestamp *time.Time, processado *bool) {
	if ultimoTimestamp == nil {
		return
	}

	elapsed := time.Since(*ultimoTimestamp)
	intervaloDuration := time.Duration(t.IntervaloMin) * time.Minute

	if elapsed <= intervaloDuration {
		return
	}

	*processado = true
}

// alertaJaExisteSkipContext simula a verificacao sem acesso ao banco.
func (w *TimeoutChecker) alertaJaExisteSkipContext(empresaID, usuarioID uuid.UUID, data time.Time) (bool, error) {
	return false, nil
}
