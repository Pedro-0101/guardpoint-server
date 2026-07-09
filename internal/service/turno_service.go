package service

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"math/big"
	"time"

	"github.com/google/uuid"

	"github.com/guardpoint/guardpoint-server/internal/model"
	"github.com/guardpoint/guardpoint-server/internal/repository"
	"github.com/guardpoint/guardpoint-server/internal/ws"
)

var (
	ErrTurnoJaAtivo              = errors.New("usuario ja possui um turno em andamento")
	ErrTurnoNaoEncontrado        = errors.New("turno nao encontrado")
	ErrTurnoJaFinalizado         = errors.New("turno ja finalizado")
	ErrPostoNaoEncontrado        = errors.New("posto nao encontrado")
	ErrTurnoNaoPertenceAoUsuario = errors.New("turno nao pertence ao usuario")
	ErrDeviceNaoRegistrado       = errors.New("device nao registrado para biometric login")
	ErrSessaoRevogada            = errors.New("sessao do turno foi revogada; reassocie com o pin")
	ErrSessaoOutroDispositivo    = errors.New("turno associado a outro dispositivo")
	ErrPinInvalido               = errors.New("pin invalido")
	ErrPinExpirado               = errors.New("pin expirado")

	// ErrSenhaVigiaInvalida e ErrVigiaSemSenhasConfiguradas sao erros internos de
	// resolverSenha. NUNCA sao retornados ao chamador HTTP -- aplicarConsequenciaSenha
	// os engole e apenas loga, pois a acao chamadora (checkin/inicio/finalizacao) ja
	// foi ou sera persistida com sucesso independente do resultado da resolucao do PIN.
	ErrSenhaVigiaInvalida         = errors.New("senha invalida")
	ErrVigiaSemSenhasConfiguradas = errors.New("vigia sem senhas configuradas")
)

type TurnoService struct {
	turnoRepo             *repository.TurnoRepository
	checkinRepo           *repository.CheckinRepository
	postoRepo             *repository.PostoRepository
	userRepo              *repository.UserRepository
	sessaoDispositivoRepo *repository.SessaoDispositivoRepository
	escalaRepo            *repository.EscalaRepository
	substituicaoRepo      *repository.SubstituicaoRepository
	senhaVigiaRepo        *repository.SenhaVigiaRepository
	alertaService         *AlertaService
	hub                   *ws.Hub
}

func NewTurnoService(
	turnoRepo *repository.TurnoRepository,
	checkinRepo *repository.CheckinRepository,
	postoRepo *repository.PostoRepository,
	userRepo *repository.UserRepository,
	sessaoDispositivoRepo *repository.SessaoDispositivoRepository,
	escalaRepo *repository.EscalaRepository,
	substituicaoRepo *repository.SubstituicaoRepository,
	senhaVigiaRepo *repository.SenhaVigiaRepository,
	alertaService *AlertaService,
	hub *ws.Hub,
) *TurnoService {
	return &TurnoService{
		turnoRepo:             turnoRepo,
		checkinRepo:           checkinRepo,
		postoRepo:             postoRepo,
		userRepo:              userRepo,
		sessaoDispositivoRepo: sessaoDispositivoRepo,
		escalaRepo:            escalaRepo,
		substituicaoRepo:      substituicaoRepo,
		senhaVigiaRepo:        senhaVigiaRepo,
		alertaService:         alertaService,
		hub:                   hub,
	}
}

// resolverSenha busca o PIN do vigia (escopado por usuario_id) que bate com o codigo
// informado. Distingue "vigia sem nenhum PIN cadastrado" de "codigo nao corresponde a
// nenhum PIN".
func (s *TurnoService) resolverSenha(ctx context.Context, empresaID, usuarioID uuid.UUID, codigo string) (*model.SenhaVigia, error) {
	total, err := s.senhaVigiaRepo.CountByUsuario(ctx, empresaID, usuarioID)
	if err != nil {
		return nil, fmt.Errorf("contar senhas do vigia: %w", err)
	}
	if total == 0 {
		return nil, ErrVigiaSemSenhasConfiguradas
	}
	senha, err := s.senhaVigiaRepo.FindByUsuarioECodigo(ctx, empresaID, usuarioID, codigo)
	if err != nil {
		return nil, fmt.Errorf("buscar senha: %w", err)
	}
	if senha == nil {
		return nil, ErrSenhaVigiaInvalida
	}
	return senha, nil
}

