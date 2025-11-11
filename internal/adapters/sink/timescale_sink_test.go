package sink

import (
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"

	"github.com/ghalamif/AegisFlow/internal/domain"
)

func TestTimescaleSinkWriteBatch(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	sink := NewTimescaleSink(db, "samples")
	ts := time.Now()

	samples := []*domain.Sample{
		{
			SensorID:     "sensor-1",
			Timestamp:    ts,
			Seq:          1,
			Values:       map[string]float64{"value": 42},
			SourceNodeID: "node-1",
			TransformVer: 2,
		},
	}

	expectedQuery := regexp.QuoteMeta("INSERT INTO samples (sensor_id, ts, seq, values, source_node_id, transform_ver) VALUES ($1,$2,$3,$4,$5,$6) ON CONFLICT (sensor_id, ts, seq) DO NOTHING")
	mock.ExpectExec(expectedQuery).
		WithArgs("sensor-1", ts, uint64(1), sqlmock.AnyArg(), "node-1", uint16(2)).
		WillReturnResult(sqlmock.NewResult(1, 1))

	if err := sink.WriteBatch(samples); err != nil {
		t.Fatalf("write batch: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestTimescaleSinkWriteBatchNoSamples(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	sink := NewTimescaleSink(db, "samples")
	if err := sink.WriteBatch(nil); err != nil {
		t.Fatalf("expected nil error for empty batch, got %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestTimescaleSinkName(t *testing.T) {
	db, _, _ := sqlmock.New()
	defer db.Close()

	sink := NewTimescaleSink(db, "samples")
	if sink.Name() != "timescaledb" {
		t.Fatalf("expected sink name timescaledb, got %s", sink.Name())
	}
}
