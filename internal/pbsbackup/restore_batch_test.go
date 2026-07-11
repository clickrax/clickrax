package pbsbackup

import (
	"testing"

	"pbs-win-backup/internal/models"
)

func TestCollectBatchTargets_folderAndFile(t *testing.T) {
	catalog := []models.SnapshotFile{
		{Path: `api`, IsDir: true},
		{Path: `api\v1\a.txt`, Size: 10},
		{Path: `api\v1\b.txt`, Size: 20},
		{Path: `readme.txt`, Size: 5},
	}

	targets, err := collectBatchTargets(catalog, []string{`api`, `readme.txt`}, []string{`C:\back`}, `C:\dest`, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(targets) != 3 {
		t.Fatalf("want 3 targets, got %d", len(targets))
	}
	byFile := map[string]string{}
	for _, t := range targets {
		byFile[t.filePath] = t.dest
	}
	if byFile[`api\v1\a.txt`] != `C:\dest\api\v1\a.txt` {
		t.Fatalf("folder dest %q", byFile[`api\v1\a.txt`])
	}
	if byFile[`readme.txt`] != `C:\dest\readme.txt` {
		t.Fatalf("file dest %q", byFile[`readme.txt`])
	}
}

func TestCollectBatchTargets_nestedFoldersDedupe(t *testing.T) {
	catalog := []models.SnapshotFile{
		{Path: `api\v1\a.txt`, Size: 10},
	}
	targets, err := collectBatchTargets(catalog, []string{`api`, `api\v1`}, []string{`C:\back`}, `C:\dest`, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(targets) != 1 {
		t.Fatalf("want 1 target, got %d", len(targets))
	}
	if targets[0].dest != `C:\dest\api\v1\a.txt` {
		t.Fatalf("dest %q", targets[0].dest)
	}
}

func TestCollectBatchTargets_folderPreservesNameInCustomDest(t *testing.T) {
	catalog := []models.SnapshotFile{
		{Path: `example.com\api.php`, Size: 10},
		{Path: `example.com\sub\file.txt`, Size: 20},
	}
	targets, err := collectBatchTargets(catalog, []string{`example.com`}, nil, `C:\restore\custom`, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(targets) != 2 {
		t.Fatalf("want 2 targets, got %d", len(targets))
	}
	want := `C:\restore\custom\example.com\api.php`
	if targets[0].dest != want {
		t.Fatalf("dest %q want %q", targets[0].dest, want)
	}
}

func TestCollectBatchTargets_toOriginal(t *testing.T) {
	catalog := []models.SnapshotFile{
		{Path: `example.com\api.php`, Size: 10},
	}
	sources := []string{`D:\backup-source`}
	targets, err := collectBatchTargets(catalog, []string{`example.com\api.php`}, sources, "", true)
	if err != nil {
		t.Fatal(err)
	}
	if len(targets) != 1 {
		t.Fatalf("want 1 target, got %d", len(targets))
	}
	want := `D:\backup-source\example.com\api.php`
	if targets[0].dest != want {
		t.Fatalf("dest %q want %q", targets[0].dest, want)
	}
}
