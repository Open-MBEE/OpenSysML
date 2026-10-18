package llm

import (
	"context"
	"path/filepath"
	"reflect"
	"testing"
)

// TestOllamaRecordingReplays replays the recorded qwen2.5-coder:7b run under
// strict digests and checks it reproduces the committed results file.
func TestOllamaRecordingReplays(t *testing.T) {
	tc := testToolchain(t)
	corpus, err := LoadCorpus()
	if err != nil {
		t.Fatal(err)
	}
	fixture, err := LoadFixture(filepath.Join("testdata", "replay", "ollama-qwen2.5-coder-7b.json"))
	if err != nil {
		t.Fatal(err)
	}
	committed, err := ReadResults(filepath.Join("..", "..", "..", "docs", "project", "llm-benchmark-ollama-qwen2.5-coder-7b.json"))
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	mcpArm, err := StartMCP(ctx, "test")
	if err != nil {
		t.Fatal(err)
	}
	defer mcpArm.Close(ctx)
	replay := NewReplay(fixture)
	replay.Strict = true
	runner := &Runner{Provider: replay, Toolchain: tc, Corpus: corpus, MCP: mcpArm, MaxRounds: committed.Settings.MaxRounds, Temperature: committed.Settings.Temperature, MaxTokens: committed.Settings.MaxTokens}
	results := &Results{Provider: replay.Name(), Model: replay.Model()}
	for _, want := range committed.Cells {
		tasks, err := corpus.Select([]string{want.Task})
		if err != nil {
			t.Fatal(err)
		}
		got := runner.RunCell(ctx, tasks[0], want.Arm, want.Sample)
		if got.Error != "" {
			t.Errorf("%s: %s", CellKey(got.Cell), got.Error)
		}
		if got.CompiledRound0 != want.CompiledRound0 || got.CompiledFinal != want.CompiledFinal ||
			got.RepairRounds != want.RepairRounds || got.Fidelity != want.Fidelity || len(got.Attempts) != len(want.Attempts) {
			t.Errorf("%s: replay %+v differs from the committed cell", CellKey(got.Cell), got)
		}
		results.Cells = append(results.Cells, got)
	}
	results.Finish()
	if !reflect.DeepEqual(results.Arms, committed.Arms) {
		t.Errorf("replayed aggregates %+v\nwant %+v", results.Arms, committed.Arms)
	}
	if results.ModelReported != "qwen2.5-coder:7b" {
		t.Errorf("modelReported = %q", results.ModelReported)
	}
}
