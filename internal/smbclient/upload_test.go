package smbclient

import (
	"context"
	"io"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/hirochachacha/go-smb2"

	"pbs-win-backup/internal/models"
)

type stallWriter struct {
	conn net.Conn
}

func (w *stallWriter) Write(p []byte) (int, error) {
	_ = w.conn.SetWriteDeadline(time.Now().Add(50 * time.Millisecond))
	_, err := w.conn.Write(p)
	return 0, err
}

func (w *stallWriter) Close() error { return nil }

func TestSMBUpload_StalledPeer_TimesOutAndCancels(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	accepted := make(chan net.Conn, 1)
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		accepted <- conn
	}()

	clientConn, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}

	oldDial := dialFunc
	oldCreate := smbShareCreate
	oldMkdir := mkdirAll
	oldCommit := commitSMBUpload
	oldRemove := smbShareRemove
	t.Cleanup(func() {
		dialFunc = oldDial
		smbShareCreate = oldCreate
		mkdirAll = oldMkdir
		commitSMBUpload = oldCommit
		smbShareRemove = oldRemove
	})

	dialFunc = func(ctx context.Context, dest models.BackupDestination, password string) (*smb2.Share, *smb2.Session, net.Conn, error) {
		return nil, nil, wrapConnForDial(ctx, clientConn), nil
	}
	mkdirAll = func(*smb2.Share, string) error { return nil }
	smbShareRemove = func(*smb2.Share, string) error { return nil }
	commitSMBUpload = func(smbFileOps, string, string, int64) error { return nil }
	smbShareRemove = func(*smb2.Share, string) error { return nil }
	smbShareCreate = func(*smb2.Share, string) (io.WriteCloser, error) {
		return &stallWriter{conn: clientConn}, nil
	}

	local := filepath.Join(t.TempDir(), "upload.zip")
	if err := os.WriteFile(local, []byte("zip-data"), 0o644); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	start := time.Now()
	err = Upload(ctx, models.BackupDestination{Type: models.DestSMB, Host: "127.0.0.1", Share: "share"}, "pw", local, "backup", "file.zip", nil)
	elapsed := time.Since(start)
	if err == nil {
		t.Fatal("expected stalled upload to fail")
	}
	if elapsed > 2*time.Second {
		t.Fatalf("upload blocked too long: %s", elapsed)
	}
}
