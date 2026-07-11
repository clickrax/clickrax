package ftpclient

import (
	"bytes"
	"context"
	"io"
	"testing"

	"pbs-win-backup/internal/models"
)

type mockFTPProbe struct {
	quitCalled bool
}

func (m *mockFTPProbe) FileSize(path string) (int64, error) {
	return 128, nil
}

func (m *mockFTPProbe) Quit() error {
	m.quitCalled = true
	return nil
}

func TestOpenRemoteFile_Success_ClosesConnection(t *testing.T) {
	mock := &mockFTPProbe{}
	old := connectForProbe
	connectForProbe = func(ctx context.Context, dest models.BackupDestination, password string) (ftpProbeConn, error) {
		return mock, nil
	}
	defer func() { connectForProbe = old }()

	dest := models.BackupDestination{Type: "ftp", Host: "example.com"}
	_, _, closeFn, err := OpenRemoteFile(context.Background(), dest, "pw", "backup-1", "archive.zip")
	if err != nil {
		t.Fatalf("open remote file: %v", err)
	}
	if !mock.quitCalled {
		t.Fatal("probe FTP connection should be closed on success")
	}
	if closeFn == nil {
		t.Fatal("expected close function")
	}
	if err := closeFn(); err != nil {
		t.Fatalf("closeFn: %v", err)
	}
}

type shortReadCloser struct {
	data []byte
}

func (s *shortReadCloser) Read(p []byte) (int, error) {
	if len(s.data) == 0 {
		return 0, io.EOF
	}
	n := copy(p, s.data)
	s.data = s.data[n:]
	if len(s.data) == 0 {
		return n, io.EOF
	}
	return n, nil
}

func (s *shortReadCloser) Close() error { return nil }

func TestFtpReadAtFrom_ShortRead_ReturnsError(t *testing.T) {
	resp := &shortReadCloser{data: bytes.Repeat([]byte("x"), 500)}
	buf := make([]byte, 1024)
	n, err := ftpReadAtFrom(resp, buf, 0, 1024)
	if err == nil {
		t.Fatalf("expected short read error, got nil (n=%d)", n)
	}
	if n != 500 {
		t.Fatalf("n=%d, want 500", n)
	}
	if err != io.ErrUnexpectedEOF && err != io.EOF {
		t.Fatalf("err=%v, want unexpected EOF", err)
	}
}

func TestFtpReadAtFrom_CloseError(t *testing.T) {
	resp := &closeErrReader{data: bytes.Repeat([]byte("a"), 16)}
	buf := make([]byte, 16)
	_, err := ftpReadAtFrom(resp, buf, 0, 16)
	if err == nil {
		t.Fatal("expected close error")
	}
}

type closeErrReader struct {
	data []byte
}

func (c *closeErrReader) Read(p []byte) (int, error) {
	if len(c.data) == 0 {
		return 0, io.EOF
	}
	n := copy(p, c.data)
	c.data = c.data[n:]
	return n, nil
}

func (c *closeErrReader) Close() error {
	return io.ErrClosedPipe
}