// aplicarConsequenciaSenha resolve o PIN e, se nao for 'ok', marca o turno como
// critico e dispara o alerta via AlertaService.CreateAlertaPorSenha. Erros de
// resolucao (PIN invalido/vigia sem PINs) sao SEMPRE ENGOLIDOS aqui (so logados) --
// a acao chamadora ja foi ou sera persistida com sucesso independente do resultado.
// Retorna o *model.SenhaVigia resolvido (ou nil) para popular
// Checkin.TipoSenha/SenhaVigiaID antes do INSERT.
func (s *TurnoService) aplicarConsequenciaSenha(ctx context.Context, empresaIDStr string, empresaID, turnoID, usuarioID uuid.UUID, codigo string) *model.SenhaVigia {
	senha, err := s.resolverSenha(ctx, empresaID, usuarioID, codigo)
	if err != nil {
		slog.Warn("senha de vigia nao resolvida", "error", err, "turno_id", turnoID, "usuario_id", usuarioID)
		return nil
	}
	if senha.Tipo == "ok" {
		return senha
	}

	_ = s.turnoRepo.UpdateStatus(ctx, turnoID, empresaID, "critico", nil)

	tipoAlerta := "senha_emergencia"
	mensagem := "Senha de emergencia detectada"
	if senha.Tipo == "customizada" {
		tipoAlerta = "senha_customizada"
		desc := ""
		if senha.Descricao != nil {
			desc = *senha.Descricao
		}
		mensagem = "Senha customizada detectada: " + desc
	}

	if _, err := s.alertaService.CreateAlertaImediato(ctx, empresaID, turnoID, tipoAlerta, mensagem); err != nil {
		slog.Error("criar alerta de senha", "error", err, "turno_id", turnoID)
	}
	s.hub.Broadcast(empresaIDStr, ws.NewStatusChangeEvent(turnoID.String(), "critico"))
	return senha
}

func (s *TurnoService) Iniciar(ctx context.Context, userID, empresaID string, req model.IniciarTurnoRequest) (*model.Turno, error) {
	parsedUserID, err := uuid.Parse(userID)
	if err != nil {
		return nil, fmt.Errorf("user_id invalido: %w", err)
	}
	parsedEmpresaID, err := uuid.Parse(empresaID)
	if err != nil {
		return nil, fmt.Errorf("empresa_id invalido: %w", err)
	}
	parsedPostoID, err := uuid.Parse(req.PostoID)
	if err != nil {
		return nil, fmt.Errorf("posto_id invalido: %w", err)
	}

	ativo, err := s.turnoRepo.FindAtivoByUsuario(ctx, parsedEmpresaID, parsedUserID)
	if err != nil {
		return nil, fmt.Errorf("verificar turno ativo: %w", err)
	}
	if ativo != nil {
		return nil, ErrTurnoJaAtivo
	}

	_, err = s.sessaoDispositivoRepo.FindByDeviceID(ctx, empresaID, req.DeviceID)
	if err != nil {
		return nil, ErrDeviceNaoRegistrado
	}

	posto, err := s.postoRepo.FindByID(ctx, parsedEmpresaID, parsedPostoID)
	if err != nil {
		return nil, fmt.Errorf("posto: %w", ErrPostoNaoEncontrado)
	}
	if !posto.Ativo {
		return nil, fmt.Errorf("posto inativo")
	}

	now := time.Now()
	esc, err := s.buscarEscalaParaInicio(ctx, parsedEmpresaID, parsedUserID, parsedPostoID, now)
	if err != nil {
		return nil, err
	}
	if esc == nil {
		return nil, ErrEscalaSemEscala
	}
	if ok, _ := VerificarToleranciaEscala(esc, now); !ok {
		return nil, ErrEscalaForaTolerancia
	}

	intervaloMin := req.IntervaloMin
	if intervaloMin <= 0 {
		intervaloMin = 30
	}

	tokenSessao := uuid.New().String()

	fimPrevisto := now.Add(12 * time.Hour)

	deviceID := req.DeviceID

	turno := &model.Turno{
		EmpresaID:      parsedEmpresaID,
		UsuarioID:      parsedUserID,
		PostoID:        parsedPostoID,
		PostoNome:      posto.Nome,
		Status:         "em_andamento",
		InicioPrevisto: now,
		FimPrevisto:    fimPrevisto,
		InicioReal:     &now,
		TokenSessao:    &tokenSessao,
		DeviceID:       &deviceID,
		IntervaloMin:   intervaloMin,
	}

	if err := s.turnoRepo.Create(ctx, turno); err != nil {
		return nil, fmt.Errorf("criar turno: %w", err)
	}

	s.hub.Broadcast(empresaID, ws.NewStatusChangeEvent(turno.ID.String(), "em_andamento"))

	flagGeofence := s.calcularGeofence(ctx, turno.PostoID, parsedEmpresaID, req.Latitude, req.Longitude)
	senha := s.aplicarConsequenciaSenha(ctx, empresaID, parsedEmpresaID, turno.ID, parsedUserID, req.Senha)

	checkinInicio := &model.Checkin{
		TurnoID:          turno.ID,
		EmpresaID:        parsedEmpresaID,
		Latitude:         req.Latitude,
		Longitude:        req.Longitude,
		TimestampCriacao: now,
		Evento:           "inicio",
		FlagGeofence:     flagGeofence,
		OrigemRede:       "online",
	}
	if senha != nil {
		tipoSenha := senha.Tipo
		checkinInicio.TipoSenha = &tipoSenha
		senhaID := senha.ID
		checkinInicio.SenhaVigiaID = &senhaID
	}

	if err := s.checkinRepo.Create(ctx, checkinInicio); err != nil {
		return nil, fmt.Errorf("criar checkin de inicio: %w", err)
	}

	if senha != nil && senha.Tipo != "ok" {
		turno.Status = "critico"
	}

	return turno, nil
}

