package pbsbackup

import "testing"

func TestShouldUploadChunkKnownIsTrusted(t *testing.T) {
	// Even if a probe would claim "missing", known digests are not re-uploaded.
	if shouldUploadChunk(true, true) {
		t.Fatal("known digest must be trusted without upload")
	}
	if shouldUploadChunk(true, false) {
		t.Fatal("known present digest must be reused")
	}
}

func TestShouldUploadChunkNewHash(t *testing.T) {
	if !shouldUploadChunk(false, false) {
		t.Fatal("unknown hash must be uploaded")
	}
	if !shouldUploadChunk(false, true) {
		t.Fatal("unknown hash must be uploaded")
	}
}

func TestChunkUploadNeeded_TrustsKnownWithoutProbe(t *testing.T) {
	var c chunkState
	c.chunkExist = newChunkExistCache()
	upload, err := c.chunkUploadNeeded("deadbeef", true, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	if upload {
		t.Fatal("known digest without chunk bytes must reuse (no GET /chunk)")
	}
	upload, err = c.chunkUploadNeeded("cafebabe", false, nil, true)
	if err != nil {
		t.Fatal(err)
	}
	if !upload {
		t.Fatal("unknown digest must upload")
	}
}
