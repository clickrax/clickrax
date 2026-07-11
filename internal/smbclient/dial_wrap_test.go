package smbclient

import (
	"context"
	"net"
	"testing"
	"time"

	"pbs-win-backup/internal/netio"
)

func TestWrapConnForDial_AppliesReadDeadline(t *testing.T) {
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	ctx := netio.WithIdleTimeout(context.Background(), 1)
	wrapped := wrapConnForDial(ctx, client)

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
		t.Fatal("wrapped dial conn did not unblock on read deadline")
	}
}

func TestWrapConnForDial_UsesIdleFromContext(t *testing.T) {
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	ctx := netio.WithIdleTimeout(context.Background(), 1)
	wrapped := wrapConnForDial(ctx, client)

	done := make(chan struct{})
	go func() {
		_, _ = wrapped.Read(make([]byte, 8))
		close(done)
	}()

	start := time.Now()
	select {
	case <-done:
		if time.Since(start) > 3*time.Second {
			t.Fatal("read took too long with 1s idle timeout")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("read did not unblock")
	}
}