// buscarEscalaParaInicio procura a escala ativa compativel com o inicio de turno
// em `now`. Primeiro verifica substituicoes pontuais (ex.: vigia cobrindo falta),
// depois cai no fluxo normal de escalas semanais.
func (s *TurnoService) buscarEscalaParaInicio(ctx context.Context, empresaID, usuarioID, postoID uuid.UUID, now time.Time) (*model.Escala, error) {
	sub, err := s.substituicaoRepo.FindAtivaByUsuarioPostoData(ctx, empresaID, usuarioID, postoID, now)
	if err != nil {
		return nil, fmt.Errorf("validar substituicao: %w", err)
	}
	if sub != nil {
		esc := &model.Escala{
			HoraInicio:    sub.HoraInicio,
			HoraFim:       sub.HoraFim,
			ToleranciaMin: sub.ToleranciaMin,
		}
		if ok, _ := VerificarToleranciaEscala(esc, now); ok {
			return esc, nil
		}
	}

	hoje := int16(now.Weekday())
	esc, err := s.escalaRepo.FindAtivaByUsuarioPostoDia(ctx, empresaID, usuarioID, postoID, hoje)
	if err != nil {
		return nil, fmt.Errorf("validar escala: %w", err)
	}
	if esc != nil {
		if ok, _ := VerificarToleranciaEscala(esc, now); ok {
			return esc, nil
		}
	}

	ontem := int16(now.AddDate(0, 0, -1).Weekday())
	if ontem == hoje {
		return esc, nil
	}
	escOntem, err := s.escalaRepo.FindAtivaByUsuarioPostoDia(ctx, empresaID, usuarioID, postoID, ontem)
	if err != nil {
		return nil, fmt.Errorf("validar escala: %w", err)
	}
	if escOntem != nil && EscalaCruzaMeiaNoite(escOntem) {
		if ok, _ := VerificarToleranciaEscala(escOntem, now); ok {
			return escOntem, nil
		}
		if esc == nil {
			esc = escOntem
		}
	}

	return esc, nil
}

