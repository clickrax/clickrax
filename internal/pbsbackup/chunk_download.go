package pbsbackup

import (
	"context"
	"fmt"
	"sync"

	"pbs-win-backup/internal/i18nconfig"
)

type chunkFetchResult struct {
	index int
	data  []byte
	err   error
}

func downloadChunksParallel(
	ctx context.Context,
	fetch func(digest string) ([]byte, error),
	records []didxRecord,
	indices []int,
	workers int,
	onFetched func(fetched, total int),
) (map[int][]byte, error) {
	if len(indices) == 0 {
		return map[int][]byte{}, nil
	}
	if fetch == nil {
		return nil, fmt.Errorf("chunk fetcher is nil")
	}
	if workers < 1 {
		workers = 1
	}
	if workers > len(indices) {
		workers = len(indices)
	}

	out := make(map[int][]byte, len(indices))
	sem := make(chan struct{}, workers)
	results := make(chan chunkFetchResult, len(indices))
	var wg sync.WaitGroup
	defer wg.Wait()

	for _, recIdx := range indices {
		if err := abortIfCancelled(ctx); err != nil {
			return nil, err
		}
		recIdx := recIdx
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			if err := abortIfCancelled(ctx); err != nil {
				results <- chunkFetchResult{index: recIdx, err: err}
				return
			}
			if recIdx < 0 || recIdx >= len(records) {
				results <- chunkFetchResult{index: recIdx, err: i18nconfig.FromConfig().Ef("pbs.chunk_index_invalid", map[string]string{"n": fmt.Sprintf("%d", recIdx)})}
				return
			}
			data, err := fetch(records[recIdx].digest)
			results <- chunkFetchResult{index: recIdx, data: data, err: err}
		}()
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	fetched := 0
	for res := range results {
		if res.err != nil {
			return nil, res.err
		}
		out[res.index] = res.data
		fetched++
		if onFetched != nil {
			onFetched(fetched, len(indices))
		}
	}
	return out, nil
}
