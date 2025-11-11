package ports

import "github.com/ghalamif/AegisFlow/internal/domain"

type Observability interface {
	LogInfo(msg string, fields ...Field)
	LogError(msg string, err error, fields ...Field)
	LogCritical(msg string, err error, fields ...Field)

	IncCounter(name string, v float64)
	ObserveLatency(name string, seconds float64)

	SetGauge(name string, v float64)

	RecordDLQ(id WALEntryID, s *domain.Sample, err error)
}

type Field struct {
	Key   string
	Value any
}
