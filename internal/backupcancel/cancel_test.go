package backupcancel

import (
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestRequestRoundTrip(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ProgramData", dir)

	jobID := uuid.NewString()
	if err := Request(jobID); err != nil {
		t.Fatal(err)
	}
	if !IsRequested(jobID) {
		t.Fatal("expected cancel request")
	}
	Clear(jobID)
	if IsRequested(jobID) {
		t.Fatal("expected request cleared")
	}
}

func TestRequestRejectsInvalidID(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ProgramData", dir)

	if err := Request("not-a-uuid"); err == nil {
		t.Fatal("expected error for invalid job id")
	}
	if err := Request(""); err == nil {
		t.Fatal("expected error for empty job id")
	}
}

func TestIsRequestedMissingDir(t *testing.T) {
	if IsRequested(uuid.NewString()) {
		t.Fatal("expected false for unknown job")
	}
}

func TestClearNoop(t *testing.T) {
	Clear(uuid.NewString())
}

func TestStaleSignalIgnored(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ProgramData", dir)

	jobID := uuid.NewString()
	if err := Request(jobID); err != nil {
		t.Fatal(err)
	}
	p, err := requestPath(jobID)
	if err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-maxSignalAge - time.Minute)
	if err := os.Chtimes(p, old, old); err != nil {
		t.Fatal(err)
	}
	if IsRequested(jobID) {
		t.Fatal("expected stale cancel signal to be ignored")
	}
}

func TestRequestPathUsesProgramData(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ProgramData", dir)
	jobID := uuid.NewString()
	if err := Request(jobID); err != nil {
		t.Fatal(err)
	}
	p, err := requestPath(jobID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(p); err != nil {
		t.Fatalf("file missing: %v", err)
	}
}
