package ports

import "github.com/ghalamif/AegisFlow/internal/domain"

type QueuedSample struct {
	ID     WALEntryID
	Sample *domain.Sample
}

type SampleQueue interface {
	Enqueue(id WALEntryID, s *domain.Sample) bool
	DequeueBatch(max int) []QueuedSample
	Len() int
}
