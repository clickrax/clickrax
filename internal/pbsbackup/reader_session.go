package pbsbackup

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"time"

	"pbs-win-backup/internal/i18n"
	"pbs-win-backup/internal/models"
	"pbscommon"

	"golang.org/x/net/http2"
)

const chunkDownloadTimeout = 3 * time.Minute

func connectReader(server models.PBSServer, secret string, ref SnapshotRef) (*pbscommon.PBSClient, error) {
	if ref.BackupID == "" {
		return nil, i18n.E("pbs.didx.backup_id_missing", nil)
	}
	unix := ref.BackupTime
	if unix == 0 {
		var err error
		unix, err = snapshotToUnix(ref.Time)
		if err != nil {
			return nil, err
		}
	}
	client := newPBSClient(server, secret, ref.BackupID)
	client.Manifest.BackupTime = unix
	client.Connect(true, "host")
	client.Client.Timeout = pbsReaderTimeout
	return client, nil
}

func closeReader(client *pbscommon.PBSClient) {
	if client == nil {
		return
	}
	if tr, ok := client.Client.Transport.(*http2.Transport); ok {
		tr.CloseIdleConnections()
	}
}

// readerReassemble opens one PBS reader session, downloads a didx index and all its chunks.
// PBS requires chunk downloads on the same reader session as the index.
func readerReassemble(
	ctx context.Context,
	server models.PBSServer,
	secret string,
	ref SnapshotRef,
	archiveName string,
	onProgress StreamProgress,
	earlyFn func(buf []byte) ([]byte, bool),
) ([]byte, []byte, error) {
	var extracted []byte
	stopFn := streamStopFn(nil)
	if earlyFn != nil {
		stopFn = func(view []byte) (bool, error) {
			payload, ok := earlyFn(view)
			if ok {
				extracted = payload
				return true, nil
			}
			return false, nil
		}
	}
	data, earlyStop, err := readerReassembleStream(ctx, server, secret, ref, archiveName, onProgress, stopFn)
	if err != nil {
		return nil, nil, err
	}
	if extracted != nil {
		return nil, extracted, nil
	}
	if earlyStop {
		return nil, nil, i18n.E("pbs.early_stop_no_data", nil)
	}
	return data, nil, nil
}

// readerDownloadIndex fetches only the index/didx header (no chunk assembly).
func readerDownloadIndex(server models.PBSServer, secret string, ref SnapshotRef, archiveName string) ([]byte, error) {
	client, err := connectReader(server, secret, ref)
	if err != nil {
		return nil, err
	}
	defer closeReader(client)
	return client.DownloadToBytes(archiveName)
}

func getChunkVerified(ctx context.Context, client *pbscommon.PBSClient, digest string, timeout time.Duration) ([]byte, error) {
	if err := abortIfCancelled(ctx); err != nil {
		return nil, err
	}
	data, err := getChunkWithTimeout(ctx, client, digest, timeout)
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(data)
	if hex.EncodeToString(sum[:]) != digest {
		return nil, i18n.Ef("pbs.chunk_sha_mismatch", map[string]string{
			"digest": digest[:min(12, len(digest))],
		})
	}
	return data, nil
}

func getChunkWithTimeout(ctx context.Context, client *pbscommon.PBSClient, digest string, timeout time.Duration) ([]byte, error) {
	if err := abortIfCancelled(ctx); err != nil {
		return nil, err
	}
	if ctx == nil {
		ctx = context.Background()
	}
	chunkCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	data, err := client.GetChunkDataWithContext(chunkCtx, digest)
	if err == nil {
		return data, nil
	}
	if errors.Is(err, context.DeadlineExceeded) && ctx.Err() == nil {
		label := digest
		if len(label) > 12 {
			label = label[:12]
		}
		return nil, i18n.Ef("pbs.chunk_download_timeout", map[string]string{
			"digest": label, "timeout": timeout.String(),
		})
	}
	return nil, err
}
