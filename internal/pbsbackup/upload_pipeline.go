package pbsbackup

import (
	"context"
	"fmt"
	"sync"

	"pbs-win-backup/internal/i18n"

	pbscommon "pbscommon"
)

type chunkUploadPipeline struct {
	ctx     context.Context
	client  *pbscommon.PBSClient
	limiter *bandwidthLimiter
	sem     chan struct{}
	wg      sync.WaitGroup
	mu      sync.Mutex
	err     error
}

func newChunkUploadPipeline(ctx context.Context, client *pbscommon.PBSClient, limiter *bandwidthLimiter, workers int) *chunkUploadPipeline {
	if workers < 1 {
		workers = 1
	}
	if ctx == nil {
		ctx = context.Background()
	}
	return &chunkUploadPipeline{
		ctx:     ctx,
		client:  client,
		limiter: limiter,
		sem:     make(chan struct{}, workers),
	}
}

func (p *chunkUploadPipeline) fail(err error) {
	if err == nil {
		return
	}
	p.mu.Lock()
	if p.err == nil {
		p.err = err
	}
	p.mu.Unlock()
}

func (p *chunkUploadPipeline) upload(wrid uint64, digest string, data []byte) error {
	if p == nil {
		return fmt.Errorf("upload pipeline is nil")
	}
	select {
	case <-p.ctx.Done():
		return p.ctx.Err()
	default:
	}
	p.mu.Lock()
	if p.err != nil {
		err := p.err
		p.mu.Unlock()
		return err
	}
	p.mu.Unlock()

	select {
	case p.sem <- struct{}{}:
	case <-p.ctx.Done():
		return p.ctx.Err()
	}

	payload := append([]byte(nil), data...)
	p.wg.Add(1)
		go func() {
		defer func() {
			<-p.sem
			p.wg.Done()
		}()
		if err := p.ctx.Err(); err != nil {
			p.fail(err)
			return
		}
		p.mu.Lock()
		if p.err != nil {
			p.mu.Unlock()
			return
		}
		p.mu.Unlock()

		if p.limiter != nil {
			p.limiter.wait(len(payload))
		}
		if err := p.ctx.Err(); err != nil {
			p.fail(err)
			return
		}
		if err := p.client.UploadDynamicCompressedChunk(wrid, digest, payload); err != nil {
			p.fail(i18n.Ewrap("pbs.chunk_upload", nil, err))
		}
	}()
	return nil
}

func (p *chunkUploadPipeline) wait() error {
	if p == nil {
		return nil
	}
	done := make(chan struct{})
	go func() {
		p.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-p.ctx.Done():
		p.wg.Wait()
		return p.ctx.Err()
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.err != nil {
		return p.err
	}
	return p.ctx.Err()
}