func (s *TurnoService) Checkin(ctx context.Context, userID, empresaID string, req model.CheckinRequest) (*model.CheckinResponse, error) {
	parsedUserID, err := uuid.Parse(userID)
	if err != nil {
		return nil, fmt.Errorf("user_id invalido: %w", err)
	}
	parsedEmpresaID, err := uuid.Parse(empresaID)
	if err != nil {
		return nil, fmt.Errorf("empresa_id invalido: %w", err)
	}
	parsedTurnoID, err := uuid.Parse(req.TurnoID)
	if err != nil {
		return nil, fmt.Errorf("turno_id invalido: %w", err)
	}

	turno, err := s.turnoRepo.FindByID(ctx, parsedEmpresaID, parsedTurnoID)
	if err != nil {
		return nil, fmt.Errorf("turno: %w", ErrTurnoNaoEncontrado)
	}

	if turno.UsuarioID != parsedUserID {
		return nil, ErrTurnoNaoPertenceAoUsuario
	}

	if turno.Status == "finalizado" {
		return nil, ErrTurnoJaFinalizado
	}

	if err := validarSessaoTurno(turno, req.DeviceID); err != nil {
		return nil, err
	}

	timestampCriacao, err := time.Parse(time.RFC3339, req.Timestamp)
	if err != nil {
		timestampCriacao = time.Now()
	}

	flagGeofence := s.calcularGeofence(ctx, turno.PostoID, parsedEmpresaID, req.Latitude, req.Longitude)

	// resolucao da senha (sem efeito colateral) para montar o Checkin antes do
	// unico INSERT -- evita um UPDATE posterior para gravar TipoSenha/SenhaVigiaID
	senhaResolvida, err := s.resolverSenha(ctx, parsedEmpresaID, parsedUserID, req.Senha)
	if err != nil {
		slog.Warn("senha de vigia nao resolvida", "error", err, "turno_id", parsedTurnoID, "usuario_id", parsedUserID)
		senhaResolvida = nil
	}

	checkin := &model.Checkin{
		TurnoID:          parsedTurnoID,
		EmpresaID:        parsedEmpresaID,
		Latitude:         req.Latitude,
		Longitude:        req.Longitude,
		TimestampCriacao: timestampCriacao,
		Evento:           "checkin",
		FlagGeofence:     flagGeofence,
		OrigemRede:       "online",
	}
	if senhaResolvida != nil {
		tipoSenha := senhaResolvida.Tipo
		checkin.TipoSenha = &tipoSenha
		senhaID := senhaResolvida.ID
		checkin.SenhaVigiaID = &senhaID
	}

	// o ultimo check-in precisa ser capturado antes do INSERT, senao a ancora
	// da janela deslizante vira o proprio check-in recem-criado (A2)
	var anterior *time.Time
	if ultimo := s.checkinRepo.FindUltimoByTurnoNoError(ctx, turno.ID); ultimo != nil {
		anterior = &ultimo.TimestampCriacao
	}

	if err := s.checkinRepo.Create(ctx, checkin); err != nil {
		return nil, fmt.Errorf("criar checkin: %w", err)
	}

	if err := s.alertaService.ResolverAlertasAtraso(ctx, parsedTurnoID); err != nil {
		slog.Error("resolver alertas de atraso apos checkin", "error", err, "turno_id", parsedTurnoID.String())
	}

	atrasado := checkinAtrasado(anterior, turno.InicioReal, turno.IntervaloMin, timestampCriacao)
	dl := timestampCriacao.Add(time.Duration(turno.IntervaloMin) * time.Minute)
	proximoDeadline := &dl

	// efeito colateral da senha (status critico + alerta), na mesma posicao
	// relativa que o antigo bloco de coacao ocupava
	s.aplicarConsequenciaSenha(ctx, empresaID, parsedEmpresaID, parsedTurnoID, parsedUserID, req.Senha)

	s.emitirGPSUpdate(empresaID, req.TurnoID, req.Latitude, req.Longitude, timestampCriacao, flagGeofence)

	posto, err := s.postoRepo.FindByID(ctx, parsedEmpresaID, turno.PostoID)
	postoNome := ""
	if err == nil {
		postoNome = posto.Nome
	}

	return &model.CheckinResponse{
		Checkin:         *checkin,
		Status:          turno.Status,
		PostoNome:       postoNome,
		ProximoDeadline: proximoDeadline,
		Atrasado:        atrasado,
	}, nil
}

func (s *TurnoService) Finalizar(ctx context.Context, userID, empresaID string, req model.FinalizarTurnoRequest) (*model.Turno, error) {
	parsedUserID, err := uuid.Parse(userID)
	if err != nil {
		return nil, fmt.Errorf("user_id invalido: %w", err)
	}
	parsedEmpresaID, err := uuid.Parse(empresaID)
	if err != nil {
		return nil, fmt.Errorf("empresa_id invalido: %w", err)
	}
	parsedTurnoID, err := uuid.Parse(req.TurnoID)
	if err != nil {
		return nil, fmt.Errorf("turno_id invalido: %w", err)
	}

	turno, err := s.turnoRepo.FindByID(ctx, parsedEmpresaID, parsedTurnoID)
	if err != nil {
		return nil, fmt.Errorf("turno: %w", ErrTurnoNaoEncontrado)
	}

	if turno.UsuarioID != parsedUserID {
		return nil, ErrTurnoNaoPertenceAoUsuario
	}

	if turno.Status == "finalizado" {
		return nil, ErrTurnoJaFinalizado
	}

	if err := validarSessaoTurno(turno, req.DeviceID); err != nil {
		return nil, err
	}

	timestampCriacao, err := time.Parse(time.RFC3339, req.Timestamp)
	if err != nil {
		timestampCriacao = time.Now()
	}

	flagGeofence := s.calcularGeofence(ctx, turno.PostoID, parsedEmpresaID, req.Latitude, req.Longitude)

	senhaResolvida, err := s.resolverSenha(ctx, parsedEmpresaID, parsedUserID, req.Senha)
	if err != nil {
		slog.Warn("senha de vigia nao resolvida", "error", err, "turno_id", parsedTurnoID, "usuario_id", parsedUserID)
		senhaResolvida = nil
	}

	checkin := &model.Checkin{
		TurnoID:          parsedTurnoID,
		EmpresaID:        parsedEmpresaID,
		Latitude:         req.Latitude,
		Longitude:        req.Longitude,
		TimestampCriacao: timestampCriacao,
		Evento:           "finalizacao",
		FlagGeofence:     flagGeofence,
		OrigemRede:       "online",
	}
	if senhaResolvida != nil {
		tipoSenha := senhaResolvida.Tipo
		checkin.TipoSenha = &tipoSenha
		senhaID := senhaResolvida.ID
		checkin.SenhaVigiaID = &senhaID
	}

	if err := s.checkinRepo.Create(ctx, checkin); err != nil {
		return nil, fmt.Errorf("criar checkin finalizacao: %w", err)
	}

	// efeito colateral da senha (status critico transitorio + alerta); o status
	// final do turno e sempre "finalizado" -- e sobrescrito logo abaixo. O
	// alerta e quem carrega a urgencia, nao o status final do turno.
	s.aplicarConsequenciaSenha(ctx, empresaID, parsedEmpresaID, parsedTurnoID, parsedUserID, req.Senha)

	now := time.Now()
	if err := s.turnoRepo.UpdateStatus(ctx, parsedTurnoID, parsedEmpresaID, "finalizado", &now); err != nil {
		return nil, fmt.Errorf("finalizar turno: %w", err)
	}

	s.hub.Broadcast(empresaID, ws.NewStatusChangeEvent(req.TurnoID, "finalizado"))
	s.emitirGPSUpdate(empresaID, req.TurnoID, req.Latitude, req.Longitude, timestampCriacao, flagGeofence)

	turno.Status = "finalizado"
	turno.FimReal = &now

	return turno, nil
}

