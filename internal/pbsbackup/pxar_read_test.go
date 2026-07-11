package pbsbackup

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"pbscommon"
)

func buildTestPXAR(t *testing.T, root string) []byte {
	t.Helper()
	var buf bytes.Buffer
	archive := &pbscommon.PXARArchive{ArchiveName: "backup.pxar.didx"}
	archive.WriteCB = func(b []byte) { buf.Write(b) }
	archive.CatalogWriteCB = func(b []byte) {}
	if _, err := archive.WriteDir(root, "", true); err != nil {
		t.Fatalf("WriteDir: %v", err)
	}
	return buf.Bytes()
}

func TestExtractFileFromPXAR_nested(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "TUNNEL")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	want := []byte("#!/bin/bash\necho ok\n")
	if err := os.WriteFile(filepath.Join(dir, "tun_back.sh"), want, 0o644); err != nil {
		t.Fatal(err)
	}

	pxar := buildTestPXAR(t, root)
	got, err := extractFileFromPXAR(pxar, `TUNNEL\tun_back.sh`)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("payload mismatch: %q", got)
	}
}

func TestExtractFileFromPXAR_deepWithSiblingDir(t *testing.T) {
	root := t.TempDir()
	// Shallow sibling that should not confuse lookup.
	api := filepath.Join(root, "api", "v1", "nested")
	if err := os.MkdirAll(api, 0o755); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 20; i++ {
		p := filepath.Join(api, fmt.Sprintf("file%d.txt", i))
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "antibot.lua"), []byte("root"), 0o644); err != nil {
		t.Fatal(err)
	}

	deep := filepath.Join(root, "example.com", "vendor", "composer", "ca-bundle", "src")
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatal(err)
	}
	want := []byte("<?php\n// CaBundle\n")
	if err := os.WriteFile(filepath.Join(deep, "CaBundle.php"), want, 0o644); err != nil {
		t.Fatal(err)
	}

	pxar := buildTestPXAR(t, root)
	got, err := extractFileFromPXAR(pxar, `example.com\vendor\composer\ca-bundle\src\CaBundle.php`)
	if err != nil {
		t.Fatalf("extract deep: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("payload mismatch: %q", got)
	}
}
func TestExtractFileFromPXAR_rootFile(t *testing.T) {
	root := t.TempDir()
	want := []byte("hello")
	if err := os.WriteFile(filepath.Join(root, "readme.txt"), want, 0o644); err != nil {
		t.Fatal(err)
	}

	pxar := buildTestPXAR(t, root)
	got, err := extractFileFromPXAR(pxar, "readme.txt")
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("payload mismatch: %q", got)
	}
}
