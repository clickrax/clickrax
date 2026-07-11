//go:build windows

package pbsbackup

import (
	"os"
	"testing"
)

func TestWithMmapViewReadOnlyFile(t *testing.T) {
	dir := t.TempDir()
	path := dir + `\readonly.pcat`
	if err := os.WriteFile(path, []byte("catalog-readonly-test"), 0o644); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		t.Fatal(err)
	}

	var got string
	if err := withMmapView(f, info.Size(), func(view []byte) error {
		got = string(view)
		return nil
	}); err != nil {
		t.Fatalf("withMmapView on read-only file: %v", err)
	}
	if got != "catalog-readonly-test" {
		t.Fatalf("got %q", got)
	}
}