func (s *TurnoService) GetStatus(ctx context.Context, userID, empresaID string) (*model.TurnoStatusResponse, error) {
	parsedUserID, err := uuid.Parse(userID)
	if err != nil {
		return nil, fmt.Errorf("user_id invalido: %w", err)
	}
	parsedEmpresaID, err := uuid.Parse(empresaID)
	if err != nil {
		return nil, fmt.Errorf("empresa_id invalido: %w", err)
	}

	turno, err := s.turnoRepo.FindAtivoByUsuario(ctx, parsedEmpresaID, parsedUserID)
	if err != nil {
		return nil, fmt.Errorf("buscar status: %w", err)
	}
	if turno == nil {
		return nil, ErrTurnoNaoEncontrado
	}

	ultimoCheckin := s.checkinRepo.FindUltimoByTurnoNoError(ctx, turno.ID)

	var proximoDeadline *time.Time
	if ultimoCheckin != nil {
		dl := ultimoCheckin.TimestampCriacao.Add(time.Duration(turno.IntervaloMin) * time.Minute)
		proximoDeadline = &dl
	} else if turno.InicioReal != nil {
		dl := turno.InicioReal.Add(time.Duration(turno.IntervaloMin) * time.Minute)
		proximoDeadline = &dl
	}

	checkinsHoje, err := s.checkinRepo.CountByTurnoHoje(ctx, turno.ID)
	if err != nil {
		checkinsHoje = 0
	}

	return &model.TurnoStatusResponse{
		Turno:           *turno,
		UltimoCheckin:   ultimoCheckin,
		ProximoDeadline: proximoDeadline,
		CheckinsHoje:    checkinsHoje,
	}, nil
}

func (s *TurnoService) GetAtivos(ctx context.Context, empresaID string) ([]model.TurnoDetalhe, error) {
	parsedEmpresaID, err := uuid.Parse(empresaID)
	if err != nil {
		return nil, fmt.Errorf("empresa_id invalido: %w", err)
	}

	turnos, err := s.turnoRepo.ListAtivos(ctx, parsedEmpresaID)
	if err != nil {
		return nil, fmt.Errorf("listar ativos: %w", err)
	}

	result := make([]model.TurnoDetalhe, 0, len(turnos))
	for _, t := range turnos {
		detalhe := model.TurnoDetalhe{Turno: t}

		posto, err := s.postoRepo.FindByID(ctx, parsedEmpresaID, t.PostoID)
		if err == nil {
			detalhe.Posto = posto
		}

		usuario, err := s.userRepo.FindByID(ctx, t.UsuarioID)
		if err == nil {
			u := *usuario
			u.SenhaHash = ""
			u.Telefone = nil
			detalhe.Usuario = &u
		}

		ultimo := s.checkinRepo.FindUltimoByTurnoNoError(ctx, t.ID)
		if ultimo != nil {
			detalhe.Checkins = []model.Checkin{*ultimo}
		}

		result = append(result, detalhe)
	}

	return result, nil
}

