package ftpclient

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/jlaffaye/ftp"

	"pbs-win-backup/internal/i18n"
	"pbs-win-backup/internal/models"
	"pbs-win-backup/internal/netio"
	"pbs-win-backup/internal/remotepath"
)

// ArchiveEntry is a remote backup archive on FTP.
type ArchiveEntry struct {
	Name    string
	Size    int64
	ModTime int64
}

// ListArchives returns .zip files in the destination backup directory.
func ListArchives(dest models.BackupDestination, password, backupID string) ([]ArchiveEntry, error) {
	c, err := connect(context.Background(), dest, password)
	if err != nil {
		return nil, err
	}
	defer c.Quit()

	dir, err := remoteDir(dest, backupID)
	if err != nil {
		return nil, err
	}
	if err := c.ChangeDir(dir); err != nil {
		return nil, i18n.Ewrap("ftp.dir", map[string]string{"path": dir}, err)
	}
	entries, err := c.List(".")
	if err != nil {
		return nil, i18n.Ewrap("ftp.list", map[string]string{"path": dir}, err)
	}
	out := make([]ArchiveEntry, 0, len(entries))
	for _, e := range entries {
		if e.Type != ftp.EntryTypeFile || !strings.HasSuffix(strings.ToLower(e.Name), ".zip") {
			continue
		}
		mod := int64(0)
		if !e.Time.IsZero() {
			mod = e.Time.UTC().Unix()
		}
		out = append(out, ArchiveEntry{
			Name:    e.Name,
			Size:    int64(e.Size),
			ModTime: mod,
		})
	}
	return out, nil
}

type ftpProbeConn interface {
	FileSize(string) (int64, error)
	Quit() error
}

var connectForProbe = func(ctx context.Context, dest models.BackupDestination, password string) (ftpProbeConn, error) {
	return connect(ctx, dest, password)
}

// OpenRemoteFile opens a remote file for random access via FTP REST.
func OpenRemoteFile(ctx context.Context, dest models.BackupDestination, password, backupID, fileName string) (io.ReaderAt, int64, func() error, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	c, err := connectForProbe(ctx, dest, password)
	if err != nil {
		return nil, 0, nil, err
	}
	dir, err := remoteDir(dest, backupID)
	if err != nil {
		c.Quit()
		return nil, 0, nil, err
	}
	remotePath, err := remotepath.JoinDir(dir, fileName)
	if err != nil {
		c.Quit()
		return nil, 0, nil, err
	}
	size, err := c.FileSize(remotePath)
	if err != nil {
		c.Quit()
		return nil, 0, nil, i18n.Ewrap("ftp.size", map[string]string{"path": remotePath}, err)
	}
	if err := c.Quit(); err != nil {
		return nil, 0, nil, err
	}
	reader := &ftpReaderAt{
		ctx:      ctx,
		dest:     dest,
		password: password,
		path:     remotePath,
		size:     int64(size),
	}
	return reader, int64(size), func() error { return nil }, nil
}

func ftpReadAtFrom(resp io.Reader, p []byte, off, fileSize int64) (int, error) {
	want := int64(len(p))
	if off+want > fileSize {
		want = fileSize - off
	}
	rc, ok := resp.(io.Closer)
	n, readErr := io.ReadFull(io.LimitReader(resp, want), p[:want])
	var closeErr error
	if ok {
		closeErr = rc.Close()
	}
	if readErr == io.EOF && n == 0 {
		if closeErr != nil {
			return 0, closeErr
		}
		return 0, io.EOF
	}
	if readErr != nil {
		return n, readErr
	}
	if closeErr != nil {
		return n, closeErr
	}
	return n, nil
}

type ftpReaderAt struct {
	ctx      context.Context
	dest     models.BackupDestination
	password string
	path     string
	size     int64
}

func (r *ftpReaderAt) ReadAt(p []byte, off int64) (int, error) {
	if r.ctx != nil {
		if err := r.ctx.Err(); err != nil {
			return 0, err
		}
	}
	if off < 0 {
		return 0, i18n.E("ftp.negative_offset", nil)
	}
	if off >= r.size {
		return 0, io.EOF
	}
	c, err := connect(r.ctx, r.dest, r.password)
	if err != nil {
		return 0, err
	}
	defer c.Quit()

	resp, err := c.RetrFrom(r.path, uint64(off))
	if err != nil {
		return 0, i18n.Ef("ftp.read_at", map[string]string{
			"path": r.path, "offset": fmt.Sprintf("%d", off), "err": err.Error(),
		})
	}
	_ = resp.SetDeadline(time.Now().Add(netio.IdleFromContext(r.ctx)))
	return ftpReadAtFrom(resp, p, off, r.size)
}
