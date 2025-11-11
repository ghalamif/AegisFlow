package pipeline

import (
	"time"

	"github.com/ghalamif/AegisFlow/internal/domain"
	"github.com/ghalamif/AegisFlow/internal/ports"
)

func RunIngestPipeline(wal ports.WAL, q ports.SampleQueue, tr ports.Transformer, sink ports.Sink, pol ports.Policy, obs ports.Observability) {
	for {
		batch := q.DequeueBatch(pol.MaxBatchSize)
		if len(batch) == 0 {
			time.Sleep(pol.IdleSleep)
			continue
		}

		var (
			out   = make([]*domain.Sample, 0, len(batch))
			maxID ports.WALEntryID
		)

		for _, item := range batch {
			s, err := tr.Transform(item.Sample)
			if err != nil {
				obs.RecordDLQ(item.ID, item.Sample, err)
				continue
			}
			s.TransformVer = tr.Version()
			out = append(out, s)
			if item.ID > maxID {
				maxID = item.ID
			}
		}

		if len(out) == 0 {
			_ = wal.Commit(maxID)
			continue
		}

		start := time.Now()
		if err := sink.WriteBatch(out); err != nil {
			obs.LogError("sink_write_failed", err)
			// keep WAL; replays later
			continue
		}
		obs.ObserveLatency("ingest_sink_latency_seconds", time.Since(start).Seconds())
		obs.IncCounter("aegis_samples_ingested_total", float64(len(out)))

		if err := wal.Commit(maxID); err != nil {
			obs.LogError("wal_commit_failed", err)
		}
	}
}
