package pbsbackup

import (
	"bufio"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestFinishPayload_DurableOrder(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "restored.txt")
	tmp := dest + ".restoring"

	f, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	bw := bufio.NewWriter(f)

	oldSync := payloadFileSync
	oldSyncDir := payloadSyncDir
	t.Cleanup(func() {
		payloadFileSync = oldSync
		payloadSyncDir = oldSyncDir
	})

	synced := false
	payloadFileSync = func(out *os.File) error {
		if out != f {
			t.Fatal("sync called on unexpected file")
		}
		synced = true
		return out.Sync()
	}
	dirSynced := false
	payloadSyncDir = func(d string) error {
		if d != dir {
			t.Fatalf("unexpected dir %q", d)
		}
		dirSynced = true
		return nil
	}

	parser := &pxarStreamParser{
		payloadTarget: &pxarRestoreTarget{FilePath: `a.txt`, Dest: dest},
		payloadOut:    f,
		payloadBuf:    bw,
		payloadTmp:    tmp,
		payloadDest:   dest,
	}
	if _, err := bw.Write([]byte("payload")); err != nil {
		t.Fatal(err)
	}

	done, err := parser.finishPayload()
	if err != nil {
		t.Fatal(err)
	}
	if !done {
		t.Fatal("expected payload finished")
	}
	if !synced {
		t.Fatal("expected file sync before close")
	}
	if !dirSynced {
		t.Fatal("expected directory sync after rename")
	}
	got, err := os.ReadFile(dest)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "payload" {
		t.Fatalf("got %q", got)
	}
}

func TestFinishPayload_SyncFailureReturnsError(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "restored.txt")
	tmp := dest + ".restoring"

	f, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	bw := bufio.NewWriter(f)

	oldSync := payloadFileSync
	t.Cleanup(func() { payloadFileSync = oldSync })
	payloadFileSync = func(*os.File) error { return errors.New("sync failed") }

	parser := &pxarStreamParser{
		payloadTarget: &pxarRestoreTarget{FilePath: `a.txt`, Dest: dest},
		payloadOut:    f,
		payloadBuf:    bw,
		payloadTmp:    tmp,
		payloadDest:   dest,
	}
	if _, err := bw.Write([]byte("x")); err != nil {
		t.Fatal(err)
	}

	_, err = parser.finishPayload()
	if err == nil {
		t.Fatal("expected sync failure")
	}
	if _, err := os.Stat(dest); !os.IsNotExist(err) {
		t.Fatal("destination should not exist on sync failure")
	}
}
