<p align="center">
  <img src="docs/assets/AegisFlow-logo.png" alt="AegisFlow Logo" width="300"/>
</p>


<p align="center">
  <em>Industrial-grade data ingestion engine for high-throughput, low-latency, and guaranteed delivery</em>
</p>

<p align="center">

## Why This Exists

Modern plants still struggle to capture every millisecond of OPC UA data when networks wobble, brokers stall, or power fails. AegisFlow is the master’s-thesis-turned-product that proves you can stream **OPC UA → WAL → bounded queue → any TSDB (Timescale by default) with zero brokers**, Prometheus-native observability, WAL replay, and policy-driven backpressure. It’s architected for real factories (clean hexagon ports/adapters) and polished enough to drop straight into production—or the chapters of your thesis—without rewrites. Prefer QuestDB, Influx, Pinot, or an in-house historian? Swap the sink adapter; nothing else changes.

- **Clean hexagonal architecture** keeps domain logic independent from hardware quirks.
- **WAL → bounded queue → TSDB** guarantees replayable durability.
- **Prometheus-native observability** lets you argue about latency and throughput with evidence.
- **Disaster readiness** (WAL replay, backpressure policies) means you can pull the plug mid-burst and still graduate.

## How It Works

1. **Collector (OPC UA)** – `internal/adapters/opcua` opens a resilient subscription to your PLCs, stamps every monitored node with server timestamps and per-sensor sequence numbers, and streams `domain.Sample` payloads.
2. **Edge pipeline** – `internal/app/pipeline/edge_pipeline.go` appends samples to the WAL, enforces queue/WAL limits (block or shed based on config), and keeps metrics up to date.
3. **Write-Ahead Log** – `internal/adapters/wal/file_wal.go` flushes entries to disk, persists commit cursors, and replays uncommitted data on restart. Power-cycle the box: every sample is re-queued before new data flows.
4. **Ingest pipeline** – `internal/app/pipeline/ingest_pipeline.go` batches queue items, runs them through a transformer (currently a no-op hook), and writes to whatever sink implements `ports.Sink` (the repo ships a Timescale adapter out of the box) with idempotent `(sensor_id, ts, seq)` keys.
5. **Observability & health** – `cmd/aegis-edge/main.go` exposes `/metrics` & `/healthz`, publishes WAL/queue gauges, and records ingestion latency histograms so your thesis graphs are one curl away.

The entire flow is config-driven (`data/config.yaml`), so swapping PLC endpoints, WAL limits, or TSDB credentials does not require recompiling.

## Quick Start

```bash
# edit data/config.yaml with your OPC UA endpoint + Timescale credentials
go run ./cmd/aegis-edge -config ./data/config.yaml

# or install the CLI globally, just like any other Go tool:
go install github.com/ghalamif/AegisFlow/cmd/aegis-edge@latest
```

Monitor progress:

- `tail -f data/wal/wal.log` – durability in action.
- `curl http://localhost:9100/metrics` – ingestion counters, WAL size, queue depth, latency.
- `SELECT COUNT(*), MIN(ts), MAX(ts) FROM samples;` – proof that Timescale keeps up.

Kill the process mid-stream and run the command again; the startup log will report `wal_replay_complete`, demonstrating crash recovery.

## Use It Inside Your Project

The repo now publishes a public package, so integrating it is as familiar as adding any other Go dependency:

```bash
go get github.com/ghalamif/AegisFlow/pkg/aegisflow@latest
```

```go
package main

import (
    "context"
    "log"

    "github.com/ghalamif/AegisFlow/pkg/aegisflow"
)

func main() {
    cfg, err := aegisflow.LoadConfig("data/config.yaml")
    if err != nil {
        log.Fatal(err)
    }

    rt, err := aegisflow.NewEdgeRuntime(cfg)
    if err != nil {
        log.Fatal(err)
    }

    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    if err := rt.Run(ctx); err != nil {
        log.Fatal(err)
    }
}
```

Need to tweak WAL thresholds or OPC UA nodes programmatically? Use the exported aliases:

```go
cfg := &aegisflow.Config{
    Policy: aegisflow.Policy{
        MaxQueueLen: 200_000,
        OnQueueFull: "block",
    },
    OPCUA: aegisflow.OPCUAConfig{
        Endpoint: "opc.tcp://plc:4840",
        Nodes: []aegisflow.OPCUANodeConfig{
            {NodeID: "ns=2;s=Demo.Dynamic.Scalar.Double"},
        },
    },
}
```

Behind the scenes the runtime still uses the battle-tested internal adapters, but your application only needs the high-level APIs exposed through `pkg/aegisflow`.

### Swap in your own sink (QuestDB, Pinot, etc.)

The sink is just an interface:

```go
type Sink interface {
    WriteBatch(samples []*domain.Sample) error
    Name() string
}
```

Drop your adapter into `ports.Sink`, point the runtime at it, and the rest of the pipeline keeps running unchanged. The provided Timescale adapter is simply the default implementation.

## Industry-Ready Features

- **Backpressure policies** – configure `policy.on_queue_full` / `policy.on_wal_full` to block or drop gracefully when storage shrinks or TSDB slows down.
- **Observability-first mindset** – Prometheus metrics include queue drops, DLQ counts, WAL size, ingest latency, and total samples, so you can write chapters about throughput, outages, and recovery curves.
- **Horizontal scaling** – run multiple `aegis-edge` instances with different `data/config.yaml` sensor groups to cover thousands of 30–100 Hz signals. Timescale’s hypertables handle the parallel writes.
- **Replay & disaster drills** – WAL replay happens before collectors re-open, letting you simulate power failures confidently.

## Contribute Like a Pro

This project doubles as academic evidence and an industrial starter kit, so contributions must keep both standards high:

1. **Discuss** – open an issue describing the plant challenge, experiment, or thesis angle you’re targeting.
2. **Code** – implement adapters or features behind the relevant port interfaces; add config knobs, observability, and tests alongside new behaviours.
3. **Test** – run `go test ./...` locally. A GitHub Actions workflow (`.github/workflows/ci.yml`) runs the same suite on every push/PR.
4. **Document** – extend this README or add ADR-style notes so future engineers (or examiners) can trace your reasoning.

New to contributing? Start with:

- Additional transformers (unit conversion, calibration).
- DLQ implementations for complex sinks.
- Config reload endpoints for zero-downtime tuning.
- Benchmark harnesses for the thesis comparison chapter.

## Continuous Testing

Every push triggers the CI pipeline:

- **`go test ./...`** – validates adapters, queues, WAL replay, and future unit tests so regressions never sneak into your thesis results.
- The pipeline is defined in `.github/workflows/ci.yml`, ready for you to extend with linting, race detection, or integration suites.

---

“Industrial resilience is not a chapter—it’s a feature.” AegisFlow Direct exists so your research narrative and your production roadmap stay aligned. Plug in your PLCs, watch the WAL fill with real telemetry, and build the dashboards that will convince both professors and plant managers. Let’s make broker-free ingest boringly reliable. 
