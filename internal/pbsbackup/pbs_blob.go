package pbsbackup

import (
	pbscommon "pbscommon"

	"pbs-win-backup/internal/eventlog"
	"pbs-win-backup/internal/i18n"
)

const (
	pbsBlobHeaderBytes     = 12 // 8-byte magic + 4-byte CRC32
	pbsMaxEncodedBlobBytes = 16777260
	pbsMaxBlobPayloadBytes = pbsMaxEncodedBlobBytes - pbsBlobHeaderBytes
)

func pbsEncodedBlobSize(payloadLen int) int {
	return pbsBlobHeaderBytes + payloadLen
}

func blobFitsPBSLimit(payloadLen int) bool {
	return payloadLen >= 0 && pbsEncodedBlobSize(payloadLen) <= pbsMaxEncodedBlobBytes
}

func uploadBlobToPBS(client *pbscommon.PBSClient, stats *Stats, blobName string, data []byte) error {
	if len(data) == 0 {
		return nil
	}
	if !blobFitsPBSLimit(len(data)) {
		msg := i18n.L("pbs.blob_skipped_limit", map[string]string{
			"name":  blobName,
			"size":  formatByteSize(int64(len(data))),
			"limit": formatByteSize(int64(pbsMaxBlobPayloadBytes)),
		})
		eventlog.Warning(msg)
		if stats != nil {
			stats.addWarning(msg)
		}
		return nil
	}
	return client.UploadBlob(blobName, data)
}
