package ftpclient

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"time"

	"github.com/jlaffaye/ftp"

	"pbs-win-backup/internal/i18n"
	"pbs-win-backup/internal/models"
	"pbs-win-backup/internal/netio"
	"pbs-win-backup/internal/remotepath"
)

func Test(dest models.BackupDestination, password string) error {
	c, err := connect(context.Background(), dest, password)
	if err != nil {
		return err
	}
	defer c.Quit()

	rd := normalizeRemote(dest.RemotePath)
	if rd != "" {
		if err := c.ChangeDir(rd); err != nil {
			return i18n.Ewrap("ftp.dir_unavailable", map[string]string{"path": rd}, err)
		}
	}
	if _, err := c.List("."); err != nil {
		return i18n.Ewrap("ftp.dir_read", nil, err)
	}
	return nil
}

func Upload(ctx context.Context, dest models.BackupDestination, password, localPath, backupID, fileName string, onProgress func(written, total int64)) error {
	info, err := os.Stat(localPath)
	if err != nil {
		return err
	}
	total := info.Size()

	c, err := connectFn(ctx, dest, password)
	if err != nil {
		return err
	}
	defer ftpConnQuit(c)

	dir, err := remoteDir(dest, backupID)
	if err != nil {
		return err
	}
	if _, err := remotepath.SafeComponent(fileName); err != nil {
		return err
	}
	if err := mkdirAll(c, dir); err != nil {
		return err
	}
	if err := ftpChangeDir(c, dir); err != nil {
		return i18n.Ewrap("ftp.dir", map[string]string{"path": dir}, err)
	}

	partialName := fileName + ".partial"
	_ = ftpDelete(c, partialName)

	src, err := os.Open(localPath)
	if err != nil {
		return err
	}
	defer src.Close()

	pr := &progressReader{r: netio.Reader(ctx, src), total: total, onProgress: onProgress}
	if err := ftpStor(c, partialName, pr); err != nil {
		_ = ftpDelete(c, partialName)
		return i18n.Ewrap("ftp.upload", map[string]string{"path": partialName}, err)
	}

	got, err := ftpFileSize(c, partialName)
	if err != nil {
		_ = ftpDelete(c, partialName)
		return i18n.Ewrap("ftp.size_check", map[string]string{"path": partialName}, err)
	}
	if int64(got) != total {
		_ = ftpDelete(c, partialName)
		return i18n.Ef("ftp.size_mismatch", map[string]string{
			"path": partialName, "expected": fmt.Sprintf("%d", total), "got": fmt.Sprintf("%d", got),
		})
	}

	if err := ftpRename(c, partialName, fileName); err != nil {
		return i18n.Ef("ftp.rename", map[string]string{
			"from": partialName, "to": fileName, "err": err.Error(),
		})
	}
	return nil
}

// VerifyUploadedSize checks remote archive size matches expected bytes.
func VerifyUploadedSize(ctx context.Context, dest models.BackupDestination, password, backupID, fileName string, expectedSize int64) error {
	c, err := connect(ctx, dest, password)
	if err != nil {
		return err
	}
	defer c.Quit()

	dir, err := remoteDir(dest, backupID)
	if err != nil {
		return err
	}
	if err := c.ChangeDir(dir); err != nil {
		return i18n.Ewrap("ftp.dir", map[string]string{"path": dir}, err)
	}
	if _, err := remotepath.SafeComponent(fileName); err != nil {
		return err
	}
	size, err := c.FileSize(fileName)
	if err != nil {
		return i18n.Ewrap("ftp.verify", map[string]string{"path": fileName}, err)
	}
	if int64(size) != expectedSize {
		return i18n.Ef("ftp.size_server_mismatch", map[string]string{
			"path": fileName, "expected": fmt.Sprintf("%d", expectedSize), "got": fmt.Sprintf("%d", size),
		})
	}
	return nil
}

// VerifyUploaded checks remote archive size and SHA-256 match the local upload.
func VerifyUploaded(ctx context.Context, dest models.BackupDestination, password, backupID, fileName string, expectedSize int64, expectedSHA256 [32]byte) error {
	if err := VerifyUploadedSize(ctx, dest, password, backupID, fileName, expectedSize); err != nil {
		return err
	}
	got, err := remoteFileSHA256(ctx, dest, password, backupID, fileName)
	if err != nil {
		return err
	}
	if got != expectedSHA256 {
		return i18n.Ef("ftp.checksum_mismatch", map[string]string{"path": fileName})
	}
	return nil
}

func remoteFileSHA256(ctx context.Context, dest models.BackupDestination, password, backupID, fileName string) ([32]byte, error) {
	c, err := connect(ctx, dest, password)
	if err != nil {
		return [32]byte{}, err
	}
	defer c.Quit()

	dir, err := remoteDir(dest, backupID)
	if err != nil {
		return [32]byte{}, err
	}
	remotePath, err := remotepath.JoinDir(dir, fileName)
	if err != nil {
		return [32]byte{}, err
	}
	resp, err := c.Retr(remotePath)
	if err != nil {
		return [32]byte{}, i18n.Ewrap("ftp.verify", map[string]string{"path": fileName}, err)
	}
	defer resp.Close()

	h := sha256.New()
	if _, err := io.Copy(h, netio.Reader(ctx, resp)); err != nil {
		return [32]byte{}, i18n.Ewrap("ftp.verify", map[string]string{"path": fileName}, err)
	}
	var sum [32]byte
	copy(sum[:], h.Sum(nil))
	return sum, nil
}

