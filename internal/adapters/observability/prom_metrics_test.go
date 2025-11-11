package observability

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestPromObsMetrics(t *testing.T) {
	origReg := prometheus.DefaultRegisterer
	origGatherer := prometheus.DefaultGatherer
	t.Cleanup(func() {
		prometheus.DefaultRegisterer = origReg
		prometheus.DefaultGatherer = origGatherer
	})

	reg := prometheus.NewRegistry()
	prometheus.DefaultRegisterer = reg
	prometheus.DefaultGatherer = reg

	obs := NewPromObs()

	obs.IncCounter("aegis_samples_ingested_total", 5)
	if got := testutil.ToFloat64(obs.counters["aegis_samples_ingested_total"]); got != 5 {
		t.Fatalf("expected ingested counter 5, got %f", got)
	}

	obs.IncCounter("aegis_queue_dropped_total", 2)
	if got := testutil.ToFloat64(obs.counters["aegis_queue_dropped_total"]); got != 2 {
		t.Fatalf("expected queue drop counter 2, got %f", got)
	}

	obs.SetGauge("aegis_wal_size_bytes", 42)
	if got := testutil.ToFloat64(obs.gauges["aegis_wal_size_bytes"]); got != 42 {
		t.Fatalf("expected wal gauge 42, got %f", got)
	}

	obs.ObserveLatency("ingest_sink_latency_seconds", 0.5)
	hCollector := obs.histos["ingest_sink_latency_seconds"].(prometheus.Collector)
	if samples := testutil.CollectAndCount(hCollector); samples != 1 {
		t.Fatalf("expected latency histogram to record 1 sample, got %d", samples)
	}

	obs.RecordDLQ(1, nil, nil)
	if got := testutil.ToFloat64(obs.counters["aegis_dlq_total"]); got != 1 {
		t.Fatalf("expected dlq counter 1, got %f", got)
	}
}
