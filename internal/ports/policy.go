package ports

import "time"

type Policy struct {
	MaxWALSizeBytes int64
	MaxQueueLen     int
	MaxBatchSize    int
	IdleSleep       time.Duration

	OnWALFull   string // "reject", "drop_low_priority", "block"
	OnQueueFull string // "reject", "block", "drop"
}
