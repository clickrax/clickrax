package paths

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

type syncTrackingFile struct {
	*os.File
	synced bool
}

func (f *syncTrackingFile) Sync() error {
	f.synced = true
	return f.File.Sync()
}

func TestAtomicWrite_DurableOrder(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "state.json")

	oldCreate := createDurableFile
	oldRename := renameFile
	oldSyncDir := syncDirAfterRename
	t.Cleanup(func() {
		createDurableFile = oldCreate
		renameFile = oldRename
		syncDirAfterRename = oldSyncDir
	})

	var tracked *syncTrackingFile
	createDurableFile = func(path string, perm os.FileMode) (durableFile, error) {
		f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, perm)
		if err != nil {
			return nil, err
		}
		tracked = &syncTrackingFile{File: f}
		return tracked, nil
	}
	dirSynced := false
	syncDirAfterRename = func(string) error {
		dirSynced = true
		return nil
	}
	renameFile = func(src, dst string) error {
		if tracked == nil || !tracked.synced {
			t.Fatal("rename before file sync")
		}
		return os.Rename(src, dst)
	}

	if err := AtomicWrite(target, []byte("payload"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !dirSynced {
		t.Fatal("expected directory sync after rename")
	}
}

func TestAtomicWrite_SyncFailureReturnsError(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "state.json")

	oldCreate := createDurableFile
	t.Cleanup(func() { createDurableFile = oldCreate })

	createDurableFile = func(path string, perm os.FileMode) (durableFile, error) {
		f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, perm)
		if err != nil {
			return nil, err
		}
		return &syncFailFile{File: f}, nil
	}

	err := AtomicWrite(target, []byte("x"), 0o644)
	if err == nil {
		t.Fatal("expected sync failure")
	}
}

type syncFailFile struct {
	*os.File
}

func (f *syncFailFile) Sync() error {
	return errors.New("sync failed")
}
