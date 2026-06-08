package librespot

import (
	"context"
	"testing"
	"time"
)

func TestCapDur(t *testing.T) {
	if got := capDur(40*time.Second, 30*time.Second); got != 30*time.Second {
		t.Errorf("capDur over max = %v, want 30s", got)
	}
	if got := capDur(2*time.Second, 30*time.Second); got != 2*time.Second {
		t.Errorf("capDur under max = %v, want 2s", got)
	}
}

func TestSleepCtxCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if sleepCtx(ctx, time.Hour) {
		t.Error("sleepCtx on a cancelled context should return false immediately")
	}
}

func TestSleepCtxElapses(t *testing.T) {
	if !sleepCtx(context.Background(), time.Millisecond) {
		t.Error("sleepCtx should return true after the duration elapses")
	}
}
