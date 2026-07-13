package pbsbackup

import (
	"strings"
	"sync"

	pbscommon "pbscommon"
)

// chunkExistCache memoizes PBS chunk existence probes (one GET per digest per backup).
type chunkExistCache struct {
	mu    sync.Mutex
	cache map[string]existResult
}

type existResult struct {
	ok  bool
	err error
}

func newChunkExistCache() *chunkExistCache {
	return &chunkExistCache{cache: make(map[string]existResult)}
}

func (c *chunkExistCache) exists(client *pbscommon.PBSClient, digestHex string) (bool, error) {
	if c == nil || client == nil {
		return false, nil
	}
	if strings.TrimSpace(client.BaseURL) == "" {
		return true, nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if r, ok := c.cache[digestHex]; ok {
		return r.ok, r.err
	}
	ok, err := client.ChunkExistsOK(digestHex)
	c.cache[digestHex] = existResult{ok: ok, err: err}
	return ok, err
}
