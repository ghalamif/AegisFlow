# AegisFlow Field Guide

Welcome—this guide is designed to walk you through *every* AegisFlow feature using the same clarity you would expect in an IELTS-style explanation: we describe the concept, illustrate the flow, and then tell you how to act on it.

---

## 1. Mental Model: From Sensor to Sink

Before writing a single line of code, picture the journey your telemetry takes:

```mermaid
%%{init: {'theme': 'base', 'themeVariables': {'primaryColor': 'var(--color-canvas-default)', 'primaryTextColor': 'var(--color-fg-default)', 'lineColor': 'var(--color-border-default)', 'secondaryColor': 'var(--color-accent-subtle)'}}}%%
flowchart LR
    OPCUA["Collector<br/>OPC UA / MQTT / etc."] --> WAL["Write-Ahead Log"]
    WAL --> Queue["Bounded Queue"]
    Queue --> Transformer["Transformer Hook"]
    Transformer --> Sink["Sink · Timescale / Callback / Channel"]
    Sink --> Dest["Any DB / API"]
```

Every feature you will see maps directly onto one of these stages.

---

## 2. Configuration: Teaching AegisFlow About Your Plant

**What it is:** YAML that describes backpressure policies, OPC UA endpoints, Timescale credentials, metrics, and WAL paths (`data/config.yaml`).

**Why it matters:** It lets you swap PLC nodes, durability limits, or database tables without recompiling.

**How to use:**

```yaml
policy:
  max_queue_len: 100000
  on_queue_full: block

opcua:
  endpoint: opc.tcp://plc:4840
  nodes:
    - node_id: "ns=2;s=Temp.Sensor"
      sensor_id: temp_sensor

timescale:
  conn_string: "postgres://user:pass@localhost:5432/aegis?sslmode=disable"
```

```go
cfg, err := aegisflow.LoadConfig("data/config.yaml")
// cfg is ready to feed into any runtime or flow
```

IELTS-style tip: always mention *where* and *why*. When briefing teammates, say “Update `data/config.yaml` to add the new PLC node” so they know the exact file and intent.

---

## 3. Flow Builder: Conf → StreamIN → StreamOUT

**What it is:** A fluent API in `pkg/aegisflow/flow.go` that lets you describe your pipeline in plain English steps.

**Why it matters:** New adopters can read the code aloud: “configure, stream in via collector X, stream out via sink Y.”

**How to use:**

```go
flow, _ := aegisflow.Conf("./data/config.yaml")

rt, _ := flow.
    StreamIN(
        aegisflow.StreamInCollector(customCollector), // optional
    ).
    StreamOUT(
        aegisflow.StreamOutCallback("printer", printBatch),
    )

ctx, cancel := context.WithCancel(context.Background())
defer cancel()
_ = rt.Run(ctx)
```

You can shrink it further:

```go
_ = flow.StreamIN().Run(ctx, aegisflow.StreamOutSink(timescaleSink))
```

---

## 4. Default Edge Runtime: Industrial Strength Out of the Box

**What it is:** `EdgeRuntime` (`pkg/aegisflow/edge.go`) wires the production-ready OPC UA collector, file WAL, in-memory queue, transformer hook, and Timescale sink.

**Why it matters:** You get guaranteed delivery, WAL replay, and Prometheus metrics the moment you type `go run ./cmd/aegis-edge`.

**How to customize:** pass `EdgeRuntimeOption` values:

```go
rt, _ := aegisflow.NewEdgeRuntime(
    cfg,
    aegisflow.WithCollector(mqttCollector),
    aegisflow.WithSink(customSink),
    aegisflow.WithTransformer(customTransformer),
)
```

---

## 5. Sink Adapters: Bridging to Unknown Databases

When your destination database or API is undecided, use the lightweight adapters in `pkg/aegisflow/sink_adapters.go`.

### Callback Sink

```go
printer := aegisflow.NewCallbackSink("stdout", func(batch []aegisflow.Sample) error {
    for _, s := range batch {
        fmt.Printf("%s %s => %+v\n", s.Timestamp, s.SensorID, s.Values)
    }
    return nil
})

flow.Run(ctx, aegisflow.StreamOutSink(printer))
```

