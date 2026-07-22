package service

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"math/big"
	"sort"
	"time"

	"github.com/google/uuid"

	"github.com/guardpoint/guardpoint-server/internal/model"
	"github.com/guardpoint/guardpoint-server/internal/timeutil"
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

type TurnoTurnoRepository interface {
	Create(ctx context.Context, t *model.Turno) error
	FindAtivoByUsuario(ctx context.Context, empresaID, usuarioID uuid.UUID) (*model.Turno, error)
	FindByID(ctx context.Context, empresaID, id uuid.UUID) (*model.Turno, error)
	UpdateStatus(ctx context.Context, id, empresaID uuid.UUID, status string, fimReal *time.Time) error
	ListAtivos(ctx context.Context, empresaID uuid.UUID) ([]model.Turno, error)
	ListHistorico(ctx context.Context, empresaID uuid.UUID, filter model.HistoricoFilter) ([]model.Turno, int, error)
	ListTurnos(ctx context.Context, empresaID uuid.UUID, filter model.TurnoFilter) ([]model.Turno, error)
	ListTurnosByDateRange(ctx context.Context, empresaID uuid.UUID, dataInicio, dataFim string, usuarioIDs, postoIDs []string) ([]model.Turno, error)
	RevogarToken(ctx context.Context, id, empresaID uuid.UUID, pin string, pinValidoAte time.Time) error
	FindAtivoComPinByUsuario(ctx context.Context, empresaID, usuarioID uuid.UUID) (*model.Turno, error)
	Reassociar(ctx context.Context, id, empresaID uuid.UUID, deviceID, tokenSessao string) error
}

type TurnoCheckinRepository interface {
	Create(ctx context.Context, c *model.Checkin) error
	FindUltimoByTurnoNoError(ctx context.Context, turnoID uuid.UUID) *model.Checkin
	CountByTurnoHoje(ctx context.Context, turnoID uuid.UUID) (int, error)
	CreateIdempotent(ctx context.Context, c *model.Checkin) (bool, error)
	ListByTurno(ctx context.Context, turnoID uuid.UUID) ([]model.Checkin, error)
}

type TurnoPostoRepository interface {
	FindByID(ctx context.Context, empresaID, id uuid.UUID) (*model.Posto, error)
}

type TurnoUserRepository interface {
	FindByID(ctx context.Context, id uuid.UUID) (*model.User, error)
}

type TurnoSessaoDispositivoRepository interface {
	FindByDeviceID(ctx context.Context, empresaID, deviceID string) (*model.SessaoDispositivo, error)
}

type TurnoEscalaRepository interface {
	FindAtivaByUsuarioPostoDia(ctx context.Context, empresaID, usuarioID, postoID uuid.UUID, diaSemana int16) (*model.Escala, error)
	ListAtivasByEmpresa(ctx context.Context, empresaID uuid.UUID, usuarioIDs, postoIDs []string) ([]model.Escala, error)
}

type TurnoSubstituicaoRepository interface {
	FindAtivaByUsuarioPostoData(ctx context.Context, empresaID, usuarioID, postoID uuid.UUID, data time.Time) (*model.Substituicao, error)
	ListAtivasByDateRange(ctx context.Context, empresaID uuid.UUID, dataInicio, dataFim string, usuarioIDs, postoIDs []string) ([]model.Substituicao, error)
}

type TurnoSenhaVigiaRepository interface {
	CountByUsuario(ctx context.Context, empresaID, usuarioID uuid.UUID) (int, error)
	FindByUsuarioECodigo(ctx context.Context, empresaID, usuarioID uuid.UUID, codigo string) (*model.SenhaVigia, error)
}

type TurnoService struct {
	turnoRepo             TurnoTurnoRepository
	checkinRepo           TurnoCheckinRepository
	postoRepo             TurnoPostoRepository
	userRepo              TurnoUserRepository
	sessaoDispositivoRepo TurnoSessaoDispositivoRepository
	escalaRepo            TurnoEscalaRepository
	substituicaoRepo      TurnoSubstituicaoRepository
	senhaVigiaRepo        TurnoSenhaVigiaRepository
	alertaService         *AlertaService
	hub                   *ws.Hub
}

