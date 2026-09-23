package logger

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"strings"
	"sync"
	"testing"
)

func TestFormatsLevelsAndFields(t *testing.T) {
	for _, format := range []string{"text", "json"} {
		for index, level := range []string{"debug", "info", "warn", "error"} {
			t.Run(format+"/"+level, func(t *testing.T) {
				var output bytes.Buffer
				logger, err := New(Config{Format: format, Level: level, Output: &output})
				if err != nil {
					t.Fatal(err)
				}
				switch format {
				case "text":
					if _, ok := logger.Handler().(*slog.TextHandler); !ok {
						t.Fatal("expected TextHandler")
					}
				case "json":
					if _, ok := logger.Handler().(*slog.JSONHandler); !ok {
						t.Fatal("expected JSONHandler")
					}
				}
				logger.Debug("debug message")
				logger.Info("info message")
				logger.Warn("warn message")
				logger.Error("error message", slog.String("operation", "test"))
				for i, name := range []string{"debug", "info", "warn", "error"} {
					if strings.Contains(output.String(), name+" message") != (i >= index) {
						t.Fatalf("level filter: %s", output.String())
					}
				}
				lines := strings.Split(strings.TrimSpace(output.String()), "\n")
				if len(lines) != 4-index {
					t.Fatalf("line count: %s", output.String())
				}
				for _, line := range lines {
					if format == "json" {
						var record map[string]any
						if err := json.Unmarshal([]byte(line), &record); err != nil {
							t.Fatal(err)
						}
						if record["service"] != "knowledge-platform" || record["time"] == nil || record["level"] == nil {
							t.Fatal(record)
						}
					} else if !strings.Contains(line, "service=knowledge-platform") {
						t.Fatal(line)
					}
				}
			})
		}
	}
}
func TestInvalidConfigDoesNotFallbackOrEchoValues(t *testing.T) {
	for _, cfg := range []Config{{Level: "test", Format: "text"}, {Level: "info", Format: "yaml"}, {Level: "", Format: "text"}, {Level: "info", Format: ""}, {Level: "secret-credential", Format: "json"}} {
		log, err := New(cfg)
		if err == nil || log != nil {
			t.Fatalf("accepted invalid config %+v", cfg)
		}
		if strings.Contains(err.Error(), "secret-credential") {
			t.Fatal("echoed input")
		}
	}
}
func TestSharedLoggerConcurrentChildren(t *testing.T) {
	var output bytes.Buffer
	logger, err := New(Config{Level: "info", Format: "json", Output: &output})
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for i := 0; i < 40; i++ {
		wg.Add(1)
		go func(i int) { defer wg.Done(); logger.With(slog.Int("worker", i)).Info("done") }(i)
	}
	wg.Wait()
	lines := strings.Split(strings.TrimSpace(output.String()), "\n")
	if len(lines) != 40 {
		t.Fatalf("lost records: %d", len(lines))
	}
	for _, line := range lines {
		var row map[string]any
		if err := json.Unmarshal([]byte(line), &row); err != nil {
			t.Fatal(err)
		}
		if row["service"] != "knowledge-platform" || row["worker"] == nil {
			t.Fatal(row)
		}
	}
}
