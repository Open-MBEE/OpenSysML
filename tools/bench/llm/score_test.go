package llm

import (
	"math"
	"testing"
)

func TestNewStat(t *testing.T) {
	s := NewStat([]float64{2, 4, 4, 4, 5, 5, 7, 9})
	if s.Mean != 5 || math.Abs(s.Stdev-2.138) > 0.001 || s.N != 8 {
		t.Fatalf("%+v", s)
	}
	if one := NewStat([]float64{3}); one.Mean != 3 || one.Stdev != 0 {
		t.Fatalf("%+v", one)
	}
}

func TestStudentAndSign(t *testing.T) {
	// Two-sided p for t = 2.0 with 10 degrees of freedom is 0.0734.
	if p := studentTwoSided(2.0, 10); math.Abs(p-0.0734) > 0.001 {
		t.Errorf("studentTwoSided(2, 10) = %v", p)
	}
	if p := studentTwoSided(0, 5); math.Abs(p-1) > 1e-9 {
		t.Errorf("studentTwoSided(0, 5) = %v", p)
	}
	// 9 wins, 1 loss: 2 * P(X <= 1 | n = 10) = 2 * 11/1024.
	if p := signTest(9, 1); math.Abs(p-22.0/1024) > 1e-9 {
		t.Errorf("signTest(9, 1) = %v", p)
	}
	if p := signTest(0, 0); p != 1 {
		t.Errorf("signTest(0, 0) = %v", p)
	}
}

func TestComparePaired(t *testing.T) {
	mk := func(arm Arm, task string, sample int, ok bool, rounds int) CellResult {
		return CellResult{Cell: Cell{Task: task, Arm: arm, Sample: sample}, CompiledFinal: ok, RepairRounds: rounds}
	}
	a := []CellResult{mk(ArmCompiler, "T1", 1, false, 3), mk(ArmCompiler, "T2", 1, true, 2), mk(ArmCompiler, "T3", 1, true, 0)}
	b := []CellResult{mk(ArmMCP, "T1", 1, true, 1), mk(ArmMCP, "T2", 1, true, 1), mk(ArmMCP, "T3", 1, true, 0), mk(ArmMCP, "T4", 1, true, 0)}
	var rounds Paired
	for _, p := range ComparePaired(a, b) {
		if p.Metric == "repairRounds" {
			rounds = p
		}
	}
	// Fewer repairs is better, so the two cells mcp repaired faster are wins
	// even though the signed difference stays mcp − compiler.
	if rounds.Pairs != 3 || rounds.Wins != 2 || rounds.Losses != 0 || math.Abs(rounds.MeanDiff+1) > 1e-9 {
		t.Fatalf("%+v", rounds)
	}
}

// TestAggregateSkipsErroredCells keeps provider failures out of every measured
// rate while still counting them.
func TestAggregateSkipsErroredCells(t *testing.T) {
	cells := []CellResult{
		{Cell: Cell{Task: "T1", Arm: ArmCompiler, Sample: 1}, Family: "structure", Difficulty: "simple", CompiledRound0: true, CompiledFinal: true, Fidelity: 1},
		{Cell: Cell{Task: "T2", Arm: ArmCompiler, Sample: 1}, Family: "structure", Difficulty: "simple", Error: "HTTP 429"},
		{Cell: Cell{Task: "T1", Arm: ArmCompiler, Sample: 2}, Family: "structure", Difficulty: "simple", Error: "HTTP 429"},
	}
	agg := AggregateCells(cells)
	if agg.Cells != 3 || agg.Errors != 2 {
		t.Fatalf("cells=%d errors=%d, want 3 and 2", agg.Cells, agg.Errors)
	}
	if agg.FinalCompile.N != 1 || agg.FinalCompile.Mean != 1 || agg.Fidelity.Mean != 1 {
		t.Errorf("errored cells leaked into the rates: %+v", agg)
	}
	if s := agg.ByFamily["structure"]; s.Cells != 1 || s.FinalCompile != 1 {
		t.Errorf("errored cells leaked into the strata: %+v", s)
	}
}

func TestReportedModels(t *testing.T) {
	cells := []CellResult{
		{Attempts: []Attempt{{Model: "gpt-x-2026-01-01"}, {Model: "gpt-x-2026-01-01"}}},
		{Attempts: []Attempt{{Model: ""}}},
		{Attempts: []Attempt{{Model: "gpt-x-2026-02-01"}}},
	}
	if got := reportedModels(cells); got != "gpt-x-2026-01-01, gpt-x-2026-02-01" {
		t.Errorf("reportedModels = %q", got)
	}
	if got := reportedModels(nil); got != "" {
		t.Errorf("reportedModels(nil) = %q", got)
	}
}
