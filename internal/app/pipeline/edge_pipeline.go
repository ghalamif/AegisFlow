package pipeline

import (
	"fmt"
	"time"

	"github.com/ghalamif/AegisFlow/internal/domain"
	"github.com/ghalamif/AegisFlow/internal/ports"
)

func RunEdgePipeline(col ports.Collector, wal ports.WAL, q ports.SampleQueue, pol ports.Policy, obs ports.Observability) error {
	ch := make(chan *domain.Sample, pol.MaxQueueLen)

	if err := col.Start(ch); err != nil {
		return err
	}

	go func() {
		for s := range ch {
			if !waitForWALCapacity(wal, pol, obs) {
				continue
			}

			id, err := wal.Append(s)
			if err != nil {
				obs.LogCritical("wal_append_failed", err)
				continue
			}

			if !enqueueWithPolicy(q, id, s, pol, obs) {
				obs.IncCounter("aegis_queue_dropped_total", 1)
			}
		}
	}()

	return nil
}

func waitForWALCapacity(wal ports.WAL, pol ports.Policy, obs ports.Observability) bool {
	if pol.MaxWALSizeBytes <= 0 {
		return true
	}
	sleep := pol.IdleSleep
	if sleep <= 0 {
		sleep = 5 * time.Millisecond
	}

	for {
		stats := wal.Stats()
		if stats.SizeBytes < pol.MaxWALSizeBytes {
			return true
		}

		switch pol.OnWALFull {
		case "block":
			time.Sleep(sleep)
		case "drop":
			obs.LogError("wal_full_drop", fmt.Errorf("size=%d limit=%d", stats.SizeBytes, pol.MaxWALSizeBytes))
			return false
		default:
			obs.LogError("wal_policy_invalid", fmt.Errorf("policy=%s", pol.OnWALFull))
			return false
		}
	}
}

func enqueueWithPolicy(q ports.SampleQueue, id ports.WALEntryID, s *domain.Sample, pol ports.Policy, obs ports.Observability) bool {
	sleep := pol.IdleSleep
	if sleep <= 0 {
		sleep = 5 * time.Millisecond
	}

	for {
		if ok := q.Enqueue(id, s); ok {
			return true
		}

		switch pol.OnQueueFull {
		case "block":
			time.Sleep(sleep)
		case "drop", "reject":
			obs.LogError("queue_full_drop", fmt.Errorf("queue length exceeded capacity %d", pol.MaxQueueLen))
			return false
		default:
			obs.LogError("queue_policy_invalid", fmt.Errorf("policy=%s", pol.OnQueueFull))
			return false
		}
	}
}
