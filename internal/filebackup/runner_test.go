package filebackup

import (
	"testing"
	"time"

	"pbs-win-backup/internal/fileindex"
)

func TestBuildManifestIncrementalChain(t *testing.T) {
	store := &fileindex.Store{
		BaseFull: "host_20260101-100000.zip",
		Archives: []fileindex.ArchiveRecord{
			{Name: "host_20260101-100000.zip", Kind: fileindex.KindFull},
			{Name: "host_20260102-100000_incr.zip", Kind: fileindex.KindIncremental},
		},
	}
	stamp := time.Date(2026, 1, 3, 10, 0, 0, 0, time.UTC)
	m := buildManifest(fileindex.KindIncremental, "host_20260103-100000_incr.zip", stamp, store, []string{`docs\gone.txt`})
	if m.BaseFull != store.BaseFull {
		t.Fatalf("base %q", m.BaseFull)
	}
	if len(m.Chain) != 3 {
		t.Fatalf("chain len %d", len(m.Chain))
	}
	if m.Chain[2] != "host_20260103-100000_incr.zip" {
		t.Fatalf("chain %v", m.Chain)
	}
	if len(m.Deleted) != 1 {
		t.Fatalf("deleted %v", m.Deleted)
	}
}

func TestLastArchiveName(t *testing.T) {
	store := &fileindex.Store{
		BaseFull: "full.zip",
		Archives: []fileindex.ArchiveRecord{{Name: "incr.zip"}},
	}
	if got := lastArchiveName(store); got != "incr.zip" {
		t.Fatalf("got %q", got)
	}
}
