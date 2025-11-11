package ports

import "aegisflow/internal/domain"

type WALEntryID uint64

type WAL interface {
	Append(s *domain.Sample) (WALEntryID, error)
	Iterate(from WALEntryID, fn func(id WALEntryID, s *domain.Sample) error) error
	Commit(upto WALEntryID) error
	TruncateCommitted() error
	Stats() WALStats
}

type WALStats struct {
	OldestUncommitted WALEntryID
	LatestAppended    WALEntryID
	SizeBytes         int64
}
