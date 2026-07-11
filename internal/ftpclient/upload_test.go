package ftpclient

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jlaffaye/ftp"

	"pbs-win-backup/internal/models"
)

func TestFTPUpload_StalledStor_Aborts(t *testing.T) {
	oldConnect := connectFn
	oldChangeDir := ftpChangeDir
	oldDelete := ftpDelete
	oldStor := ftpStor
	oldFileSize := ftpFileSize
	oldRename := ftpRename
	oldMkdir := mkdirAllFn
	t.Cleanup(func() {
		connectFn = oldConnect
		ftpChangeDir = oldChangeDir
		ftpDelete = oldDelete
		ftpStor = oldStor
		ftpFileSize = oldFileSize
		ftpRename = oldRename
		mkdirAllFn = oldMkdir
	})

	connectFn = func(context.Context, models.BackupDestination, string) (*ftp.ServerConn, error) {
		return nil, nil
	}
	mkdirAllFn = func(*ftp.ServerConn, string) error { return nil }
	ftpChangeDir = func(*ftp.ServerConn, string) error { return nil }
	ftpDelete = func(*ftp.ServerConn, string) error { return nil }
	ftpFileSize = func(*ftp.ServerConn, string) (int64, error) { return 7, nil }
	ftpRename = func(*ftp.ServerConn, string, string) error { return nil }
	ftpStor = func(_ *ftp.ServerConn, _ string, r io.Reader) error {
		buf := make([]byte, 4096)
		for {
			if _, err := r.Read(buf); err != nil {
				return err
			}
		}
	}

	local := filepath.Join(t.TempDir(), "upload.zip")
	if err := os.WriteFile(local, []byte("payload"), 0o644); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	start := time.Now()
	err := Upload(ctx, models.BackupDestination{Type: models.DestFTP, Host: "127.0.0.1"}, "pw", local, "backup", "file.zip", nil)
	elapsed := time.Since(start)
	if err == nil {
		t.Fatal("expected stalled FTP upload to fail")
	}
	if elapsed > 2*time.Second {
		t.Fatalf("upload blocked too long: %s", elapsed)
	}
}
