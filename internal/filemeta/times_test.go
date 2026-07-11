package filemeta

import (
	"testing"
	"time"

	"pbs-win-backup/internal/winattr"
)

func TestMergeModifiedFallback(t *testing.T) {
	e := winattr.Entry{}
	when := time.Date(2024, 3, 15, 10, 30, 0, 0, time.UTC)
	MergeModifiedFallback(&e, when.Format(time.RFC3339))
	if e.MtimeNS != when.UnixNano() {
		t.Fatalf("mtime_ns = %d, want %d", e.MtimeNS, when.UnixNano())
	}

	e2 := winattr.Entry{MtimeNS: 123}
	MergeModifiedFallback(&e2, when.Format(time.RFC3339))
	if e2.MtimeNS != 123 {
		t.Fatal("existing mtime should not be overwritten")
	}
}

func TestPrepareEntryUsesCatalogFallback(t *testing.T) {
	when := time.Date(2025, 1, 2, 3, 4, 5, 0, time.UTC)
	a := NewArchive()
	e := PrepareEntry(a, `docs\a.txt`, when.Format(time.RFC3339))
	if e.MtimeNS != when.UnixNano() {
		t.Fatalf("expected catalog fallback mtime, got %d", e.MtimeNS)
	}
}
