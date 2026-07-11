package pbsbackup

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"

	"pbscommon"
)

func TestGoodbyeLookup_tunnel(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "TUNNEL")
	os.MkdirAll(dir, 0755)
	os.WriteFile(filepath.Join(dir, "tun_back.sh"), []byte("x"), 0644)

	var buf bytes.Buffer
	archive := &pbscommon.PXARArchive{ArchiveName: "backup.pxar.didx"}
	archive.WriteCB = func(b []byte) { buf.Write(b) }
	archive.CatalogWriteCB = func(b []byte) {}
	archive.WriteDir(root, "", true)
	data := buf.Bytes()

	pos, _ := skipEntry(data, 0)
	goodbyePos, err := scanToGoodbye(data, pos)
	if err != nil {
		t.Fatal(err)
	}
	items, _, err := parseGoodbyeBlock(data, goodbyePos)
	if err != nil {
		t.Fatal(err)
	}
	hash := pxarNameHash("TUNNEL")
	idx, ok := goodbyeBSTFind(items, hash)
	if !ok {
		t.Fatalf("BST miss for TUNNEL hash=%x", hash)
	}
	itemPos := goodbyePos + 16 + idx*24
	item := items[idx]
	t.Logf("goodbyePos=%d itemPos=%d storedOffset=%d hash=%x", goodbyePos, itemPos, item.offset, item.hash)

	for _, calc := range []struct {
		name string
		pos  int
	}{
		{"itemPos-offset", itemPos - int(item.offset)},
		{"goodbye-offset", goodbyePos - int(item.offset)},
		{"goodbye+offset", goodbyePos + int(item.offset)},
		{"itemPos+offset", itemPos + int(item.offset)},
	} {
		if calc.pos < 0 || calc.pos+8 > len(data) {
			t.Logf("%s: out of range %d", calc.name, calc.pos)
			continue
		}
		hdr := binary.LittleEndian.Uint64(data[calc.pos:])
		t.Logf("%s: pos=%d hdr=0x%x filename=%v", calc.name, calc.pos, hdr, hdr == pxarFilename)
	}
}
