package pbsbackup

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"

	"github.com/cornelk/hashmap"
)

// knownChunks stores SHA-256 digests from the previous PBS index for deduplication.
type knownChunks struct {
	byDigest map[[32]byte]struct{}
}

func newKnownChunks(capacity int) *knownChunks {
	k := &knownChunks{byDigest: make(map[[32]byte]struct{}, capacity)}
	return k
}

func (k *knownChunks) Len() int {
	if k == nil {
		return 0
	}
	return len(k.byDigest)
}

func (k *knownChunks) Has(digest [32]byte) bool {
	if k == nil {
		return false
	}
	_, ok := k.byDigest[digest]
	return ok
}

func (k *knownChunks) Add(digest [32]byte) {
	if k == nil {
		return
	}
	k.byDigest[digest] = struct{}{}
}

func (k *knownChunks) ToHashmap() *hashmap.Map[string, bool] {
	out := hashmap.New[string, bool]()
	if k == nil {
		return out
	}
	for digest := range k.byDigest {
		out.Set(hex.EncodeToString(digest[:]), true)
	}
	return out
}

func parseKnownFromPrevious(previous []byte) (*knownChunks, int, error) {
	known := newKnownChunks(0)
	if len(previous) < 8 || !bytes.HasPrefix(previous, didxMagic) {
		return known, 0, nil
	}
	if len(previous) < 4096 {
		return known, 0, nil
	}
	body := previous[4096:]
	count := len(body) / 40
	if count > 0 {
		known.byDigest = make(map[[32]byte]struct{}, count)
	}
	for i := 0; i+40 <= len(body); i += 40 {
		var digest [32]byte
		copy(digest[:], body[i+8:i+40])
		known.byDigest[digest] = struct{}{}
		_ = binary.LittleEndian.Uint64(body[i : i+8]) // offset stored in PBS index
	}
	return known, known.Len(), nil
}
