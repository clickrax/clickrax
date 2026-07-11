package backupqueue

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestMain(m *testing.M) {
	oldRecent := inflightRecentlyActive
	inflightRecentlyActive = func(string) bool { return false }
	code := m.Run()
	inflightRecentlyActive = oldRecent
	os.Exit(code)
}

func TestDrain_PermanentStartError_DeadLetters(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ProgramData", dir)
	progData := filepath.Join(dir, "ClickRAX")
	if err := os.MkdirAll(progData, 0o755); err != nil {
		t.Fatal(err)
	}

	item := Item{
		JobID:      "perm-job",
		Trigger:    "manual",
		JobName:    "Broken",
		EnqueuedAt: time.Now(),
	}
	if _, err := Enqueue(item); err != nil {
		t.Fatal(err)
	}

	Drain(
		func() bool { return false },
		func() bool { return true },
		func(Item) error { return PermanentStartError(os.ErrNotExist) },
	)

	list, err := List()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 0 {
		t.Fatalf("queue should be empty, got %d", len(list))
	}
	dead, err := loadDeadLetters()
	if err != nil {
		t.Fatal(err)
	}
	if len(dead) != 1 || dead[0].Item.JobID != item.JobID {
		t.Fatalf("dead-letter = %+v", dead)
	}
}

func TestReconcileInflight_RestoresPoppedItem(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ProgramData", dir)
	progData := filepath.Join(dir, "ClickRAX")
	if err := os.MkdirAll(progData, 0o755); err != nil {
		t.Fatal(err)
	}

	item := Item{JobID: "crash-job", Trigger: "manual", JobName: "Crash", EnqueuedAt: time.Now()}
	if err := markInflight(item); err != nil {
		t.Fatal(err)
	}
	if err := ReconcileInflight(); err != nil {
		t.Fatal(err)
	}
	list, err := List()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].JobID != item.JobID {
		t.Fatalf("reconciled queue = %+v", list)
	}
}

func TestClaimNext_WritesInflightBeforePop(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ProgramData", dir)
	progData := filepath.Join(dir, "ClickRAX")
	if err := os.MkdirAll(progData, 0o755); err != nil {
		t.Fatal(err)
	}

	item := Item{JobID: "claim-job", Trigger: "manual", JobName: "Claim", EnqueuedAt: time.Now()}
	if _, err := Enqueue(item); err != nil {
		t.Fatal(err)
	}

	claimed, ok, err := ClaimNext()
	if err != nil || !ok {
		t.Fatalf("ClaimNext = %v ok=%v err=%v", claimed, ok, err)
	}
	if claimed.JobID != item.JobID {
		t.Fatalf("claimed = %+v", claimed)
	}
	list, err := List()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 0 {
		t.Fatalf("queue should be empty after claim, got %d", len(list))
	}
	path, err := inflightPath()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("inflight marker missing after claim: %v", err)
	}
}

func TestClaimNext_ReconcilesOrphanInflightBeforeOverwrite(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ProgramData", dir)
	progData := filepath.Join(dir, "ClickRAX")
	if err := os.MkdirAll(progData, 0o755); err != nil {
		t.Fatal(err)
	}

	orphan := Item{JobID: "orphan-job", Trigger: "manual", JobName: "Orphan", EnqueuedAt: time.Now()}
	if err := markInflight(orphan); err != nil {
		t.Fatal(err)
	}
	next := Item{JobID: "next-job", Trigger: "manual", JobName: "Next", EnqueuedAt: time.Now().Add(time.Second)}
	if _, err := Enqueue(next); err != nil {
		t.Fatal(err)
	}

	claimed, ok, err := ClaimNext()
	if err != nil || !ok {
		t.Fatalf("ClaimNext = %v ok=%v err=%v", claimed, ok, err)
	}
	if claimed.JobID != orphan.JobID {
		t.Fatalf("orphan should be reclaimed first, got %q", claimed.JobID)
	}
	list, err := List()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].JobID != next.JobID {
		t.Fatalf("queue after orphan reclaim = %+v", list)
	}
}

func TestReconcileInflight_SkipsWhenBackupLockHeld(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ProgramData", dir)
	progData := filepath.Join(dir, "ClickRAX")
	if err := os.MkdirAll(progData, 0o755); err != nil {
		t.Fatal(err)
	}

	item := Item{JobID: "active-job", Trigger: "manual", JobName: "Active", EnqueuedAt: time.Now()}
	if err := markInflight(item); err != nil {
		t.Fatal(err)
	}

	oldHeld := backupLockHeld
	t.Cleanup(func() { backupLockHeld = oldHeld })
	backupLockHeld = func() bool { return true }

	if err := ReconcileInflight(); err != nil {
		t.Fatal(err)
	}
	list, err := List()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 0 {
		t.Fatalf("queue should stay empty while backup lock held, got %d", len(list))
	}
	path, err := inflightPath()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("inflight marker should remain while backup lock held: %v", err)
	}
}

func TestReconcileInflight_CorruptInflight_DeadLetters(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ProgramData", dir)
	progData := filepath.Join(dir, "ClickRAX")
	if err := os.MkdirAll(progData, 0o755); err != nil {
		t.Fatal(err)
	}

	path, err := inflightPath()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("{not-json"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := ReconcileInflight(); err == nil {
		t.Fatal("expected corrupt inflight error")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("corrupt inflight should be removed, stat err=%v", err)
	}
	dead, err := loadDeadLetters()
	if err != nil {
		t.Fatal(err)
	}
	if len(dead) != 1 {
		t.Fatalf("expected dead-letter for corrupt inflight, got %d", len(dead))
	}
}

func TestReconcileInflight_SkipsWhenAlreadyQueued(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ProgramData", dir)
	progData := filepath.Join(dir, "ClickRAX")
	if err := os.MkdirAll(progData, 0o755); err != nil {
		t.Fatal(err)
	}

	item := Item{JobID: "dup-job", Trigger: "manual", JobName: "Dup", EnqueuedAt: time.Now()}
	if _, err := Enqueue(item); err != nil {
		t.Fatal(err)
	}
	if err := markInflight(item); err != nil {
		t.Fatal(err)
	}
	if err := ReconcileInflight(); err != nil {
		t.Fatal(err)
	}
	list, err := List()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 {
		t.Fatalf("expected single queue item, got %d", len(list))
	}
	path, err := inflightPath()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("inflight marker should be cleared, stat err=%v", err)
	}
}
