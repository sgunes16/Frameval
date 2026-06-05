package storage_test

import (
	"context"
	"errors"
	"testing"

	"github.com/mustafaselman/frameval/engine/internal/storage"
	"github.com/mustafaselman/frameval/engine/test/support"
)

// stubRunner is the smallest OpenCodeModelsRunner that lets us drive
// SeedOpenCodeModels with canned `opencode models` output. Subsequent
// calls return successive outputs so a test can simulate upstream
// adding / dropping models between reseeds.
type stubRunner struct {
	calls   int
	outputs []string
	err     error
}

func (s *stubRunner) RunShell(_ context.Context, _ string, _ map[string]string, _ string) (string, error) {
	if s.err != nil {
		return "", s.err
	}
	out := s.outputs[0]
	if s.calls < len(s.outputs)-1 {
		s.outputs = s.outputs[1:]
	}
	s.calls++
	return out, nil
}

func opencodeIDs(t *testing.T, store *storage.Store) []string {
	t.Helper()
	cfgs, err := store.ListModelConfigs(context.Background())
	if err != nil {
		t.Fatalf("ListModelConfigs: %v", err)
	}
	out := []string{}
	for _, c := range cfgs {
		if c.Provider == "opencode" {
			out = append(out, c.ModelID)
		}
	}
	return out
}

func TestSeedOpenCodeModelsInsertsFreshList(t *testing.T) {
	store := support.TmpStore(t)
	runner := &stubRunner{outputs: []string{
		"opencode/foo-free\nopencode/bar-free\nopencode/baz\n",
	}}
	if err := store.SeedOpenCodeModels(context.Background(), runner); err != nil {
		t.Fatalf("SeedOpenCodeModels: %v", err)
	}
	got := opencodeIDs(t, store)
	want := map[string]bool{"opencode/foo-free": true, "opencode/bar-free": true, "opencode/baz": true}
	if len(got) != 3 {
		t.Fatalf("count: got %d want 3 (got %v)", len(got), got)
	}
	for _, id := range got {
		if !want[id] {
			t.Errorf("unexpected id %q", id)
		}
	}
}

func TestSeedOpenCodeModelsPrunesStaleEntries(t *testing.T) {
	store := support.TmpStore(t)
	// First boot: upstream lists 3 models.
	runner := &stubRunner{outputs: []string{
		"opencode/foo-free\nopencode/bar-free\nopencode/baz\n",
		"opencode/foo-free\nopencode/baz\n", // bar-free deprecated upstream
	}}
	if err := store.SeedOpenCodeModels(context.Background(), runner); err != nil {
		t.Fatalf("first seed: %v", err)
	}
	if got := opencodeIDs(t, store); len(got) != 3 {
		t.Fatalf("first seed count: got %d want 3", len(got))
	}
	// Second boot: bar-free is gone — it must be pruned, not retained.
	if err := store.SeedOpenCodeModels(context.Background(), runner); err != nil {
		t.Fatalf("second seed: %v", err)
	}
	got := opencodeIDs(t, store)
	for _, id := range got {
		if id == "opencode/bar-free" {
			t.Errorf("stale id %q should have been pruned; got %v", id, got)
		}
	}
	if len(got) != 2 {
		t.Errorf("after-prune count: got %d want 2 (%v)", len(got), got)
	}
}

func TestSeedOpenCodeModelsLeavesNonOpenCodeRowsAlone(t *testing.T) {
	store := support.TmpStore(t)
	// Seed the static defaults (ollama, openai, anthropic, etc.) first.
	if err := store.SeedModelConfigs(context.Background()); err != nil {
		t.Fatalf("SeedModelConfigs: %v", err)
	}
	beforeAll, _ := store.ListModelConfigs(context.Background())
	nonOC := 0
	for _, c := range beforeAll {
		if c.Provider != "opencode" {
			nonOC++
		}
	}
	// Then seed an empty opencode list — the prune step must not touch the
	// ollama / openai / anthropic rows.
	runner := &stubRunner{outputs: []string{""}}
	if err := store.SeedOpenCodeModels(context.Background(), runner); err != nil {
		t.Fatalf("SeedOpenCodeModels: %v", err)
	}
	afterAll, _ := store.ListModelConfigs(context.Background())
	nonOCAfter := 0
	for _, c := range afterAll {
		if c.Provider != "opencode" {
			nonOCAfter++
		}
	}
	if nonOCAfter != nonOC {
		t.Errorf("non-opencode rows changed: before=%d after=%d", nonOC, nonOCAfter)
	}
}

