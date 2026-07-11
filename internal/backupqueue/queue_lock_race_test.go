//go:build windows

package backupqueue

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestWithQueueLock_Concurrent(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ProgramData", dir)
	progData := filepath.Join(dir, "ClickRAX")
	if err := os.MkdirAll(progData, 0o755); err != nil {
		t.Fatal(err)
	}

	const workers = 32
	var wg sync.WaitGroup
	errCh := make(chan error, workers)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := withQueueLock(func(items *[]Item) error {
				time.Sleep(2 * time.Millisecond)
				return nil
			})
			if err != nil {
				errCh <- err
			}
		}()
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Fatalf("concurrent withQueueLock: %v", err)
	}

	lockPath := filepath.Join(progData, "backup_queue.lock")
	if _, err := os.Stat(lockPath); !os.IsNotExist(err) {
		t.Fatal("queue lock should be released after concurrent operations")
	}
}
