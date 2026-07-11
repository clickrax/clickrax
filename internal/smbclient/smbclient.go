package smbclient

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"time"

	"github.com/hirochachacha/go-smb2"

	"pbs-win-backup/internal/i18n"
	"pbs-win-backup/internal/models"
	"pbs-win-backup/internal/netio"
	"pbs-win-backup/internal/remotepath"
)

func Test(dest models.BackupDestination, password string) error {
	sh, sess, conn, err := dial(context.Background(), dest, password)
	if err != nil {
		return err
	}
	defer conn.Close()
	defer sess.Logoff()
	defer sh.Umount()

	rd := normalizeRemote(dest.RemotePath)
	if rd != "" {
		if _, err := sh.ReadDir(rd); err != nil {
			return i18n.Ewrap("smb.dir_unavailable", map[string]string{"path": rd}, err)
		}
		return nil
	}
	if _, err := sh.ReadDir("."); err != nil {
		return i18n.Ewrap("smb.share_read", nil, err)
	}
	return nil
}

func Upload(ctx context.Context, dest models.BackupDestination, password, localPath, backupID, fileName string, onProgress func(written, total int64)) error {
	info, err := os.Stat(localPath)
	if err != nil {
		return err
	}
	total := info.Size()

	sh, sess, conn, err := dialFunc(ctx, dest, password)
	if err != nil {
		return err
	}
	defer conn.Close()
	if sess != nil {
		defer sess.Logoff()
	}
	if sh != nil {
		defer sh.Umount()
	}

	dir, err := remoteDir(dest, backupID)
	if err != nil {
		return err
	}
	if _, err := remotepath.SafeComponent(fileName); err != nil {
		return err
	}
	if err := mkdirAll(sh, dir); err != nil {
		return err
	}
	remotePath, err := remotepath.JoinDir(dir, fileName)
	if err != nil {
		return err
	}
	partialPath := remotePath + ".partial"

	_ = smbShareRemove(sh, partialPath)

	src, err := os.Open(localPath)
	if err != nil {
		return err
	}
	defer src.Close()

	dst, err := smbShareCreate(sh, partialPath)
	if err != nil {
		return i18n.Ewrap("smb.create", map[string]string{"path": partialPath}, err)
	}

	idle := netio.IdleFromContext(ctx)
	pr := &progressReader{r: netio.Reader(ctx, src), total: total, onProgress: onProgress}
	if _, err := netio.CopyWithWriteDeadline(ctx, dst, pr, conn, idle); err != nil {
		_ = dst.Close()
		_ = smbShareRemove(sh, partialPath)
		return i18n.Ewrap("smb.transfer", nil, err)
	}
	if err := dst.Close(); err != nil {
		_ = smbShareRemove(sh, partialPath)
		return i18n.Ewrap("smb.transfer_close", nil, err)
	}

	if err := commitSMBUpload(sh, partialPath, remotePath, total); err != nil {
		return err
	}
	return nil
}

type smbFileOps interface {
	Stat(path string) (os.FileInfo, error)
	Remove(path string) error
	Rename(oldpath, newpath string) error
}

var commitSMBUpload = func(sh smbFileOps, partialPath, remotePath string, expectedSize int64) error {
	info, err := sh.Stat(partialPath)
	if err != nil {
		_ = sh.Remove(partialPath)
		return i18n.Ewrap("smb.size_check", map[string]string{"path": partialPath}, err)
	}
	if info.Size() != expectedSize {
		_ = sh.Remove(partialPath)
		return i18n.Ef("smb.size_mismatch", map[string]string{
			"path": partialPath, "expected": fmt.Sprintf("%d", expectedSize), "got": fmt.Sprintf("%d", info.Size()),
		})
	}
	if err := sh.Rename(partialPath, remotePath); err == nil {
		return nil
	}
	backupPath := remotePath + ".bak"
	_ = sh.Remove(backupPath)
	if err := sh.Rename(remotePath, backupPath); err != nil {
		return i18n.Ewrap("smb.rename", map[string]string{"path": partialPath}, err)
	}
	if err := sh.Rename(partialPath, remotePath); err != nil {
		_ = sh.Rename(backupPath, remotePath)
		return i18n.Ewrap("smb.rename", map[string]string{"path": partialPath}, err)
	}
	_ = sh.Remove(backupPath)
	return nil
}

// VerifyUploadedSize checks remote archive size matches expected bytes.
func VerifyUploadedSize(ctx context.Context, dest models.BackupDestination, password, backupID, fileName string, expectedSize int64) error {
	if ctx == nil {
		ctx = context.Background()
	}
	sh, sess, conn, err := dial(ctx, dest, password)
	if err != nil {
		return err
	}
	defer conn.Close()
	defer sess.Logoff()
	defer sh.Umount()

	dir, err := remoteDir(dest, backupID)
	if err != nil {
		return err
	}
	remotePath, err := remotepath.JoinDir(dir, fileName)
	if err != nil {
		return err
	}
	info, err := sh.Stat(remotePath)
	if err != nil {
		return i18n.Ewrap("smb.stat", map[string]string{"path": remotePath}, err)
	}
	if info.Size() != expectedSize {
		return i18n.Ef("smb.size_mismatch", map[string]string{
			"path": remotePath, "expected": fmt.Sprintf("%d", expectedSize), "got": fmt.Sprintf("%d", info.Size()),
		})
	}
	return nil
}

