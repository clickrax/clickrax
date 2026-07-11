//go:build windows

package backupqueue

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLegacyQueueLockRemoved(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ProgramData", dir)
	progData := filepath.Join(dir, "ClickRAX")
	if err := os.MkdirAll(progData, 0o755); err != nil {
		t.Fatal(err)
	}
	lockPath := filepath.Join(progData, "backup_queue.lock")
	if err := os.WriteFile(lockPath, []byte("1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !isLegacyQueueLock(lockPath) {
		t.Fatal("expected legacy lock removal")
	}
	if _, err := os.Stat(lockPath); !os.IsNotExist(err) {
		t.Fatal("legacy lock file should be removed")
	}
}

func TestEnqueueAfterLegacyLock(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ProgramData", dir)
	progData := filepath.Join(dir, "ClickRAX")
	if err := os.MkdirAll(progData, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(progData, "backup_queue.lock"), []byte("1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Enqueue(Item{JobID: "job-1", Trigger: "manual"}); err != nil {
		t.Fatalf("enqueue after legacy lock: %v", err)
	}
}
