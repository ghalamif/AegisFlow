package wal

import (
	"bufio"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"aegisflow/internal/domain"
	"aegisflow/internal/ports"
)

const recordHeaderLen = 12

type FileWAL struct {
	mu        sync.Mutex
	path      string
	metaPath  string
	file      *os.File
	writer    *bufio.Writer
	nextID    ports.WALEntryID
	committed ports.WALEntryID
	sizeBytes int64
}

func NewFileWAL(dir string) (*FileWAL, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	path := filepath.Join(dir, "wal.log")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0o644)
	if err != nil {
		return nil, err
	}
	w := bufio.NewWriterSize(f, 1<<20)

	wal := &FileWAL{
		path:     path,
		metaPath: filepath.Join(dir, "wal.meta"),
		file:     f,
		writer:   w,
	}
	if err := wal.bootstrap(); err != nil {
		return nil, err
	}
	return wal, nil
}

func (w *FileWAL) bootstrap() error {
	if err := w.scanExisting(); err != nil {
		return err
	}
	if err := w.loadCommitted(); err != nil {
		return err
	}
	if w.nextID < w.committed {
		w.nextID = w.committed
	}
	_, err := w.file.Seek(0, io.SeekEnd)
	return err
}

func (w *FileWAL) scanExisting() error {
	stat, err := os.Stat(w.path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err != nil || stat.Size() == 0 {
		return nil
	}

	rf, err := os.Open(w.path)
	if err != nil {
		return err
	}
	defer rf.Close()

	reader := bufio.NewReader(rf)
	var (
		offset int64
		lastID ports.WALEntryID
	)

	for {
		var hdr [recordHeaderLen]byte
		if _, err := io.ReadFull(reader, hdr[:]); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			if errors.Is(err, io.ErrUnexpectedEOF) {
				if err := w.file.Truncate(offset); err != nil {
					return err
				}
				break
			}
			return fmt.Errorf("wal scan header: %w", err)
		}
		id := ports.WALEntryID(binary.BigEndian.Uint64(hdr[0:8]))
		length := binary.BigEndian.Uint32(hdr[8:12])
		offset += recordHeaderLen

		if length > 0 {
			if _, err := io.CopyN(io.Discard, reader, int64(length)); err != nil {
				if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
					if err := w.file.Truncate(offset); err != nil {
						return err
					}
					break
				}
				return fmt.Errorf("wal scan body: %w", err)
			}
			offset += int64(length)
		}
		lastID = id
	}

	if err := w.file.Truncate(offset); err != nil {
		return err
	}
	w.sizeBytes = offset
	w.nextID = lastID
	return nil
}

func (w *FileWAL) loadCommitted() error {
	data, err := os.ReadFile(w.metaPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	val := strings.TrimSpace(string(data))
	if val == "" {
		return nil
	}
	u, err := strconv.ParseUint(val, 10, 64)
	if err != nil {
		return fmt.Errorf("wal meta parse: %w", err)
	}
	w.committed = ports.WALEntryID(u)
	return nil
}

func (w *FileWAL) Append(s *domain.Sample) (ports.WALEntryID, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	id := w.nextID + 1

	b, err := json.Marshal(s)
	if err != nil {
		return 0, err
	}

	// entry format: [8 bytes id][4 bytes len][len bytes json]
	var hdr [recordHeaderLen]byte
	binary.BigEndian.PutUint64(hdr[0:8], uint64(id))
	binary.BigEndian.PutUint32(hdr[8:12], uint32(len(b)))

	if _, err := w.writer.Write(hdr[:]); err != nil {
		return 0, err
	}
	if _, err := w.writer.Write(b); err != nil {
		return 0, err
	}

	// group commit is allowed; fsync handled externally or on thresholds
	w.nextID = id
	w.sizeBytes += int64(len(b) + len(hdr))

	return id, nil
}

func (w *FileWAL) Iterate(from ports.WALEntryID, fn func(id ports.WALEntryID, s *domain.Sample) error) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if err := w.writer.Flush(); err != nil {
		return err
	}

	f, err := os.Open(w.path)
	if err != nil {
		return err
	}
	defer f.Close()

	r := bufio.NewReader(f)

	for {
		var hdr [recordHeaderLen]byte
		if _, err := io.ReadFull(r, hdr[:]); err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			if errors.Is(err, io.ErrUnexpectedEOF) {
				return fmt.Errorf("wal iterate truncated header: %w", err)
			}
			return err
		}
		id := ports.WALEntryID(binary.BigEndian.Uint64(hdr[0:8]))
		l := binary.BigEndian.Uint32(hdr[8:12])

		b := make([]byte, l)
		if _, err := io.ReadFull(r, b); err != nil {
			if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
				return fmt.Errorf("corrupt WAL: %w", err)
			}
			return fmt.Errorf("corrupt WAL: %w", err)
		}
		if id < from {
			continue
		}

		var s domain.Sample
		if err := json.Unmarshal(b, &s); err != nil {
			return fmt.Errorf("corrupt WAL entry: %w", err)
		}
		if err := fn(id, &s); err != nil {
			return err
		}
	}
}

func (w *FileWAL) Commit(upto ports.WALEntryID) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if upto > w.committed {
		w.committed = upto
	}
	return w.persistMetaLocked()
}

func (w *FileWAL) TruncateCommitted() error {
	// v0: keep simple; real impl: rewrite from committed+1.
	return nil
}

func (w *FileWAL) Stats() ports.WALStats {
	w.mu.Lock()
	defer w.mu.Unlock()
	return ports.WALStats{
		OldestUncommitted: w.committed + 1,
		LatestAppended:    w.nextID,
		SizeBytes:         w.sizeBytes,
	}
}

func (w *FileWAL) persistMetaLocked() error {
	data := []byte(fmt.Sprintf("%d\n", w.committed))
	return os.WriteFile(w.metaPath, data, 0o644)
}
