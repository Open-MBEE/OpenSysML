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
	if len(fixture.Tools) == 0 {
		t.Fatal("the recording carries no tool specs")
	}
	for _, name := range specDrift(fixture.Tools, mcpArm.Specs()) {
		t.Logf("sysml-mcp describes %s differently from the recording; the committed results are about the recorded description", name)
	}
	replay := NewReplay(fixture)
	replay.Strict = true
	runner := &Runner{Provider: replay, Toolchain: tc, Corpus: corpus, MCP: mcpArm, Tools: fixture.Tools, MaxRounds: committed.Settings.MaxRounds, Temperature: committed.Settings.Temperature, MaxTokens: committed.Settings.MaxTokens}
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

// specDrift names the tools whose live spec differs from the recorded one.
func specDrift(recorded, live []ToolSpec) []string {
	digest := func(s ToolSpec) string { return RequestDigest(Request{Tools: []ToolSpec{s}}) }
	liveBy := map[string]string{}
	for _, s := range live {
		liveBy[s.Name] = digest(s)
	}
	var names []string
	for _, s := range recorded {
		if liveBy[s.Name] != digest(s) {
			names = append(names, s.Name)
		}
	}
	return names
}

// offeredTools answers every request with a document and keeps the tool
// specs each request offered.
type offeredTools struct {
	offered [][]ToolSpec
}

func (p *offeredTools) Name() string  { return "capture" }
func (p *offeredTools) Model() string { return "capture" }
func (p *offeredTools) Complete(_ context.Context, req Request) (Response, error) {
	p.offered = append(p.offered, req.Tools)
	return Response{Content: "package P;"}, nil
}

// TestRecordingKeepsOfferedTools records an MCP-arm cell and replays it with
// the recorded specs after the server's description changed.
func TestRecordingKeepsOfferedTools(t *testing.T) {
	tc := testToolchain(t)
	corpus, err := LoadCorpus()
	if err != nil {
		t.Fatal(err)
	}
	tasks, err := corpus.Select([]string{"S01"})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	mcpArm, err := StartMCP(ctx, "test")
	if err != nil {
		t.Fatal(err)
	}
	defer mcpArm.Close(ctx)

	live := &offeredTools{}
	recorder := NewRecorder(live)
	runner := &Runner{Provider: recorder, Toolchain: tc, Corpus: corpus, MCP: mcpArm}
	if cell := runner.RunCell(ctx, tasks[0], ArmMCP, 1); cell.Error != "" {
		t.Fatal(cell.Error)
	}
	if !reflect.DeepEqual(recorder.Fixture.Tools, mcpArm.Specs()) {
		t.Fatalf("recorded tools %+v, want the server's %+v", recorder.Fixture.Tools, mcpArm.Specs())
	}
	if len(live.offered) != 1 || !reflect.DeepEqual(live.offered[0], mcpArm.Specs()) {
		t.Fatalf("the live request offered %+v", live.offered)
	}

	reworded := make([]ToolSpec, len(mcpArm.Specs()))
	copy(reworded, mcpArm.Specs())
	reworded[0].Description += " (reworded)"
	replay := NewReplay(recorder.Fixture)
	replay.Strict = true
	runner = &Runner{Provider: replay, Toolchain: tc, Corpus: corpus, MCP: mcpArm, Tools: reworded}
	if cell := runner.RunCell(ctx, tasks[0], ArmMCP, 1); cell.Error == "" {
		t.Fatal("a strict replay accepted a request offering reworded tools")
	}
	replay = NewReplay(recorder.Fixture)
	replay.Strict = true
	runner = &Runner{Provider: replay, Toolchain: tc, Corpus: corpus, MCP: mcpArm, Tools: recorder.Fixture.Tools}
	if cell := runner.RunCell(ctx, tasks[0], ArmMCP, 1); cell.Error != "" {
		t.Fatalf("replay with the recorded tools: %s", cell.Error)
	}
	if got := specDrift(recorder.Fixture.Tools, reworded); len(got) != 1 || got[0] != reworded[0].Name {
		t.Fatalf("specDrift = %v, want [%s]", got, reworded[0].Name)
	}
}
