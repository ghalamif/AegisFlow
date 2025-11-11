package aegisflow

import (
	"github.com/ghalamif/AegisFlow/internal/adapters/opcua"
	"github.com/ghalamif/AegisFlow/internal/app/config"
	"github.com/ghalamif/AegisFlow/internal/ports"
)

// Config re-exports the root configuration struct so downstream projects can
// construct or modify it programmatically.
type Config = config.Config

type (
	// Policy controls WAL/queue thresholds.
	Policy = ports.Policy
	// OPCUAConfig holds connection + node details.
	OPCUAConfig = opcua.Config
	// OPCUANodeConfig describes a monitored tag.
	OPCUANodeConfig = opcua.NodeConfig
	// TimescaleConfig configures the sink.
	TimescaleConfig = config.TimescaleConfig
	// MetricsConfig configures the metrics HTTP server.
	MetricsConfig = config.MetricsConfig
	// WALConfig configures on-disk durability.
	WALConfig = config.WALConfig
)

// LoadConfig loads YAML from disk using the internal config reader.
func LoadConfig(path string) (*Config, error) {
	return config.Load(path)
}
