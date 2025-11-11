package wal

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ghalamif/AegisFlow/internal/domain"
	"github.com/ghalamif/AegisFlow/internal/ports"
)

func TestFileWALAppendIterateAndReplay(t *testing.T) {
	dir := t.TempDir()

	w, err := NewFileWAL(dir)
	if err != nil {
		t.Fatalf("new wal: %v", err)
	}

	s1 := &domain.Sample{SensorID: "sensor-1"}
	s2 := &domain.Sample{SensorID: "sensor-2"}

	id1, err := w.Append(s1)
	if err != nil || id1 == 0 {
		t.Fatalf("append sample 1: %v id=%d", err, id1)
	}
	id2, err := w.Append(s2)
	if err != nil || id2 == 0 {
		t.Fatalf("append sample 2: %v id=%d", err, id2)
	}

	if err := w.writer.Flush(); err != nil {
		t.Fatalf("flush: %v", err)
	}

	var iterated []string
	if err := w.Iterate(1, func(id ports.WALEntryID, s *domain.Sample) error {
		iterated = append(iterated, s.SensorID)
		return nil
	}); err != nil {
		t.Fatalf("iterate: %v", err)
	}
	if len(iterated) != 2 {
		t.Fatalf("expected 2 samples, got %d", len(iterated))
	}

	if err := w.Commit(id2); err != nil {
		t.Fatalf("commit: %v", err)
	}

	if err := w.file.Close(); err != nil {
		t.Fatalf("close wal: %v", err)
	}

	// Reopen and ensure committed metadata was persisted.
	w2, err := NewFileWAL(dir)
	if err != nil {
		t.Fatalf("reopen wal: %v", err)
	}
	defer w2.file.Close()

	stats := w2.Stats()
	if stats.LatestAppended != id2 {
		t.Fatalf("expected latest appended %d, got %d", id2, stats.LatestAppended)
	}
	if stats.OldestUncommitted != id2+1 {
		t.Fatalf("expected oldest uncommitted %d, got %d", id2+1, stats.OldestUncommitted)
	}

	// Ensure truncation handles partial writes by manually corrupting the log.
	path := filepath.Join(dir, "wal.log")
	if err := appendGarbage(path); err != nil {
		t.Fatalf("append garbage: %v", err)
	}

	if err := w2.file.Close(); err != nil {
		t.Fatalf("close wal2: %v", err)
	}

	if _, err := NewFileWAL(dir); err != nil {
		t.Fatalf("reopen after garbage: %v", err)
	}
}

func appendGarbage(path string) error {
	f, err := openAppend(path)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := f.Write([]byte{0xFF, 0xAA}); err != nil {
		return err
	}
	return nil
}

func openAppend(path string) (*os.File, error) {
	return os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0o644)
}
