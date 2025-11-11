package aegisflow

import (
	"github.com/ghalamif/AegisFlow/internal/domain"
	"github.com/ghalamif/AegisFlow/internal/ports"
)

// PipelineSample is the data structure that flows through the WAL→queue→sink pipeline.
// It mirrors internal/domain.Sample but is exported so custom adapters can reference it.
type PipelineSample = domain.Sample

// QueuedSample represents an item buffered inside the bounded queue.
type QueuedSample = ports.QueuedSample

// Collector streams samples from any data source (OPC UA, Modbus, MQTT, etc.) into the pipeline.
type Collector = ports.Collector

// SampleQueue is the bounded, in-memory queue that decouples the collector and sink.
type SampleQueue = ports.SampleQueue

// Transformer lets callers mutate samples (unit conversion, calibration, enrichment) before persistence.
type Transformer = ports.Transformer

// Sink consumes batches of samples and persists them to any downstream system.
type Sink = ports.Sink

// Observability emits metrics/logs about throughput, latency, and DLQ conditions.
type Observability = ports.Observability

// Field is a structured log/metric field used by Observability implementations.
type Field = ports.Field

// WAL abstracts the write-ahead log used for durability and crash recovery.
type WAL = ports.WAL

// WALStats exposes WAL metadata for observability.
type WALStats = ports.WALStats

// WALEntryID uniquely identifies a WAL entry.
type WALEntryID = ports.WALEntryID