func NewTurnoService(
	turnoRepo TurnoTurnoRepository,
	checkinRepo TurnoCheckinRepository,
	postoRepo TurnoPostoRepository,
	userRepo TurnoUserRepository,
	sessaoDispositivoRepo TurnoSessaoDispositivoRepository,
	escalaRepo TurnoEscalaRepository,
	substituicaoRepo TurnoSubstituicaoRepository,
	senhaVigiaRepo TurnoSenhaVigiaRepository,
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
func (s *TurnoService) aplicarConsequenciaSenha(ctx context.Context, empresaIDStr string, empresaID, turnoID, usuarioID, postoID uuid.UUID, codigo string) *model.SenhaVigia {
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
		mensagem = "Senha customizada detectada"
	}

	if _, err := s.alertaService.CreateAlertaImediato(ctx, empresaID, turnoID, postoID, tipoAlerta, mensagem, senha.NivelEscalonamentoID); err != nil {
		slog.Error("criar alerta de senha", "error", err, "turno_id", turnoID)
	}
	s.hub.Broadcast(empresaIDStr, ws.NewStatusChangeEvent(turnoID.String(), "critico"))
	return senha
}

// calcularProximoDeadline determina o proximo deadline e seu tipo a partir da
// ancora (timestamp do checkin atual ou inicio do turno). Se ancora + intervaloMin
// alcanca ou ultrapassa o fim previsto do turno, o deadline vira o proprio
// fim do turno com tipo "finalizar" — senao, vira ancora + intervaloMin com
// tipo "checkin".
func calcularProximoDeadline(ancora time.Time, intervaloMin int, fimPrevisto time.Time) (time.Time, string) {
	dl := ancora.Add(time.Duration(intervaloMin) * time.Minute)
	if !dl.Before(fimPrevisto) {
		return fimPrevisto, "finalizar"
	}
	return dl, "checkin"
}

func (s *TurnoService) Iniciar(ctx context.Context, userID, empresaID string, req model.IniciarTurnoRequest) (*model.IniciarResponse, error) {
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

	now := timeutil.NowBRT()
	esc, subID, err := s.buscarEscalaParaInicio(ctx, parsedEmpresaID, parsedUserID, parsedPostoID, now)
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

	dateStr := now.Format("2006-01-02")
	fimPrevisto, err := parseHoraData(dateStr, esc.HoraFim)
	if err != nil {
		fimPrevisto = now.Add(12 * time.Hour)
	}
	if !fimPrevisto.After(now) {
		fimPrevisto = fimPrevisto.AddDate(0, 0, 1)
	}

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
		SubstituicaoID: subID,
	}

	if err := s.turnoRepo.Create(ctx, turno); err != nil {
		return nil, fmt.Errorf("criar turno: %w", err)
	}

	s.hub.Broadcast(empresaID, ws.NewStatusChangeEvent(turno.ID.String(), "em_andamento"))

	flagGeofence := s.calcularGeofence(ctx, turno.PostoID, parsedEmpresaID, req.Latitude, req.Longitude)
	senha := s.aplicarConsequenciaSenha(ctx, empresaID, parsedEmpresaID, turno.ID, parsedUserID, turno.PostoID, req.Senha)

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

	dl, tipo := calcularProximoDeadline(now, intervaloMin, fimPrevisto)

	return &model.IniciarResponse{
		Turno:               *turno,
		ProximoDeadline:     dl,
		TipoProximoDeadline: tipo,
		Atrasado:            false,
	}, nil
}

