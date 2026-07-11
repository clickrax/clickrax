package pbsbackup

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"pbs-win-backup/internal/filemeta"
)

func TestPxarStreamParser_writesFilesInSmallChunks(t *testing.T) {
	root := t.TempDir()
	src := t.TempDir()
	api := filepath.Join(src, "api", "v1")
	if err := os.MkdirAll(api, 0o755); err != nil {
		t.Fatal(err)
	}
	wantA := []byte("alpha")
	wantB := []byte("beta")
	if err := os.WriteFile(filepath.Join(api, "a.txt"), wantA, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(api, "b.txt"), wantB, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "readme.txt"), []byte("root"), 0o644); err != nil {
		t.Fatal(err)
	}

	pxar := buildTestPXAR(t, src)
	targets := []pxarRestoreTarget{
		{FilePath: `api\v1\a.txt`, Dest: filepath.Join(root, "a.txt")},
		{FilePath: `api\v1\b.txt`, Dest: filepath.Join(root, "b.txt")},
	}
	parser := newPxarStreamParser(context.Background(), targets, filemeta.Archive{}, "overwrite", true, nil)

	chunkSize := 4096
	for off := 0; off < len(pxar); off += chunkSize {
		end := off + chunkSize
		if end > len(pxar) {
			end = len(pxar)
		}
		done, err := parser.feed(pxar[off:end])
		if err != nil {
			t.Fatalf("feed at %d: %v", off, err)
		}
		if done && off+chunkSize < len(pxar) {
			t.Fatalf("finished too early at offset %d", off)
		}
	}
	if _, err := parser.finish(); err != nil {
		t.Fatalf("finish: %v", err)
	}
	gotA, err := os.ReadFile(filepath.Join(root, "a.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(gotA, wantA) {
		t.Fatalf("a.txt: %q", gotA)
	}
	gotB, err := os.ReadFile(filepath.Join(root, "b.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(gotB, wantB) {
		t.Fatalf("b.txt: %q", gotB)
	}
}

func TestPxarStreamParser_skipsUnrelatedSubtree(t *testing.T) {
	root := t.TempDir()
	src := t.TempDir()
	deep := filepath.Join(src, "example.com", "vendor", "composer", "ca-bundle", "src")
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatal(err)
	}
	want := []byte("<?php\n")
	if err := os.WriteFile(filepath.Join(deep, "CaBundle.php"), want, 0o644); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 20; i++ {
		p := filepath.Join(src, "api", "v1", fmt.Sprintf("file%d.txt", i))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	pxar := buildTestPXAR(t, src)
	dest := filepath.Join(root, "CaBundle.php")
	parser := newPxarStreamParser(context.Background(), []pxarRestoreTarget{
		{FilePath: `example.com\vendor\composer\ca-bundle\src\CaBundle.php`, Dest: dest},
	}, filemeta.Archive{}, "overwrite", true, nil)

	for off := 0; off < len(pxar); off += 8192 {
		end := off + 8192
		if end > len(pxar) {
			end = len(pxar)
		}
		if _, err := parser.feed(pxar[off:end]); err != nil {
			t.Fatalf("feed: %v", err)
		}
	}
	if _, err := parser.finish(); err != nil {
		t.Fatalf("finish: %v", err)
	}
	got, err := os.ReadFile(dest)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("payload %q", got)
	}
}

func TestPxarStreamParser_stopsEarlyWhenAllTargetsDone(t *testing.T) {
	src := t.TempDir()
	if err := os.WriteFile(filepath.Join(src, "early.txt"), []byte("e"), 0o644); err != nil {
		t.Fatal(err)
	}
	large := filepath.Join(src, "zzz_noise")
	if err := os.MkdirAll(large, 0o755); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 50; i++ {
		p := filepath.Join(large, fmt.Sprintf("f%d.dat", i))
		if err := os.WriteFile(p, bytes.Repeat([]byte("z"), 64*1024), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	pxar := buildTestPXAR(t, src)
	parser := newPxarStreamParser(context.Background(), []pxarRestoreTarget{
		{FilePath: "early.txt", Dest: filepath.Join(t.TempDir(), "early.txt")},
	}, filemeta.Archive{}, "overwrite", true, nil)

	consumed := 0
	for off := 0; off < len(pxar); off += 1024 {
		end := off + 1024
		if end > len(pxar) {
			end = len(pxar)
		}
		consumed = end
		done, err := parser.feed(pxar[off:end])
		if err != nil {
			t.Fatalf("feed: %v", err)
		}
		if done {
			break
		}
	}
	if consumed >= len(pxar) {
		t.Fatalf("expected early stop before full archive, consumed %d of %d", consumed, len(pxar))
	}
}
