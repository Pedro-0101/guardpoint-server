package model

import (
	"testing"

	"github.com/go-playground/validator/v10"
)

func TestRegisterRequest_EmailObrigatorioSalvoParaVigia(t *testing.T) {
	v := validator.New()

	if err := v.Struct(RegisterRequest{Nome: "Admin", Password: "123456", Role: "admin"}); err == nil {
		t.Error("esperava erro de validacao: admin sem email deveria falhar")
	}

	if err := v.Struct(RegisterRequest{Nome: "Vigia", Password: "123456", Role: "vigia"}); err != nil {
		t.Errorf("vigia sem email deveria ser valido, erro: %v", err)
	}
}

func TestCreateUsuarioRequest_EmailObrigatorioSalvoParaVigia(t *testing.T) {
	v := validator.New()

	if err := v.Struct(CreateUsuarioRequest{Nome: "Supervisor", Senha: "123456", Cargo: "supervisor"}); err == nil {
		t.Error("esperava erro de validacao: supervisor sem email deveria falhar")
	}

	if err := v.Struct(CreateUsuarioRequest{Nome: "Vigia", Senha: "123456", Cargo: "vigia"}); err != nil {
		t.Errorf("vigia sem email deveria ser valido, erro: %v", err)
	}
}

func TestLoginRequest_ExigeEmailOuNomeComCodigo(t *testing.T) {
	v := validator.New()

	if err := v.Struct(LoginRequest{Password: "123456"}); err == nil {
		t.Error("esperava erro de validacao: sem email e sem nome/codigo deveria falhar")
	}

	if err := v.Struct(LoginRequest{Email: "x@y.com", Password: "123456"}); err != nil {
		t.Errorf("login por email deveria ser valido, erro: %v", err)
	}

	if err := v.Struct(LoginRequest{Nome: "Vigia X", Password: "123456"}); err == nil {
		t.Error("esperava erro de validacao: nome sem codigo da empresa deveria falhar")
	}

	if err := v.Struct(LoginRequest{CodigoEmpresa: "ABCD1234", Password: "123456"}); err == nil {
		t.Error("esperava erro de validacao: codigo sem nome deveria falhar")
	}

	if err := v.Struct(LoginRequest{Nome: "Vigia X", CodigoEmpresa: "ABCD1234", Password: "123456"}); err != nil {
		t.Errorf("login por nome+codigo deveria ser valido, erro: %v", err)
	}
}
