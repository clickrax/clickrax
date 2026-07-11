package pbsbackup

// shouldUploadChunk decides whether chunk bytes must be uploaded to PBS.
// When a digest is in the known set but the blob was garbage-collected server-side,
// we must upload again instead of silently reusing a missing chunk.
func shouldUploadChunk(inKnown, missingOnServer bool) (upload bool) {
	if !inKnown {
		return true
	}
	return missingOnServer
}
