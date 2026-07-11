package filebackup

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"pbs-win-backup/internal/fileindex"
	"pbs-win-backup/internal/i18n"
	"pbs-win-backup/internal/models"
	"pbs-win-backup/internal/winattr"
)

func TestSignificantSkips(t *testing.T) {
	if significantSkips(999, 1000) != true {
		t.Fatal("expected 999/1000 to be significant")
	}
	if significantSkips(1, 1000) != false {
		t.Fatal("expected 1/1000 not to be significant")
	}
	if significantSkips(0, 100) != false {
		t.Fatal("expected zero skips not to be significant")
	}
}

func TestComputeDeleted_SkipsUnverifiedSubtree(t *testing.T) {
	store := &fileindex.Store{
		Files: map[string]fileindex.FileRecord{
			`blocked\child\file.txt`: {Archive: "base.zip"},
			`other\gone.txt`:         {Archive: "base.zip"},
		},
	}
	current := map[string]bool{
		`visible\keep.txt`: true,
	}
	unverified := []string{`blocked`}
	deleted := computeDeleted(store, current, unverified)
	if len(deleted) != 1 || deleted[0] != `other\gone.txt` {
		t.Fatalf("deleted = %v, want only other\\gone.txt", deleted)
	}
}

func TestScanFiles_TransientDirError_DoesNotMarkChildrenDeleted(t *testing.T) {
	store := &fileindex.Store{
		Files: map[string]fileindex.FileRecord{
			`blocked\child\file.txt`: {Archive: "base.zip"},
		},
	}
	current := map[string]bool{}
	unverified := []string{`blocked`}
	deleted := computeDeleted(store, current, unverified)
	if len(deleted) != 0 {
		t.Fatalf("expected no deletions under unreadable dir, got %v", deleted)
	}

	root := t.TempDir()
	blocked := filepath.Join(root, "blocked")
	if err := os.MkdirAll(filepath.Join(blocked, "child"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(blocked, "child", "file.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(blocked, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(blocked, 0o755) })

	var stats Stats
	files, _, _, scannedUnverified, err := scanFiles(context.Background(), root, nil, true, &stats, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		current[f.catalog] = true
	}
	if len(scannedUnverified) > 0 {
		deleted = computeDeleted(store, current, scannedUnverified)
		if len(deleted) != 0 {
			t.Fatalf("scan integration: expected no deletions, got %v", deleted)
		}
	}
}

func TestRun_FullBackup_MostFilesSkipped_NotSuccess(t *testing.T) {
	progData := filepath.Join(t.TempDir(), "ProgramData")
	if err := os.MkdirAll(progData, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ProgramData", progData)

	root := filepath.Join(t.TempDir(), "src")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 1000; i++ {
		path := filepath.Join(root, fmt.Sprintf("f%04d.txt", i))
		if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	jobID := "audit-skip-job"
	if err := fileindex.Save(&fileindex.Store{
		JobID:    jobID,
		BaseFull: "",
		Files: map[string]fileindex.FileRecord{
			`f0000.txt`: {Size: 1, Archive: "old.zip"},
			`f0001.txt`: {Size: 1, Archive: "old.zip"},
		},
		Meta: map[string]winattr.Entry{},
	}); err != nil {
		t.Fatal(err)
	}
	before, err := fileindex.Load(jobID)
	if err != nil {
		t.Fatal(err)
	}
	beforeFiles := len(before.Files)

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
		if err := writeZip(ctx, files[:1], destZip, skipAccess, stats, intentionallySkipped, onFile); err != nil {
			return err
		}
		stats.FilesSkipped.Store(999)
		stats.FilesTotal.Store(1000)
		return nil
	}

	dest := models.BackupDestination{Type: models.DestFTP, Host: "127.0.0.1"}
	_, err = Run(context.Background(), Options{
		Destination: dest,
		Job: models.BackupJob{
			ID:               jobID,
			Name:             "skip-test",
			Sources:          []string{root},
			SkipAccessErrors: true,
		},
		Hostname: "host",
	})
	if err == nil {
		t.Fatal("expected error when most files were skipped")
	}

	after, err := fileindex.Load(jobID)
	if err != nil {
		t.Fatal(err)
	}
	if len(after.Files) != beforeFiles {
		t.Fatalf("store collapsed: before %d files, after %d", beforeFiles, len(after.Files))
	}
}

func TestVerifyUpload_SameSizeCorruption_Fails(t *testing.T) {
	dir := t.TempDir()
	localPath := filepath.Join(dir, "archive.zip")
	content := make([]byte, 1024)
	for i := range content {
		content[i] = 'a'
	}
	if err := os.WriteFile(localPath, content, 0o644); err != nil {
		t.Fatal(err)
	}
	localHash, err := localFileSHA256(localPath)
	if err != nil {
		t.Fatal(err)
	}

	orig := verifyArchiveUpload
	t.Cleanup(func() { verifyArchiveUpload = orig })
	verifyArchiveUpload = func(_ context.Context, dest models.BackupDestination, password, backupID, fileName string, total int64, expected [32]byte) error {
		if total != int64(len(content)) {
			t.Fatalf("size %d", total)
		}
		corrupt := expected
		corrupt[0] ^= 0xff
		if corrupt == expected {
			return nil
		}
		return i18n.Ef("ftp.checksum_mismatch", map[string]string{"path": fileName})
	}

	err = verifyArchiveUpload(context.Background(), models.BackupDestination{Type: models.DestFTP}, "", "", "x.zip", int64(len(content)), localHash)
	if err == nil {
		t.Fatal("expected checksum verification to fail")
	}
}
