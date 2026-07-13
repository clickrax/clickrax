package filebackup

import (
	"archive/zip"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"golang.org/x/sys/windows"

	"pbs-win-backup/internal/fileindex"
	"pbs-win-backup/internal/models"
	"pbs-win-backup/internal/winattr"
)

func TestRun_SkipAccessErrors_SingleLockedFile_Succeeds(t *testing.T) {
	progData := filepath.Join(t.TempDir(), "ProgramData")
	if err := os.MkdirAll(progData, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ProgramData", progData)

	root := filepath.Join(t.TempDir(), "src")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 10; i++ {
		path := filepath.Join(root, fmt.Sprintf("ok%02d.txt", i))
		if err := os.WriteFile(path, []byte("ok"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "locked.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	jobID := "skip-one-job"
	if err := fileindex.Save(&fileindex.Store{
		JobID:    jobID,
		BaseFull: "",
		Files:    map[string]fileindex.FileRecord{},
		Meta:     map[string]winattr.Entry{},
	}); err != nil {
		t.Fatal(err)
	}

	origUpload := runUploadArchive
	origExists := runRemoteArchiveExists
	origWriteZip := runWriteZip
	t.Cleanup(func() {
		runUploadArchive = origUpload
		runRemoteArchiveExists = origExists
		runWriteZip = origWriteZip
	})
	runUploadArchive = func(context.Context, models.BackupDestination, string, string, string, string, int64, func(int64, int64)) error {
		return nil
	}
	runRemoteArchiveExists = func(models.BackupDestination, string, string, string) bool {
		return true
	}
	runWriteZip = func(ctx context.Context, files []localFile, destZip string, skipAccess bool, stats *Stats, intentionallySkipped map[string]struct{}, onFile func(string)) error {
		if !skipAccess {
			t.Fatal("expected SkipAccessErrors")
		}
		intentionallySkipped[`locked.txt`] = struct{}{}
		stats.FilesSkipped.Store(1)
		writable := make([]localFile, 0, len(files)-1)
		for _, f := range files {
			if f.catalog == `locked.txt` {
				continue
			}
			writable = append(writable, f)
		}
		return writeZip(ctx, writable, destZip, false, stats, intentionallySkipped, onFile)
	}

	stats, err := Run(context.Background(), Options{
		Destination: models.BackupDestination{Type: models.DestFTP, Host: "127.0.0.1"},
		Job: models.BackupJob{
			ID:               jobID,
			Name:             "skip-one",
			Sources:          []string{root},
			SkipAccessErrors: true,
		},
		Hostname: "host",
	})
	if err != nil {
		t.Fatalf("expected success with one skipped file: %v", err)
	}
	if stats.FilesSkipped.Load() < 1 {
		t.Fatalf("FilesSkipped = %d, want >= 1", stats.FilesSkipped.Load())
	}

	after, err := fileindex.Load(jobID)
	if err != nil {
		t.Fatal(err)
	}
	if after.BaseFull == "" {
		t.Fatal("index should advance after successful skip")
	}
}

func TestWriteZip_StatOkOpenFails_NoPhantomZipEntry(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("requires Windows exclusive file lock")
	}
	dir := t.TempDir()
	goodPath := filepath.Join(dir, "good.txt")
	if err := os.WriteFile(goodPath, []byte("ok"), 0o644); err != nil {
		t.Fatal(err)
	}
	blockedPath := filepath.Join(dir, "blocked.txt")
	if err := os.WriteFile(blockedPath, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	unlock := lockPathExclusive(t, blockedPath)
	defer unlock()

	files := []localFile{
		{absPath: goodPath, rel: "good.txt", catalog: "good.txt"},
		{absPath: blockedPath, rel: "blocked.txt", catalog: "blocked.txt"},
	}
	destZip := filepath.Join(dir, "out.zip")
	var stats Stats
	skipped := map[string]struct{}{}

	if err := writeZip(context.Background(), files, destZip, true, &stats, skipped, nil); err != nil {
		t.Fatalf("writeZip skipAccess: %v", err)
	}
	if stats.FilesSkipped.Load() != 1 {
		t.Fatalf("FilesSkipped = %d, want 1", stats.FilesSkipped.Load())
	}

	zr, err := zip.OpenReader(destZip)
	if err != nil {
		t.Fatal(err)
	}
	defer zr.Close()
	if len(zr.File) != 1 {
		t.Fatalf("zip entries = %d, want 1 (no phantom blocked entry)", len(zr.File))
	}
	if zr.File[0].Name != "good.txt" {
		t.Fatalf("unexpected entry %q", zr.File[0].Name)
	}
	if _, ok := skipped["blocked.txt"]; !ok {
		t.Fatal("blocked file should be in intentionallySkipped")
	}
}

func lockPathExclusive(t *testing.T, path string) func() {
	t.Helper()
	path16, err := windows.UTF16PtrFromString(path)
	if err != nil {
		t.Fatal(err)
	}
	h, err := windows.CreateFile(path16, windows.GENERIC_READ|windows.GENERIC_WRITE, 0, nil, windows.OPEN_EXISTING, windows.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		t.Fatal(err)
	}
	return func() {
		_ = windows.CloseHandle(h)
	}
}

func TestCheckPlannedFilesArchived_IgnoresIntentionalSkips(t *testing.T) {
	planned := []localFile{{catalog: `a.txt`}, {catalog: `locked.txt`}}
	archived := []localFile{{catalog: `a.txt`}}
	skipped := map[string]struct{}{`locked.txt`: {}}
	if err := checkPlannedFilesArchived(planned, archived, skipped); err != nil {
		t.Fatalf("expected success: %v", err)
	}
}
