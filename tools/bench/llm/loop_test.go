package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"path/filepath"
	"strings"
	"testing"
)

func loadSynthetic(t *testing.T) *Fixture {
	t.Helper()
	f, err := LoadFixture(filepath.Join("testdata", "replay", "synthetic.json"))
	if err != nil {
		t.Fatal(err)
	}
	return f
}

func cellByKey(cells []CellResult, key string) CellResult {
	for _, c := range cells {
		if CellKey(c.Cell) == key {
			return c
		}
	}
	return CellResult{Error: "missing cell " + key}
}

// TestReplayLoop drives both arms over the committed synthetic fixture and
// checks every metric the loop records.
func TestReplayLoop(t *testing.T) {
	tc := testToolchain(t)
	corpus, err := LoadCorpus()
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	mcpArm, err := StartMCP(ctx, "test")
	if err != nil {
		t.Fatal(err)
	}
	defer mcpArm.Close(ctx)
	if got := len(mcpArm.Specs()); got != len(MCPTools) {
		t.Fatalf("mcp arm offers %d tools, want %d", got, len(MCPTools))
	}
	replay := NewReplay(loadSynthetic(t))
	runner := &Runner{Provider: replay, Toolchain: tc, Corpus: corpus, MCP: mcpArm, MaxRounds: DefaultMaxRounds, Pilot: tc.Pilot != ""}
	results := &Results{Provider: replay.Name(), Model: replay.Model()}
	tasks, err := corpus.Select([]string{"S01", "B01", "R01", "C01", "V01", "S02"})
	if err != nil {
		t.Fatal(err)
	}
	for _, task := range tasks {
		for _, arm := range Arms {
			for sample := 1; sample <= 2; sample++ {
				results.Cells = append(results.Cells, runner.RunCell(ctx, task, arm, sample))
			}
		}
	}
	results.Finish()
	for _, c := range results.Cells {
		if c.Error != "" {
			t.Errorf("%s: %s", CellKey(c.Cell), c.Error)
		}
		if c.Elapsed == "" {
			t.Errorf("%s: no elapsed time recorded", CellKey(c.Cell))
		}
	}

	repaired := cellByKey(results.Cells, "S01/compiler/1")
	if repaired.CompiledRound0 || !repaired.CompiledFinal || repaired.RepairRounds != 1 {
		t.Errorf("S01/compiler/1: round0=%v final=%v repairs=%d, want one repair", repaired.CompiledRound0, repaired.CompiledFinal, repaired.RepairRounds)
	}
	if len(repaired.Hallucinated) != 1 || repaired.Hallucinated[0] != "BogusType" {
		t.Errorf("S01/compiler/1: hallucinated %v, want BogusType", repaired.Hallucinated)
	}
	if repaired.Fidelity != 1 {
		t.Errorf("S01/compiler/1: fidelity %v of the gold against itself, want 1", repaired.Fidelity)
	}
	if tc.Pilot != "" && (repaired.PilotAccepted == nil || !*repaired.PilotAccepted) {
		t.Errorf("S01/compiler/1: pilot verdict %v, want accepted", repaired.PilotAccepted)
	}
	if len(repaired.Attempts) != 2 || len(repaired.Attempts[0].Diagnostics) == 0 {
		t.Errorf("S01/compiler/1: attempts %+v, want a diagnosed first round", repaired.Attempts)
	}

	fenced := cellByKey(results.Cells, "S01/compiler/2")
	if !fenced.CompiledRound0 || fenced.RepairRounds != 0 {
		t.Errorf("S01/compiler/2: a fenced gold must compile at round 0: %+v", fenced)
	}

	tools := cellByKey(results.Cells, "S01/mcp/1")
	if tools.ToolCalls != 2 || tools.RepairRounds != 1 {
		t.Errorf("S01/mcp/1: tools=%d repairs=%d, want 2 tool calls and one repair", tools.ToolCalls, tools.RepairRounds)
	}
	var names []string
	for _, a := range tools.Attempts {
		for _, rec := range a.ToolCalls {
			names = append(names, rec.Name)
			if rec.IsError {
				t.Errorf("S01/mcp/1: %s answered an error: %s", rec.Name, rec.Result)
			}
			if rec.ResultBytes == 0 || len(rec.ResultDigest) != 64 {
				t.Errorf("S01/mcp/1: %s recorded no answer: %+v", rec.Name, rec)
			}
		}
	}
	if strings.Join(names, ",") != "parse,explain_diagnostic" {
		t.Errorf("S01/mcp/1: tool calls %v", names)
	}

	failed := cellByKey(results.Cells, "S02/compiler/1")
	if failed.CompiledFinal || failed.RepairRounds != DefaultMaxRounds || len(failed.Attempts) != DefaultMaxRounds+1 || failed.Fidelity != 0 {
		t.Errorf("S02/compiler/1: %+v, want %d failed attempts", failed, DefaultMaxRounds+1)
	}
	for _, key := range []string{"S02/mcp/1", "S02/mcp/2"} {
		c := cellByKey(results.Cells, key)
		if c.ToolCalls != 1 {
			t.Errorf("%s: %d tool calls, want 1", key, c.ToolCalls)
		}
	}

	comp, mcp := results.Arms[ArmCompiler], results.Arms[ArmMCP]
	if comp.Cells != 12 || mcp.Cells != 12 {
		t.Fatalf("arm cells %d/%d, want 12 each", comp.Cells, mcp.Cells)
	}
	// Sample 1 compiles no cell at round 0; sample 2 compiles every cell.
	if comp.CompileRate0.N != 2 || comp.CompileRate0.Mean != 0.5 {
		t.Errorf("compiler CR@0 %+v, want mean 0.5 over 2 samples", comp.CompileRate0)
	}
	if math.Abs(comp.FinalCompile.Mean-11.0/12) > 1e-9 {
		t.Errorf("compiler FCR %+v, want 11/12", comp.FinalCompile)
	}
	if mcp.ToolCalls.Mean == 0 {
		t.Errorf("mcp arm recorded no tool calls: %+v", mcp.ToolCalls)
	}
	if len(results.Significance) != len(PairedMetrics) {
		t.Fatalf("significance rows %d, want %d", len(results.Significance), len(PairedMetrics))
	}
	for _, p := range results.Significance {
		if p.Pairs != 12 {
			t.Errorf("%s: %d pairs, want 12", p.Metric, p.Pairs)
		}
		if p.PT < 0 || p.PT > 1 || p.PSign < 0 || p.PSign > 1 {
			t.Errorf("%s: p-values out of range: %+v", p.Metric, p)
		}
	}
	if !strings.Contains(MarkdownTable([]*Results{results}), "| replay/synthetic | mcp |") {
		t.Error("markdown table lacks the mcp row")
	}
}

