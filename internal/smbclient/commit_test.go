package smbclient

import (
	"errors"
	"io/fs"
	"os"
	"testing"
	"time"
)

type mockFileInfo struct {
	size int64
}

func (m mockFileInfo) Name() string       { return "x" }
func (m mockFileInfo) Size() int64        { return m.size }
func (m mockFileInfo) Mode() fs.FileMode  { return 0 }
func (m mockFileInfo) ModTime() time.Time { return time.Now() }
func (m mockFileInfo) IsDir() bool        { return false }
func (m mockFileInfo) Sys() any           { return nil }

type renameFailOps struct {
	removedPartial bool
}

func (m *renameFailOps) Stat(path string) (os.FileInfo, error) {
	return mockFileInfo{size: 5}, nil
}

func (m *renameFailOps) Remove(path string) error {
	if path == "a.partial" {
		m.removedPartial = true
	}
	return nil
}

func (m *renameFailOps) Rename(oldpath, newpath string) error {
	return errors.New("rename failed")
}

func TestCommitSMBUpload_RenameFailure_KeepsPartial(t *testing.T) {
	ops := &renameFailOps{}
	err := commitSMBUpload(ops, "a.partial", "a.zip", 5)
	if err == nil {
		t.Fatal("expected rename failure")
	}
	if ops.removedPartial {
		t.Fatal("partial should not be removed on rename failure")
	}
}