### Channel Sink

```go
chanSink, batches, closeFn := aegisflow.NewChannelSink("fanout", 128)
defer closeFn()

go func() {
    for batch := range batches {
        forwardToFutureDB(batch)
    }
}()

flow.Run(ctx, aegisflow.StreamOutSink(chanSink))
```

Mermaid view:

```mermaid
%%{init: {'theme': 'base', 'themeVariables': {'primaryColor': 'var(--color-canvas-default)', 'primaryTextColor': 'var(--color-fg-default)', 'lineColor': 'var(--color-border-default)', 'secondaryColor': 'var(--color-accent-subtle)'}}}%%
flowchart LR
    Queue["Bounded Queue"] --> ChannelSink["Channel Sink"]
    ChannelSink --> WorkerA["Worker A"]
    ChannelSink --> WorkerB["Worker B"]
```

---

## 6. External Publisher: Bring Your Own Data Source

**What it is:** `ExternalPublisher` (`pkg/aegisflow/external.go`) exposes the WAL → queue → sink durability to *your* producers, even if they are not OPC UA.

```go
pub, _ := aegisflow.NewExternalPublisher(&aegisflow.ExternalPublisherConfig{
    Policy: cfg.Policy,
    WAL:    cfg.WAL,
}, func(batch []aegisflow.Sample) error {
    return sendToKafka(batch)
})

_ = pub.Publish(aegisflow.Sample{
    SensorID:  "custom",
    Timestamp: time.Now(),
    Values:    map[string]float64{"value": 42},
})
```

Use this when you already have a collector but still want WAL durability and backpressure.

---

## 7. Observability & Health

**Metrics:** Every runtime exposes `/metrics` and `/healthz` (see `pkg/aegisflow/edge.go:139-188`). Counters include `aegis_samples_ingested_total`, queue length, WAL size, and ingest latency.

**Custom instrumentation:** Use `StreamInObservability` / `StreamOutObservability` to inject OpenTelemetry or your own logging layer.

```go
flow.StreamIN(
    aegisflow.StreamInObservability(myObs),
).StreamOUT(
    aegisflow.StreamOutObservability(myObs),
)
```

IELTS tip: always mention evidence—tell stakeholders, “Check `/metrics` for `aegis_queue_length` to confirm backpressure behaviour.”

---

## 8. Disaster Readiness: Policies, WAL Replay, DLQ

*Policies:* configure `policy.on_queue_full` / `on_wal_full` to `block`, `drop`, or `reject`.

*Replay:* `replayWALIntoQueue` ensures that after a crash, every uncommitted entry re-enters the queue before new data flows.

*DLQ Hook:* Transformers can return an error; samples then land in the observability DLQ via `RecordDLQ`.

This triad lets you state, “Even if power fails mid-burst, AegisFlow reloads the WAL and the queue resumes without data loss.”

---

## 9. Hands-On Examples

Browse [`example/`](../example/README.md) for runnable scenarios:

1. **Basic** – OPC UA → Timescale (`go run ./example/basic`)
2. **Callback** – OPC UA → custom callback → unknown DB
3. **Channel** – OPC UA → channel sink → multiple workers

Each example is paired with a Mermaid diagram so you can see the architecture before running it.

---

## 10. Next Steps

- Swap the collector with MQTT, Modbus, or file replayers by implementing `aegisflow.Collector`.
- Write a custom `Sink` when your target DB stabilises—use the callback sink as scaffolding.
- Extend observability with `StreamOutObservability` to plug into OpenTelemetry.
- Contribute transformers, DLQs, or config hot-reloaders following the guidelines in `README.md`.

Whenever you introduce AegisFlow to teammates or customers, summarise it like this:

> “Configure once, reuse everywhere: OPC UA feeds a durable WAL, the queue enforces policy, and the sink is whatever database or service we need today—or tomorrow.”

That single sentence keeps everyone aligned with the architecture you just explored.
