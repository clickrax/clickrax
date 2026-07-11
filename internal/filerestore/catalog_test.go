package filerestore

import (
	"testing"
	"time"

	"pbs-win-backup/internal/models"
)

func TestParseArchiveTime(t *testing.T) {
	tm := parseArchiveTime("myhost_20260707-120405.zip")
	if tm.IsZero() {
		t.Fatal("expected parsed time")
	}
	want := time.Date(2026, 7, 7, 12, 4, 5, 0, time.UTC)
	if !tm.Equal(want) {
		t.Fatalf("got %v want %v", tm, want)
	}
}

func TestResolveArchiveByTime(t *testing.T) {
	archives := []archiveRef{
		{FileName: "a_20260101-100000.zip", Time: time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)},
		{FileName: "a_20260102-100000.zip", Time: time.Date(2026, 1, 2, 10, 0, 0, 0, time.UTC)},
	}
	got, err := resolveArchive(archives, "2026-01-02T10:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	if got.FileName != "a_20260102-100000.zip" {
		t.Fatalf("got %q", got.FileName)
	}
}

func TestSafeZipEntryName(t *testing.T) {
	if safeZipEntryName(`docs\file.txt`) != true {
		t.Fatal("expected safe path")
	}
	if safeZipEntryName(`../escape.txt`) != false {
		t.Fatal("expected zip-slip rejected")
	}
	if safeZipEntryName(`/etc/passwd`) != false {
		t.Fatal("expected absolute rejected")
	}
}

func TestPathsMatch(t *testing.T) {
	if !pathsMatch(`folder\file.txt`, "folder/file.txt") {
		t.Fatal("paths should match")
	}
}

func TestIsFolderSelection(t *testing.T) {
	catalog := []models.SnapshotFile{
		{Path: `docs\a.txt`, IsDir: false},
		{Path: `docs\b.txt`, IsDir: false},
	}
	if !isFolderSelection(`docs`, catalog) {
		t.Fatal("docs should be folder selection")
	}
	if isFolderSelection(`docs\a.txt`, catalog) {
		t.Fatal("single file should not be folder selection")
	}
}
