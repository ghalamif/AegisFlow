package ports

import "aegisflow/internal/domain"

type Transformer interface {
	Transform(*domain.Sample) (*domain.Sample, error)
	Version() uint16
}
