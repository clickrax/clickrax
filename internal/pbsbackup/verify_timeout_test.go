package pbsbackup

import (
	"context"
	"errors"
	"testing"
)

func TestVerifyJobTimeoutScalesWithSize(t *testing.T) {
	small := verifyJobTimeout(0)
	large := verifyJobTimeout(3 * 1024 * 1024 * 1024 * 1024)
	if large <= small {
		t.Fatalf("expected larger timeout for 3 TiB, small=%s large=%s", small, large)
	}
	if large > verifyMaxTimeout {
		t.Fatalf("timeout capped at %s, got %s", verifyMaxTimeout, large)
	}
}

func TestVerifyTimeout(t *testing.T) {
	if !VerifyTimeout(context.DeadlineExceeded) {
		t.Fatal("expected deadline exceeded")
	}
	if VerifyTimeout(errors.New("verify PBS: context deadline exceeded")) != true {
		t.Fatal("expected wrapped deadline message")
	}
	if VerifyTimeout(errors.New("checksum mismatch")) {
		t.Fatal("unexpected timeout classification")
	}
}
