package pbsbackup

import (
	"os"
)

// pxarSpillThreshold is the in-memory high-water mark before spilling to a temp file.
const pxarSpillThreshold = 64 << 20 // 64 MiB

// pxarSpillBuffer accumulates pxar bytes with optional spill to disk to avoid huge Go heap slices.
type pxarSpillBuffer struct {
	mem  []byte
	file *os.File
	size int64
}

func newPxarSpillBuffer() *pxarSpillBuffer {
	return &pxarSpillBuffer{}
}

func (b *pxarSpillBuffer) Close() error {
	if b.file == nil {
		return nil
	}
	name := b.file.Name()
	_ = b.file.Close()
	b.file = nil
	return os.Remove(name)
}

func (b *pxarSpillBuffer) append(chunk []byte) error {
	if len(chunk) == 0 {
		return nil
	}
	if b.file == nil {
		b.mem = append(b.mem, chunk...)
		if len(b.mem) > pxarSpillThreshold {
			return b.spillLocked()
		}
		return nil
	}
	n, err := b.file.Write(chunk)
	if err != nil {
		return err
	}
	b.size += int64(n)
	return nil
}

func (b *pxarSpillBuffer) spillLocked() error {
	tmp, err := os.CreateTemp("", "clickrax-pxar-*")
	if err != nil {
		return err
	}
	if _, err := tmp.Write(b.mem); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmp.Name())
		return err
	}
	b.file = tmp
	b.size = int64(len(b.mem))
	b.mem = nil
	return nil
}

// persistTo writes the buffer to destPath and closes any temp file handle.
func (b *pxarSpillBuffer) persistTo(destPath string) error {
	if b.file != nil {
		if err := b.file.Sync(); err != nil {
			return err
		}
		src := b.file.Name()
		if err := b.file.Close(); err != nil {
			return err
		}
		b.file = nil
		_ = os.Remove(destPath)
		return os.Rename(src, destPath)
	}
	return os.WriteFile(destPath, b.mem, 0o644)
}

func (b *pxarSpillBuffer) withView(fn func([]byte) error) error {
	if b.file == nil {
		return fn(b.mem)
	}
	if err := b.file.Sync(); err != nil {
		return err
	}
	return withMmapView(b.file, b.size, fn)
}

func (b *pxarSpillBuffer) bytes() ([]byte, error) {
	if b.file == nil {
		out := make([]byte, len(b.mem))
		copy(out, b.mem)
		return out, nil
	}
	var out []byte
	err := withMmapView(b.file, b.size, func(view []byte) error {
		out = make([]byte, len(view))
		copy(out, view)
		return nil
	})
	return out, err
}
