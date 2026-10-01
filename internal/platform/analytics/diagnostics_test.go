package analytics

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/Kyrapatka/knowledge-platform/internal/platform/metrics"
	"github.com/google/uuid"
)

func TestDiagnosticDropAggregationAndRedaction(t *testing.T) {
	var output bytes.Buffer
	release := make(chan struct{})
	sink := &testSink{writes: make(chan []Event, 4), closed: make(chan struct{}), block: release, err: errors.New("private sink response")}
	w, err := NewWorker(sink, Options{BufferSize: 1, BatchSize: 1, FlushInterval: time.Hour, WriteTimeout: time.Second}, metrics.New(), slog.New(slog.NewJSONHandler(&output, nil)))
	if err != nil {
		t.Fatal(err)
	}
	w.Publish(context.Background(), New(UserRegistered, uuid.New()))
	receive(t, sink.writes)
	w.Publish(context.Background(), New(UserRegistered, uuid.New()))
	for range 4 {
		w.Publish(context.Background(), New(UserRegistered, uuid.New()))
	}
	for range 7 {
		w.Publish(context.Background(), Event{})
	}
	close(release)
	if err := w.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(output.String(), "private sink response") {
		t.Fatal("sink response leaked")
	}
	counts := map[string]int{}
	for _, line := range strings.Split(strings.TrimSpace(output.String()), "\n") {
		var record map[string]any
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			t.Fatal(err)
		}
		reason, _ := record["reason"].(string)
		counts[reason]++
		switch reason {
		case "buffer_full", "invalid":
			want := float64(4)
			if reason == "invalid" {
				want = 7
			}
			if record["count"] != want || record["level"] != "WARN" || record["queue_capacity"] != float64(1) {
				t.Fatalf("incorrect drop report: %v", record)
			}
		case "flush_error":
			if record["level"] != "ERROR" || record["batch_size"] != float64(1) || record["error"] != "sink_write_failed" {
				t.Fatalf("incorrect flush report: %v", record)
			}
		default:
			t.Fatalf("unexpected diagnostic: %v", record)
		}
	}
	for _, reason := range []string{"buffer_full", "invalid", "flush_error"} {
		if counts[reason] != 1 {
			t.Fatalf("%s: expected one aggregated/throttled record, got %d", reason, counts[reason])
		}
	}
}
