package ports

import "github.com/ghalamif/AegisFlow/internal/domain"

type Sink interface {
	WriteBatch(samples []*domain.Sample) error
	Name() string
}
