package aegisflow

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/ghalamif/AegisFlow/internal/adapters/observability"
	"github.com/ghalamif/AegisFlow/internal/adapters/queue"
	"github.com/ghalamif/AegisFlow/internal/adapters/wal"
	"github.com/ghalamif/AegisFlow/internal/domain"
	"github.com/ghalamif/AegisFlow/internal/ports"
)

// ErrQueueFull indicates the in-memory queue rejected the sample according to policy.
var ErrQueueFull = errors.New("aegisflow: queue full")

// ErrWALFull indicates the WAL is at capacity and OnWALFull != "block".
var ErrWALFull = errors.New("aegisflow: wal full")

// Sample mirrors the internal domain.Sample but is safe for external callers.
type Sample struct {
	SensorID     string
	Timestamp    time.Time
	Seq          uint64
	Values       map[string]float64
	SourceNodeID string
	TransformVer uint16
}

// SampleBatchSink is invoked with ordered batches dequeued from the pipeline.
type SampleBatchSink func([]Sample) error

// ExternalPublisherConfig configures the WAL-backed publisher used by callers.
type ExternalPublisherConfig struct {
	Policy Policy
	WAL    WALConfig
}

// applyDefaults fills in sane thresholds so callers only override what they need.
func (c *ExternalPublisherConfig) applyDefaults() {
	if c.Policy.MaxWALSizeBytes == 0 {
		c.Policy.MaxWALSizeBytes = 10 << 30
	}
	if c.Policy.MaxQueueLen == 0 {
		c.Policy.MaxQueueLen = 100_000
	}
	if c.Policy.MaxBatchSize == 0 {
		c.Policy.MaxBatchSize = 5_000
	}
	if c.Policy.IdleSleep == 0 {
		c.Policy.IdleSleep = 5 * time.Millisecond
	}
	if c.Policy.OnQueueFull == "" {
		c.Policy.OnQueueFull = "block"
	}
	if c.Policy.OnWALFull == "" {
		c.Policy.OnWALFull = "block"
	}
	if c.WAL.Dir == "" {
		c.WAL.Dir = "./data/aegisflow-wal"
	}
}

func (c *ExternalPublisherConfig) validate() error {
	if c.WAL.Dir == "" {
		return fmt.Errorf("wal.dir is required")
	}
	if c.Policy.MaxQueueLen <= 0 {
		return fmt.Errorf("policy.max_queue_len must be > 0")
	}
	if c.Policy.MaxBatchSize <= 0 {
		return fmt.Errorf("policy.max_batch_size must be > 0")
	}
	return nil
}

// ExternalPublisher exposes the WAL→queue→sink pipeline to external producers.
type ExternalPublisher struct {
	policy      Policy
	wal         ports.WAL
	queue       ports.SampleQueue
	obs         ports.Observability
	transformer ports.Transformer
	sink        SampleBatchSink

	stopCh   chan struct{}
	doneCh   chan struct{}
	stopOnce sync.Once
}

// NewExternalPublisher wires a WAL + bounded queue + sink callback so callers can
// push arbitrary samples while reusing the durability/backpressure policies.
func NewExternalPublisher(cfg *ExternalPublisherConfig, sink SampleBatchSink) (*ExternalPublisher, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is required")
	}
	if sink == nil {
		return nil, fmt.Errorf("sink callback is required")
	}
	cfg.applyDefaults()
	if err := cfg.validate(); err != nil {
		return nil, err
	}

	walAdapter, err := wal.NewFileWAL(cfg.WAL.Dir)
	if err != nil {
		return nil, err
	}
	q := queue.NewMemQueue(cfg.Policy.MaxQueueLen)
	obs := observability.NewPromObs()

	if err := replayWALIntoQueue(walAdapter, q, cfg.Policy, obs); err != nil {
		return nil, err
	}

	pub := &ExternalPublisher{
		policy:      cfg.Policy,
		wal:         walAdapter,
		queue:       q,
		obs:         obs,
		transformer: &noopTransformer{},
		sink:        sink,
		stopCh:      make(chan struct{}),
		doneCh:      make(chan struct{}),
	}

	go pub.runIngest()
	return pub, nil
}

// Publish appends the sample to the WAL and enqueues it according to policy.
func (p *ExternalPublisher) Publish(sample Sample) error {
	dom := sample.toDomain()

	if !waitForLocalWALCapacity(p.wal, p.policy, p.obs) {
		return ErrWALFull
	}

	id, err := p.wal.Append(dom)
	if err != nil {
		return err
	}

	if !enqueueWithLocalPolicy(p.queue, id, dom, p.policy, p.obs) {
		return ErrQueueFull
	}
	return nil
}

// Close waits for the ingest loop to exit, respecting the provided context.
func (p *ExternalPublisher) Close(ctx context.Context) error {
	p.stopOnce.Do(func() {
		close(p.stopCh)
	})

	select {
	case <-p.doneCh:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (p *ExternalPublisher) runIngest() {
	defer close(p.doneCh)
	idle := p.policy.IdleSleep
	if idle <= 0 {
		idle = 5 * time.Millisecond
	}

	for {
		select {
		case <-p.stopCh:
			return
		default:
		}

		batch := p.queue.DequeueBatch(p.policy.MaxBatchSize)
		if len(batch) == 0 {
			time.Sleep(idle)
			continue
		}

		var (
			converted = make([]Sample, 0, len(batch))
			maxID     ports.WALEntryID
		)

		for _, item := range batch {
			s, err := p.transformer.Transform(item.Sample)
			if err != nil {
				p.obs.RecordDLQ(item.ID, item.Sample, err)
				continue
			}
			s.TransformVer = p.transformer.Version()
			converted = append(converted, sampleFromDomain(s))
			if item.ID > maxID {
				maxID = item.ID
			}
		}

		if len(converted) == 0 {
			_ = p.wal.Commit(maxID)
			continue
		}

		if err := p.sink(converted); err != nil {
			p.obs.LogError("external_sink_failed", err)
			time.Sleep(idle)
			continue
		}

		p.obs.IncCounter("aegis_samples_ingested_total", float64(len(converted)))
		if err := p.wal.Commit(maxID); err != nil {
			p.obs.LogError("wal_commit_failed", err)
		}
	}
}

func (s Sample) toDomain() *domain.Sample {
	return &domain.Sample{
		SensorID:     s.SensorID,
		Timestamp:    s.Timestamp,
		Seq:          s.Seq,
		Values:       copyValues(s.Values),
		SourceNodeID: s.SourceNodeID,
		TransformVer: s.TransformVer,
	}
}

func sampleFromDomain(s *domain.Sample) Sample {
	return Sample{
		SensorID:     s.SensorID,
		Timestamp:    s.Timestamp,
		Seq:          s.Seq,
		Values:       copyValues(s.Values),
		SourceNodeID: s.SourceNodeID,
		TransformVer: s.TransformVer,
	}
}

func copyValues(src map[string]float64) map[string]float64 {
	if len(src) == 0 {
		return nil
	}
	dst := make(map[string]float64, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

func waitForLocalWALCapacity(wal ports.WAL, pol ports.Policy, obs ports.Observability) bool {
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

func enqueueWithLocalPolicy(q ports.SampleQueue, id ports.WALEntryID, s *domain.Sample, pol ports.Policy, obs ports.Observability) bool {
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
