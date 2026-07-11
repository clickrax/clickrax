package pbsbackup

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"pbs-win-backup/internal/filemeta"
)

func TestUnionChunkIndices(t *testing.T) {
	records := []didxRecord{
		{offset: 100},
		{offset: 250},
		{offset: 400},
		{offset: 600},
	}
	got := unionChunkIndices(records, [][2]uint64{{10, 120}, {300, 350}})
	want := []int{0, 1, 2}
	if len(got) != len(want) {
		t.Fatalf("got %v want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v want %v", got, want)
		}
	}
}

func TestPxarIndexRecorder_buildsOffsets(t *testing.T) {
	root := t.TempDir()
	src := t.TempDir()
	if err := os.WriteFile(filepath.Join(src, "a.txt"), []byte("aaa"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "b.txt"), []byte("bbbb"), 0o644); err != nil {
		t.Fatal(err)
	}
	pxar := buildTestPXAR(t, src)

	rec := newPxarIndexRecorder()
	if _, err := rec.feed(pxar); err != nil {
		t.Fatal(err)
	}
	if rec.index == nil || len(rec.index.Files) < 2 {
		t.Fatalf("index files: %d", len(rec.index.Files))
	}

	posA, ok := rec.index.lookup("a.txt")
	if !ok {
		t.Fatal("a.txt missing")
	}
	posB, ok := rec.index.lookup("b.txt")
	if !ok {
		t.Fatal("b.txt missing")
	}

	view := newChunkView([]didxRecord{{offset: uint64(len(pxar))}})
	view.add(0, pxar)
	gotA, err := extractPayloadFromView(view, posA)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(gotA, []byte("aaa")) {
		t.Fatalf("a: %q", gotA)
	}
	gotB, err := extractPayloadFromView(view, posB)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(gotB, []byte("bbbb")) {
		t.Fatalf("b: %q", gotB)
	}

	destA := filepath.Join(root, "a.txt")
	targets := []pxarRestoreTarget{{FilePath: "a.txt", Dest: destA}}
	ranges, err := indexRangesForTargets(*rec.index, targets)
	if err != nil {
		t.Fatal(err)
	}
	indices := unionChunkIndices([]didxRecord{{offset: uint64(len(pxar))}}, ranges)
	if len(indices) != 1 {
		t.Fatalf("expected 1 chunk, got %d", len(indices))
	}
	_ = filemeta.Archive{}
}

func TestPartitionTargetsByIndex(t *testing.T) {
	idx := newPxarFileIndex()
	idx.set("a.txt", pxarFilePos{Offset: 10, Size: 3})
	targets := []pxarRestoreTarget{
		{FilePath: "a.txt"},
		{FilePath: "missing.txt"},
	}
	indexed, missing := partitionTargetsByIndex(*idx, targets)
	if len(indexed) != 1 || len(missing) != 1 {
		t.Fatalf("indexed=%d missing=%d", len(indexed), len(missing))
	}
}

func TestResolvePxarIndex_localCache(t *testing.T) {
	t.Setenv("ProgramData", t.TempDir())
	ref := SnapshotRef{BackupID: "job-test", Time: "2026-07-06T00:00:00Z"}
	idx := newPxarFileIndex()
	idx.set("foo.txt", pxarFilePos{Offset: 100, Size: 3})
	if err := saveLocalPxarIndex(ref, idx); err != nil {
		t.Fatal(err)
	}
	loaded, ok, err := loadLocalPxarIndex(ref)
	if err != nil || !ok {
		t.Fatalf("load: ok=%v err=%v", ok, err)
	}
	if _, ok := loaded.lookup("foo.txt"); !ok {
		t.Fatal("foo.txt missing in cache")
	}
}
