package queue

import (
	"sync"

	"github.com/ghalamif/AegisFlow/internal/domain"
	"github.com/ghalamif/AegisFlow/internal/ports"
)

// MemQueue is a bounded in-memory queue that preserves FIFO ordering.
type MemQueue struct {
	mu   sync.Mutex
	data []ports.QueuedSample
	cap  int
}

func NewMemQueue(capacity int) *MemQueue {
	return &MemQueue{
		data: make([]ports.QueuedSample, 0, capacity),
		cap:  capacity,
	}
}

func (q *MemQueue) Enqueue(id ports.WALEntryID, s *domain.Sample) bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	if len(q.data) >= q.cap {
		return false
	}
	q.data = append(q.data, ports.QueuedSample{ID: id, Sample: s})
	return true
}

func (q *MemQueue) DequeueBatch(max int) []ports.QueuedSample {
	q.mu.Lock()
	defer q.mu.Unlock()
	if len(q.data) == 0 {
		return nil
	}
	if max <= 0 || max > len(q.data) {
		max = len(q.data)
	}
	out := make([]ports.QueuedSample, max)
	copy(out, q.data[:max])
	q.data = append(q.data[:0], q.data[max:]...)
	return out
}

func (q *MemQueue) Len() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.data)
}

var _ ports.SampleQueue = (*MemQueue)(nil)
