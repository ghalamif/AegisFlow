package aegisflow

import (
	"testing"
	"time"
)

func TestNewEdgeRuntimeWithCustomAdapters(t *testing.T) {
	cfg := &Config{
		Policy: Policy{
			MaxQueueLen:  8,
			MaxBatchSize: 4,
			IdleSleep:    time.Millisecond,
		},
		OPCUA: OPCUAConfig{
			Endpoint: "opc.tcp://test:4840",
			Nodes: []OPCUANodeConfig{
				{NodeID: "ns=1;s=demo", SensorID: "demo"},
			},
		},
		Timescale: TimescaleConfig{
			ConnString: "postgres://user:pass@localhost:5432/db?sslmode=disable",
			Table:      "samples",
		},
		Metrics: MetricsConfig{Addr: ":0"},
		WAL:     WALConfig{Dir: t.TempDir()},
	}

	queueStub := &stubQueue{}
	collectorStub := &stubCollector{}
	sinkStub := &stubSink{}
	transformerStub := &stubTransformer{}
	walStub := &stubWAL{}
	obsStub := &stubObservability{}

	rt, err := NewEdgeRuntime(
		cfg,
		WithCollector(collectorStub),
		WithSink(sinkStub),
		WithTransformer(transformerStub),
		WithWAL(walStub),
		WithSampleQueue(queueStub),
		WithObservability(obsStub),
	)
	if err != nil {
		t.Fatalf("NewEdgeRuntime returned error: %v", err)
	}

	if rt.collector != collectorStub {
		t.Fatalf("expected custom collector to be used")
	}
	if rt.sink != sinkStub {
		t.Fatalf("expected custom sink to be used")
	}
	if rt.transformer != transformerStub {
		t.Fatalf("expected custom transformer to be used")
	}
	if rt.wal != walStub {
		t.Fatalf("expected custom WAL to be used")
	}
	if rt.queue != queueStub {
		t.Fatalf("expected custom queue to be used")
	}
	if rt.obs != obsStub {
		t.Fatalf("expected custom observability to be used")
	}
	if rt.db != nil {
		t.Fatalf("expected db to be nil when custom sink is provided")
	}
}

type stubCollector struct{}

func (s *stubCollector) Start(out chan<- *PipelineSample) error { return nil }
func (s *stubCollector) Stop() error                            { return nil }

type stubSink struct{}

func (s *stubSink) WriteBatch(samples []*PipelineSample) error { return nil }
func (s *stubSink) Name() string                               { return "stub" }

type stubTransformer struct{}

func (s *stubTransformer) Transform(sample *PipelineSample) (*PipelineSample, error) {
	return sample, nil
}
func (s *stubTransformer) Version() uint16 { return 42 }

type stubQueue struct{}

func (s *stubQueue) Enqueue(id WALEntryID, sample *PipelineSample) bool { return true }
func (s *stubQueue) DequeueBatch(max int) []QueuedSample                { return nil }
func (s *stubQueue) Len() int                                           { return 0 }

type stubWAL struct{}

func (s *stubWAL) Append(sample *PipelineSample) (WALEntryID, error) { return 0, nil }
func (s *stubWAL) Iterate(from WALEntryID, fn func(id WALEntryID, sample *PipelineSample) error) error {
	return nil
}
func (s *stubWAL) Commit(upto WALEntryID) error { return nil }
func (s *stubWAL) TruncateCommitted() error     { return nil }
func (s *stubWAL) Stats() WALStats              { return WALStats{} }

type stubObservability struct{}

func (s *stubObservability) LogInfo(string, ...Field)                     {}
func (s *stubObservability) LogError(string, error, ...Field)             {}
func (s *stubObservability) LogCritical(string, error, ...Field)          {}
func (s *stubObservability) IncCounter(string, float64)                   {}
func (s *stubObservability) ObserveLatency(string, float64)               {}
func (s *stubObservability) SetGauge(string, float64)                     {}
func (s *stubObservability) RecordDLQ(WALEntryID, *PipelineSample, error) {}
