//go:build windows

package datalock

import (
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"testing"
	"time"
)

func TestClearStale_ConcurrentAcquire_DoesNotRemoveLiveLock(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ProgramData", dir)
	progData := filepath.Join(dir, "ClickRAX")
	if err := os.MkdirAll(progData, 0o755); err != nil {
		t.Fatal(err)
	}

	path, err := lockPath("concurrent-live")
	if err != nil {
		t.Fatal(err)
	}

	held := make(chan struct{})
	release := make(chan struct{})
	go func() {
		_ = With("concurrent-live", func() error {
			close(held)
			<-release
			return nil
		})
	}()
	<-held

	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = clearStale(path)
		}()
	}
	wg.Wait()

	if _, err := os.Stat(path); err != nil {
		t.Fatal("live lock file should remain while holder is active")
	}

	release <- struct{}{}
	time.Sleep(50 * time.Millisecond)
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("lock file should be removed after holder releases")
	}
}

func TestIsProcessAlive_AccessDenied_ReturnsTrue(t *testing.T) {
	old := openProcessHandle
	openProcessHandle = func(pid int) (uintptr, syscall.Errno) {
		return 0, syscall.ERROR_ACCESS_DENIED
	}
	defer func() { openProcessHandle = old }()

	if !isProcessAlive(12345) {
		t.Fatal("ACCESS_DENIED should be treated as alive process")
	}
}
