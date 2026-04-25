package experiment

import (
	"context"
	"sync"
	"sync/atomic"

	"github.com/mustafaselman/frameval/engine/internal/models"
)

type Job func(context.Context)

type Queue struct {
	jobs       chan Job
	maxWorkers int
	active     atomic.Int64
	cancel     context.CancelFunc
	wg         sync.WaitGroup
}

func NewQueue(parent context.Context, maxWorkers int) *Queue {
	if maxWorkers <= 0 {
		maxWorkers = 1
	}
	ctx, cancel := context.WithCancel(parent)
	q := &Queue{jobs: make(chan Job, 256), maxWorkers: maxWorkers, cancel: cancel}
	for worker := 0; worker < maxWorkers; worker++ {
		q.wg.Add(1)
		go func() {
			defer q.wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				case job := <-q.jobs:
					if job == nil {
						continue
					}
					q.active.Add(1)
					job(ctx)
					q.active.Add(-1)
				}
			}
		}()
	}
	return q
}

func (q *Queue) Enqueue(job Job) {
	q.jobs <- job
}

func (q *Queue) Snapshot() models.QueueStatus {
	return models.QueueStatus{Depth: len(q.jobs), ActiveWorkers: int(q.active.Load()), MaxWorkers: q.maxWorkers}
}

func (q *Queue) Close() {
	q.cancel()
	q.wg.Wait()
}
