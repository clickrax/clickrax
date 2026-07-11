package paths

import (
	"fmt"
	"os"
	"path/filepath"
)

type durableFile interface {
	Write([]byte) (int, error)
	Sync() error
	Close() error
}

var (
	createDurableFile = func(path string, perm os.FileMode) (durableFile, error) {
		return os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, perm)
	}
	syncDirAfterRename = syncDir
)

func durableWriteFile(path string, data []byte, perm os.FileMode) error {
	f, err := createDurableFile(path, perm)
	if err != nil {
		return err
	}
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		_ = os.Remove(path)
		return err
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		_ = os.Remove(path)
		return err
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(path)
		return err
	}
	return nil
}

func durableCloseRename(f *os.File, tmpPath, finalPath string) error {
	if err := f.Sync(); err != nil {
		_ = f.Close()
		_ = os.Remove(tmpPath)
		return err
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	if err := renameFile(tmpPath, finalPath); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("rename %s %s: %w", tmpPath, finalPath, err)
	}
	return syncDirAfterRename(filepath.Dir(finalPath))
}

// SyncDir fsyncs a directory entry after atomic rename.
func SyncDir(dir string) error {
	return syncDir(dir)
}
