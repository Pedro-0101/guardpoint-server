package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/guardpoint/guardpoint-server/internal/model"
	"github.com/guardpoint/guardpoint-server/internal/repository"
)

var (
	ErrSenhaNaoEncontrada              = errors.New("senha nao encontrada")
	ErrSenhaCodigoDuplicado            = errors.New("codigo ja usado por outra senha deste vigia")
	ErrSenhaTipoJaExiste               = errors.New("vigia ja possui uma senha deste tipo")
	ErrSenhaObrigatoriaNaoRemovivel    = errors.New("senha obrigatoria (ok/emergencia) nao pode ser removida")
	ErrSenhaCampoNaoEditavelParaTipo   = errors.New("campo nao editavel para este tipo de senha")
	ErrNivelEscalonamentoNaoEncontrado = errors.New("nivel de escalonamento nao encontrado")
	ErrNivelInvalidoParaTipo           = errors.New("nivel de escalonamento nao pode ser definido para senha ok/emergencia")

	// ErrUsuarioNaoPertenceAEmpresa ja e declarado em alerta_service.go -- reaproveitado
	// aqui, nao redeclarado, para nao duplicar sentinela no mesmo pacote.
)

const (
	tipoSenhaOK          = "ok"
	tipoSenhaEmergencia  = "emergencia"
	tipoSenhaCustomizada = "customizada"
)

// SenhaVigiaService implementa o CRUD administrativo de senhas por vigia: cada vigia
// deve sempre ter exatamente uma senha "ok" e uma "emergencia" (nao removiveis, com
// nivel_escalonamento_id sempre implicito/dinamico), podendo ter qualquer numero de
// senhas "customizada" (descricao obrigatoria, nivel de escalonamento fixo ou dinamico
// a escolha do admin).
type SenhaVigiaService struct {
	senhaRepo  *repository.SenhaVigiaRepository
	userRepo   *repository.UserRepository
	configRepo *repository.ConfigEscalonamentoRepository
}

func NewSenhaVigiaService(
	senhaRepo *repository.SenhaVigiaRepository,
	userRepo *repository.UserRepository,
	configRepo *repository.ConfigEscalonamentoRepository,
) *SenhaVigiaService {
	return &SenhaVigiaService{senhaRepo: senhaRepo, userRepo: userRepo, configRepo: configRepo}
}

// List retorna todas as senhas cadastradas para o vigia, escopado a empresa do
// chamador.
func (s *SenhaVigiaService) List(ctx context.Context, empresaID, usuarioID uuid.UUID) ([]model.SenhaVigia, error) {
	if err := s.validarUsuarioDaEmpresa(ctx, empresaID, usuarioID); err != nil {
		return nil, err
	}
	return s.senhaRepo.ListByUsuario(ctx, empresaID, usuarioID)
}

// Create cria uma nova senha para o vigia. Para os tipos fixos (ok/emergencia) o nivel
// de escalonamento nunca pode ser informado (e sempre resolvido dinamicamente em
// runtime no momento do disparo do alerta); um vigia so pode ter uma senha de cada tipo
// fixo. Para o tipo customizada, a descricao e obrigatoria e o nivel de
// escalonamento, quando informado, precisa existir e pertencer a mesma empresa.
func (s *SenhaVigiaService) Create(ctx context.Context, empresaID, usuarioID uuid.UUID, req model.CreateSenhaVigiaRequest) (*model.SenhaVigia, error) {
	if err := s.validarUsuarioDaEmpresa(ctx, empresaID, usuarioID); err != nil {
		return nil, err
	}

	if err := validarNivelParaTipoFixo(req.Tipo, req.NivelEscalonamentoID); err != nil {
		return nil, err
	}

	var nivelID *uuid.UUID
	if req.Tipo == tipoSenhaCustomizada {
		// A tag `required_if` do model ja valida isso no handler; reforcado aqui
		// para o service nao depender exclusivamente da camada HTTP.
		if req.Descricao == nil || strings.TrimSpace(*req.Descricao) == "" {
			return nil, errors.New("descricao obrigatoria para senha customizada")
		}

		if req.NivelEscalonamentoID != nil {
			parsed, cfg, err := s.buscarNivelEscalonamento(ctx, empresaID, *req.NivelEscalonamentoID)
			if err != nil {
				return nil, err
			}
			if cfg == nil {
				return nil, ErrNivelEscalonamentoNaoEncontrado
			}
			nivelID = &parsed
		}
	}

	existentes, err := s.senhaRepo.ListByUsuario(ctx, empresaID, usuarioID)
	if err != nil {
		return nil, err
	}

	if req.Tipo != tipoSenhaCustomizada && tipoJaExiste(existentes, req.Tipo) {
		return nil, ErrSenhaTipoJaExiste
	}
	if codigoDuplicado(existentes, req.Codigo, uuid.Nil) {
		return nil, ErrSenhaCodigoDuplicado
	}

	senha := &model.SenhaVigia{
		EmpresaID:            empresaID,
		UsuarioID:            usuarioID,
		Tipo:                 req.Tipo,
		Codigo:               req.Codigo,
		Descricao:            req.Descricao,
		NivelEscalonamentoID: nivelID,
	}
	if err := s.senhaRepo.Create(ctx, senha); err != nil {
		return nil, err
	}
	return senha, nil
}

