package pbsbackup

import (
	"pbs-win-backup/internal/i18n"

	pbscommon "pbscommon"
)

// shouldUploadChunk decides whether chunk bytes must be uploaded to PBS.
// When a digest is in the known set but the blob was garbage-collected server-side,
// we must upload again instead of silently reusing a missing chunk.
func shouldUploadChunk(inKnown, missingOnServer bool) (upload bool) {
	if !inKnown {
		return true
	}
	return missingOnServer
}

func (c *chunkState) chunkUploadNeeded(digestHex string, inKnown bool, client *pbscommon.PBSClient, hasChunkData bool) (upload bool, err error) {
	if !inKnown {
		return true, nil
	}
	exists, err := c.chunkExist.exists(client, digestHex)
	if err != nil {
		return false, err
	}
	if !shouldUploadChunk(inKnown, !exists) {
		return false, nil
	}
	if !hasChunkData {
		return false, i18n.Ef("pbs.chunk_missing_on_server", map[string]string{
			"digest": digestHex[:min(12, len(digestHex))],
		})
	}
	return true, nil
}
