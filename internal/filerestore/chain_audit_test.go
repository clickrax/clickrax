package filerestore

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"pbs-win-backup/internal/fileindex"
	"pbs-win-backup/internal/filemeta"
	"pbs-win-backup/internal/models"
)

var emptyZipBytes = []byte{
	0x50, 0x4b, 0x05, 0x06, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
}

func TestBuildSnapshotView_MissingIntermediateManifest_Errors(t *testing.T) {
	target := "host_20260708-120000_incr.zip"
	base := "host_20260707-120000.zip"
	incr1 := "host_20260707-180000_incr.zip"

	origList := snapshotListArchives
	origLoad := snapshotLoadManifest
	origMeta := snapshotLoadArchiveMeta
	origOpen := snapshotOpenRemoteZip
	t.Cleanup(func() {
		snapshotListArchives = origList
		snapshotLoadManifest = origLoad
		snapshotLoadArchiveMeta = origMeta
		snapshotOpenRemoteZip = origOpen
	})

	snapshotListArchives = func(models.BackupDestination, string, string) ([]archiveRef, error) {
		return []archiveRef{{
			FileName: target,
			Time:     time.Date(2026, 7, 8, 12, 0, 0, 0, time.UTC),
		}}, nil
	}

	snapshotLoadManifest = func(_ context.Context, _ models.BackupDestination, _ string, _ string, name string) (*fileindex.Manifest, error) {
		switch name {
		case target:
			return &fileindex.Manifest{
				Kind:     fileindex.KindIncremental,
				Chain:    []string{base, incr1, target},
				Deleted:  nil,
				BaseFull: base,
			}, nil
		case base:
			return &fileindex.Manifest{Kind: fileindex.KindFull, Chain: []string{base}}, nil
		case incr1:
			return nil, nil
		default:
			return nil, nil
		}
	}

	snapshotLoadArchiveMeta = func(context.Context, models.BackupDestination, string, string, string) (filemeta.Archive, error) {
		return filemeta.NewArchive(), nil
	}

	snapshotOpenRemoteZip = func(_ context.Context, _ models.BackupDestination, _ string, _ string, _ string) (*remoteZip, error) {
		data := append([]byte(nil), emptyZipBytes...)
		return &remoteZip{
			reader: bytes.NewReader(data),
			size:   int64(len(data)),
			close:  func() error { return nil },
		}, nil
	}

	_, err := buildSnapshotView(context.Background(), models.BackupDestination{Type: models.DestFTP}, "", models.BackupJob{BackupID: "host"}, "latest")
	if err == nil {
		t.Fatal("expected error for missing intermediate manifest")
	}
	if !strings.Contains(err.Error(), incr1) {
		t.Fatalf("error should mention missing archive %q: %v", incr1, err)
	}
}
