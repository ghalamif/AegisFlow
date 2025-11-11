package ports

import "github.com/ghalamif/AegisFlow/internal/domain"

type Collector interface {
	Start(out chan<- *domain.Sample) error
	Stop() error
}
