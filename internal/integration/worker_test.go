//go:build integration

package integration

import (
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/guardpoint/guardpoint-server/internal/repository"
)

// D3: TimeoutChecker gera atraso_nX conforme o atraso acumulado e nao duplica.
func TestTimeoutCheckerAtrasos(t *testing.T) {
	c := novoCenario(t)
	turno := c.iniciarTurno()

	supervisor := c.e.criarUsuario(c.empresa.ID, "Supervisor N1", "sup.n1@a.com", "senha123", "supervisor", true)
	gerente := c.e.criarUsuario(c.empresa.ID, "Gerente N2", "gerente.n2@a.com", "senha123", "supervisor", true)
	diretor := c.e.criarUsuario(c.empresa.ID, "Diretor N3", "diretor.n3@a.com", "senha123", "admin", true)

	c.e.reqJSON(http.MethodPut, "/api/v1/config/escalonamento", c.adminToken, []map[string]any{
		{"nivel": 1, "atraso_minutos": 5, "usuario_ids": []string{supervisor.ID.String()}},
		{"nivel": 2, "atraso_minutos": 15, "usuario_ids": []string{gerente.ID.String()}},
		{"nivel": 3, "atraso_minutos": 60, "usuario_ids": []string{diretor.ID.String()}},
	}, http.StatusOK, nil)

	// turno comecou ha 50 min sem nenhuma atividade desde entao; intervalo de 30
	// min => atraso de ~20 min
	c.backdatarCheckinInicio(turno.ID, 50*time.Minute)

	c.e.app.TimeoutChecker.CheckOnce(t.Context())

	if n := c.contarAlertas("atraso_n1"); n != 1 {
		t.Errorf("alertas atraso_n1 = %d, esperado 1", n)
	}
	if n := c.contarAlertas("atraso_n2"); n != 1 {
		t.Errorf("alertas atraso_n2 = %d, esperado 1", n)
	}
	if n := c.contarAlertas("atraso_n3"); n != 0 {
		t.Errorf("alertas atraso_n3 = %d, esperado 0 (atraso de 20 min < 60)", n)
	}

	t.Run("segundo ciclo nao duplica", func(t *testing.T) {
		c.e.app.TimeoutChecker.CheckOnce(t.Context())
		if n := c.contarAlertas("atraso_n1"); n != 1 {
			t.Errorf("alertas atraso_n1 apos segundo ciclo = %d, esperado 1", n)
		}
		if n := c.contarAlertas("atraso_n2"); n != 1 {
			t.Errorf("alertas atraso_n2 apos segundo ciclo = %d, esperado 1", n)
		}
	})
}

// D3: escala ativa sem turno apos hora_inicio + tolerancia gera no_show uma vez.
func TestTimeoutCheckerNoShow(t *testing.T) {
	c := novoCenario(t)

	// vigia2 tem escala que comecou ha 1h (tolerancia 15) e nunca iniciou turno
	vigia2 := c.e.criarUsuario(c.empresa.ID, "Vigia Ausente", "ausente@a.com", "senha123", "vigia", true)
	c.criarEscala(vigia2.ID, c.posto.ID, time.Now().Add(-1*time.Hour), 15)

	c.e.app.TimeoutChecker.CheckOnce(t.Context())

	if n := c.contarAlertas("no_show"); n != 1 {
		t.Fatalf("alertas no_show = %d, esperado 1", n)
	}

	t.Run("segundo ciclo nao regenera", func(t *testing.T) {
		c.e.app.TimeoutChecker.CheckOnce(t.Context())
		if n := c.contarAlertas("no_show"); n != 1 {
			t.Errorf("alertas no_show apos segundo ciclo = %d, esperado 1", n)
		}
	})
}

