package pbsbackup

import "testing"

func TestChunkSpansFromAssignments(t *testing.T) {
	assignments := []string{"aa", "bb", "cc"}
	offsets := []uint64{0, 100, 250}
	spans := chunkSpansFromAssignments(assignments, offsets, 400, 50, 250)
	if len(spans) != 2 {
		t.Fatalf("spans %d", len(spans))
	}
	if spans[0].Digest != "aa" || spans[0].Len != 50 {
		t.Fatalf("first %+v", spans[0])
	}
	if spans[1].Digest != "bb" || spans[1].Len != 150 {
		t.Fatalf("second %+v", spans[1])
	}
}
func TestChunkSpansForRange(t *testing.T) {
	records := []didxRecord{
		{offset: 100, digest: "a"},
		{offset: 250, digest: "b"},
		{offset: 400, digest: "c"},
	}
	spans := chunkSpansForRange(records, 50, 300)
	if len(spans) != 3 {
		t.Fatalf("spans %d", len(spans))
	}
}

func TestSpansReusableForFastReuse(t *testing.T) {
	if spansReusableForFastReuse([]fileChunkSpan{{Digest: "a", Len: 100}}) != true {
		t.Fatal("full span should be reusable")
	}
	if spansReusableForFastReuse([]fileChunkSpan{{Digest: "a", Len: 50, Partial: true}}) != false {
		t.Fatal("partial span must not be reused")
	}
}

func TestNormalizeIndexKeyCase(t *testing.T) {
	a := normalizeIndexKey(`Folder\File.TXT`)
	b := normalizeIndexKey(`folder\file.txt`)
	if a != b {
		t.Fatalf("%q vs %q", a, b)
	}
}