func (s *TurnoService) GetByID(ctx context.Context, empresaID, turnoID string) (*model.TurnoDetalhe, error) {
	parsedEmpresaID, err := uuid.Parse(empresaID)
	if err != nil {
		return nil, fmt.Errorf("empresa_id invalido: %w", err)
	}
	parsedTurnoID, err := uuid.Parse(turnoID)
	if err != nil {
		return nil, fmt.Errorf("turno_id invalido: %w", err)
	}

	turno, err := s.turnoRepo.FindByID(ctx, parsedEmpresaID, parsedTurnoID)
	if err != nil {
		return nil, err
	}

	detalhe := &model.TurnoDetalhe{Turno: *turno}

	checkins, err := s.checkinRepo.ListByTurno(ctx, parsedTurnoID)
	if err == nil {
		detalhe.Checkins = checkins
	}

	posto, err := s.postoRepo.FindByID(ctx, parsedEmpresaID, turno.PostoID)
	if err == nil {
		detalhe.Posto = posto
	}

	usuario, err := s.userRepo.FindByID(ctx, turno.UsuarioID)
	if err == nil {
		u := *usuario
		u.SenhaHash = ""
		u.Telefone = nil
		detalhe.Usuario = &u
	}

	return detalhe, nil
}

// validarSessaoTurno garante sessao unica por turno: token_sessao nulo indica
// sessao revogada (aguardando resgate por PIN) e, quando o turno registrou o
// dispositivo de origem, o check-in deve vir dele. Turnos antigos sem
// device_id gravado nao sao bloqueados.
func validarSessaoTurno(turno *model.Turno, deviceID string) error {
	if turno.TokenSessao == nil {
		return ErrSessaoRevogada
	}
	if turno.DeviceID != nil && *turno.DeviceID != deviceID {
		return ErrSessaoOutroDispositivo
	}
	return nil
}

// Revogar invalida a sessao do dispositivo atual do turno e gera um PIN de uso
// unico para o vigia reassociar o turno em um novo aparelho. O turno permanece
// em andamento (PLANNING 8.5).
func (s *TurnoService) Revogar(ctx context.Context, empresaID, turnoID string) (*model.RevogarResponse, error) {
	parsedEmpresaID, err := uuid.Parse(empresaID)
	if err != nil {
		return nil, fmt.Errorf("empresa_id invalido: %w", err)
	}
	parsedTurnoID, err := uuid.Parse(turnoID)
	if err != nil {
		return nil, fmt.Errorf("turno_id invalido: %w", err)
	}

	turno, err := s.turnoRepo.FindByID(ctx, parsedEmpresaID, parsedTurnoID)
	if err != nil {
		return nil, fmt.Errorf("turno: %w", ErrTurnoNaoEncontrado)
	}
	if turno.Status == "finalizado" {
		return nil, ErrTurnoJaFinalizado
	}

	pin, err := generatePIN(6)
	if err != nil {
		return nil, fmt.Errorf("gerar pin: %w", err)
	}
	validadeMinutos := 15
	pinValidoAte := time.Now().Add(time.Duration(validadeMinutos) * time.Minute)

	if err := s.turnoRepo.RevogarToken(ctx, parsedTurnoID, parsedEmpresaID, pin, pinValidoAte); err != nil {
		return nil, fmt.Errorf("revogar turno: %w", err)
	}

	return &model.RevogarResponse{
		PinNovoDispositivo: pin,
		ValidadeMinutos:    validadeMinutos,
	}, nil
}

// Reassociar consome o PIN gerado na revogacao e vincula o turno ativo do
// usuario a um novo dispositivo (que ja deve ter biometria registrada),
// emitindo um novo token de sessao.
func (s *TurnoService) Reassociar(ctx context.Context, userID, empresaID string, req model.ReassociarRequest) (*model.Turno, error) {
	parsedUserID, err := uuid.Parse(userID)
	if err != nil {
		return nil, fmt.Errorf("user_id invalido: %w", err)
	}
	parsedEmpresaID, err := uuid.Parse(empresaID)
	if err != nil {
		return nil, fmt.Errorf("empresa_id invalido: %w", err)
	}

	sessao, err := s.sessaoDispositivoRepo.FindByDeviceID(ctx, empresaID, req.DeviceID)
	if err != nil || sessao.UsuarioID != parsedUserID {
		return nil, ErrDeviceNaoRegistrado
	}

	turno, err := s.turnoRepo.FindAtivoComPinByUsuario(ctx, parsedEmpresaID, parsedUserID)
	if err != nil {
		return nil, fmt.Errorf("buscar turno ativo: %w", err)
	}
	if turno == nil {
		return nil, ErrTurnoNaoEncontrado
	}
	if turno.Pin == nil || *turno.Pin != req.Pin {
		return nil, ErrPinInvalido
	}
	if turno.PinValidoAte == nil || time.Now().After(*turno.PinValidoAte) {
		return nil, ErrPinExpirado
	}

	tokenSessao := uuid.New().String()
	if err := s.turnoRepo.Reassociar(ctx, turno.ID, parsedEmpresaID, req.DeviceID, tokenSessao); err != nil {
		return nil, fmt.Errorf("reassociar turno: %w", err)
	}

	turno.TokenSessao = &tokenSessao
	deviceID := req.DeviceID
	turno.DeviceID = &deviceID
	turno.Pin = nil
	turno.PinValidoAte = nil

	return turno, nil
}

