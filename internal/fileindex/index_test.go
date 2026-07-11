package fileindex

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNeedsBackup(t *testing.T) {
	prev := FileRecord{Size: 100, Mtime: time.Now().UnixNano(), ACLHash: "aaa"}
	if !NeedsContentBackup(prev, 101, time.Now(), true) {
		t.Fatal("size change should need backup")
	}
	if NeedsContentBackup(prev, 100, time.Unix(0, prev.Mtime), true) {
		t.Fatal("same size+mtime should skip content")
	}
	if !NeedsBackup(FileRecord{}, 1, time.Now(), false, "h1") {
		t.Fatal("new file should need backup")
	}
	if !NeedsACLBackup(prev, "bbb", true) {
		t.Fatal("acl change should need backup")
	}
}

func TestSaveLoadClear(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ProgramData", dir)

	st := &Store{
		JobID:    "job1",
		BaseFull: "host_20260101-100000.zip",
		Files: map[string]FileRecord{
			`docs\a.txt`: {Size: 10, Mtime: 1, Archive: "host_20260101-100000.zip"},
		},
	}
	if err := Save(st); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load("job1")
	if err != nil {
		t.Fatal(err)
	}
	if loaded.BaseFull != st.BaseFull {
		t.Fatalf("base %q", loaded.BaseFull)
	}
	if err := Clear("job1"); err != nil {
		t.Fatal(err)
	}
	loaded, err = Load("job1")
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.Files) != 0 || loaded.BaseFull != "" {
		t.Fatal("expected empty store after clear")
	}
	p := filepath.Join(dir, "PbsWinBackup", "index", "job1", "fileindex.json")
	if _, err := os.Stat(p); err == nil {
		t.Fatal("index file should be removed")
	}
}
