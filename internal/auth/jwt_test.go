package auth

import (
	"errors"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

const testSecret = "unit-test-secret-with-enough-length"

func TestGenerateAndValidateAccessToken(t *testing.T) {
	svc := NewJWTService(testSecret)
	userID := uuid.New()
	empresaID := uuid.New()

	token, err := svc.GenerateAccessToken(userID, empresaID, "vigia@example.com", "vigia", "Vigia Teste")
	if err != nil {
		t.Fatalf("GenerateAccessToken() erro: %v", err)
	}

	claims, err := svc.ValidateToken(token)
	if err != nil {
		t.Fatalf("ValidateToken() erro: %v", err)
	}

	if claims.UserID != userID.String() {
		t.Errorf("UserID = %q, esperado %q", claims.UserID, userID)
	}
	if claims.EmpresaID != empresaID.String() {
		t.Errorf("EmpresaID = %q, esperado %q", claims.EmpresaID, empresaID)
	}
	if claims.Email != "vigia@example.com" {
		t.Errorf("Email = %q", claims.Email)
	}
	if claims.Role != "vigia" {
		t.Errorf("Role = %q", claims.Role)
	}
	if claims.Nome != "Vigia Teste" {
		t.Errorf("Nome = %q", claims.Nome)
	}
	if claims.Issuer != "guardpoint-server" {
		t.Errorf("Issuer = %q", claims.Issuer)
	}
}

func TestValidateTokenExpirado(t *testing.T) {
	svc := NewJWTService(testSecret)

	claims := Claims{
		UserID: uuid.New().String(),
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-1 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now().Add(-2 * time.Hour)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(testSecret))
	if err != nil {
		t.Fatalf("assinar token: %v", err)
	}

	_, err = svc.ValidateToken(signed)
	if !errors.Is(err, ErrTokenExpired) {
		t.Errorf("ValidateToken(token expirado) = %v, esperado ErrTokenExpired", err)
	}
}

func TestValidateTokenAssinaturaInvalida(t *testing.T) {
	outroSvc := NewJWTService("outro-segredo-completamente-diferente")
	token, err := outroSvc.GenerateAccessToken(uuid.New(), uuid.New(), "a@b.c", "vigia", "X")
	if err != nil {
		t.Fatalf("gerar token: %v", err)
	}

	svc := NewJWTService(testSecret)
	if _, err := svc.ValidateToken(token); err == nil {
		t.Error("ValidateToken() aceitou token assinado com outro segredo")
	}
}

func TestValidateTokenMetodoAssinaturaTrocado(t *testing.T) {
	svc := NewJWTService(testSecret)

	// alg "none": token sem assinatura deve ser rejeitado.
	noneToken := jwt.NewWithClaims(jwt.SigningMethodNone, Claims{UserID: "x"})
	signed, err := noneToken.SignedString(jwt.UnsafeAllowNoneSignatureType)
	if err != nil {
		t.Fatalf("assinar token none: %v", err)
	}
	if _, err := svc.ValidateToken(signed); err == nil {
		t.Error("ValidateToken() aceitou token com alg=none")
	}

	// Header forjado declarando RS256 mas com assinatura HMAC nao deve passar.
	forged := jwt.NewWithClaims(jwt.SigningMethodHS256, Claims{UserID: "x"})
	forged.Header["alg"] = "RS256"
	signedForged, err := forged.SignedString([]byte(testSecret))
	if err != nil {
		t.Fatalf("assinar token forjado: %v", err)
	}
	if _, err := svc.ValidateToken(signedForged); err == nil {
		t.Error("ValidateToken() aceitou token com alg forjado (RS256)")
	}
}

func TestRefreshTokenRoundTrip(t *testing.T) {
	svc := NewJWTService(testSecret)
	userID := uuid.New()

	refresh, err := svc.GenerateRefreshToken(userID)
	if err != nil {
		t.Fatalf("GenerateRefreshToken() erro: %v", err)
	}

	sub, err := svc.ValidateRefreshToken(refresh)
	if err != nil {
		t.Fatalf("ValidateRefreshToken() erro: %v", err)
	}
	if sub != userID.String() {
		t.Errorf("subject = %q, esperado %q", sub, userID)
	}
}
