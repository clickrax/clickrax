package status

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"pbs-win-backup/internal/models"
	"pbs-win-backup/internal/paths"
)

func TestWriteLastStatusWarningUpdatesLastSuccess(t *testing.T) {
	dir := t.TempDir()
	oldData := os.Getenv("ProgramData")
	t.Setenv("ProgramData", dir)
	defer func() { _ = os.Setenv("ProgramData", oldData) }()

	okAt := time.Date(2026, 7, 5, 12, 0, 0, 0, time.UTC)
	if err := WriteLastStatus(models.LastStatus{
		Hostname:    "host",
		JobName:     "job1",
		LastRun:     okAt.Format(time.RFC3339),
		LastSuccess: okAt.Format(time.RFC3339),
		Status:      "ok",
	}); err != nil {
		t.Fatal(err)
	}

	warnAt := time.Date(2026, 7, 7, 12, 0, 0, 0, time.UTC)
	if err := WriteLastStatus(FromJobResult(models.JobRunResult{
		JobName:    "job1",
		Status:     "warning",
		StartedAt:  warnAt,
		FinishedAt: warnAt,
	}, "host")); err != nil {
		t.Fatal(err)
	}

	got, err := ReadLastStatus()
	if err != nil {
		t.Fatal(err)
	}
	if got.LastSuccess != warnAt.Format(time.RFC3339) {
		t.Fatalf("warning should update last_success: got %q want %q", got.LastSuccess, warnAt.Format(time.RFC3339))
	}
}

func TestWriteLastStatusPreservesLastSuccess(t *testing.T) {
	dir := t.TempDir()
	oldData := os.Getenv("ProgramData")
	t.Setenv("ProgramData", dir)
	defer func() { _ = os.Setenv("ProgramData", oldData) }()

	okAt := time.Date(2026, 7, 5, 12, 0, 0, 0, time.UTC)
	if err := WriteLastStatus(models.LastStatus{
		Hostname:    "host",
		JobName:     "job1",
		LastRun:     okAt.Format(time.RFC3339),
		LastSuccess: okAt.Format(time.RFC3339),
		Status:      "ok",
	}); err != nil {
		t.Fatal(err)
	}

	failAt := time.Date(2026, 7, 6, 12, 0, 0, 0, time.UTC)
	if err := WriteLastStatus(FromJobResult(models.JobRunResult{
		JobName:    "job1",
		Status:     "error",
		StartedAt:  failAt,
		FinishedAt: failAt,
		Error:      "boom",
	}, "host")); err != nil {
		t.Fatal(err)
	}

	p, err := paths.LastStatusPath()
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Clean(p))
	if err != nil {
		t.Fatal(err)
	}
	if !contains(string(data), okAt.Format(time.RFC3339)) {
		t.Fatalf("last_success lost after failure: %s", data)
	}
}

func contains(s, sub string) bool {
	return len(sub) == 0 || (len(s) >= len(sub) && (s == sub || len(s) > 0 && stringIndex(s, sub) >= 0))
}

func stringIndex(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
