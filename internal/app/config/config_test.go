package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadAppliesDefaults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	data := `
policy:
  max_queue_len: 1000
opcua:
  endpoint: opc.tcp://localhost:4840
  nodes:
    - node_id: "ns=2;s=Demo.Dynamic.Scalar.Double"
timescale:
  conn_string: "postgres://user:pass@localhost/db?sslmode=disable"
`
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if cfg.Policy.IdleSleep != 5*time.Millisecond {
		t.Fatalf("expected IdleSleep default 5ms, got %s", cfg.Policy.IdleSleep)
	}
	if cfg.Policy.MaxBatchSize != 5000 {
		t.Fatalf("expected MaxBatchSize default 5000, got %d", cfg.Policy.MaxBatchSize)
	}
	if cfg.Metrics.Addr != ":9100" {
		t.Fatalf("expected default metrics addr :9100, got %s", cfg.Metrics.Addr)
	}
	if cfg.WAL.Dir != "./data/wal" {
		t.Fatalf("expected default wal dir ./data/wal, got %s", cfg.WAL.Dir)
	}
	if cfg.OPCUA.Nodes[0].SensorID != "ns=2;s=Demo.Dynamic.Scalar.Double" {
		t.Fatalf("expected sensor ID fallback to node ID, got %s", cfg.OPCUA.Nodes[0].SensorID)
	}
}
