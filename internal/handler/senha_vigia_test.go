package handler

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/guardpoint/guardpoint-server/internal/service"
)

func TestWriteSenhaError(t *testing.T) {
	h := &SenhaVigiaHandler{}

	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantBody   string
	}{
		{"usuario nao pertence a empresa → 404", service.ErrUsuarioNaoPertenceAEmpresa, http.StatusNotFound, `{"error":"usuario nao encontrado"}`},
		{"senha nao encontrada → 404", service.ErrSenhaNaoEncontrada, http.StatusNotFound, `{"error":"senha nao encontrada"}`},
		{"codigo duplicado → 400", service.ErrSenhaCodigoDuplicado, http.StatusBadRequest, `{"error":"codigo ja usado por outra senha deste vigia"}`},
		{"tipo ja existe → 400", service.ErrSenhaTipoJaExiste, http.StatusBadRequest, `{"error":"vigia ja possui uma senha deste tipo"}`},
		{"nivel invalido para tipo → 400", service.ErrNivelInvalidoParaTipo, http.StatusBadRequest, `{"error":"nivel de escalonamento nao pode ser definido para este tipo de senha"}`},
		{"campo nao editavel → 400", service.ErrSenhaCampoNaoEditavelParaTipo, http.StatusBadRequest, `{"error":"campo nao editavel para este tipo de senha"}`},
		{"senha obrigatoria nao removivel → 409", service.ErrSenhaObrigatoriaNaoRemovivel, http.StatusConflict, `{"error":"senha obrigatoria nao pode ser removida"}`},
		{"nivel escalonamento nao encontrado → 400", service.ErrNivelEscalonamentoNaoEncontrado, http.StatusBadRequest, `{"error":"nivel de escalonamento nao encontrado"}`},
		{"erro desconhecido → false", errors.New("erro qualquer"), 0, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			handled := h.writeSenhaError(rec, tt.err)

			if tt.wantStatus == 0 {
				if handled {
					t.Errorf("writeSenhaError(%v) = true, esperado false (erro nao mapeado)", tt.err)
				}
				return
			}

			if !handled {
				t.Errorf("writeSenhaError(%v) = false, esperado true", tt.err)
			}
			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, esperado %d", rec.Code, tt.wantStatus)
			}
			if body := rec.Body.String(); body != tt.wantBody+"\n" {
				t.Errorf("corpo = %q, esperado %q", body, tt.wantBody+"\n")
			}
		})
	}
}

func TestWriteSenhaError_CobreTodosSentinela(t *testing.T) {
	h := &SenhaVigiaHandler{}
	erros := []error{
		service.ErrUsuarioNaoPertenceAEmpresa,
		service.ErrSenhaNaoEncontrada,
		service.ErrSenhaCodigoDuplicado,
		service.ErrSenhaTipoJaExiste,
		service.ErrNivelInvalidoParaTipo,
		service.ErrSenhaCampoNaoEditavelParaTipo,
		service.ErrSenhaObrigatoriaNaoRemovivel,
		service.ErrNivelEscalonamentoNaoEncontrado,
	}

	for _, err := range erros {
		rec := httptest.NewRecorder()
		if !h.writeSenhaError(rec, err) {
			t.Errorf("writeSenhaError(%v) nao tratou um erro sentinela conhecido", err)
		}
		if rec.Code < 400 {
			t.Errorf("writeSenhaError(%v) retornou status %d < 400", err, rec.Code)
		}
	}
}

func TestWriteSenhaError_NilPanic(t *testing.T) {
	h := &SenhaVigiaHandler{}
	rec := httptest.NewRecorder()
	handled := h.writeSenhaError(rec, nil)
	if handled {
		t.Error("writeSenhaError(nil) tratou um erro nil")
	}
}
