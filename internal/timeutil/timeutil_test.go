package timeutil

import (
	"testing"
	"time"
)

func TestBRTIsNotNil(t *testing.T) {
	if BRT == nil {
		t.Fatal("BRT location é nil")
	}
}

func TestNowBRT(t *testing.T) {
	now := NowBRT()
	if now.Location().String() != "America/Sao_Paulo" && now.Location().String() != "BRT" {
		t.Errorf("NowBRT() retornou timezone %q, esperado BRT", now.Location().String())
	}
	if now.IsZero() {
		t.Error("NowBRT() retornou time zero")
	}
}

func TestNowBRT_IsRecent(t *testing.T) {
	now := NowBRT()
	utc := time.Now().UTC()
	diff := now.UTC().Sub(utc)
	if diff < 0 {
		diff = -diff
	}
	if diff > 5*time.Second {
		t.Errorf("NowBRT() difere %v do relógio UTC, esperado < 5s", diff)
	}
}
