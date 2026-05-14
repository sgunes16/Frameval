package logging_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"

	"github.com/mustafaselman/frameval/engine/internal/logging"
)

func parseLines(t *testing.T, buf *bytes.Buffer) []map[string]any {
	t.Helper()
	var out []map[string]any
	for _, line := range strings.Split(strings.TrimSpace(buf.String()), "\n") {
		if line == "" {
			continue
		}
		var record map[string]any
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			t.Fatalf("malformed log line %q: %v", line, err)
		}
		out = append(out, record)
	}
	return out
}

func TestNew_DefaultProducesJSONOutput(t *testing.T) {
	buf := &bytes.Buffer{}
	logger := logging.New(logging.Config{Level: slog.LevelInfo, Format: "json", Output: buf})
	logger.Info("hello", "k", "v")

	records := parseLines(t, buf)
	if len(records) != 1 {
		t.Fatalf("want 1 record, got %d", len(records))
	}
	if records[0]["msg"] != "hello" {
		t.Errorf("msg field: %v", records[0]["msg"])
	}
	if records[0]["k"] != "v" {
		t.Errorf("k=v attribute lost: %v", records[0])
	}
}

func TestNew_PrettyFormatProducesTextOutput(t *testing.T) {
	buf := &bytes.Buffer{}
	logger := logging.New(logging.Config{Level: slog.LevelInfo, Format: "pretty", Output: buf})
	logger.Info("startup")

	text := buf.String()
	if !strings.Contains(text, "startup") {
		t.Errorf("pretty handler should mention msg, got %q", text)
	}
	// Pretty (text) handler emits key=value pairs, not JSON.
	if strings.HasPrefix(strings.TrimSpace(text), "{") {
		t.Errorf("pretty handler should not emit JSON, got %q", text)
	}
}

func TestWithTraceID_StoresAndRetrievesValue(t *testing.T) {
	ctx := logging.WithTraceID(context.Background(), "abc-123")
	got := logging.TraceIDFromContext(ctx)
	if got != "abc-123" {
		t.Errorf("trace id roundtrip: want abc-123, got %q", got)
	}
}

func TestTraceIDFromContext_EmptyWhenUnset(t *testing.T) {
	got := logging.TraceIDFromContext(context.Background())
	if got != "" {
		t.Errorf("expected empty trace id when unset, got %q", got)
	}
}

func TestFromContext_AttachesTraceAndRunIDs(t *testing.T) {
	buf := &bytes.Buffer{}
	root := logging.New(logging.Config{Level: slog.LevelInfo, Format: "json", Output: buf})

	ctx := logging.WithTraceID(context.Background(), "t-1")
	ctx = logging.WithRunID(ctx, "run-7")
	ctx = logging.WithExperimentID(ctx, "exp-42")

	logger := logging.FromContext(ctx, root)
	logger.Info("did the thing")

	records := parseLines(t, buf)
	rec := records[0]
	if rec["trace_id"] != "t-1" {
		t.Errorf("trace_id missing: %v", rec)
	}
	if rec["run_id"] != "run-7" {
		t.Errorf("run_id missing: %v", rec)
	}
	if rec["experiment_id"] != "exp-42" {
		t.Errorf("experiment_id missing: %v", rec)
	}
}

func TestFromContext_NoIDsWhenContextEmpty(t *testing.T) {
	buf := &bytes.Buffer{}
	root := logging.New(logging.Config{Level: slog.LevelInfo, Format: "json", Output: buf})

	logger := logging.FromContext(context.Background(), root)
	logger.Info("no ids attached")

	records := parseLines(t, buf)
	for _, key := range []string{"trace_id", "run_id", "experiment_id"} {
		if _, ok := records[0][key]; ok {
			t.Errorf("did not expect %s in record: %v", key, records[0])
		}
	}
}