func generatePIN(digits int) (string, error) {
	max := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(digits)), nil)
	n, err := rand.Int(rand.Reader, max)
	if err != nil {
		return "", fmt.Errorf("gerar numero aleatorio: %w", err)
	}
	return fmt.Sprintf("%0*d", digits, n), nil
}

func (s *TurnoService) GetHistorico(ctx context.Context, empresaID string, filter model.HistoricoFilter) ([]model.Turno, int, error) {
	parsedEmpresaID, err := uuid.Parse(empresaID)
	if err != nil {
		return nil, 0, fmt.Errorf("empresa_id invalido: %w", err)
	}

	return s.turnoRepo.ListHistorico(ctx, parsedEmpresaID, filter)
}

func (s *TurnoService) Sabotagem(ctx context.Context, userID, empresaID string, req model.SabotagemRequest) (*model.SabotagemResponse, error) {
	parsedUserID, err := uuid.Parse(userID)
	if err != nil {
		return nil, fmt.Errorf("user_id invalido: %w", err)
	}
	parsedEmpresaID, err := uuid.Parse(empresaID)
	if err != nil {
		return nil, fmt.Errorf("empresa_id invalido: %w", err)
	}
	parsedTurnoID, err := uuid.Parse(req.TurnoID)
	if err != nil {
		return nil, fmt.Errorf("turno_id invalido: %w", err)
	}

	turno, err := s.turnoRepo.FindByID(ctx, parsedEmpresaID, parsedTurnoID)
	if err != nil {
		return nil, fmt.Errorf("turno: %w", ErrTurnoNaoEncontrado)
	}

	if turno.UsuarioID != parsedUserID {
		return nil, ErrTurnoNaoPertenceAoUsuario
	}

	if turno.Status == "finalizado" {
		return nil, ErrTurnoJaFinalizado
	}

	if err := validarSessaoTurno(turno, req.DeviceID); err != nil {
		return nil, err
	}

	timestampCriacao, err := time.Parse(time.RFC3339, req.Timestamp)
	if err != nil {
		timestampCriacao = time.Now()
	}

	flagGeofence := s.calcularGeofence(ctx, turno.PostoID, parsedEmpresaID, req.Latitude, req.Longitude)

	_ = s.turnoRepo.UpdateStatus(ctx, parsedTurnoID, parsedEmpresaID, "critico", nil)
	s.hub.Broadcast(empresaID, ws.NewStatusChangeEvent(req.TurnoID, "critico"))

	checkin := &model.Checkin{
		TurnoID:          parsedTurnoID,
		EmpresaID:        parsedEmpresaID,
		Latitude:         req.Latitude,
		Longitude:        req.Longitude,
		TimestampCriacao: timestampCriacao,
		Evento:           "sabotagem",
		FlagGeofence:     flagGeofence,
		OrigemRede:       "online",
	}
	_ = s.checkinRepo.Create(ctx, checkin)

	s.emitirGPSUpdate(empresaID, req.TurnoID, req.Latitude, req.Longitude, timestampCriacao, flagGeofence)

	alerta, err := s.alertaService.CreateAlertaImediato(ctx, parsedEmpresaID, parsedTurnoID, "sabotagem", "Sabotagem reportada pelo vigia. Motivo: "+req.Motivo)
	alertaID := ""
	if err == nil && alerta != nil {
		alertaID = alerta.ID.String()
	} else {
		alertaID = uuid.New().String()
	}

	return &model.SabotagemResponse{
		AlertaID: alertaID,
		Status:   "registrado",
		Mensagem: "sabotagem reportada com sucesso",
	}, nil
}