// VerifyUploaded checks remote archive size and SHA-256 match the local upload.
func VerifyUploaded(ctx context.Context, dest models.BackupDestination, password, backupID, fileName string, expectedSize int64, expectedSHA256 [32]byte) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := VerifyUploadedSize(ctx, dest, password, backupID, fileName, expectedSize); err != nil {
		return err
	}
	got, err := remoteFileSHA256(ctx, dest, password, backupID, fileName)
	if err != nil {
		return err
	}
	if got != expectedSHA256 {
		return i18n.Ef("smb.checksum_mismatch", map[string]string{"path": fileName})
	}
	return nil
}

func remoteFileSHA256(ctx context.Context, dest models.BackupDestination, password, backupID, fileName string) ([32]byte, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	sh, sess, conn, err := dial(ctx, dest, password)
	if err != nil {
		return [32]byte{}, err
	}
	defer conn.Close()
	defer sess.Logoff()
	defer sh.Umount()

	dir, err := remoteDir(dest, backupID)
	if err != nil {
		return [32]byte{}, err
	}
	idle := netio.IdleFromContext(ctx)
	remotePath, err := remotepath.JoinDir(dir, fileName)
	if err != nil {
		return [32]byte{}, err
	}
	f, err := sh.Open(remotePath)
	if err != nil {
		return [32]byte{}, i18n.Ewrap("smb.stat", map[string]string{"path": remotePath}, err)
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, netio.ReaderWithConn(ctx, f, conn, idle)); err != nil {
		return [32]byte{}, i18n.Ewrap("smb.transfer", nil, err)
	}
	var sum [32]byte
	copy(sum[:], h.Sum(nil))
	return sum, nil
}

// RemoteFileExists reports whether a file exists in the backup directory.
func RemoteFileExists(dest models.BackupDestination, password, backupID, fileName string) bool {
	sh, sess, conn, err := dial(context.Background(), dest, password)
	if err != nil {
		return false
	}
	defer conn.Close()
	defer sess.Logoff()
	defer sh.Umount()

	dir, err := remoteDir(dest, backupID)
	if err != nil {
		return false
	}
	remotePath, err := remotepath.JoinDir(dir, fileName)
	if err != nil {
		return false
	}
	_, err = sh.Stat(remotePath)
	return err == nil
}

// RemoveRemote deletes a file from the backup directory (best effort).
func RemoveRemote(dest models.BackupDestination, password, backupID, fileName string) error {
	sh, sess, conn, err := dial(context.Background(), dest, password)
	if err != nil {
		return err
	}
	defer conn.Close()
	defer sess.Logoff()
	defer sh.Umount()

	dir, err := remoteDir(dest, backupID)
	if err != nil {
		return err
	}
	remotePath, err := remotepath.JoinDir(dir, fileName)
	if err != nil {
		return err
	}
	if err := sh.Remove(remotePath); err != nil {
		return i18n.Ewrap("smb.delete", map[string]string{"path": remotePath}, err)
	}
	return nil
}

func dial(ctx context.Context, dest models.BackupDestination, password string) (*smb2.Share, *smb2.Session, net.Conn, error) {
	host := strings.TrimSpace(dest.Host)
	shareName := strings.Trim(strings.TrimSpace(dest.Share), `\`)
	if host == "" || shareName == "" {
		return nil, nil, nil, i18n.E("smb.host_share_required", nil)
	}
	port := dest.Port
	if port <= 0 {
		port = 445
	}
	if ctx == nil {
		ctx = context.Background()
	}
	var dialer net.Dialer
	dialer.Timeout = 20 * time.Second
	conn, err := dialer.DialContext(ctx, "tcp", fmt.Sprintf("%s:%d", host, port))
	if err != nil {
		return nil, nil, nil, i18n.Ef("smb.connect", map[string]string{
			"host": host, "port": fmt.Sprintf("%d", port), "err": err.Error(),
		})
	}
	conn = wrapConnForDial(ctx, conn)
	d := &smb2.Dialer{
		Initiator: &smb2.NTLMInitiator{
			User:     strings.TrimSpace(dest.Username),
			Password: password,
			Domain:   strings.TrimSpace(dest.Domain),
		},
	}
	sess, err := d.Dial(conn)
	if err != nil {
		conn.Close()
		return nil, nil, nil, i18n.Ewrap("smb.auth", nil, err)
	}
	sh, err := sess.Mount(shareName)
	if err != nil {
		sess.Logoff()
		conn.Close()
		return nil, nil, nil, fmt.Errorf("SMB: share %q: %w", shareName, err)
	}
	return sh, sess, conn, nil
}

func normalizeRemote(p string) string {
	p = strings.TrimSpace(p)
	p = strings.Trim(p, `/\`)
	return strings.ReplaceAll(p, `\`, `/`)
}

func remoteDir(dest models.BackupDestination, backupID string) (string, error) {
	return remotepath.RemoteDir(normalizeRemote(dest.RemotePath), backupID)
}

var mkdirAll = func(sh *smb2.Share, dir string) error {
	if dir == "" || dir == "." {
		return nil
	}
	cur := ""
	for _, p := range strings.Split(dir, "/") {
		if p == "" {
			continue
		}
		if cur == "" {
			cur = p
		} else {
			cur = cur + "/" + p
		}
		_ = sh.Mkdir(cur, 0o755)
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
	dialFunc        = dial
	wrapConnForDial = func(ctx context.Context, conn net.Conn) net.Conn {
		return netio.WrapConn(ctx, conn, netio.IdleFromContext(ctx))
	}
	smbShareCreate  = func(sh *smb2.Share, path string) (io.WriteCloser, error) { return sh.Create(path) }
	smbShareRemove  = func(sh *smb2.Share, path string) error { return sh.Remove(path) }
)
