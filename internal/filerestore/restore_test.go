package filerestore

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

type blockedReader struct {
	ctx context.Context
}

func (b *blockedReader) Read(p []byte) (int, error) {
	select {
	case <-b.ctx.Done():
		return 0, b.ctx.Err()
	case <-time.After(time.Hour):
		return 0, errors.New("unexpected wait")
	}
}

func TestRestore_StalledRead_Cancellable(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	destPath := filepath.Join(t.TempDir(), "restored.txt")
	reader := &blockedReader{ctx: ctx}

	start := time.Now()
	err := writeRestoredFromReader(ctx, destPath, reader)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected stalled restore to fail")
	}
	if elapsed > 2*time.Second {
		t.Fatalf("restore blocked too long: %s", elapsed)
	}
	if !errors.Is(err, context.DeadlineExceeded) && !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context cancellation, got %v", err)
	}
	if _, err := os.Stat(destPath + ".restoring"); !os.IsNotExist(err) {
		t.Fatal("partial restore file should be removed on failure")
	}
}