// A1: FindEscalasSemTurno precisa enxergar a escala noturna iniciada "ontem"
// e nao disparar para turnos ja encerrados. Usa horaCorte fabricada para ser
// deterministico.
func TestFindEscalasSemTurnoNoturna(t *testing.T) {
	e := newEnv(t)
	ctx := t.Context()

	empresa := e.criarEmpresa("Empresa Noturna", "33333333000191")
	vigia := e.criarUsuario(empresa.ID, "Vigia Noturno", "noturno@a.com", "senha123", "vigia", true)

	var postoID uuid.UUID
	if err := e.pool.QueryRow(ctx,
		`INSERT INTO postos (empresa_id, nome, latitude, longitude, raio_m) VALUES ($1, 'Posto Noturno', 0, 0, 100) RETURNING id`,
		empresa.ID).Scan(&postoID); err != nil {
		t.Fatalf("criar posto: %v", err)
	}

	criarEscalaSQL := func(diaInicio, diaFim int16, horaInicio, horaFim string, tol int) uuid.UUID {
		var id uuid.UUID
		if err := e.pool.QueryRow(ctx, `
			INSERT INTO escalas (empresa_id, usuario_id, posto_id, dia_semana_inicio, dia_semana_fim, hora_inicio, hora_fim, tolerancia_min)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8) RETURNING id`,
			empresa.ID, vigia.ID, postoID, diaInicio, diaFim, horaInicio, horaFim, tol).Scan(&id); err != nil {
			t.Fatalf("criar escala %s-%s: %v", horaInicio, horaFim, err)
		}
		return id
	}

	// 2026-07-10 e uma sexta (5); 2026-07-09, quinta (4).
	noturnaQuinta := criarEscalaSQL(4, 5, "23:50", "06:00", 30)
	diurnaSexta := criarEscalaSQL(5, 5, "08:00", "17:00", 15)

	repo := repository.NewEscalaRepository(e.pool)

	contem := func(list []repository.EscalaSemTurno, id uuid.UUID) bool {
		for _, esc := range list {
			if esc.ID == id {
				return true
			}
		}
		return false
	}

	t.Run("madrugada de sexta enxerga a noturna de quinta", func(t *testing.T) {
		horaCorte := time.Date(2026, 7, 10, 0, 30, 0, 0, time.Local)
		list, err := repo.FindEscalasSemTurno(ctx, horaCorte)
		if err != nil {
			t.Fatalf("FindEscalasSemTurno: %v", err)
		}
		if !contem(list, noturnaQuinta) {
			t.Error("escala noturna de quinta nao detectada as 00:30 de sexta")
		}
		if contem(list, diurnaSexta) {
			t.Error("escala diurna de sexta detectada antes do horario de inicio")
		}
	})

	t.Run("apos o fim do turno noturno nao dispara mais", func(t *testing.T) {
		horaCorte := time.Date(2026, 7, 10, 7, 0, 0, 0, time.Local)
		list, err := repo.FindEscalasSemTurno(ctx, horaCorte)
		if err != nil {
			t.Fatalf("FindEscalasSemTurno: %v", err)
		}
		if contem(list, noturnaQuinta) {
			t.Error("escala noturna ainda detectada as 07:00, apos hora_fim")
		}
	})

	t.Run("diurna dispara dentro do turno e para depois do fim", func(t *testing.T) {
		dentro := time.Date(2026, 7, 10, 9, 0, 0, 0, time.Local)
		list, err := repo.FindEscalasSemTurno(ctx, dentro)
		if err != nil {
			t.Fatalf("FindEscalasSemTurno: %v", err)
		}
		if !contem(list, diurnaSexta) {
			t.Error("escala diurna nao detectada as 09:00")
		}

		depois := time.Date(2026, 7, 10, 18, 0, 0, 0, time.Local)
		list, err = repo.FindEscalasSemTurno(ctx, depois)
		if err != nil {
			t.Fatalf("FindEscalasSemTurno: %v", err)
		}
		if contem(list, diurnaSexta) {
			t.Error("escala diurna ainda detectada as 18:00, apos hora_fim")
		}
	})
}