// TestReplayStrictDigest makes a strict replay refuse a request it did not record.
func TestReplayStrictDigest(t *testing.T) {
	replay := NewReplay(loadSynthetic(t))
	replay.Strict = true
	ctx := WithCell(context.Background(), Cell{Task: "S01", Arm: ArmCompiler, Sample: 2})
	if _, err := replay.Complete(ctx, Request{Messages: []Message{{Role: "user", Content: "unrecorded"}}}); err == nil {
		t.Fatal("strict replay accepted an unrecorded request")
	}
	replay.Strict = false
	if _, err := replay.Complete(ctx, Request{}); err != nil {
		t.Fatalf("lenient replay: %v", err)
	}
	if _, err := replay.Complete(ctx, Request{}); err == nil {
		t.Fatal("replay served more answers than recorded")
	}
	missing := WithCell(context.Background(), Cell{Task: "Z99", Arm: ArmCompiler, Sample: 1})
	if _, err := replay.Complete(missing, Request{}); err == nil {
		t.Fatal("replay served a cell it never recorded")
	}
}

// TestToolBudgetAndRounds bounds the tool calls a round may make and honours a
// zero-round run, over an in-code fixture.
func TestToolBudgetAndRounds(t *testing.T) {
	tc := testToolchain(t)
	corpus, err := LoadCorpus()
	if err != nil {
		t.Fatal(err)
	}
	tasks, err := corpus.Select([]string{"S01"})
	if err != nil {
		t.Fatal(err)
	}
	task := tasks[0]
	gold, err := corpus.GoldSource(task)
	if err != nil {
		t.Fatal(err)
	}
	catalog := func(id string) ToolCall {
		return ToolCall{ID: id, Name: "tool_catalog", Arguments: json.RawMessage(`{}`)}
	}
	document := Recording{Response: Response{Content: gold}}
	var burst []ToolCall
	for i := 0; i <= MaxToolCallsPerRound; i++ {
		burst = append(burst, catalog(fmt.Sprintf("b%d", i)))
	}
	var oneByOne []Recording
	for i := 0; i <= MaxToolCallsPerRound; i++ {
		oneByOne = append(oneByOne, Recording{Response: Response{ToolCalls: []ToolCall{catalog(fmt.Sprintf("s%d", i))}}})
	}
	fixture := &Fixture{Provider: "replay", Model: "budget", Cells: map[string][]Recording{
		"S01/mcp/1":      {{Response: Response{ToolCalls: burst}}, document},
		"S01/mcp/2":      append(oneByOne, document),
		"S01/compiler/1": {{Response: Response{Content: "package Bicycle {\n    part p : BogusType;\n}\n"}}},
	}}
	ctx := context.Background()
	mcpArm, err := StartMCP(ctx, "test")
	if err != nil {
		t.Fatal(err)
	}
	defer mcpArm.Close(ctx)
	runner := &Runner{Provider: NewReplay(fixture), Toolchain: tc, Corpus: corpus, MCP: mcpArm, MaxRounds: DefaultMaxRounds}

	burstCell := runner.RunCell(ctx, task, ArmMCP, 1)
	if burstCell.Error != "" || !burstCell.CompiledFinal || burstCell.ToolCalls != MaxToolCallsPerRound+1 {
		t.Fatalf("burst cell: %+v", burstCell)
	}
	refused := 0
	for _, rec := range burstCell.Attempts[0].ToolCalls {
		if rec.Refused {
			refused++
			if !rec.IsError || rec.ResultBytes == 0 || len(rec.ResultDigest) != 64 || !strings.Contains(rec.Result, "budget") {
				t.Errorf("refused call lacks the answer the model saw: %+v", rec)
			}
		}
	}
	if refused != 1 {
		t.Errorf("refused %d calls of a burst of %d, want 1", refused, MaxToolCallsPerRound+1)
	}

	runaway := runner.RunCell(ctx, task, ArmMCP, 2)
	if runaway.Error == "" || !strings.Contains(runaway.Error, "budget") || runaway.CompiledFinal {
		t.Fatalf("a model that keeps calling tools after the budget must fail the cell: %+v", runaway)
	}

	runner.MaxRounds = 0
	once := runner.RunCell(ctx, task, ArmCompiler, 1)
	if once.Error != "" || len(once.Attempts) != 1 || once.CompiledFinal || once.RepairRounds != 0 {
		t.Fatalf("zero rounds: %+v", once)
	}
	runner.MaxRounds = -1
	if neg := runner.RunCell(ctx, task, ArmCompiler, 1); neg.Error == "" {
		t.Fatal("negative rounds accepted")
	}
}
