package pbsbackup

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"

	pbscommon "pbscommon"
)

func TestFlushPendingChunkBeforeReuse(t *testing.T) {
	var client pbscommon.PBSClient
	c := chunkState{}
	stats := &Stats{}
	known := newKnownChunks(0)
	c.init(nil, stats, known, nil)

	c.currentChunk = []byte("partial-tail-bytes")
	if err := c.flushPendingChunk(&client); err != nil {
		t.Fatalf("flush pending: %v", err)
	}
	if len(c.currentChunk) != 0 {
		t.Fatal("flush should clear pending chunk")
	}
	if c.pos == 0 {
		t.Fatal("flush should advance stream position")
	}
}

func TestPxarStreamBytesTracksReusedChunks(t *testing.T) {
	var pxarStreamBytes uint64
	header := []byte{1, 2, 3}
	chunks := []pbscommon.PXARFastChunk{{Len: 100}, {Len: 50}}
	pxarStreamBytes += uint64(len(header))
	for _, ch := range chunks {
		if ch.Len > 0 {
			pxarStreamBytes += uint64(ch.Len)
		}
	}
	want := uint64(len(header) + 150)
	if pxarStreamBytes != want {
		t.Fatalf("pxarStreamBytes=%d want %d", pxarStreamBytes, want)
	}
}

func TestReuseChunksCommitsPendingFirst(t *testing.T) {
	var client pbscommon.PBSClient
	c := chunkState{}
	stats := &Stats{}
	known := newKnownChunks(0)
	c.init(nil, stats, known, nil)

	pending := []byte("abc")
	c.currentChunk = append([]byte(nil), pending...)
	digest := sha256.Sum256(pending)
	known.Add(digest)
	c.knownChunks = known

	reuse := []pbscommon.PXARFastChunk{{
		DigestHex: hex.EncodeToString(digest[:]),
		Len:       len(pending),
	}}
	if err := c.reuseChunks(reuse, &client); err != nil {
		t.Fatalf("reuse: %v", err)
	}
	if c.pos != uint64(len(pending)*2) {
		t.Fatalf("pos=%d want %d", c.pos, len(pending)*2)
	}
}