// Update altera uma senha existente do vigia. Para os tipos fixos (ok/emergencia)
// apenas o codigo pode ser alterado -- descricao, nivel_escalonamento_id e
// nivel_dinamico nao se aplicam e sao rejeitados se vierem preenchidos no request.
// Para customizada, os campos informados sao aplicados; quando NivelDinamico e
// NivelEscalonamentoID vierem preenchidos no mesmo request, NivelDinamico:true
// vence (forca nivel_escalonamento_id = NULL, ignorando o ID informado).
func (s *SenhaVigiaService) Update(ctx context.Context, empresaID, usuarioID, senhaID uuid.UUID, req model.UpdateSenhaVigiaRequest) (*model.SenhaVigia, error) {
	existing, err := s.senhaRepo.FindByID(ctx, empresaID, senhaID)
	if err != nil {
		return nil, err
	}
	if existing == nil || existing.UsuarioID != usuarioID {
		return nil, ErrSenhaNaoEncontrada
	}

	if existing.Tipo == tipoSenhaOK || existing.Tipo == tipoSenhaEmergencia {
		if campoNaoEditavelParaTipoFixo(req) {
			return nil, ErrSenhaCampoNaoEditavelParaTipo
		}
	} else {
		if req.Descricao != nil {
			existing.Descricao = req.Descricao
		}

		novoNivel, forcarDinamico := resolverNivelAtualizacao(req.NivelDinamico, req.NivelEscalonamentoID)
		switch {
		case forcarDinamico:
			existing.NivelEscalonamentoID = nil
		case novoNivel != nil:
			parsed, cfg, err := s.buscarNivelEscalonamento(ctx, empresaID, *novoNivel)
			if err != nil {
				return nil, err
			}
			if cfg == nil {
				return nil, ErrNivelEscalonamentoNaoEncontrado
			}
			existing.NivelEscalonamentoID = &parsed
		}
	}

	if req.Codigo != nil {
		outras, err := s.senhaRepo.ListByUsuario(ctx, empresaID, usuarioID)
		if err != nil {
			return nil, err
		}
		if codigoDuplicado(outras, *req.Codigo, senhaID) {
			return nil, ErrSenhaCodigoDuplicado
		}
		existing.Codigo = *req.Codigo
	}

	if err := s.senhaRepo.Update(ctx, senhaID, empresaID, existing); err != nil {
		return nil, err
	}
	return existing, nil
}

// Delete remove uma senha customizada do vigia. Senhas fixas (ok/emergencia) nunca podem
// ser removidas -- todo vigia precisa manter exatamente uma de cada.
func (s *SenhaVigiaService) Delete(ctx context.Context, empresaID, usuarioID, senhaID uuid.UUID) error {
	existing, err := s.senhaRepo.FindByID(ctx, empresaID, senhaID)
	if err != nil {
		return err
	}
	if existing == nil || existing.UsuarioID != usuarioID {
		return ErrSenhaNaoEncontrada
	}
	if existing.Tipo == tipoSenhaOK || existing.Tipo == tipoSenhaEmergencia {
		return ErrSenhaObrigatoriaNaoRemovivel
	}
	return s.senhaRepo.Delete(ctx, senhaID, empresaID)
}

