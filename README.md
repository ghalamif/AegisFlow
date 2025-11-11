<p align="center">
  <img src="docs/assets/AegisFlow.png" alt="AegisFlow Logo" width="200"/>
</p>


<p align="center">
  <em>Industrial-grade data ingestion engine for high-throughput, low-latency, and guaranteed delivery</em>
</p>

<p align="center">

## Why This Exists

Modern plants still struggle to capture every millisecond of OPC UA data when networks wobble, brokers stall, or power fails. AegisFlow Direct is the thesis project that proved we can ingest **real OPC UA streams straight into TimescaleDB with WAL-grade durability**, zero Kafka, and instrumentation you can defend in an academic viva and deploy on a factory floor the next day.

- **Clean hexagonal architecture** keeps domain logic independent from hardware quirks.
- **WAL → bounded queue → TSDB** guarantees replayable durability.
- **Prometheus-native observability** lets you argue about latency and throughput with evidence.
- **Disaster readiness** (WAL replay, backpressure policies) means you can pull the plug mid-burst and still graduate.

## How It Works

1. **Collector (OPC UA)** – `internal/adapters/opcua` opens a resilient subscription to your PLCs, stamps every monitored node with server timestamps and per-sensor sequence numbers, and streams `domain.Sample` payloads.
2. **Edge pipeline** – `internal/app/pipeline/edge_pipeline.go` appends samples to the WAL, enforces queue/WAL limits (block or shed based on config), and keeps metrics up to date.
3. **Write-Ahead Log** – `internal/adapters/wal/file_wal.go` flushes entries to disk, persists commit cursors, and replays uncommitted data on restart. Power-cycle the box: every sample is re-queued before new data flows.
4. **Ingest pipeline** – `internal/app/pipeline/ingest_pipeline.go` batches queue items, runs them through a transformer (currently a no-op hook), and writes to TimescaleDB with idempotent `(sensor_id, ts, seq)` keys.
5. **Observability & health** – `cmd/aegis-edge/main.go` exposes `/metrics` & `/healthz`, publishes WAL/queue gauges, and records ingestion latency histograms so your thesis graphs are one curl away.

The entire flow is config-driven (`data/config.yaml`), so swapping PLC endpoints, WAL limits, or TSDB credentials does not require recompiling.

## Quick Start

```bash
# edit data/config.yaml with your OPC UA endpoint + Timescale credentials
go run ./cmd/aegis-edge -config ./data/config.yaml
```

Monitor progress:

- `tail -f data/wal/wal.log` – durability in action.
- `curl http://localhost:9100/metrics` – ingestion counters, WAL size, queue depth, latency.
- `SELECT COUNT(*), MIN(ts), MAX(ts) FROM samples;` – proof that Timescale keeps up.

Kill the process mid-stream and run the command again; the startup log will report `wal_replay_complete`, demonstrating crash recovery.

## Use It Inside Your Project

AegisFlow is packaged as a Go module (`module aegisflow`). You can embed the pipelines inside another service:

```go
import (
    "aegisflow/internal/app/pipeline"
    "aegisflow/internal/ports"
)

func boot(myCollector ports.Collector, wal ports.WAL, queue ports.SampleQueue, sink ports.Sink, pol ports.Policy, obs ports.Observability, tr ports.Transformer) {
    if err := pipeline.RunEdgePipeline(myCollector, wal, queue, pol, obs); err != nil {
        panic(err)
    }
    go pipeline.RunIngestPipeline(wal, queue, tr, sink, pol, obs)
}
```

Bring your own adapters (QuestDB, DLQ, rate-limiters) by implementing the port interfaces under `internal/ports`. When you are ready to share the library outside the repo, move reusable packages from `internal` to `pkg/` or a top-level module path.

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

“Industrial resilience is not a chapter—it’s a feature.” AegisFlow Direct exists so your research narrative and your production roadmap stay aligned. Plug in your PLCs, watch the WAL fill with real telemetry, and build the dashboards that will convince both professors and plant managers. Let’s make broker-free ingest boringly reliable. 💡
