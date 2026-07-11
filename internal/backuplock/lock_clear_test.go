package backuplock

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestTryRemoveStaleLockMissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "backup.lock")
	if tryRemoveStaleLock(path) {
		t.Fatal("missing lock file should not be reported as removed")
	}
}

func TestClearStaleSemanticsNoFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "backup.lock")
	if !clearStaleAt(path) {
		t.Fatal("no lock file should mean backup slot is available")
	}
}

func TestClearStaleSemanticsLiveLock(t *testing.T) {
	path := filepath.Join(t.TempDir(), "backup.lock")
	if err := os.WriteFile(path, []byte(fmt.Sprintf("%d %d\n", os.Getpid(), time.Now().Unix())), 0o644); err != nil {
		t.Fatal(err)
	}
	if clearStaleAt(path) {
		t.Fatal("live PID lock should block backup")
	}
}

func TestClearStaleSemanticsDeadLock(t *testing.T) {
	path := filepath.Join(t.TempDir(), "backup.lock")
	if err := os.WriteFile(path, []byte("999999 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !clearStaleAt(path) {
		t.Fatal("dead PID lock should be cleared")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("stale lock file should be removed")
	}
}

func TestClearStaleSemanticsExpiredLock(t *testing.T) {
	path := filepath.Join(t.TempDir(), "backup.lock")
	old := time.Now().Add(-48 * time.Hour).Unix()
	if err := os.WriteFile(path, []byte(fmt.Sprintf("%d %d\n", os.Getpid(), old)), 0o644); err != nil {
		t.Fatal(err)
	}
	if !clearStaleAt(path) {
		t.Fatal("age-expired lock should be cleared even when PID appears alive")
	}
}

// clearStaleAt mirrors ClearStale for an explicit path (test helper).
func clearStaleAt(path string) bool {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return true
	}
	return tryRemoveStaleLock(path)
}
