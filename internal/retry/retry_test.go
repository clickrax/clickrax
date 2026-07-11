package retry

import (
	"errors"
	"testing"
	"time"
)

func TestBackoffCapsAttemptOverflow(t *testing.T) {
	d := Backoff(100, time.Second)
	if d != 2*time.Minute {
		t.Fatalf("expected cap 2m, got %v", d)
	}
}

func TestAuthErrorNotRetryable(t *testing.T) {
	err := errors.New(`загрузка предыдущего индекса: Get "https://pbs:8007/previous": Authentication error`)
	if IsRetryable(err) {
		t.Fatal("authentication errors must not be retried")
	}
}

func TestConnectionResetRetryable(t *testing.T) {
	err := errors.New("connection reset by peer")
	if !IsRetryable(err) {
		t.Fatal("connection reset should be retryable")
	}
}
