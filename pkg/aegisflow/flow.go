package aegisflow

import (
	"context"
	"fmt"
)

// Flow is a convenience builder that lets callers say Conf → StreamIN → StreamOUT
// without touching the underlying hexagonal wiring.
type Flow struct {
	cfg  *Config
	opts []EdgeRuntimeOption
}

// FlowOption mutates the Flow after configuration is loaded.
type FlowOption func(*Flow)

// StreamInOption configures the collector/WAL/queue side of the pipeline.
type StreamInOption func(*Flow)

// StreamOutOption configures the sink/transformer/observability side of the pipeline.
type StreamOutOption func(*Flow)

// Conf loads YAML from disk, applies FlowOption values, and returns a Flow builder.
func Conf(path string, opts ...FlowOption) (*Flow, error) {
	cfg, err := LoadConfig(path)
	if err != nil {
		return nil, err
	}
	return ConfFromConfig(cfg, opts...)
}

// ConfFromConfig bootstraps a Flow from an in-memory Config.
func ConfFromConfig(cfg *Config, opts ...FlowOption) (*Flow, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is required")
	}
	f := &Flow{cfg: cfg}
	for _, opt := range opts {
		if opt != nil {
			opt(f)
		}
	}
	return f, nil
}

// Config returns the underlying configuration so callers can tweak it before building a runtime.
func (f *Flow) Config() *Config {
	if f == nil {
		return nil
	}
	return f.cfg
}

// Options appends raw EdgeRuntimeOption values to the builder for advanced scenarios.
func (f *Flow) Options(opts ...EdgeRuntimeOption) *Flow {
	if f == nil {
		return nil
	}
	f.appendOptions(opts...)
	return f
}

// StreamIN records collector-side overrides (collector, WAL, queue, observability).
func (f *Flow) StreamIN(opts ...StreamInOption) *Flow {
	if f == nil {
		return nil
	}
	for _, opt := range opts {
		if opt != nil {
			opt(f)
		}
	}
	return f
}

// StreamOUT records sink-side overrides and builds an EdgeRuntime ready to run.
func (f *Flow) StreamOUT(opts ...StreamOutOption) (*EdgeRuntime, error) {
	if f == nil {
		return nil, fmt.Errorf("flow is nil")
	}
	for _, opt := range opts {
		if opt != nil {
			opt(f)
		}
	}
	return NewEdgeRuntime(f.cfg, f.opts...)
}

// Run is a shortcut for StreamOUT + runtime.Run.
func (f *Flow) Run(ctx context.Context, opts ...StreamOutOption) error {
	rt, err := f.StreamOUT(opts...)
	if err != nil {
		return err
	}
	return rt.Run(ctx)
}

// WithFlowOptions appends EdgeRuntimeOption values during Conf.
func WithFlowOptions(opts ...EdgeRuntimeOption) FlowOption {
	return func(f *Flow) {
		if f != nil {
			f.appendOptions(opts...)
		}
	}
}

// StreamInCollector injects a custom collector (MQTT, Modbus, simulators, etc.).
func StreamInCollector(col Collector) StreamInOption {
	return func(f *Flow) {
		if f != nil && col != nil {
			f.appendOptions(WithCollector(col))
		}
	}
}

// StreamInQueue swaps the in-memory queue for a caller-provided implementation.
func StreamInQueue(q SampleQueue) StreamInOption {
	return func(f *Flow) {
		if f != nil && q != nil {
			f.appendOptions(WithSampleQueue(q))
		}
	}
}

// StreamInWAL lets callers bring their own WAL implementation.
func StreamInWAL(w WAL) StreamInOption {
	return func(f *Flow) {
		if f != nil && w != nil {
			f.appendOptions(WithWAL(w))
		}
	}
}

// StreamInObservability overrides the default Prometheus-based observability stack.
func StreamInObservability(obs Observability) StreamInOption {
	return func(f *Flow) {
		if f != nil && obs != nil {
			f.appendOptions(WithObservability(obs))
		}
	}
}

// StreamOutSink injects a custom ports.Sink implementation.
func StreamOutSink(s Sink) StreamOutOption {
	return func(f *Flow) {
		if f != nil && s != nil {
			f.appendOptions(WithSink(s))
		}
	}
}

// StreamOutTransformer overrides the default no-op transformer before data hits the sink.
func StreamOutTransformer(tr Transformer) StreamOutOption {
	return func(f *Flow) {
		if f != nil && tr != nil {
			f.appendOptions(WithTransformer(tr))
		}
	}
}

// StreamOutObservability replaces the default observability backend.
func StreamOutObservability(obs Observability) StreamOutOption {
	return func(f *Flow) {
		if f != nil && obs != nil {
			f.appendOptions(WithObservability(obs))
		}
	}
}

// StreamOutCallback installs a sink built from a simple callback function.
func StreamOutCallback(name string, fn SampleBatchSink) StreamOutOption {
	return func(f *Flow) {
		if f != nil {
			f.appendOptions(WithSink(NewCallbackSink(name, fn)))
		}
	}
}

func (f *Flow) appendOptions(opts ...EdgeRuntimeOption) {
	for _, opt := range opts {
		if opt != nil {
			f.opts = append(f.opts, opt)
		}
	}
}
