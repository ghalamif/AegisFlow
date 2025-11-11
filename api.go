package aegisflow

import (
	base "github.com/ghalamif/AegisFlow/pkg/aegisflow"
)

// Re-exported errors for convenience.
var (
	ErrQueueFull         = base.ErrQueueFull
	ErrWALFull           = base.ErrWALFull
	ErrChannelSinkClosed = base.ErrChannelSinkClosed
)

// Type aliases so consumers can import github.com/ghalamif/AegisFlow directly.
type (
	Config                  = base.Config
	Policy                  = base.Policy
	OPCUAConfig             = base.OPCUAConfig
	OPCUANodeConfig         = base.OPCUANodeConfig
	TimescaleConfig         = base.TimescaleConfig
	MetricsConfig           = base.MetricsConfig
	WALConfig               = base.WALConfig
	Flow                    = base.Flow
	FlowOption              = base.FlowOption
	StreamInOption          = base.StreamInOption
	StreamOutOption         = base.StreamOutOption
	EdgeRuntime             = base.EdgeRuntime
	EdgeRuntimeOption       = base.EdgeRuntimeOption
	Sample                  = base.Sample
	SampleBatchSink         = base.SampleBatchSink
	Collector               = base.Collector
	Sink                    = base.Sink
	Transformer             = base.Transformer
	SampleQueue             = base.SampleQueue
	WAL                     = base.WAL
	Observability           = base.Observability
	QueuedSample            = base.QueuedSample
	WALEntryID              = base.WALEntryID
	WALStats                = base.WALStats
	ExternalPublisher       = base.ExternalPublisher
	ExternalPublisherConfig = base.ExternalPublisherConfig
)

// Config helpers.
func LoadConfig(path string) (*Config, error) {
	return base.LoadConfig(path)
}

// Flow builder helpers.
func Conf(path string, opts ...FlowOption) (*Flow, error) {
	return base.Conf(path, opts...)
}

func ConfFromConfig(cfg *Config, opts ...FlowOption) (*Flow, error) {
	return base.ConfFromConfig(cfg, opts...)
}

func WithFlowOptions(opts ...EdgeRuntimeOption) FlowOption {
	return base.WithFlowOptions(opts...)
}

func StreamInCollector(col Collector) StreamInOption {
	return base.StreamInCollector(col)
}

func StreamInQueue(q SampleQueue) StreamInOption {
	return base.StreamInQueue(q)
}

func StreamInWAL(w WAL) StreamInOption {
	return base.StreamInWAL(w)
}

func StreamInObservability(obs Observability) StreamInOption {
	return base.StreamInObservability(obs)
}

func StreamOutSink(s Sink) StreamOutOption {
	return base.StreamOutSink(s)
}

func StreamOutTransformer(tr Transformer) StreamOutOption {
	return base.StreamOutTransformer(tr)
}

func StreamOutObservability(obs Observability) StreamOutOption {
	return base.StreamOutObservability(obs)
}

func StreamOutCallback(name string, fn SampleBatchSink) StreamOutOption {
	return base.StreamOutCallback(name, fn)
}

// Edge runtime and options.
func NewEdgeRuntime(cfg *Config, opts ...EdgeRuntimeOption) (*EdgeRuntime, error) {
	return base.NewEdgeRuntime(cfg, opts...)
}

func WithCollector(col Collector) EdgeRuntimeOption {
	return base.WithCollector(col)
}

func WithSink(s Sink) EdgeRuntimeOption {
	return base.WithSink(s)
}

func WithTransformer(tr Transformer) EdgeRuntimeOption {
	return base.WithTransformer(tr)
}

func WithWAL(w WAL) EdgeRuntimeOption {
	return base.WithWAL(w)
}

func WithSampleQueue(q SampleQueue) EdgeRuntimeOption {
	return base.WithSampleQueue(q)
}

func WithObservability(obs Observability) EdgeRuntimeOption {
	return base.WithObservability(obs)
}

// Sink adapters.
func NewCallbackSink(name string, fn SampleBatchSink) Sink {
	return base.NewCallbackSink(name, fn)
}

func NewChannelSink(name string, buffer int) (Sink, <-chan []Sample, func()) {
	return base.NewChannelSink(name, buffer)
}

// External publisher.
func NewExternalPublisher(cfg *ExternalPublisherConfig, sink SampleBatchSink) (*ExternalPublisher, error) {
	return base.NewExternalPublisher(cfg, sink)
}
