package pbsbackup

import (
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"pbs-win-backup/internal/backup/exclude"
	"pbs-win-backup/internal/config"
	"pbscommon"
)

func TestWriteDirSkipsDefaultExcludedFolders(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "ok.txt"), []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}
	svi := filepath.Join(dir, "System Volume Information")
	if err := os.Mkdir(svi, 0o000); err != nil {
		t.Fatal(err)
	}

	var total, skipped atomic.Int64
	exc := exclude.NewForRoot(dir, config.DefaultExclusions())
	archive := &pbscommon.PXARArchive{
		ArchiveName:        "backup.pxar.didx",
		FilesTotal:         &total,
		FilesSkipped:       &skipped,
		SkipUnreadableDirs: true,
		WriteCB:            func([]byte) {},
		CatalogWriteCB:     func([]byte) {},
		ShouldSkip: func(fullPath, name string, isDir bool) bool {
			return exc.MatchPath(fullPath, name, isDir)
		},
	}
	if _, err := archive.WriteDir(dir, "", true); err != nil {
		t.Fatalf("WriteDir: %v", err)
	}
	if total.Load() < 1 {
		t.Fatalf("expected readable file in archive, files=%d", total.Load())
	}
	if skipped.Load() < 1 {
		t.Fatalf("expected excluded SVI to be skipped, skipped=%d", skipped.Load())
	}
}
