package pbsbackup

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"pbscommon"
)

func TestWriteDirAbortsBeforeRead(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	archive := &pbscommon.PXARArchive{
		ArchiveName:    "backup.pxar.didx",
		WriteCB:        func([]byte) {},
		CatalogWriteCB: func([]byte) {},
		Abort: func() error {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
				return nil
			}
		},
	}
	_, err := archive.WriteDir(dir, "", true)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected cancel, got %v", err)
	}
}

func TestWriteFileAbortsDuringRead(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "big.bin")
	payload := make([]byte, 2<<20)
	if err := os.WriteFile(path, payload, 0o644); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	reads := 0
	archive := &pbscommon.PXARArchive{
		WriteCB: func([]byte) {
			reads++
			if reads == 3 {
				cancel()
			}
		},
		CatalogWriteCB: func([]byte) {},
		Abort: func() error {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
				return nil
			}
		},
	}
	_, err := archive.WriteFile(path, "big.bin")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected cancel during read, got %v (reads=%d)", err, reads)
	}
}