func TestSeedOpenCodeModelsSkipsPruneOnRunnerError(t *testing.T) {
	store := support.TmpStore(t)
	// Pre-seed 2 opencode models.
	runner := &stubRunner{outputs: []string{"opencode/foo-free\nopencode/bar-free\n"}}
	if err := store.SeedOpenCodeModels(context.Background(), runner); err != nil {
		t.Fatalf("first seed: %v", err)
	}
	// Now simulate a daemon / CLI failure on reseed — must NOT prune.
	failing := &stubRunner{err: errors.New("opencode not authenticated")}
	if err := store.SeedOpenCodeModels(context.Background(), failing); err != nil {
		t.Fatalf("second seed swallows the error: %v", err)
	}
	got := opencodeIDs(t, store)
	if len(got) != 2 {
		t.Errorf("count after failing reseed: got %d want 2 (%v)", len(got), got)
	}
}

type ocRow struct {
	provider string
	display  string
}

// opencodeCatalog returns the seeded opencode (Zen) and opencode-go (Go)
// rows keyed by model id, exposing only the fields these tests assert on.
func opencodeCatalog(t *testing.T, store *storage.Store) map[string]ocRow {
	t.Helper()
	cfgs, err := store.ListModelConfigs(context.Background())
	if err != nil {
		t.Fatalf("ListModelConfigs: %v", err)
	}
	out := map[string]ocRow{}
	for _, c := range cfgs {
		if c.Provider == "opencode" || c.Provider == "opencode-go" {
			out[c.ModelID] = ocRow{provider: c.Provider, display: c.DisplayName}
		}
	}
	return out
}

func TestSeedOpenCodeModelsSeedsAndPrunesBothCatalogs(t *testing.T) {
	store := support.TmpStore(t)
	runner := &stubRunner{outputs: []string{
		// First boot: Zen (opencode/*) and Go (opencode-go/*) catalogs
		// interleaved, plus a junk line that must be ignored.
		"opencode/deepseek-v4-flash-free\nopencode-go/kimi-k2.6\nopencode-go/deepseek-v4-pro\nnot-a-model\n",
		// Second boot: upstream drops the Go deepseek-v4-pro model.
		"opencode/deepseek-v4-flash-free\nopencode-go/kimi-k2.6\n",
	}}
	if err := store.SeedOpenCodeModels(context.Background(), runner); err != nil {
		t.Fatalf("first seed: %v", err)
	}

	got := opencodeCatalog(t, store)
	if c, ok := got["opencode/deepseek-v4-flash-free"]; !ok || c.provider != "opencode" || c.display != "Deepseek V4 Flash (opencode zen free)" {
		t.Errorf("zen entry wrong: %+v (ok=%v)", c, ok)
	}
	if c, ok := got["opencode-go/kimi-k2.6"]; !ok || c.provider != "opencode-go" || c.display != "Kimi K2.6 (opencode go)" {
		t.Errorf("go entry wrong: %+v (ok=%v)", c, ok)
	}
	if _, ok := got["opencode-go/deepseek-v4-pro"]; !ok {
		t.Errorf("go deepseek-v4-pro should be seeded on first boot")
	}
	if _, ok := got["not-a-model"]; ok {
		t.Errorf("junk line must be skipped")
	}

	// Second boot must prune the dropped Go model while keeping the live
	// Zen and Go entries — the prune now spans both opencode catalogs.
	if err := store.SeedOpenCodeModels(context.Background(), runner); err != nil {
		t.Fatalf("second seed: %v", err)
	}
	got = opencodeCatalog(t, store)
	if _, ok := got["opencode-go/deepseek-v4-pro"]; ok {
		t.Errorf("stale go model should have been pruned; got %v", got)
	}
	if _, ok := got["opencode-go/kimi-k2.6"]; !ok {
		t.Errorf("live go model must survive prune")
	}
	if _, ok := got["opencode/deepseek-v4-flash-free"]; !ok {
		t.Errorf("live zen model must survive prune")
	}
}
