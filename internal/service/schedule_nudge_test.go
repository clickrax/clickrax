package service

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"pbs-win-backup/internal/paths"
)

func TestConsumeScheduleNudge(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ProgramData", dir)
	dir, err := paths.DataDir()
	if err != nil {
		t.Skip(err)
	}
	p := filepath.Join(dir, scheduleNudgeFile)
	_ = os.Remove(p)
	if ConsumeScheduleNudge() {
		t.Fatal("expected no pending nudge")
	}
	if err := NudgeScheduler(); err != nil {
		t.Fatal(err)
	}
	if !ConsumeScheduleNudge() {
		t.Fatal("expected nudge to be consumed")
	}
	if ConsumeScheduleNudge() {
		t.Fatal("nudge should be cleared after consume")
	}
	_ = time.Now() // keep import if build tags change
}
