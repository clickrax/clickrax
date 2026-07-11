package pbsbackup

import "testing"

func TestShouldUploadChunkKnownButMissingOnServer(t *testing.T) {
	if !shouldUploadChunk(true, true) {
		t.Fatal("known hash absent on PBS must trigger upload")
	}
}

func TestShouldUploadChunkKnownAndPresent(t *testing.T) {
	if shouldUploadChunk(true, false) {
		t.Fatal("present known chunk should be reused")
	}
}

func TestShouldUploadChunkNewHash(t *testing.T) {
	if !shouldUploadChunk(false, false) {
		t.Fatal("unknown hash must be uploaded")
	}
}
