package pbsbackup

import (
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"pbscommon"
)

func TestWriteDirSkipsUnreadableFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "readable.txt"), []byte("ok"), 0o644); err != nil {
		t.Fatal(err)
	}
	locked := filepath.Join(dir, "locked.dat")
	lf, err := os.OpenFile(locked, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	defer lf.Close()

	var total, skipped atomic.Int64
	archive := &pbscommon.PXARArchive{
		ArchiveName:  "backup.pxar.didx",
		FilesTotal:   &total,
		FilesSkipped: &skipped,
		WriteCB:      func([]byte) {},
		CatalogWriteCB: func([]byte) {},
	}
	if _, err := archive.WriteDir(dir, "", true); err != nil {
		t.Fatal(err)
	}
	if total.Load() < 1 {
		t.Fatalf("expected at least 1 readable file, got total=%d", total.Load())
	}
	if skipped.Load() != 0 {
		t.Fatalf("locked file may still be readable on this platform, skipped=%d", skipped.Load())
	}
}

func TestWriteFileMissingPathReturnsError(t *testing.T) {
	var skipped atomic.Int64
	archive := &pbscommon.PXARArchive{FilesSkipped: &skipped}
	_, err := archive.WriteFile(filepath.Join(t.TempDir(), "missing.bin"), "missing.bin")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}
