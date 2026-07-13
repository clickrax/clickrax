package netio

import (
	"context"
	"io"
	"net"
	"time"
)

const DefaultIdleTimeout = 60 * time.Second

type ctxIdleKey struct{}

// WithIdleTimeout stores network idle timeout (seconds) in ctx for SMB/FTP I/O.
func WithIdleTimeout(ctx context.Context, sec int) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, ctxIdleKey{}, sec)
}

// IdleFromContext returns the idle timeout from ctx or DefaultIdleTimeout.
func IdleFromContext(ctx context.Context) time.Duration {
	if ctx != nil {
		if sec, ok := ctx.Value(ctxIdleKey{}).(int); ok {
			return IdleTimeout(sec)
		}
	}
	return DefaultIdleTimeout
}

func IdleTimeout(sec int) time.Duration {
	if sec <= 0 {
		return DefaultIdleTimeout
	}
	return time.Duration(sec) * time.Second
}

type ctxReader struct {
	ctx context.Context
	r   io.Reader
}

// Reader wraps r to return ctx.Err() before each read.
func Reader(ctx context.Context, r io.Reader) io.Reader {
	if ctx == nil || r == nil {
		return r
	}
	return &ctxReader{ctx: ctx, r: r}
}

func (c *ctxReader) Read(p []byte) (int, error) {
	if err := c.ctx.Err(); err != nil {
		return 0, err
	}
	return c.r.Read(p)
}

type connReader struct {
	ctx  context.Context
	r    io.Reader
	conn net.Conn
	idle time.Duration
}

// ReaderWithConn wraps r and refreshes read deadlines on conn before each read.
func ReaderWithConn(ctx context.Context, r io.Reader, conn net.Conn, idle time.Duration) io.Reader {
	if r == nil {
		return r
	}
	if conn == nil {
		return Reader(ctx, r)
	}
	if idle <= 0 {
		idle = DefaultIdleTimeout
	}
	return &connReader{ctx: ctx, r: r, conn: conn, idle: idle}
}

func (c *connReader) Read(p []byte) (int, error) {
	if c.ctx != nil {
		if err := c.ctx.Err(); err != nil {
			return 0, err
		}
	}
	_ = c.conn.SetReadDeadline(time.Now().Add(c.idle))
	n, err := c.r.Read(p)
	_ = c.conn.SetReadDeadline(time.Time{})
	return n, err
}

// ReaderAtConn wraps ReaderAt with per-read deadlines and ctx cancellation.
type ReaderAtConn struct {
	io.ReaderAt
	Conn net.Conn
	Ctx  context.Context
	Idle time.Duration
}

func (r *ReaderAtConn) ReadAt(p []byte, off int64) (int, error) {
	if r.Ctx != nil {
		if err := r.Ctx.Err(); err != nil {
			return 0, err
		}
	}
	idle := r.Idle
	if idle <= 0 {
		idle = DefaultIdleTimeout
	}
	if r.Conn != nil {
		_ = r.Conn.SetReadDeadline(time.Now().Add(idle))
	}
	n, err := r.ReaderAt.ReadAt(p, off)
	if r.Conn != nil {
		_ = r.Conn.SetReadDeadline(time.Time{})
	}
	return n, err
}

type idleConn struct {
	net.Conn
	ctx  context.Context
	idle time.Duration
}

// WrapConn applies sliding read/write deadlines and honors ctx on each I/O.
func WrapConn(ctx context.Context, conn net.Conn, idle time.Duration) net.Conn {
	if conn == nil {
		return nil
	}
	if idle <= 0 {
		idle = DefaultIdleTimeout
	}
	ic := &idleConn{Conn: conn, ctx: ctx, idle: idle}
	if ctx != nil {
		go func() {
			<-ctx.Done()
			_ = conn.Close()
		}()
	}
	return ic
}

func (c *idleConn) Read(p []byte) (int, error) {
	if c.ctx != nil {
		if err := c.ctx.Err(); err != nil {
			return 0, err
		}
	}
	_ = c.Conn.SetReadDeadline(time.Now().Add(c.idle))
	n, err := c.Conn.Read(p)
	_ = c.Conn.SetReadDeadline(time.Time{})
	return n, err
}

func (c *idleConn) Write(p []byte) (int, error) {
	if c.ctx != nil {
		if err := c.ctx.Err(); err != nil {
			return 0, err
		}
	}
	_ = c.Conn.SetWriteDeadline(time.Now().Add(c.idle))
	n, err := c.Conn.Write(p)
	_ = c.Conn.SetWriteDeadline(time.Time{})
	return n, err
}

// CopyWithWriteDeadline copies src to dst while refreshing write deadlines on conn.
func CopyWithWriteDeadline(ctx context.Context, dst io.Writer, src io.Reader, conn net.Conn, idle time.Duration) (int64, error) {
	if idle <= 0 {
		idle = DefaultIdleTimeout
	}
	src = Reader(ctx, src)
	buf := make([]byte, 32*1024)
	var written int64
	for {
		if ctx != nil {
			if err := ctx.Err(); err != nil {
				return written, err
			}
		}
		if conn != nil {
			_ = conn.SetWriteDeadline(time.Now().Add(idle))
		}
		nr, er := src.Read(buf)
		if nr > 0 {
			nw, ew := dst.Write(buf[:nr])
			written += int64(nw)
			if ew != nil {
				clearWriteDeadline(conn)
				return written, ew
			}
			if nw != nr {
				clearWriteDeadline(conn)
				return written, io.ErrShortWrite
			}
		}
		if er != nil {
			clearWriteDeadline(conn)
			if er == io.EOF {
				return written, nil
			}
			return written, er
		}
	}
}

func clearWriteDeadline(conn net.Conn) {
	if conn != nil {
		_ = conn.SetWriteDeadline(time.Time{})
	}
}