func (s *TurnoService) ProcessarLote(ctx context.Context, userID, empresaID string, checkins []model.CheckinRequest) ([]model.CheckinResponse, error) {
	parsedUserID, err := uuid.Parse(userID)
	if err != nil {
		return nil, fmt.Errorf("user_id invalido: %w", err)
	}
	parsedEmpresaID, err := uuid.Parse(empresaID)
	if err != nil {
		return nil, fmt.Errorf("empresa_id invalido: %w", err)
	}

	resultados := make([]model.CheckinResponse, 0, len(checkins))

	for _, req := range checkins {
		parsedTurnoID, err := uuid.Parse(req.TurnoID)
		if err != nil {
			continue
		}

		turno, err := s.turnoRepo.FindByID(ctx, parsedEmpresaID, parsedTurnoID)
		if err != nil {
			continue
		}

		if turno.UsuarioID != parsedUserID {
			continue
		}

		if turno.Status == "finalizado" {
			continue
		}

		if err := validarSessaoTurno(turno, req.DeviceID); err != nil {
			continue
		}

		timestampCriacao, err := time.Parse(time.RFC3339, req.Timestamp)
		if err != nil {
			timestampCriacao = time.Now()
		}

		flagGeofence := s.calcularGeofence(ctx, turno.PostoID, parsedEmpresaID, req.Latitude, req.Longitude)

		origemRede := "offline_sincronizado"

		senhaResolvida, err := s.resolverSenha(ctx, parsedEmpresaID, parsedUserID, req.Senha)
		if err != nil {
			slog.Warn("senha de vigia nao resolvida", "error", err, "turno_id", parsedTurnoID, "usuario_id", parsedUserID)
			senhaResolvida = nil
		}

		checkin := &model.Checkin{
			TurnoID:          parsedTurnoID,
			EmpresaID:        parsedEmpresaID,
			Latitude:         req.Latitude,
			Longitude:        req.Longitude,
			TimestampCriacao: timestampCriacao,
			Evento:           "checkin",
			FlagGeofence:     flagGeofence,
			OrigemRede:       origemRede,
		}
		if senhaResolvida != nil {
			tipoSenha := senhaResolvida.Tipo
			checkin.TipoSenha = &tipoSenha
			senhaID := senhaResolvida.ID
			checkin.SenhaVigiaID = &senhaID
		}

		var anterior *time.Time
		if ultimo := s.checkinRepo.FindUltimoByTurnoNoError(ctx, turno.ID); ultimo != nil {
			anterior = &ultimo.TimestampCriacao
		}

		if req.ClienteCheckinID != "" {
			cid := req.ClienteCheckinID
			checkin.ClienteCheckinID = &cid
			if _, err := s.checkinRepo.CreateIdempotent(ctx, checkin); err != nil {
				continue
			}
		} else {
			if err := s.checkinRepo.Create(ctx, checkin); err != nil {
				continue
			}
		}

		s.aplicarConsequenciaSenha(ctx, empresaID, parsedEmpresaID, parsedTurnoID, parsedUserID, req.Senha)

		s.emitirGPSUpdate(empresaID, req.TurnoID, req.Latitude, req.Longitude, timestampCriacao, flagGeofence)

		atrasado := checkinAtrasado(anterior, turno.InicioReal, turno.IntervaloMin, timestampCriacao)
		dl := timestampCriacao.Add(time.Duration(turno.IntervaloMin) * time.Minute)
		proximoDeadline := &dl

		posto, err := s.postoRepo.FindByID(ctx, parsedEmpresaID, turno.PostoID)
		postoNome := ""
		if err == nil {
			postoNome = posto.Nome
		}

		resultados = append(resultados, model.CheckinResponse{
			Checkin:         *checkin,
			Status:          turno.Status,
			PostoNome:       postoNome,
			ProximoDeadline: proximoDeadline,
			Atrasado:        atrasado,
		})
	}

	return resultados, nil
}

func (s *TurnoService) calcularGeofence(ctx context.Context, postoID, empresaID uuid.UUID, lat, lon float64) *string {
	posto, err := s.postoRepo.FindByID(ctx, empresaID, postoID)
	if err != nil {
		geofence := "ok"
		return &geofence
	}

	geofence := classificarGeofence(lat, lon, posto.Latitude, posto.Longitude, posto.RaioM)
	return &geofence
}

func classificarGeofence(lat, lon, postoLat, postoLon float64, raioM int) string {
	if haversine(lat, lon, postoLat, postoLon) > float64(raioM) {
		return "desvio_rota"
	}
	return "ok"
}

// checkinAtrasado indica se um check-in em `ts` estourou a janela deslizante.
// A ancora e o check-in imediatamente anterior ou, no primeiro check-in do
// turno, o inicio real.
func checkinAtrasado(anterior, inicioReal *time.Time, intervaloMin int, ts time.Time) bool {
	ancora := anterior
	if ancora == nil {
		ancora = inicioReal
	}
	if ancora == nil {
		return false
	}
	return ts.After(ancora.Add(time.Duration(intervaloMin) * time.Minute))
}

func haversine(lat1, lon1, lat2, lon2 float64) float64 {
	const earthRadius = 6371000.0

	dLat := (lat2 - lat1) * (math.Pi / 180.0)
	dLon := (lon2 - lon1) * (math.Pi / 180.0)

	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1*(math.Pi/180.0))*math.Cos(lat2*(math.Pi/180.0))*
			math.Sin(dLon/2)*math.Sin(dLon/2)

	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))

	return earthRadius * c
}

func (s *TurnoService) emitirGPSUpdate(empresaID, turnoID string, lat, lon float64, ts time.Time, flagGeofence *string) {
	if s.hub == nil {
		return
	}
	s.hub.Broadcast(empresaID, ws.NewGPSUpdateEvent(turnoID, ts.UTC().Format(time.RFC3339), lat, lon, flagGeofence))
}
