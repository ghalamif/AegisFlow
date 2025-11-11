package config

import (
	"fmt"
	"os"
	"time"

	"github.com/ghalamif/AegisFlow/internal/adapters/opcua"
	"github.com/ghalamif/AegisFlow/internal/ports"
	"gopkg.in/yaml.v3"
)

type Config struct {
	Policy    ports.Policy    `yaml:"policy"`
	OPCUA     opcua.Config    `yaml:"opcua"`
	Timescale TimescaleConfig `yaml:"timescale"`
	Metrics   MetricsConfig   `yaml:"metrics"`
	WAL       WALConfig       `yaml:"wal"`
}

type TimescaleConfig struct {
	ConnString string `yaml:"conn_string"`
	Table      string `yaml:"table"`
}

type MetricsConfig struct {
	Addr string `yaml:"addr"`
}

type WALConfig struct {
	Dir string `yaml:"dir"`
}

func Load(path string) (*Config, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := yaml.Unmarshal(raw, &cfg); err != nil {
		return nil, err
	}

	cfg.applyDefaults()
	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func (c *Config) applyDefaults() {
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
	if c.Metrics.Addr == "" {
		c.Metrics.Addr = ":9100"
	}
	if c.Timescale.Table == "" {
		c.Timescale.Table = "samples"
	}
	if c.WAL.Dir == "" {
		c.WAL.Dir = "./data/wal"
	}

	c.OPCUA.ApplyDefaults()
}

func (c *Config) validate() error {
	if err := c.OPCUA.Validate(); err != nil {
		return fmt.Errorf("opcua config: %w", err)
	}
	if c.Timescale.ConnString == "" {
		return fmt.Errorf("timescale.conn_string is required")
	}
	if c.Metrics.Addr == "" {
		return fmt.Errorf("metrics.addr is required")
	}
	if c.WAL.Dir == "" {
		return fmt.Errorf("wal.dir is required")
	}
	return nil
}
