package gopool

import (
	"container/heap"
	"context"
	"sync"
)

// newJob returns a PriorityJob with the given task and priority
func newJob(job Runnable, priority int) JobEntry {
	return &pt{
		priority: priority,
		job:      job,
		lock:     &sync.Mutex{},
	}
}

// pt is an internal implementation of the PriorityJob
type pt struct {
	priority   int
	job        Runnable
	lock       *sync.Mutex
	cancelFunc context.CancelFunc
}

func (t *pt) String() string {
	return t.job.String()
}

func (t *pt) Priority() int {
	return t.priority
}

func (t *pt) Run(jobContext context.Context, cancelFunc context.CancelFunc) {
	t.lock.Lock()
	t.cancelFunc = cancelFunc
	t.lock.Unlock()

	t.job.Run(jobContext)
}

func (t *pt) Cancel() {
	var cancelFunc context.CancelFunc
	t.lock.Lock()
	cancelFunc = t.cancelFunc
	t.cancelFunc = nil
	t.lock.Unlock()

	if cancelFunc != nil {
		cancelFunc()
	}
}

// PriorityQueue is an implementation of a Sourcer using a priority
// queue. Higher priority tasks will be done first.
type PriorityQueue struct {
	q *pq
}

// NewPriorityQueue creates a new PriorityQueue.
func NewPriorityQueue() *PriorityQueue {
	q := &PriorityQueue{q: &pq{}}
	heap.Init(q.q)
	return q
}

func (q *PriorityQueue) Length() int {
	return q.q.Len()
}

// Next implements Sourcer.Next.
func (q *PriorityQueue) Next() JobEntry {
	if q.q.Len() < 1 {
		return nil
	}
	return heap.Pop(q.q).(JobEntry)
}

func (q *PriorityQueue) Add(t JobEntry) {
	heap.Push(q.q, t)
}

// internal representation of priority queue
type pq []JobEntry

func (q pq) Len() int           { return len(q) }
func (q pq) Less(i, j int) bool { return q[i].Priority() > q[j].Priority() }
func (q pq) Swap(i, j int)      { q[i], q[j] = q[j], q[i] }

func (q *pq) Push(x interface{}) {
	*q = append(*q, x.(JobEntry))
}

func (q *pq) Pop() interface{} {
	old := *q
	n := len(old)
	t := old[n-1]
	*q = old[0 : n-1]
	return t
}
