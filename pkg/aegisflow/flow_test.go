package aegisflow

import (
	"context"
	"testing"
	"time"
)

func TestConfFromConfigAndStreamBuilder(t *testing.T) {
	cfg := &Config{
		Policy: Policy{
			MaxWALSizeBytes: 1024 * 1024,
			MaxQueueLen:     8,
			MaxBatchSize:    4,
			IdleSleep:       time.Millisecond,
			OnWALFull:       "block",
			OnQueueFull:     "block",
		},
		OPCUA: OPCUAConfig{
			Endpoint: "opc.tcp://test:4840",
			Nodes: []OPCUANodeConfig{
				{NodeID: "ns=1;s=demo"},
			},
		},
		Timescale: TimescaleConfig{
			ConnString: "postgres://user:pass@localhost:5432/db?sslmode=disable",
			Table:      "samples",
		},
		Metrics: MetricsConfig{Addr: ":0"},
		WAL:     WALConfig{Dir: t.TempDir()},
	}

	flow, err := ConfFromConfig(cfg)
	if err != nil {
		t.Fatalf("ConfFromConfig returned error: %v", err)
	}
	if flow.Config() != cfg {
		t.Fatalf("expected Config to be returned verbatim")
	}

	col := &stubCollector{}
	sink := &stubSink{}

	rt, err := flow.
		StreamIN(
			StreamInCollector(col),
			StreamInObservability(&stubObservability{}),
		).
		StreamOUT(
			StreamOutSink(sink),
			StreamOutTransformer(&stubTransformer{}),
			StreamOutObservability(&stubObservability{}),
		)
	if err != nil {
		t.Fatalf("StreamOUT returned error: %v", err)
	}
	if rt.collector != col {
		t.Fatalf("expected custom collector to be wired")
	}
	if rt.sink != sink {
		t.Fatalf("expected custom sink to be wired")
	}
}

func TestFlowRunUsesStreamOutOptions(t *testing.T) {
	cfg := &Config{
		Policy: Policy{
			MaxWALSizeBytes: 1024 * 1024,
			MaxQueueLen:     4,
			MaxBatchSize:    2,
			IdleSleep:       time.Millisecond,
			OnWALFull:       "block",
			OnQueueFull:     "block",
		},
		OPCUA: OPCUAConfig{
			Endpoint: "opc.tcp://test:4840",
			Nodes: []OPCUANodeConfig{
				{NodeID: "ns=1;s=demo"},
			},
		},
		Timescale: TimescaleConfig{
			ConnString: "postgres://user:pass@localhost:5432/db?sslmode=disable",
			Table:      "samples",
		},
		Metrics: MetricsConfig{Addr: ":0"},
		WAL:     WALConfig{Dir: t.TempDir()},
	}

	flow, err := ConfFromConfig(cfg)
	if err != nil {
		t.Fatalf("ConfFromConfig returned error: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Stop immediately to avoid waiting on real OPC UA.
	cancel()
	if err := flow.StreamIN(
		StreamInCollector(&stubCollector{}),
		StreamInObservability(&stubObservability{}),
	).Run(ctx,
		StreamOutSink(&stubSink{}),
		StreamOutObservability(&stubObservability{}),
	); err != nil && err != context.Canceled {
		t.Fatalf("Run returned unexpected error: %v", err)
	}
}
