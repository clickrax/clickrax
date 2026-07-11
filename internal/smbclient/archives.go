package smbclient

import (
	"context"
	"io"
	"strings"

	"pbs-win-backup/internal/i18n"
	"pbs-win-backup/internal/models"
	"pbs-win-backup/internal/netio"
	"pbs-win-backup/internal/remotepath"
)

// ArchiveEntry is a remote backup archive on SMB.
type ArchiveEntry struct {
	Name    string
	Size    int64
	ModTime int64 // unix seconds, 0 if unknown
}

// ListArchives returns .zip files in the destination backup directory.
func ListArchives(dest models.BackupDestination, password, backupID string) ([]ArchiveEntry, error) {
	sh, sess, conn, err := dial(context.Background(), dest, password)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	defer sess.Logoff()
	defer sh.Umount()

	dir, err := remoteDir(dest, backupID)
	if err != nil {
		return nil, err
	}
	entries, err := sh.ReadDir(dir)
	if err != nil {
		return nil, i18n.Ewrap("smb.dir", map[string]string{"path": dir}, err)
	}
	out := make([]ArchiveEntry, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(strings.ToLower(e.Name()), ".zip") {
			continue
		}
		out = append(out, ArchiveEntry{
			Name:    e.Name(),
			Size:    e.Size(),
			ModTime: e.ModTime().Unix(),
		})
	}
	return out, nil
}

// OpenRemoteFile opens a remote file for random access (zip catalog / selective extract).
func OpenRemoteFile(ctx context.Context, dest models.BackupDestination, password, backupID, fileName string) (io.ReaderAt, int64, func() error, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	sh, sess, conn, err := dial(ctx, dest, password)
	if err != nil {
		return nil, 0, nil, err
	}
	dir, err := remoteDir(dest, backupID)
	if err != nil {
		conn.Close()
		sess.Logoff()
		return nil, 0, nil, err
	}
	remotePath, err := remotepath.JoinDir(dir, fileName)
	if err != nil {
		conn.Close()
		sess.Logoff()
		return nil, 0, nil, err
	}
	f, err := sh.Open(remotePath)
	if err != nil {
		conn.Close()
		sess.Logoff()
		return nil, 0, nil, i18n.Ewrap("smb.open", map[string]string{"path": remotePath}, err)
	}
	info, err := f.Stat()
	if err != nil {
		f.Close()
		conn.Close()
		sess.Logoff()
		return nil, 0, nil, err
	}
	closeFn := func() error {
		err1 := f.Close()
		err2 := sess.Logoff()
		err3 := conn.Close()
		if err1 != nil {
			return err1
		}
		if err2 != nil {
			return err2
		}
		return err3
	}
	ra := &netio.ReaderAtConn{
		ReaderAt: f,
		Conn:     conn,
		Ctx:      ctx,
		Idle:     netio.IdleFromContext(ctx),
	}
	return ra, info.Size(), closeFn, nil
}
