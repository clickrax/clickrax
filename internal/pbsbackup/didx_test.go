package pbsbackup

import (
	"bytes"
	"testing"
)

func TestReassembleFromRecordsEndOffsets(t *testing.T) {
	chunkA := []byte("hello")
	chunkB := []byte(" world")
	records := []didxRecord{
		{offset: uint64(len(chunkA)), digest: "a"},
		{offset: uint64(len(chunkA) + len(chunkB)), digest: "b"},
	}
	chunks := map[string][]byte{"a": chunkA, "b": chunkB}
	out, err := reassembleFromRecords(records, func(digest string) ([]byte, error) {
		return chunks[digest], nil
	})
	if err != nil {
		t.Fatalf("reassemble: %v", err)
	}
	if !bytes.Equal(out, []byte("hello world")) {
		t.Fatalf("got %q", out)
	}
}

func TestReassembleFromRecordsCatalogMagicAtStart(t *testing.T) {
	catalog := buildTestCatalog()
	records := []didxRecord{{offset: uint64(len(catalog)), digest: "c"}}
	out, err := reassembleFromRecords(records, func(digest string) ([]byte, error) {
		return catalog, nil
	})
	if err != nil {
		t.Fatalf("reassemble: %v", err)
	}
	files, err := parseCatalogAll(out)
	if err != nil {
		t.Fatalf("parseCatalogAll: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}
}
