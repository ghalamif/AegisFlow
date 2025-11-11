package ports

import "aegisflow/internal/domain"

type Collector interface {
	Start(out chan<- *domain.Sample) error
	Stop() error
}