// RemoteFileExists reports whether a file exists in the backup directory.
func RemoteFileExists(dest models.BackupDestination, password, backupID, fileName string) bool {
	c, err := connect(context.Background(), dest, password)
	if err != nil {
		return false
	}
	defer c.Quit()

	dir, err := remoteDir(dest, backupID)
	if err != nil {
		return false
	}
	remotePath, err := remotepath.JoinDir(dir, fileName)
	if err != nil {
		return false
	}
	_, err = c.FileSize(remotePath)
	return err == nil
}

// RemoveRemote deletes a file from the backup directory (best effort).
func RemoveRemote(dest models.BackupDestination, password, backupID, fileName string) error {
	c, err := connect(context.Background(), dest, password)
	if err != nil {
		return err
	}
	defer c.Quit()

	dir, err := remoteDir(dest, backupID)
	if err != nil {
		return err
	}
	if err := c.ChangeDir(dir); err != nil {
		return i18n.Ewrap("ftp.dir", map[string]string{"path": dir}, err)
	}
	if _, err := remotepath.SafeComponent(fileName); err != nil {
		return err
	}
	if err := c.Delete(fileName); err != nil {
		return i18n.Ewrap("ftp.delete", map[string]string{"path": fileName}, err)
	}
	return nil
}

func connect(ctx context.Context, dest models.BackupDestination, password string) (*ftp.ServerConn, error) {
	host := strings.TrimSpace(dest.Host)
	if host == "" {
		return nil, i18n.E("ftp.host_required", nil)
	}
	port := dest.Port
	if port <= 0 {
		if dest.TLS {
			port = 990
		} else {
			port = 21
		}
	}
	addr := fmt.Sprintf("%s:%d", host, port)
	idle := netio.IdleFromContext(ctx)
	opts := []ftp.DialOption{
		ftp.DialWithTimeout(30 * time.Second),
		ftp.DialWithShutTimeout(idle),
		ftp.DialWithDialFunc(func(network, address string) (net.Conn, error) {
			d := net.Dialer{Timeout: 30 * time.Second}
			conn, err := d.Dial(network, address)
			if err != nil {
				return nil, err
			}
			return netio.WrapConn(ctx, conn, idle), nil
		}),
	}
	if ctx != nil {
		opts = append(opts, ftp.DialWithContext(ctx))
	}
	if dest.TLS {
		tlsCfg := &tls.Config{
			MinVersion: tls.VersionTLS12,
			ServerName: host,
		}
		if port == 990 {
			opts = append(opts, ftp.DialWithTLS(tlsCfg))
		} else {
			opts = append(opts, ftp.DialWithExplicitTLS(tlsCfg))
		}
	}
	if !dest.Passive {
		opts = append(opts, ftp.DialWithDisabledEPSV(true))
	}
	c, err := ftp.Dial(addr, opts...)
	if err != nil {
		return nil, i18n.Ewrap("ftp.connect", nil, err)
	}
	user := strings.TrimSpace(dest.Username)
	if user == "" {
		user = "anonymous"
	}
	if err := c.Login(user, password); err != nil {
		c.Quit()
		return nil, i18n.Ewrap("ftp.auth", nil, err)
	}
	return c, nil
}

func normalizeRemote(p string) string {
	p = strings.TrimSpace(p)
	p = strings.Trim(p, `/\`)
	return strings.ReplaceAll(p, `\`, "/")
}

func remoteDir(dest models.BackupDestination, backupID string) (string, error) {
	return remotepath.RemoteDir(normalizeRemote(dest.RemotePath), backupID)
}

func mkdirAll(c *ftp.ServerConn, dir string) error {
	return mkdirAllFn(c, dir)
}

var mkdirAllFn = func(c *ftp.ServerConn, dir string) error {
	dir = normalizeRemote(dir)
	if dir == "" || dir == "." {
		return nil
	}
	parts := strings.Split(dir, "/")
	cur := ""
	for _, p := range parts {
		if p == "" {
			continue
		}
		if cur == "" {
			cur = p
		} else {
			cur = cur + "/" + p
		}
		_ = c.MakeDir(cur)
	}
	return nil
}

type progressReader struct {
	r          io.Reader
	total      int64
	written    int64
	onProgress func(written, total int64)
}

func (p *progressReader) Read(b []byte) (int, error) {
	n, err := p.r.Read(b)
	if n > 0 {
		p.written += int64(n)
		if p.onProgress != nil {
			p.onProgress(p.written, p.total)
		}
	}
	return n, err
}

var (
	connectFn    = connect
	ftpConnQuit  = func(c *ftp.ServerConn) error {
		if c == nil {
			return nil
		}
		return c.Quit()
	}
	ftpChangeDir = func(c *ftp.ServerConn, dir string) error { return c.ChangeDir(dir) }
	ftpDelete    = func(c *ftp.ServerConn, path string) error { return c.Delete(path) }
	ftpStor      = func(c *ftp.ServerConn, path string, r io.Reader) error { return c.Stor(path, r) }
	ftpFileSize  = func(c *ftp.ServerConn, path string) (int64, error) { return c.FileSize(path) }
	ftpRename    = func(c *ftp.ServerConn, from, to string) error { return c.Rename(from, to) }
)
