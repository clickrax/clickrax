package smbclient

import (
	"os"
	"testing"
	"time"
)

type memFileInfo struct {
	name string
	size int64
}

func (m memFileInfo) Name() string       { return m.name }
func (m memFileInfo) Size() int64        { return m.size }
func (m memFileInfo) Mode() os.FileMode  { return 0o644 }
func (m memFileInfo) ModTime() time.Time { return time.Time{} }
func (m memFileInfo) IsDir() bool        { return false }
func (m memFileInfo) Sys() any           { return nil }

type memShare struct {
	files map[string][]byte
}

func (m *memShare) Stat(path string) (os.FileInfo, error) {
	data, ok := m.files[path]
	if !ok {
		return nil, os.ErrNotExist
	}
	return memFileInfo{name: path, size: int64(len(data))}, nil
}

func (m *memShare) Remove(path string) error {
	delete(m.files, path)
	return nil
}

func (m *memShare) Rename(oldpath, newpath string) error {
	data, ok := m.files[oldpath]
	if !ok {
		return os.ErrNotExist
	}
	delete(m.files, oldpath)
	m.files[newpath] = data
	return nil
}

func TestSMBUpload_SizeMismatch_NoOrphanAtFinalPath(t *testing.T) {
	sh := &memShare{
		files: map[string][]byte{
			"backup/host.zip.partial": make([]byte, 50),
		},
	}
	const expected int64 = 100
	err := commitSMBUpload(sh, "backup/host.zip.partial", "backup/host.zip", expected)
	if err == nil {
		t.Fatal("expected size mismatch error")
	}
	if _, ok := sh.files["backup/host.zip"]; ok {
		t.Fatal("orphan file exists at final path after size mismatch")
	}
	if _, ok := sh.files["backup/host.zip.partial"]; ok {
		t.Fatal("partial file should be removed after size mismatch")
	}
}

func TestSMBUpload_SizeMatch_RenamesToFinalPath(t *testing.T) {
	content := []byte("full upload")
	sh := &memShare{
		files: map[string][]byte{
			"backup/host.zip.partial": content,
		},
	}
	if err := commitSMBUpload(sh, "backup/host.zip.partial", "backup/host.zip", int64(len(content))); err != nil {
		t.Fatal(err)
	}
	if _, ok := sh.files["backup/host.zip.partial"]; ok {
		t.Fatal("partial should be gone")
	}
	got, ok := sh.files["backup/host.zip"]
	if !ok || string(got) != string(content) {
		t.Fatalf("final file missing or wrong: %q", got)
	}
}
