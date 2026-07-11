package backupqueue

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestEnqueueDedupeScheduledSlot(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ProgramData", dir)
	progData := filepath.Join(dir, "ClickRAX")
	if err := os.MkdirAll(progData, 0o755); err != nil {
		t.Fatal(err)
	}

	item := Item{
		JobID:      "job-1",
		SlotKey:    "2026-07-07T02:00",
		Trigger:    "scheduled",
		JobName:    "Nightly",
		EnqueuedAt: time.Now(),
	}
	if _, err := Enqueue(item); err != nil {
		t.Fatal(err)
	}
	if _, err := Enqueue(item); err == nil {
		t.Fatal("expected duplicate enqueue error")
	}
	list, err := List()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 item, got %d", len(list))
	}
}

func TestPopNextFIFO(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ProgramData", dir)
	progData := filepath.Join(dir, "ClickRAX")
	if err := os.MkdirAll(progData, 0o755); err != nil {
		t.Fatal(err)
	}

	_, _ = Enqueue(Item{JobID: "a", Trigger: "manual", EnqueuedAt: time.Now()})
	_, _ = Enqueue(Item{JobID: "b", Trigger: "manual", EnqueuedAt: time.Now().Add(time.Second)})

	first, ok, err := PopNext()
	if err != nil || !ok || first.JobID != "a" {
		t.Fatalf("pop first: ok=%v id=%s err=%v", ok, first.JobID, err)
	}
	second, ok, err := PopNext()
	if err != nil || !ok || second.JobID != "b" {
		t.Fatalf("pop second: ok=%v id=%s err=%v", ok, second.JobID, err)
	}
}

func TestEnqueueFront(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ProgramData", dir)
	progData := filepath.Join(dir, "ClickRAX")
	if err := os.MkdirAll(progData, 0o755); err != nil {
		t.Fatal(err)
	}

	_, _ = Enqueue(Item{JobID: "b", Trigger: "manual"})
	item := Item{JobID: "a", Trigger: "manual"}
	if err := EnqueueFront(item); err != nil {
		t.Fatal(err)
	}
	got, ok, err := PopNext()
	if err != nil || !ok || got.JobID != "a" {
		t.Fatalf("expected a first, got %v ok=%v err=%v", got.JobID, ok, err)
	}
}
