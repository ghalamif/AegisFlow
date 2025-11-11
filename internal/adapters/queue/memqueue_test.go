package queue

import (
	"testing"

	"aegisflow/internal/domain"
)

func TestMemQueueEnqueueDequeueOrder(t *testing.T) {
	q := NewMemQueue(4)

	s1 := &domain.Sample{SensorID: "s1"}
	s2 := &domain.Sample{SensorID: "s2"}

	if !q.Enqueue(1, s1) || !q.Enqueue(2, s2) {
		t.Fatalf("expected successful enqueue")
	}

	batch := q.DequeueBatch(1)
	if len(batch) != 1 || batch[0].ID != 1 || batch[0].Sample.SensorID != "s1" {
		t.Fatalf("unexpected first batch: %+v", batch)
	}

	remaining := q.DequeueBatch(10)
	if len(remaining) != 1 || remaining[0].ID != 2 {
		t.Fatalf("unexpected second batch: %+v", remaining)
	}

	if q.Len() != 0 {
		t.Fatalf("queue should be empty, got %d", q.Len())
	}
}

func TestMemQueueCapacity(t *testing.T) {
	q := NewMemQueue(2)

	sample := &domain.Sample{SensorID: "cap"}

	if !q.Enqueue(1, sample) || !q.Enqueue(2, sample) {
		t.Fatalf("expected enqueue within capacity")
	}
	if q.Enqueue(3, sample) {
		t.Fatalf("enqueue should fail when capacity exceeded")
	}

	q.DequeueBatch(1)
	if !q.Enqueue(4, sample) {
		t.Fatalf("expected enqueue to succeed after dequeue")
	}
}
