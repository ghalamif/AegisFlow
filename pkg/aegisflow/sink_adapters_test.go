package aegisflow

import (
	"errors"
	"testing"
	"time"
)

func TestNewCallbackSink(t *testing.T) {
	var received []Sample
	sink := NewCallbackSink("cb", func(batch []Sample) error {
		received = append(received, batch...)
		return nil
	})

	input := Sample{
		SensorID:  "sensor-1",
		Timestamp: time.Unix(1, 0),
		Seq:       42,
		Values:    map[string]float64{"value": 3.14},
	}

	if err := sink.WriteBatch([]*PipelineSample{input.toDomain()}); err != nil {
		t.Fatalf("WriteBatch returned error: %v", err)
	}
	if len(received) != 1 {
		t.Fatalf("expected 1 batch entry, got %d", len(received))
	}
	got := received[0]
	if got.SensorID != input.SensorID || got.Seq != input.Seq {
		t.Fatalf("mismatched sample payload: %+v vs %+v", got, input)
	}
	if got.Values["value"] != 3.14 {
		t.Fatalf("expected value to be copied, got %v", got.Values["value"])
	}
}

func TestNewCallbackSinkNilHandler(t *testing.T) {
	sink := NewCallbackSink("", nil)
	s := Sample{SensorID: "s"}
	err := sink.WriteBatch([]*PipelineSample{s.toDomain()})
	if err == nil {
		t.Fatalf("expected error when callback is nil")
	}
}

func TestNewChannelSink(t *testing.T) {
	sink, ch, closeFn := NewChannelSink("chan", 1)
	defer closeFn()

	input := Sample{SensorID: "sensor-2", Seq: 7}
	errCh := make(chan error, 1)

	go func() {
		errCh <- sink.WriteBatch([]*PipelineSample{input.toDomain()})
	}()

	var batch []Sample
	select {
	case batch = <-ch:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for channel batch")
	}

	if err := <-errCh; err != nil {
		t.Fatalf("WriteBatch returned error: %v", err)
	}
	if len(batch) != 1 || batch[0].SensorID != input.SensorID {
		t.Fatalf("unexpected batch data: %+v", batch)
	}

	closeFn()
	if err := sink.WriteBatch([]*PipelineSample{input.toDomain()}); !errors.Is(err, ErrChannelSinkClosed) {
		t.Fatalf("expected ErrChannelSinkClosed, got %v", err)
	}
}
