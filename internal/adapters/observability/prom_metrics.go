package observability

import (
	"log"

	"aegisflow/internal/domain"
	"aegisflow/internal/ports"
	"github.com/prometheus/client_golang/prometheus"
)

type PromObs struct {
	counters map[string]prometheus.Counter
	gauges   map[string]prometheus.Gauge
	histos   map[string]prometheus.Observer
}

func NewPromObs() *PromObs {
	ingested := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "aegis_samples_ingested_total",
		Help: "Total samples successfully written to sink.",
	})
	walGauge := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "aegis_wal_size_bytes",
		Help: "Size of WAL on disk.",
	})
	queueGauge := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "aegis_queue_length",
		Help: "Current number of samples buffered in the in-memory queue.",
	})
	latency := prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "ingest_sink_latency_seconds",
		Help:    "End-to-end latency from dequeued sample to sink commit.",
		Buckets: prometheus.ExponentialBuckets(0.001, 2, 12),
	})
	dlq := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "aegis_dlq_total",
		Help: "Samples sent to DLQ due to transform/sink failures.",
	})
	queueDrops := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "aegis_queue_dropped_total",
		Help: "Samples lost due to queue backpressure policies.",
	})

	prometheus.MustRegister(ingested, walGauge, queueGauge, latency, dlq, queueDrops)

	return &PromObs{
		counters: map[string]prometheus.Counter{
			"aegis_samples_ingested_total": ingested,
			"aegis_dlq_total":              dlq,
			"aegis_queue_dropped_total":    queueDrops,
		},
		gauges: map[string]prometheus.Gauge{
			"aegis_wal_size_bytes": walGauge,
			"aegis_queue_length":   queueGauge,
		},
		histos: map[string]prometheus.Observer{
			"ingest_sink_latency_seconds": latency,
		},
	}
}

func (p *PromObs) LogInfo(msg string, fields ...ports.Field) {}

func (p *PromObs) LogError(msg string, err error, fields ...ports.Field) {
	if err != nil {
		log.Printf("ERROR: %s: %v", msg, err)
	}
}

func (p *PromObs) LogCritical(msg string, err error, fields ...ports.Field) {
	if err != nil {
		log.Printf("CRITICAL: %s: %v", msg, err)
	}
}

func (p *PromObs) IncCounter(name string, v float64) {
	if c, ok := p.counters[name]; ok {
		c.Add(v)
	}
}

func (p *PromObs) ObserveLatency(name string, seconds float64) {
	if h, ok := p.histos[name]; ok {
		h.Observe(seconds)
	}
}

func (p *PromObs) SetGauge(name string, v float64) {
	if g, ok := p.gauges[name]; ok {
		g.Set(v)
	}
}

func (p *PromObs) RecordDLQ(id ports.WALEntryID, s *domain.Sample, err error) {
	p.IncCounter("aegis_dlq_total", 1)
	if err != nil {
		log.Printf("DLQ sample sensor=%s err=%v", s.SensorID, err)
	}
}
