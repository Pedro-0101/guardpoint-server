package service

import (
	"testing"

	"github.com/google/uuid"

	"github.com/guardpoint/guardpoint-server/internal/model"
)

func TestValidarNivelParaTipoFixo(t *testing.T) {
	nivel := uuid.NewString()

	tests := []struct {
		name    string
		tipo    string
		nivelID *string
		wantErr error
	}{
		{"ok sem nivel", tipoSenhaOK, nil, nil},
		{"emergencia sem nivel", tipoSenhaEmergencia, nil, nil},
		{"customizada com nivel", tipoSenhaCustomizada, &nivel, nil},
		{"customizada sem nivel", tipoSenhaCustomizada, nil, nil},
		{"ok com nivel", tipoSenhaOK, &nivel, ErrNivelInvalidoParaTipo},
		{"emergencia com nivel", tipoSenhaEmergencia, &nivel, ErrNivelInvalidoParaTipo},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validarNivelParaTipoFixo(tt.tipo, tt.nivelID)
			if err != tt.wantErr {
				t.Errorf("validarNivelParaTipoFixo(%q, %v) = %v, esperado %v", tt.tipo, tt.nivelID, err, tt.wantErr)
			}
		})
	}
}

func TestCampoNaoEditavelParaTipoFixo(t *testing.T) {
	desc := "nova descricao"
	nivel := uuid.NewString()
	dinamicoTrue := true
	dinamicoFalse := false
	codigo := "1234"

	tests := []struct {
		name string
		req  model.UpdateSenhaVigiaRequest
		want bool
	}{
		{"so codigo", model.UpdateSenhaVigiaRequest{Codigo: &codigo}, false},
		{"request vazio", model.UpdateSenhaVigiaRequest{}, false},
		{"descricao preenchida", model.UpdateSenhaVigiaRequest{Descricao: &desc}, true},
		{"nivel preenchido", model.UpdateSenhaVigiaRequest{NivelEscalonamentoID: &nivel}, true},
		{"nivel_dinamico true", model.UpdateSenhaVigiaRequest{NivelDinamico: &dinamicoTrue}, true},
		{"nivel_dinamico false (ainda assim presente no request)", model.UpdateSenhaVigiaRequest{NivelDinamico: &dinamicoFalse}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := campoNaoEditavelParaTipoFixo(tt.req); got != tt.want {
				t.Errorf("campoNaoEditavelParaTipoFixo(%+v) = %v, esperado %v", tt.req, got, tt.want)
			}
		})
	}
}

func TestResolverNivelAtualizacao(t *testing.T) {
	nivel := uuid.NewString()
	dinamicoTrue := true
	dinamicoFalse := false

	t.Run("nenhum campo informado: mantem valor atual", func(t *testing.T) {
		novoValor, forcarDinamico := resolverNivelAtualizacao(nil, nil)
		if novoValor != nil || forcarDinamico {
			t.Errorf("esperado (nil, false), obtido (%v, %v)", novoValor, forcarDinamico)
		}
	})

	t.Run("so nivel_escalonamento_id: aplica o novo nivel", func(t *testing.T) {
		novoValor, forcarDinamico := resolverNivelAtualizacao(nil, &nivel)
		if novoValor == nil || *novoValor != nivel || forcarDinamico {
			t.Errorf("esperado (%q, false), obtido (%v, %v)", nivel, novoValor, forcarDinamico)
		}
	})

	t.Run("so nivel_dinamico=true: forca nivel dinamico", func(t *testing.T) {
		novoValor, forcarDinamico := resolverNivelAtualizacao(&dinamicoTrue, nil)
		if novoValor != nil || !forcarDinamico {
			t.Errorf("esperado (nil, true), obtido (%v, %v)", novoValor, forcarDinamico)
		}
	})

	t.Run("nivel_dinamico=false explicito e sem id: mantem valor atual", func(t *testing.T) {
		novoValor, forcarDinamico := resolverNivelAtualizacao(&dinamicoFalse, nil)
		if novoValor != nil || forcarDinamico {
			t.Errorf("esperado (nil, false), obtido (%v, %v)", novoValor, forcarDinamico)
		}
	})

	t.Run("nivel_dinamico=false explicito com id: aplica o id informado", func(t *testing.T) {
		novoValor, forcarDinamico := resolverNivelAtualizacao(&dinamicoFalse, &nivel)
		if novoValor == nil || *novoValor != nivel || forcarDinamico {
			t.Errorf("esperado (%q, false), obtido (%v, %v)", nivel, novoValor, forcarDinamico)
		}
	})

	t.Run("nivel_dinamico=true E nivel_escalonamento_id preenchidos: dinamico vence", func(t *testing.T) {
		novoValor, forcarDinamico := resolverNivelAtualizacao(&dinamicoTrue, &nivel)
		if novoValor != nil || !forcarDinamico {
			t.Errorf("precedencia violada: esperado (nil, true) mesmo com nivel=%q informado, obtido (%v, %v)", nivel, novoValor, forcarDinamico)
		}
	})
}

func TestTipoJaExiste(t *testing.T) {
	existentes := []model.SenhaVigia{
		{Tipo: tipoSenhaOK},
		{Tipo: tipoSenhaCustomizada},
	}

	if !tipoJaExiste(existentes, tipoSenhaOK) {
		t.Error("esperado true para tipo ok ja existente")
	}
	if tipoJaExiste(existentes, tipoSenhaEmergencia) {
		t.Error("esperado false para tipo emergencia inexistente")
	}
	if tipoJaExiste(nil, tipoSenhaOK) {
		t.Error("esperado false para lista vazia")
	}
}

func TestCodigoDuplicado(t *testing.T) {
	id1 := uuid.New()
	id2 := uuid.New()
	existentes := []model.SenhaVigia{
		{ID: id1, Codigo: "1111"},
		{ID: id2, Codigo: "2222"},
	}

	t.Run("codigo novo nao colide", func(t *testing.T) {
		if codigoDuplicado(existentes, "3333", uuid.Nil) {
			t.Error("nao deveria detectar duplicidade para codigo inedito")
		}
	})

	t.Run("codigo usado por outra senha colide", func(t *testing.T) {
		if !codigoDuplicado(existentes, "2222", uuid.Nil) {
			t.Error("deveria detectar duplicidade com outra senha do mesmo codigo")
		}
	})

	t.Run("codigo igual ao da propria senha (ignorada) nao colide", func(t *testing.T) {
		if codigoDuplicado(existentes, "1111", id1) {
			t.Error("nao deveria colidir com o proprio registro durante update")
		}
	})

	t.Run("codigo igual ao de outra senha mesmo ignorando a propria ainda colide", func(t *testing.T) {
		if !codigoDuplicado(existentes, "2222", id1) {
			t.Error("deveria colidir com o codigo de uma senha diferente da ignorada")
		}
	})
}
