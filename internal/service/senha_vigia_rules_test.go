package service

import (
	"testing"

	"github.com/google/uuid"

	"github.com/guardpoint/guardpoint-server/internal/model"
)

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
