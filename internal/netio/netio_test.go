package netio

import (
	"context"
	"io"
	"net"
	"testing"
	"time"
)

func TestWrapConn_ContextCancelClosesConn(t *testing.T) {
	server, client := net.Pipe()
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	wrapped := WrapConn(ctx, client, time.Minute)
	cancel()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		_, err := wrapped.Read(make([]byte, 1))
		if err != nil {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("wrapped conn should close when context is cancelled")
}

func TestIdleConn_ReadDeadlineUnblocks(t *testing.T) {
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	idle := 50 * time.Millisecond
	wrapped := WrapConn(context.Background(), client, idle)

	done := make(chan struct{})
	go func() {
		_, _ = wrapped.Read(make([]byte, 16))
		close(done)
	}()

	start := time.Now()
	select {
	case <-done:
		elapsed := time.Since(start)
		if elapsed > 2*time.Second {
			t.Fatalf("read blocked too long: %s", elapsed)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("read did not unblock on deadline")
	}
}

func TestReaderWithConn_ReadDeadlineUnblocks(t *testing.T) {
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	idle := 50 * time.Millisecond
	reader := ReaderWithConn(context.Background(), client, client, idle)

	done := make(chan struct{})
	go func() {
		_, _ = io.ReadAll(reader)
		close(done)
	}()

	start := time.Now()
	select {
	case <-done:
		elapsed := time.Since(start)
		if elapsed > 2*time.Second {
			t.Fatalf("read blocked too long: %s", elapsed)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("read did not unblock on deadline")
	}
}

func TestCopyWithWriteDeadline_ReadDeadlineUnblocks(t *testing.T) {
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	idle := 50 * time.Millisecond
	wrapped := WrapConn(context.Background(), client, idle)

	done := make(chan struct{})
	go func() {
		_, _ = CopyWithWriteDeadline(context.Background(), io.Discard, wrapped, wrapped, idle)
		close(done)
	}()

	start := time.Now()
	select {
	case <-done:
		elapsed := time.Since(start)
		if elapsed > 2*time.Second {
			t.Fatalf("copy blocked too long: %s", elapsed)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("copy did not unblock on deadline")
	}
}

func TestReaderAtConn_ReadAtDeadlineUnblocks(t *testing.T) {
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	ra := &ReaderAtConn{
		ReaderAt: readerAtConn{client},
		Conn:     client,
		Ctx:      context.Background(),
		Idle:     50 * time.Millisecond,
	}

	done := make(chan struct{})
	go func() {
		_, _ = ra.ReadAt(make([]byte, 8), 0)
		close(done)
	}()

	start := time.Now()
	select {
	case <-done:
		elapsed := time.Since(start)
		if elapsed > 2 * time.Second {
			t.Fatalf("readat blocked too long: %s", elapsed)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("readat did not unblock on deadline")
	}
}

type readerAtConn struct {
	net.Conn
}

func (r readerAtConn) ReadAt(p []byte, off int64) (int, error) {
	if off != 0 {
		return 0, io.EOF
	}
	return r.Conn.Read(p)
}
