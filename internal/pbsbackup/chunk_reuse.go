package pbsbackup

import (
	pbscommon "pbscommon"
)

// shouldUploadChunk decides whether chunk bytes must be uploaded to PBS.
// Known digests (previous index / chunks.json) are trusted — backup-protocol
// GET /chunk probes return false 404s and must not force a full re-upload.
func shouldUploadChunk(inKnown, missingOnServer bool) (upload bool) {
	if !inKnown {
		return true
	}
	_ = missingOnServer
	return false
}

// chunkUploadNeeded reports whether this digest must be uploaded.
// Digests present in the previous PBS index (or local chunks.json) are reused
// without probing GET /chunk on the backup session (regression in 2.3.14;
// restored trust model from 2.3.11).
func (c *chunkState) chunkUploadNeeded(digestHex string, inKnown bool, client *pbscommon.PBSClient, hasChunkData bool) (upload bool, err error) {
	_ = digestHex
	_ = client
	_ = hasChunkData
	if inKnown {
		return false, nil
	}
	return true, nil
}
