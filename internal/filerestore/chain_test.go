package filerestore

import (
	"strings"
	"testing"
	"time"

	"pbs-win-backup/internal/fileindex"
	"pbs-win-backup/internal/models"
)

func TestParseArchiveTimeIncremental(t *testing.T) {
	tm := parseArchiveTime("myhost_20260707-120405_incr.zip")
	if tm.IsZero() {
		t.Fatal("expected parsed incremental time")
	}
	want := time.Date(2026, 7, 7, 12, 4, 5, 0, time.UTC)
	if !tm.Equal(want) {
		t.Fatalf("got %v want %v", tm, want)
	}
}

func TestApplyIncrementalDeletes(t *testing.T) {
	view := &snapshotView{
		files: map[string]models.SnapshotFile{
			strings.ToLower(`docs\old.txt`):  {Path: `docs\old.txt`},
			strings.ToLower(`docs\keep.txt`): {Path: `docs\keep.txt`},
		},
		fileSource: map[string]string{
			strings.ToLower(`docs\old.txt`):  "base.zip",
			strings.ToLower(`docs\keep.txt`): "base.zip",
		},
	}
	m := fileindex.Manifest{
		Kind:    fileindex.KindIncremental,
		Deleted: []string{`docs\old.txt`},
	}
	for _, del := range m.Deleted {
		key := strings.ToLower(del)
		delete(view.files, key)
		delete(view.fileSource, key)
	}
	if len(view.files) != 1 {
		t.Fatalf("want 1 file, got %d", len(view.files))
	}
	if _, ok := view.files[strings.ToLower(`docs\keep.txt`)]; !ok {
		t.Fatal("keep.txt should remain")
	}
}
