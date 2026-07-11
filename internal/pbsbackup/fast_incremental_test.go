package pbsbackup

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"pbs-win-backup/internal/fileindex"
	"pbs-win-backup/internal/filemeta"
	"pbs-win-backup/internal/winattr"
)

func TestPBSFileIndexCanReuse(t *testing.T) {
	idx := &PBSFileIndex{
		Files: map[string]PBSFileRecord{
			`dir\file.txt`: {
				Size:  100,
				Mtime: 1000,
				ChunkSpans: []fileChunkSpan{
					{Digest: "abc", Len: 100},
				},
			},
		},
	}
	rec, ok := idx.canReuse(`dir\file.txt`, 100, 1000, "")
	if !ok {
		t.Fatal("expected reuse")
	}
	if len(rec.ChunkSpans) != 1 {
		t.Fatalf("spans %d", len(rec.ChunkSpans))
	}
	_, ok = idx.canReuse(`dir\file.txt`, 101, 1000, "")
	if ok {
		t.Fatal("size change should disable reuse")
	}
	_, ok = idx.canReuse(`dir\file.txt`, 100, 2e9, "")
	if ok {
		t.Fatal("mtime change should disable reuse")
	}
	idx.Files[`dir\file2.txt`] = PBSFileRecord{Size: 10, Mtime: 5e9, ChunkSpans: []fileChunkSpan{{Digest: "x", Len: 10}}}
	_, ok = idx.canReuse(`dir\file2.txt`, 10, 5e9+500, "")
	if !ok {
		t.Fatal("expected second-precision mtime match")
	}
	_, ok = idx.canReuse(`dir\file.txt`, 100, 1000, "acl-new")
	if ok {
		t.Fatal("acl change with empty cached acl should disable reuse")
	}
	idx.Files[`dir\file.txt`] = PBSFileRecord{
		Size: 100, Mtime: 1000, ACLHash: "acl-old",
		ChunkSpans: []fileChunkSpan{{Digest: "abc", Len: 100}},
	}
	_, ok = idx.canReuse(`dir\file.txt`, 100, 1000, "acl-new")
	if ok {
		t.Fatal("acl mismatch should disable reuse")
	}
	_, ok = idx.canReuse(`dir\file.txt`, 100, 1000, "acl-old")
	if !ok {
		t.Fatal("matching acl should allow reuse")
	}
}

func TestNeedsPBSBootstrap(t *testing.T) {
	dir := t.TempDir()
	orig := os.Getenv("ProgramData")
	t.Setenv("ProgramData", dir)
	defer os.Setenv("ProgramData", orig)

	if !needsPBSBootstrap("job-x") {
		t.Fatal("empty index should need bootstrap")
	}
	idx := &PBSFileIndex{JobID: "job-x", Files: map[string]PBSFileRecord{
		"a": {ChunkSpans: []fileChunkSpan{{Digest: "d", Len: 1}}},
	}}
	if err := SavePBSFileIndex(idx); err != nil {
		t.Fatal(err)
	}
	if needsPBSBootstrap("job-x") {
		t.Fatal("index with chunk spans should not need bootstrap")
	}
}

func TestPBSFileIndexNeedsBackupCompat(t *testing.T) {
	prev := PBSFileRecord{Size: 10, Mtime: 20}
	fr := fileindex.FileRecord{Size: prev.Size, Mtime: prev.Mtime}
	if fileindex.NeedsContentBackup(fr, 10, time.Unix(0, 20), true) {
		t.Fatal("same mtime_ns should not need backup")
	}
}

func TestFastIncrementalReuseRoundTrip(t *testing.T) {
	dir := t.TempDir()
	orig := os.Getenv("ProgramData")
	t.Setenv("ProgramData", dir)
	defer os.Setenv("ProgramData", orig)

	jobID := "job1"
	root := filepath.Join(dir, "root")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	filePath := filepath.Join(root, "f.txt")
	if err := os.WriteFile(filePath, []byte("payload"), 0o644); err != nil {
		t.Fatal(err)
	}

	spans := []fileChunkSpan{{Digest: "deadbeef", Len: 7}}
	fi1, err := newFastIncremental(jobID, root, true)
	if err != nil {
		t.Fatal(err)
	}
	fi1.recordFile(filePath, mustStat(t, filePath), spans)
	idx := fi1.live.snapshot("2026-01-01T00:00:00Z")
	rec := idx.Files["f.txt"]
	if e, err := filemeta.CaptureFile(filePath); err == nil {
		rec.ACLHash = winattr.ACLHash(e)
	}
	idx.Files = make(map[string]PBSFileRecord, minCacheFiles+1)
	idx.Files["f.txt"] = rec
	for i := 0; i < minCacheFiles; i++ {
		idx.Files[fmt.Sprintf("file%d.txt", i)] = rec
	}
	if err := SavePBSFileIndex(idx); err != nil {
		t.Fatal(err)
	}
	_ = markPBSCacheReady(jobID)
	fi1.close()

	fi2, err := newFastIncremental(jobID, root, false)
	if err != nil {
		t.Fatal(err)
	}
	defer fi2.close()
	if !fi2.reuseEnabled {
		t.Fatal("expected reuse enabled")
	}
	info, err := os.Stat(filePath)
	if err != nil {
		t.Fatal(err)
	}
	got, ok := fi2.reuseChunkSpans(filePath, "f.txt", info)
	if !ok || len(got) != 1 || got[0].Len != 7 {
		t.Fatalf("got %+v ok=%v", got, ok)
	}
}

func mustStat(t *testing.T, path string) os.FileInfo {
	t.Helper()
	st, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	return st
}
