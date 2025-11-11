package sink

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/ghalamif/AegisFlow/internal/domain"
	"github.com/ghalamif/AegisFlow/internal/ports"
)

type TimescaleSink struct {
	db        *sql.DB
	tableName string
}

func NewTimescaleSink(db *sql.DB, table string) *TimescaleSink {
	return &TimescaleSink{db: db, tableName: table}
}

func (t *TimescaleSink) Name() string { return "timescaledb" }

func (t *TimescaleSink) WriteBatch(samples []*domain.Sample) error {
	if len(samples) == 0 {
		return nil
	}

	// Example: INSERT ... ON CONFLICT DO NOTHING (idempotent via unique key)
	var b strings.Builder
	b.WriteString("INSERT INTO ")
	b.WriteString(t.tableName)
	b.WriteString(" (sensor_id, ts, seq, values, source_node_id, transform_ver) VALUES ")

	args := make([]any, 0, len(samples)*6)
	for i, s := range samples {
		if i > 0 {
			b.WriteString(",")
		}
		b.WriteString(fmt.Sprintf("($%d,$%d,$%d,$%d,$%d,$%d)",
			len(args)+1, len(args)+2, len(args)+3, len(args)+4, len(args)+5, len(args)+6))
		vals, err := json.Marshal(s.Values)
		if err != nil {
			return fmt.Errorf("marshal values: %w", err)
		}

		args = append(args,
			s.SensorID,
			s.Timestamp,
			s.Seq,
			vals,
			s.SourceNodeID,
			s.TransformVer,
		)
	}

	b.WriteString(" ON CONFLICT (sensor_id, ts, seq) DO NOTHING")

	_, err := t.db.Exec(b.String(), args...)
	return err
}

var _ ports.Sink = (*TimescaleSink)(nil)
