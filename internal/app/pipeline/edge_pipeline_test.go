package pipeline

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/ghalamif/AegisFlow/internal/domain"
	"github.com/ghalamif/AegisFlow/internal/ports"
)

func TestWaitForWALCapacityBlockThenSucceed(t *testing.T) {
	wal := &mockWAL{
		sizes: []int64{150, 50},
	}
	pol := ports.Policy{
		MaxWALSizeBytes: 100,
		OnWALFull:       "block",
		IdleSleep:       time.Millisecond,
	}
	obs := &mockObs{}

	if ok := waitForWALCapacity(wal, pol, obs); !ok {
		t.Fatalf("expected waitForWALCapacity to eventually succeed")
	}
	if wal.calls < 2 {
		t.Fatalf("expected multiple stats calls, got %d", wal.calls)
	}
}

func TestWaitForWALCapacityDrop(t *testing.T) {
	wal := &mockWAL{
		sizes: []int64{200, 200},
	}
	pol := ports.Policy{
		MaxWALSizeBytes: 100,
		OnWALFull:       "drop",
	}
	obs := &mockObs{}

	if ok := waitForWALCapacity(wal, pol, obs); ok {
		t.Fatalf("expected waitForWALCapacity to drop and return false")
	}
	if len(obs.errors) == 0 {
		t.Fatalf("expected error to be logged")
	}
}

func TestEnqueueWithPolicyBlock(t *testing.T) {
	queue := &mockQueue{}
	queue.failures = 1

	pol := ports.Policy{
		OnQueueFull: "block",
		IdleSleep:   time.Millisecond,
	}
	obs := &mockObs{}

	if ok := enqueueWithPolicy(queue, 1, &domain.Sample{}, pol, obs); !ok {
		t.Fatalf("expected enqueue to eventually succeed")
	}
	if queue.calls != 2 {
		t.Fatalf("expected two enqueue attempts, got %d", queue.calls)
	}
}

func TestEnqueueWithPolicyDrop(t *testing.T) {
	queue := &mockQueue{failAlways: true}
	pol := ports.Policy{
		OnQueueFull: "drop",
	}
	obs := &mockObs{}

	if ok := enqueueWithPolicy(queue, 1, &domain.Sample{}, pol, obs); ok {
		t.Fatalf("expected enqueueWithPolicy to fail")
	}
	if len(obs.errors) == 0 {
		t.Fatalf("expected drop to log an error")
	}
}

type mockWAL struct {
	ports.WAL
	sizes []int64
	calls int
}

func (m *mockWAL) Stats() ports.WALStats {
	idx := m.calls
	if idx >= len(m.sizes) {
		idx = len(m.sizes) - 1
	}
	m.calls++
	return ports.WALStats{
		SizeBytes: m.sizes[idx],
	}
}

type mockQueue struct {
	failures   int32
	failAlways bool
	calls      int
}

func (m *mockQueue) Enqueue(id ports.WALEntryID, s *domain.Sample) bool {
	m.calls++
	if m.failAlways {
		return false
	}
	if atomic.LoadInt32(&m.failures) > 0 {
		atomic.AddInt32(&m.failures, -1)
		return false
	}
	return true
}

func (m *mockQueue) DequeueBatch(int) []ports.QueuedSample { return nil }
func (m *mockQueue) Len() int                              { return 0 }

type mockObs struct {
	errors []error
}

func (m *mockObs) LogInfo(string, ...ports.Field)                    {}
func (m *mockObs) LogError(_ string, err error, _ ...ports.Field)    { m.errors = append(m.errors, err) }
func (m *mockObs) LogCritical(string, error, ...ports.Field)         {}
func (m *mockObs) IncCounter(string, float64)                        {}
func (m *mockObs) ObserveLatency(string, float64)                    {}
func (m *mockObs) SetGauge(string, float64)                          {}
func (m *mockObs) RecordDLQ(ports.WALEntryID, *domain.Sample, error) {}
