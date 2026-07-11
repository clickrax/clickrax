package pbsbackup

import (
	"os"
	"testing"
)

func TestPBSCacheReadyPartialCleared(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ProgramData", dir)
	jobID := "job-partial"

	idx := &PBSFileIndex{
		JobID: jobID,
		Files: map[string]PBSFileRecord{
			"a.txt": {Size: 1, ChunkSpans: []fileChunkSpan{{Digest: "a", Len: 1}}},
		},
	}
	if err := SavePBSFileIndex(idx); err != nil {
		t.Fatal(err)
	}

	if isPBSCacheReady(jobID) {
		t.Fatal("partial cache should not be ready")
	}
}

func TestPBSCacheReadyMarkerAloneNotEnough(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ProgramData", dir)
	jobID := "job-marker-only"
	if err := markPBSCacheReady(jobID); err != nil {
		t.Fatal(err)
	}
	if isPBSCacheReady(jobID) {
		t.Fatal("marker without substantial cache must not be ready")
	}
}

func TestClearPBSFileIndexRemovesMarker(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ProgramData", dir)
	jobID := "job-clear"
	_ = markPBSCacheReady(jobID)
	_ = ClearPBSFileIndex(jobID)
	path, _ := pbsCacheReadyPath(jobID)
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("marker should be removed")
	}
}
