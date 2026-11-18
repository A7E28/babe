package image

import (
	"context"
	"sync"
)

type opDecodeTask struct {
	idx      int
	op       installOp
	verify   bool
	resultCh chan<- decodedOpChunk
	ctx      context.Context
}

type sharedOpPool struct {
	src     *payloadSource
	codecs  *payloadCodecPool
	sem     chan struct{}
	tasksCh chan opDecodeTask
	wg      sync.WaitGroup
}

func newSharedOpPool(ctx context.Context, src *payloadSource, codecs *payloadCodecPool, jobs int) (*sharedOpPool, error) {
	maxInFlight := max(2, jobs*2)
	pool := &sharedOpPool{
		src:     src,
		codecs:  codecs,
		sem:     make(chan struct{}, maxInFlight),
		tasksCh: make(chan opDecodeTask, maxInFlight),
	}

	type workerHandle struct {
		r     interface{ ReadAt([]byte, int64) (int, error) }
		close func()
	}
	handles := make([]workerHandle, 0, jobs)
	for i := 0; i < jobs; i++ {
		r, closeFn, err := src.openWorkerReader()
		if err != nil {
			for _, h := range handles {
				h.close()
			}
			return nil, err
		}
		handles = append(handles, workerHandle{r: r, close: closeFn})
	}

	for i := 0; i < jobs; i++ {
		h := handles[i]
		pool.wg.Add(1)
		go func() {
			defer pool.wg.Done()
			defer h.close()
			for task := range pool.tasksCh {
				// Always send 1 chunk per submitted task so coordinators can drain every buffer & sem slot cleanly.
				if err := task.ctx.Err(); err != nil {
					task.resultCh <- decodedOpChunk{idx: task.idx, err: err}
					continue
				}
				buf, n, isZero, err := codecs.decodeOp(h.r, src.size, src.dataStart, src.manifest.BlockSize, task.op, task.verify)
				task.resultCh <- decodedOpChunk{idx: task.idx, buf: buf, n: n, isZero: isZero, err: err}
			}
		}()
	}
	return pool, nil
}

func (p *sharedOpPool) acquireSlot(ctx context.Context) bool {
	select {
	case p.sem <- struct{}{}:
		return true
	case <-ctx.Done():
		return false
	}
}

func (p *sharedOpPool) releaseSlot() {
	select {
	case <-p.sem:
	default:
	}
}

func (p *sharedOpPool) submit(task opDecodeTask) bool {
	select {
	case p.tasksCh <- task:
		return true
	case <-task.ctx.Done():
		p.releaseSlot()
		return false
	}
}

func (p *sharedOpPool) maxInFlight() int {
	return cap(p.sem)
}

func (p *sharedOpPool) Close() {
	close(p.tasksCh)
	p.wg.Wait()
}
