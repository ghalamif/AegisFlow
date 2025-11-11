package aegisflow

import (
	"errors"
	"fmt"
	"sync"

	"github.com/ghalamif/AegisFlow/internal/domain"
)

// ErrChannelSinkClosed is returned when a channel sink is written to after being closed.
var ErrChannelSinkClosed = errors.New("aegisflow: channel sink closed")

// NewCallbackSink adapts a SampleBatchSink into a full ports.Sink implementation so callers
// can plug arbitrary functions without defining structs.
func NewCallbackSink(name string, fn SampleBatchSink) Sink {
	if name == "" {
		name = "callback"
	}
	return &callbackSink{name: name, fn: fn}
}

// NewChannelSink exposes batches via a channel; it returns the sink, the read-only channel,
// and a close function that the caller should invoke during shutdown.
func NewChannelSink(name string, buffer int) (Sink, <-chan []Sample, func()) {
	if name == "" {
		name = "channel"
	}
	if buffer < 0 {
		buffer = 0
	}
	ch := make(chan []Sample, buffer)
	s := &channelSink{
		name:   name,
		ch:     ch,
		closed: make(chan struct{}),
	}
	return s, ch, func() { s.close() }
}

type callbackSink struct {
	name string
	fn   SampleBatchSink
}

func (s *callbackSink) WriteBatch(samples []*domain.Sample) error {
	if s.fn == nil {
		return fmt.Errorf("callback sink %q: nil handler", s.name)
	}
	if len(samples) == 0 {
		return nil
	}
	return s.fn(convertDomainBatch(samples))
}

func (s *callbackSink) Name() string { return s.name }

type channelSink struct {
	name   string
	ch     chan []Sample
	closed chan struct{}
	once   sync.Once
}

func (s *channelSink) WriteBatch(samples []*domain.Sample) error {
	select {
	case <-s.closed:
		return ErrChannelSinkClosed
	default:
	}

	if len(samples) == 0 {
		return nil
	}

	batch := convertDomainBatch(samples)

	select {
	case <-s.closed:
		return ErrChannelSinkClosed
	case s.ch <- batch:
		return nil
	}
}

func (s *channelSink) Name() string { return s.name }

func (s *channelSink) close() {
	s.once.Do(func() {
		close(s.closed)
		close(s.ch)
	})
}

func convertDomainBatch(samples []*domain.Sample) []Sample {
	if len(samples) == 0 {
		return nil
	}
	out := make([]Sample, len(samples))
	for i, sample := range samples {
		out[i] = sampleFromDomain(sample)
	}
	return out
}
