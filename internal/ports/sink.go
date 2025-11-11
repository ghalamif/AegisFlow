package ports

import "aegisflow/internal/domain"

type Sink interface {
	WriteBatch(samples []*domain.Sample) error
	Name() string
}