// buscarEscalaParaInicio procura a escala ativa compativel com o inicio de turno
// em `now`. Primeiro verifica substituicoes pontuais (ex.: vigia cobrindo falta),
// depois cai no fluxo normal de escalas semanais.
// Retorna a escala encontrada e, se aplicavel, o ID da substituicao que a originou.
func (s *TurnoService) buscarEscalaParaInicio(ctx context.Context, empresaID, usuarioID, postoID uuid.UUID, now time.Time) (*model.Escala, *uuid.UUID, error) {
	sub, err := s.substituicaoRepo.FindAtivaByUsuarioPostoData(ctx, empresaID, usuarioID, postoID, now)
	if err != nil {
		return nil, nil, fmt.Errorf("validar substituicao: %w", err)
	}
	if sub != nil {
		esc := &model.Escala{
			HoraInicio:    sub.HoraInicio,
			HoraFim:       sub.HoraFim,
			ToleranciaMin: sub.ToleranciaMin,
		}
		if ok, _ := VerificarToleranciaEscala(esc, now); ok {
			return esc, &sub.ID, nil
		}
	}

	hoje := int16(now.Weekday())
	esc, err := s.escalaRepo.FindAtivaByUsuarioPostoDia(ctx, empresaID, usuarioID, postoID, hoje)
	if err != nil {
		return nil, nil, fmt.Errorf("validar escala: %w", err)
	}
	if esc != nil {
		if ok, _ := VerificarToleranciaEscala(esc, now); ok {
			return esc, nil, nil
		}
	}

	ontem := int16(now.AddDate(0, 0, -1).Weekday())
	if ontem == hoje {
		return esc, nil, nil
	}
	escOntem, err := s.escalaRepo.FindAtivaByUsuarioPostoDia(ctx, empresaID, usuarioID, postoID, ontem)
	if err != nil {
		return nil, nil, fmt.Errorf("validar escala: %w", err)
	}
	if escOntem != nil && EscalaCruzaMeiaNoite(escOntem) {
		if ok, _ := VerificarToleranciaEscala(escOntem, now); ok {
			return escOntem, nil, nil
		}
		if esc == nil {
			esc = escOntem
		}
	}

	return esc, nil, nil
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

	if turno.Status == "finalizado" || turno.Status == "atrasado" {
		return nil, ErrTurnoJaFinalizado
	}

	if err := validarSessaoTurno(turno, req.DeviceID); err != nil {
		return nil, err
	}

	timestampCriacao, err := time.Parse(time.RFC3339, req.Timestamp)
	if err != nil {
		timestampCriacao = timeutil.NowBRT()
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
	dl, tipoProximo := calcularProximoDeadline(timestampCriacao, turno.IntervaloMin, turno.FimPrevisto)
	proximoDeadline := &dl

	// efeito colateral da senha (status critico + alerta), na mesma posicao
	// relativa que o antigo bloco de coacao ocupava
	s.aplicarConsequenciaSenha(ctx, empresaID, parsedEmpresaID, parsedTurnoID, parsedUserID, turno.PostoID, req.Senha)

	s.emitirGPSUpdate(empresaID, req.TurnoID, req.Latitude, req.Longitude, timestampCriacao, flagGeofence)

	posto, err := s.postoRepo.FindByID(ctx, parsedEmpresaID, turno.PostoID)
	postoNome := ""
	if err == nil {
		postoNome = posto.Nome
	}

	return &model.CheckinResponse{
		Checkin:             *checkin,
		Status:              turno.Status,
		PostoNome:           postoNome,
		ProximoDeadline:     proximoDeadline,
		TipoProximoDeadline: tipoProximo,
		Atrasado:            atrasado,
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

	if turno.Status == "finalizado" || turno.Status == "atrasado" {
		return nil, ErrTurnoJaFinalizado
	}

	if err := validarSessaoTurno(turno, req.DeviceID); err != nil {
		return nil, err
	}

	timestampCriacao, err := time.Parse(time.RFC3339, req.Timestamp)
	if err != nil {
		timestampCriacao = timeutil.NowBRT()
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
	s.aplicarConsequenciaSenha(ctx, empresaID, parsedEmpresaID, parsedTurnoID, parsedUserID, turno.PostoID, req.Senha)

	now := timeutil.NowBRT()
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
	tipoProximoDeadline := ""
	if ultimoCheckin != nil {
		dl, tipo := calcularProximoDeadline(ultimoCheckin.TimestampCriacao, turno.IntervaloMin, turno.FimPrevisto)
		proximoDeadline = &dl
		tipoProximoDeadline = tipo
	} else if turno.InicioReal != nil {
		dl, tipo := calcularProximoDeadline(*turno.InicioReal, turno.IntervaloMin, turno.FimPrevisto)
		proximoDeadline = &dl
		tipoProximoDeadline = tipo
	}

	checkinsHoje, err := s.checkinRepo.CountByTurnoHoje(ctx, turno.ID)
	if err != nil {
		checkinsHoje = 0
	}

	atrasado := false
	if proximoDeadline != nil && timeutil.NowBRT().After(*proximoDeadline) {
		atrasado = true
	}

	return &model.TurnoStatusResponse{
		Turno:               *turno,
		UltimoCheckin:       ultimoCheckin,
		ProximoDeadline:     proximoDeadline,
		TipoProximoDeadline: tipoProximoDeadline,
		CheckinsHoje:        checkinsHoje,
		Atrasado:            atrasado,
	}, nil
}

func (s *TurnoService) GetVigiaTurno(ctx context.Context, userID, empresaID string) (*model.VigiaTurnoResponse, error) {
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
		return nil, fmt.Errorf("buscar turno ativo: %w", err)
	}

	if turno != nil {
		posto, _ := s.postoRepo.FindByID(ctx, parsedEmpresaID, turno.PostoID)

		ultimoCheckin := s.checkinRepo.FindUltimoByTurnoNoError(ctx, turno.ID)

		var proximoDeadline *time.Time
		tipoProximoDeadline := ""
		if ultimoCheckin != nil {
			dl, tipo := calcularProximoDeadline(ultimoCheckin.TimestampCriacao, turno.IntervaloMin, turno.FimPrevisto)
			proximoDeadline = &dl
			tipoProximoDeadline = tipo
		} else if turno.InicioReal != nil {
			dl, tipo := calcularProximoDeadline(*turno.InicioReal, turno.IntervaloMin, turno.FimPrevisto)
			proximoDeadline = &dl
			tipoProximoDeadline = tipo
		}

		atrasado := false
		if proximoDeadline != nil && timeutil.NowBRT().After(*proximoDeadline) {
			atrasado = true
		}

		checkinsHoje, _ := s.checkinRepo.CountByTurnoHoje(ctx, turno.ID)

		postoNome := ""
		if posto != nil {
			postoNome = posto.Nome
		} else if turno.PostoNome != "" {
			postoNome = turno.PostoNome
		}

		return &model.VigiaTurnoResponse{
			TemTurnoAtivo: true,
			Mensagem:      "turno em andamento",
			Turno: &model.VigiaTurnoInfo{
				ID:                  turno.ID,
				Status:              turno.Status,
				Posto:               posto,
				PostoNome:           postoNome,
				TokenSessao:         turno.TokenSessao,
				InicioPrevisto:      turno.InicioPrevisto,
				FimPrevisto:         turno.FimPrevisto,
				InicioReal:          turno.InicioReal,
				IntervaloMin:        turno.IntervaloMin,
				ProximoDeadline:     proximoDeadline,
				TipoProximoDeadline: tipoProximoDeadline,
				Atrasado:            atrasado,
				CheckinsHoje:        checkinsHoje,
				UltimoCheckin:       ultimoCheckin,
			},
		}, nil
	}

	proximo, err := s.buscarProximoTurnoAgendado(ctx, parsedEmpresaID, parsedUserID)
	if err != nil {
		return nil, fmt.Errorf("buscar proximo turno: %w", err)
	}
	if proximo != nil {
		return &model.VigiaTurnoResponse{
			TemTurnoAtivo: false,
			Mensagem:      "nenhum turno ativo",
			ProximoTurno:  proximo,
		}, nil
	}

	return &model.VigiaTurnoResponse{
		TemTurnoAtivo: false,
		Mensagem:      "nenhum turno ativo ou agendado encontrado",
	}, nil
}

func (s *TurnoService) buscarProximoTurnoAgendado(ctx context.Context, empresaID, usuarioID uuid.UUID) (*model.VigiaProximoTurno, error) {
	now := timeutil.NowBRT()
	dataInicio := now.Format("2006-01-02")
	dataFim := now.AddDate(0, 0, 7).Format("2006-01-02")

	escalas, err := s.escalaRepo.ListAtivasByEmpresa(ctx, empresaID, []string{usuarioID.String()}, nil)
	if err != nil {
		return nil, fmt.Errorf("listar escalas: %w", err)
	}

	subs, err := s.substituicaoRepo.ListAtivasByDateRange(ctx, empresaID, dataInicio, dataFim, []string{usuarioID.String()}, nil)
	if err != nil {
		return nil, fmt.Errorf("listar substituicoes: %w", err)
	}

	subByUserPostoDate := make(map[string]*model.Substituicao)
	for i := range subs {
		sub := &subs[i]
		current := sub.DataInicio
		for !current.After(sub.DataFim) {
			dateStr := current.Format("2006-01-02")
			key := sub.UsuarioID.String() + "|" + sub.PostoID.String() + "|" + dateStr
			subByUserPostoDate[key] = sub
			current = current.AddDate(0, 0, 1)
		}
	}

	type candidato struct {
		inicio time.Time
		fim    time.Time
		posto  *model.Posto
		horaInicio string
		horaFim    string
	}

	var candidatos []candidato

	for d := now; !d.After(now.AddDate(0, 0, 7)); d = d.AddDate(0, 0, 1) {
		diaSemana := int16(d.Weekday())
		dateStr := d.Format("2006-01-02")

		for _, esc := range escalas {
			if esc.UsuarioID != usuarioID {
				continue
			}
			if esc.DiaSemanaInicio != diaSemana {
				continue
			}

			key := usuarioID.String() + "|" + esc.PostoID.String() + "|" + dateStr
			horaInicio := esc.HoraInicio
			horaFim := esc.HoraFim

			if sub, ok := subByUserPostoDate[key]; ok {
				horaInicio = sub.HoraInicio
				horaFim = sub.HoraFim
			}

			inicio, err := parseHoraData(dateStr, horaInicio)
			if err != nil {
				continue
			}
			fim, err := parseHoraData(dateStr, horaFim)
			if err != nil {
				continue
			}
			if !fim.After(inicio) {
				fim = fim.AddDate(0, 0, 1)
			}

			if inicio.Before(now) {
				continue
			}

			posto, err := s.postoRepo.FindByID(ctx, empresaID, esc.PostoID)
			if err != nil {
				continue
			}

			candidatos = append(candidatos, candidato{
				inicio:     inicio,
				fim:        fim,
				posto:      posto,
				horaInicio: horaInicio,
				horaFim:    horaFim,
			})
		}
	}

	if len(candidatos) == 0 {
		return nil, nil
	}

	maisProximo := candidatos[0]
	for _, c := range candidatos[1:] {
		if c.inicio.Before(maisProximo.inicio) {
			maisProximo = c
		}
	}

	return &model.VigiaProximoTurno{
		Posto:          maisProximo.posto,
		InicioPrevisto: maisProximo.inicio,
		FimPrevisto:    maisProximo.fim,
		Data:           maisProximo.inicio.Format("2006-01-02"),
		HoraInicio:     maisProximo.horaInicio,
		HoraFim:        maisProximo.horaFim,
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

func (s *TurnoService) GetTurnosMapa(ctx context.Context, empresaID string) ([]model.TurnoMapa, error) {
	parsedEmpresaID, err := uuid.Parse(empresaID)
	if err != nil {
		return nil, fmt.Errorf("empresa_id invalido: %w", err)
	}

	turnos, err := s.turnoRepo.ListAtivos(ctx, parsedEmpresaID)
	if err != nil {
		return nil, fmt.Errorf("listar ativos: %w", err)
	}

	result := make([]model.TurnoMapa, 0, len(turnos))
	for _, t := range turnos {
		item := model.TurnoMapa{
			ID:             t.ID,
			UsuarioNome:    t.UsuarioNome,
			PostoID:        t.PostoID,
			PostoNome:      t.PostoNome,
			Status:         t.Status,
			InicioPrevisto: t.InicioPrevisto,
			FimPrevisto:    t.FimPrevisto,
		}

		posto, err := s.postoRepo.FindByID(ctx, parsedEmpresaID, t.PostoID)
		if err == nil {
			item.PostoLatitude = posto.Latitude
			item.PostoLongitude = posto.Longitude
			item.PostoRaioM = posto.RaioM
			if item.PostoNome == "" {
				item.PostoNome = posto.Nome
			}
		}

		item.UltimoCheckin = s.checkinRepo.FindUltimoByTurnoNoError(ctx, t.ID)

		result = append(result, item)
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
	if turno.Status == "finalizado" || turno.Status == "atrasado" {
		return nil, ErrTurnoJaFinalizado
	}

	pin, err := generatePIN(6)
	if err != nil {
		return nil, fmt.Errorf("gerar pin: %w", err)
	}
	validadeMinutos := 15
	pinValidoAte := timeutil.NowBRT().Add(time.Duration(validadeMinutos) * time.Minute)

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
	if turno.PinValidoAte == nil || timeutil.NowBRT().After(*turno.PinValidoAte) {
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

func (s *TurnoService) GetTurnos(ctx context.Context, empresaID string, filter model.TurnoFilter) ([]model.TurnoDetalhe, int, error) {
	parsedEmpresaID, err := uuid.Parse(empresaID)
	if err != nil {
		return nil, 0, fmt.Errorf("empresa_id invalido: %w", err)
	}

	requestedStatuses := filter.Status
	hasAgendado := false
	hasReal := false
	for _, st := range requestedStatuses {
		if st == "agendado" {
			hasAgendado = true
		} else {
			hasReal = true
		}
	}
	if len(requestedStatuses) == 0 {
		hasAgendado = true
		hasReal = true
	}

	var allTurnos []model.TurnoDetalhe

	if hasAgendado {
		agendados, err := s.gerarTurnosAgendados(ctx, parsedEmpresaID, filter)
		if err != nil {
			return nil, 0, fmt.Errorf("gerar turnos agendados: %w", err)
		}
		for _, t := range agendados {
			allTurnos = append(allTurnos, model.TurnoDetalhe{Turno: t})
		}
	}

	if hasReal {
		reais, err := s.turnoRepo.ListTurnos(ctx, parsedEmpresaID, filter)
		if err != nil {
			return nil, 0, fmt.Errorf("listar turnos reais: %w", err)
		}
		for _, t := range reais {
			allTurnos = append(allTurnos, model.TurnoDetalhe{Turno: t})
		}
	}

	sortBy := filter.SortBy
	if sortBy == "" {
		sortBy = "inicio_previsto"
	}
	sortDesc := filter.SortOrder == "desc"

	sort.Slice(allTurnos, func(i, j int) bool {
		a, b := allTurnos[i].Turno, allTurnos[j].Turno
		var less bool
		switch sortBy {
		case "created_at":
			less = a.CreatedAt.Before(b.CreatedAt)
		case "status":
			less = a.Status < b.Status
		default:
			less = a.InicioPrevisto.Before(b.InicioPrevisto)
		}
		if sortDesc {
			return !less
		}
		return less
	})

	total := len(allTurnos)

	if filter.Limit <= 0 {
		filter.Limit = 20
	}
	if filter.Offset < 0 {
		filter.Offset = 0
	}
	if filter.Offset >= len(allTurnos) {
		return []model.TurnoDetalhe{}, total, nil
	}
	end := filter.Offset + filter.Limit
	if end > len(allTurnos) {
		end = len(allTurnos)
	}

	return allTurnos[filter.Offset:end], total, nil
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

func parseHoraData(dateStr, hora string) (time.Time, error) {
	formats := []string{
		"2006-01-02 15:04:05",
		"2006-01-02 15:04",
	}
	for _, f := range formats {
		t, err := time.ParseInLocation(f, dateStr+" "+hora, timeutil.BRT)
		if err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("formato de hora invalido: %s", hora)
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

	if turno.Status == "finalizado" || turno.Status == "atrasado" {
		return nil, ErrTurnoJaFinalizado
	}

	if err := validarSessaoTurno(turno, req.DeviceID); err != nil {
		return nil, err
	}

	timestampCriacao, err := time.Parse(time.RFC3339, req.Timestamp)
	if err != nil {
		timestampCriacao = timeutil.NowBRT()
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

	alerta, err := s.alertaService.CreateAlertaImediato(ctx, parsedEmpresaID, parsedTurnoID, turno.PostoID, "sabotagem", "Sabotagem reportada pelo vigia. Motivo: "+req.Motivo, nil)
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
