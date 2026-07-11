package backupqueue

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDrain_RequeueFailure_ItemNotLost(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ProgramData", dir)
	progData := filepath.Join(dir, "ClickRAX")
	if err := os.MkdirAll(progData, 0o755); err != nil {
		t.Fatal(err)
	}

	item := Item{
		JobID:      "lost-job",
		Trigger:    "manual",
		JobName:    "Manual backup",
		EnqueuedAt: time.Now(),
	}
	if _, err := Enqueue(item); err != nil {
		t.Fatal(err)
	}

	oldEnqueueFront := enqueueFrontHook
	t.Cleanup(func() { enqueueFrontHook = oldEnqueueFront })
	enqueueFrontHook = func(Item) error {
		return errors.New("disk full")
	}

	Drain(
		func() bool { return false },
		func() bool { return true },
		func(Item) error { return errors.New("start failed") },
	)

	list, err := List()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 0 {
		t.Fatalf("queue should be empty after durable pop, got %d items", len(list))
	}

	dead, err := loadDeadLetters()
	if err != nil {
		t.Fatal(err)
	}
	if len(dead) != 1 {
		t.Fatalf("expected 1 dead-letter record, got %d", len(dead))
	}
	if dead[0].Item.JobID != item.JobID {
		t.Fatalf("dead-letter job id = %q, want %q", dead[0].Item.JobID, item.JobID)
	}
}