// validarUsuarioDaEmpresa confirma que usuarioID pertence a empresaID, distinguindo
// "usuario nao encontrado" (mapeado para ErrUsuarioNaoPertenceAEmpresa) de uma falha
// real de banco (propagada como esta, sem mascarar) -- mesmo padrao adotado em
// AlertaService.validarUsuariosDaEmpresa.
func (s *SenhaVigiaService) validarUsuarioDaEmpresa(ctx context.Context, empresaID, usuarioID uuid.UUID) error {
	if _, err := s.userRepo.FindByIDEmpresa(ctx, empresaID, usuarioID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrUsuarioNaoPertenceAEmpresa
		}
		return fmt.Errorf("verificar usuario %s: %w", usuarioID, err)
	}
	return nil
}

// buscarNivelEscalonamento faz o parse de um nivel_escalonamento_id recebido como
// string no request e, se valido, busca o registro correspondente escopado a
// empresa. Retorna (uuid.Nil, nil, err) se a string nao for um UUID valido (nao
// deveria acontecer em request ja validado pela tag `uuid` do handler, mas o service
// nao confia cegamente nisso), e (id, nil, nil) se o UUID for valido mas nao
// existir/nao pertencer a empresa -- o chamador decide o erro de negocio
// (ErrNivelEscalonamentoNaoEncontrado) nesse caso.
func (s *SenhaVigiaService) buscarNivelEscalonamento(ctx context.Context, empresaID uuid.UUID, nivelEscalonamentoID string) (uuid.UUID, *model.ConfigEscalonamento, error) {
	parsed, err := uuid.Parse(nivelEscalonamentoID)
	if err != nil {
		return uuid.Nil, nil, fmt.Errorf("nivel_escalonamento_id invalido: %w", err)
	}
	cfg, err := s.configRepo.FindByID(ctx, parsed, empresaID)
	if err != nil {
		return uuid.Nil, nil, err
	}
	return parsed, cfg, nil
}

// validarNivelParaTipoFixo rejeita nivel_escalonamento_id em senhas fixas (ok/emergencia):
// o nivel dessas senhas e sempre resolvido dinamicamente em runtime, nunca fixado na
// criacao.
func validarNivelParaTipoFixo(tipo string, nivelEscalonamentoID *string) error {
	if (tipo == tipoSenhaOK || tipo == tipoSenhaEmergencia) && nivelEscalonamentoID != nil {
		return ErrNivelInvalidoParaTipo
	}
	return nil
}

// campoNaoEditavelParaTipoFixo reporta se o request de update de uma senha fixa
// (ok/emergencia) tenta alterar um campo que so se aplica a senhas customizada.
// Nesses tipos, apenas Codigo e editavel.
func campoNaoEditavelParaTipoFixo(req model.UpdateSenhaVigiaRequest) bool {
	return req.Descricao != nil || req.NivelEscalonamentoID != nil || req.NivelDinamico != nil
}

// resolverNivelAtualizacao decide, a partir dos campos de update informados, o que
// fazer com o nivel_escalonamento_id de uma senha customizada. NivelDinamico:true tem
// precedencia sobre NivelEscalonamentoID quando ambos vierem preenchidos no mesmo
// request.
//
// Retorno:
//   - forcarDinamico=true: nivel_escalonamento_id deve virar NULL (novoValor e
//     ignorado nesse caso, mesmo que nao seja nil).
//   - forcarDinamico=false e novoValor != nil: um novo nivel_escalonamento_id foi
//     informado e precisa ser validado/aplicado.
//   - forcarDinamico=false e novoValor == nil: nenhuma mudanca de nivel foi
//     solicitada; o valor atual deve ser mantido.
func resolverNivelAtualizacao(nivelDinamico *bool, nivelEscalonamentoID *string) (novoValor *string, forcarDinamico bool) {
	if nivelDinamico != nil && *nivelDinamico {
		return nil, true
	}
	return nivelEscalonamentoID, false
}

// tipoJaExiste reporta se ja existe, entre as senhas informadas, uma do tipo dado --
// usado para impor "no maximo uma senha ok e uma emergencia por vigia".
func tipoJaExiste(existentes []model.SenhaVigia, tipo string) bool {
	for _, s := range existentes {
		if s.Tipo == tipo {
			return true
		}
	}
	return false
}

// codigoDuplicado reporta se alguma senha em existentes (exceto a de ID ignorarID, usado
// para o proprio registro durante um update) ja usa o codigo informado.
func codigoDuplicado(existentes []model.SenhaVigia, codigo string, ignorarID uuid.UUID) bool {
	for _, s := range existentes {
		if s.ID == ignorarID {
			continue
		}
		if s.Codigo == codigo {
			return true
		}
	}
	return false
}
