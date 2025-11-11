package ports

import "github.com/ghalamif/AegisFlow/internal/domain"

type Transformer interface {
	Transform(*domain.Sample) (*domain.Sample, error)
	Version() uint16
}
