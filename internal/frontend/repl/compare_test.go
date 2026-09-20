package repl

import (
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/exec/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/translate/simresults"
)

// compareModel: a target whose behavior shaky fails on some draws (10 / k, k drawn
// from {0, 1}) and whose behavior steady never does.
const compareModel = `package Cfg {
	private import ScalarValues::*;
	private import RandomFunctions::*;
	part def Probe {
		attribute total : Real = 1.0;
		action shaky {
			attribute k : Integer = uniformInteger(0, 1);
			first start;
			then assign total := 10.0 / k;
			then done;
		}
		action steady { first start; then assign total := 4.0; then done; }
	}
	individual def probe :> Probe;
	action def 'Group 0' { part target : probe; perform action run ::> target.shaky; }
	action def 'Group 1' { part target : probe; perform action run ::> target.steady; }
	action def 'Sub::Group' { part target : probe; perform action run ::> target.steady; }
}`

func compareSession(t *testing.T) *Session {
	t.Helper()
	s := NewSession()
	if errs := errorDiagnostics(s.Submit(compareModel).Diagnostics); len(errs) > 0 {
		t.Fatalf("model has errors: %v", errs)
	}
	return s
}

func compareResults(name string, runs int64) *simresults.Results {
	return &simresults.Results{Source: "probe.xmi", Configurations: []simresults.ConfigurationResults{{
		ID: "_c", Name: "Cfg::" + name, Runs: runs, Target: "target", Behavior: "run",
		Location:    "Results",
		Observables: []string{"total"},
		Snapshots: []simresults.Snapshot{
			{ID: "_r1", Values: map[string]float64{"total": 4.0}},
			{ID: "_r2", Values: map[string]float64{"total": 6.0}},
		},
	}}}
}

// A comparison holds when every run completes and fails when any run failed,
// as a %runs table with a failed row does, with each error under the table.
func TestCompareFailsWhenAnyRunFails(t *testing.T) {
	s := compareSession(t)
	seed := uint64(1)

	steady := s.CompareResults(compareResults("'Group 1'", 4), CompareOptions{Seed: &seed})
	if len(steady) != 1 || !steady[0].Holds() {
		t.Fatalf("every run completing = %+v", steady)
	}
	lines := strings.Join(steady[0].Lines, "\n")
	if !strings.Contains(lines, "4 run(s) by OpenSysML") || strings.Contains(lines, "error:") {
		t.Errorf("lines of a comparison every run of which completed:\n%s", lines)
	}

	shaky := s.CompareResults(compareResults("'Group 0'", 8), CompareOptions{Seed: &seed})
	if len(shaky) != 1 {
		t.Fatalf("CompareResults = %d verdict(s), want 1", len(shaky))
	}
	v := shaky[0]
	lines = strings.Join(v.Lines, "\n")
	if v.Status != VerdictFails {
		t.Fatalf("a comparison some runs of which failed = %s, want fails:\n%s", v.Status, lines)
	}
	if !strings.Contains(lines, "error: ") || !strings.Contains(lines, "division by zero") {
		t.Errorf("the failure is not under the table:\n%s", lines)
	}
	completed := 0
	for _, line := range v.Lines {
		if _, err := fmt.Sscanf(line, "compare Cfg::'Group 0' — 2 stored run(s) in Results; %d run(s) by OpenSysML", &completed); err == nil {
			break
		}
	}
	if completed <= 0 || completed >= 8 {
		t.Errorf("the header counts %d completed run(s), want some of 8:\n%s", completed, lines)
	}
	if !strings.Contains(lines, "| tool ") || !strings.Contains(lines, "| OpenSysML") {
		t.Errorf("the completed runs are not tabled beside the stored ones:\n%s", lines)
	}
}

// A comparison draws under the configuration's own policy without setting the
// session's: whoever reads %draws while the runs are made sees the session's.
func TestCompareKeepsTheSessionDrawPolicy(t *testing.T) {
	s := compareSession(t)
	seed := uint64(1)
	results := compareResults("'Group 0'", 64)
	results.Configurations[0].Draws = "max"

	done := make(chan struct{})
	var seen sync.Map
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-done:
				return
			default:
				seen.Store(s.Draws(), true)
			}
		}
	}()
	got := s.CompareResults(results, CompareOptions{Seed: &seed})
	close(done)
	wg.Wait()

	seen.Range(func(policy, _ any) bool {
		if policy != runtime.DrawRandom {
			t.Errorf("the session's draws read %s while the comparison ran, want %s", policy, runtime.DrawRandom)
		}
		return true
	})
	if s.Draws() != runtime.DrawRandom {
		t.Errorf("the session's draws are %s after the comparison, want %s", s.Draws(), runtime.DrawRandom)
	}
	if len(got) != 1 || !got[0].Holds() {
		t.Fatalf("a comparison under max = %+v, want it to hold", got)
	}
	if lines := strings.Join(got[0].Lines, "\n"); !strings.Contains(lines, "64 run(s) by OpenSysML, draws max") {
		t.Errorf("the runs were not made under the configuration's policy:\n%s", lines)
	}
}

// An observable the completed runs produce in more than one unit — a quantity in
// some, a bare number or another unit in others — has no one distribution to set
// beside the tool's, whichever run comes first; runs in one unit are compared in it.
func TestComparisonTableRefusesMixedUnits(t *testing.T) {
	quantity := func(n float64, unit string) runtime.Value {
		return runtime.NewQuantityValue(&runtime.Quantity{
			Num:  semantics.Value{Kind: semantics.ValReal, Real: n},
			Unit: runtime.Unit{Text: unit},
		})
	}
	bare := runtime.Value{Kind: runtime.ValConst, Const: semantics.Value{Kind: semantics.ValReal, Real: 3}}
	row := func(v runtime.Value) runtime.SweepRow {
		return runtime.SweepRow{Outputs: []runtime.CalcOutputValue{{Name: "target.total", Value: v}}}
	}
	cfg := &compareResults("'Group 1'", 2).Configurations[0]
	for name, rows := range map[string][]runtime.SweepRow{
		"quantity first":   {row(quantity(2, "s")), row(bare)},
		"bare first":       {row(bare), row(quantity(2, "s"))},
		"two units":        {row(quantity(2, "s")), row(quantity(2000, "ms"))},
		"failed run apart": {row(quantity(2, "s")), {Err: fmt.Errorf("boom")}, row(bare)},
	} {
		got := strings.Join(comparisonTable(cfg, runtime.SweepTable{Target: "Cfg::'Group 1'", Rows: rows}, nil), "\n")
		if !strings.Contains(got, "note: target.total came to numbers in more than one unit (") || !strings.Contains(got, "so total is not compared") {
			t.Errorf("%s: mixed units compared:\n%s", name, got)
		}
		if strings.Contains(got, "difference") {
			t.Errorf("%s: a difference is given over mixed units:\n%s", name, got)
		}
	}
	same := runtime.SweepTable{Target: "Cfg::'Group 1'", Rows: []runtime.SweepRow{row(quantity(2, "s")), row(quantity(4, "s"))}}
	got := strings.Join(comparisonTable(cfg, same, nil), "\n")
	if !strings.Contains(got, "2.0 [s]") || !strings.Contains(got, "difference") {
		t.Errorf("one unit not compared in it:\n%s", got)
	}
}

// An observable some completed runs produce as no number — a boolean, a string —
// is not compared over the runs that produced a number, whichever come first: the
// note counts the runs that hold no number, as it says when none does.
func TestComparisonTableRefusesNonnumericRuns(t *testing.T) {
	number := func(n float64) runtime.Value {
		return runtime.Value{Kind: runtime.ValConst, Const: semantics.Value{Kind: semantics.ValReal, Real: n}}
	}
	flag := runtime.Value{Kind: runtime.ValConst, Const: semantics.Value{Kind: semantics.ValBool, Bool: true}}
	row := func(v runtime.Value) runtime.SweepRow {
		return runtime.SweepRow{Outputs: []runtime.CalcOutputValue{{Name: "target.total", Value: v}}}
	}
	cfg := &compareResults("'Group 1'", 2).Configurations[0]
	for name, rows := range map[string][]runtime.SweepRow{
		"number first":     {row(number(2)), row(flag), row(number(4))},
		"no number first":  {row(flag), row(number(2)), row(number(4))},
		"failed run apart": {row(number(2)), {Err: fmt.Errorf("boom")}, row(flag), row(number(4))},
	} {
		got := strings.Join(comparisonTable(cfg, runtime.SweepTable{Target: "Cfg::'Group 1'", Rows: rows}, nil), "\n")
		if !strings.Contains(got, "note: target.total holds no number in 1 of the 3 completed run(s) that produced it, so total is not compared") {
			t.Errorf("%s: a run holding no number is not counted:\n%s", name, got)
		}
		if strings.Contains(got, "difference") {
			t.Errorf("%s: a difference is given over runs holding no number:\n%s", name, got)
		}
	}
	none := runtime.SweepTable{Target: "Cfg::'Group 1'", Rows: []runtime.SweepRow{row(flag), row(flag)}}
	got := strings.Join(comparisonTable(cfg, none, nil), "\n")
	if !strings.Contains(got, "note: target.total holds no number in any completed run, so total is not compared") {
		t.Errorf("no run holding a number:\n%s", got)
	}
}

// An observable some completed runs do not produce at all — the target's attribute
// was never given a value in them — is not compared over the runs that produced it,
// whichever come first: the note counts the runs that produced it against all completed.
func TestComparisonTableRefusesRunsMissingTheObservable(t *testing.T) {
	number := func(n float64) runtime.Value {
		return runtime.Value{Kind: runtime.ValConst, Const: semantics.Value{Kind: semantics.ValReal, Real: n}}
	}
	flag := runtime.Value{Kind: runtime.ValConst, Const: semantics.Value{Kind: semantics.ValBool, Bool: true}}
	row := func(v runtime.Value) runtime.SweepRow {
		return runtime.SweepRow{Outputs: []runtime.CalcOutputValue{{Name: "target.total", Value: v}}}
	}
	without := runtime.SweepRow{Outputs: []runtime.CalcOutputValue{{Name: "clock", Value: number(1)}}}
	cfg := &compareResults("'Group 1'", 2).Configurations[0]
	for name, rows := range map[string][]runtime.SweepRow{
		"number first":     {row(number(2)), without, row(number(4))},
		"missing first":    {without, row(number(2)), row(number(4))},
		"failed run apart": {row(number(2)), {Err: fmt.Errorf("boom")}, without, row(number(4))},
	} {
		got := strings.Join(comparisonTable(cfg, runtime.SweepTable{Target: "Cfg::'Group 1'", Rows: rows}, nil), "\n")
		if !strings.Contains(got, "note: target.total was produced by 2 of the 3 completed run(s), so total is not compared") {
			t.Errorf("%s: a run producing no value is not counted:\n%s", name, got)
		}
		if strings.Contains(got, "difference") {
			t.Errorf("%s: a difference is given over runs producing no value:\n%s", name, got)
		}
	}
	mixed := runtime.SweepTable{Target: "Cfg::'Group 1'", Rows: []runtime.SweepRow{row(number(2)), without, row(flag)}}
	got := strings.Join(comparisonTable(cfg, mixed, nil), "\n")
	if !strings.Contains(got, "note: target.total was produced by 2 of the 3 completed run(s) and holds no number in 1 of those, so total is not compared") {
		t.Errorf("a run producing no value beside one holding no number:\n%s", got)
	}
}

// A configuration whose tool stored no snapshot is run all the same: its table
// has the runs' row under one saying there is no stored result, over every
// numeric feature the runs produced, or over the observable its analysis
// summarises when it has one; none is skipped.
func TestCompareRunsConfigurationsWithoutStoredResults(t *testing.T) {
	s := compareSession(t)
	seed := uint64(1)
	results := compareResults("'Group 1'", 3)
	results.Configurations[0].Snapshots = nil
	results.Configurations[0].Observables = nil
	results.Configurations[0].Location = "Empty"

	got := s.CompareResults(results, CompareOptions{Seed: &seed})
	if len(got) != 1 || !got[0].Holds() {
		t.Fatalf("a configuration without stored results = %+v, want it run", got)
	}
	lines := strings.Join(got[0].Lines, "\n")
	for _, want := range []string{
		"compare Cfg::'Group 1' — no stored run in Empty; 3 run(s) by OpenSysML",
		"clock      | tool (no stored result to compare) | 0    |",
		"           | OpenSysML (clock)                  | 3    | 0.0 [s] | 0.0 [s] | 0.0 [s] | 0.0 [s] | 0.0 [s]",
		"total      | tool (no stored result to compare) | 0    |",
		"           | OpenSysML (target.total)           | 3    | 4.0     | 4.0     | 4.0     | 4.0     | 4.0",
	} {
		if !strings.Contains(lines, want) {
			t.Errorf("the table lacks %q:\n%s", want, lines)
		}
	}
	if strings.Contains(lines, "difference") {
		t.Errorf("a difference is given against no stored result:\n%s", lines)
	}

	results.Configurations[0].Analysis = "total"
	got = s.CompareResults(results, CompareOptions{Seed: &seed})
	if len(got) != 1 || !got[0].Holds() {
		t.Fatalf("a configuration analysing total = %+v, want it run", got)
	}
	lines = strings.Join(got[0].Lines, "\n")
	if !strings.Contains(lines, "total      | tool (no stored result to compare)") {
		t.Errorf("the analysed observable is not compared:\n%s", lines)
	}
	results.Configurations[0].Analysis = "elapsed"
	got = s.CompareResults(results, CompareOptions{Seed: &seed})
	lines = strings.Join(got[0].Lines, "\n")
	if !strings.Contains(lines, "elapsed    | tool (no stored result to compare)") || strings.Contains(lines, "\ntotal ") {
		t.Errorf("the analysed observable alone is not compared:\n%s", lines)
	}
	if !strings.Contains(lines, "note: no completed run produced target.elapsed, which answers elapsed") {
		t.Errorf("an analysed observable the runs never produce is not noted:\n%s", lines)
	}
}

// A configuration stating no numberOfRuns is run once, as its tool runs it, and
// the table says so; -runs overrides that as it does a stated count.
func TestCompareDefaultsToOneRun(t *testing.T) {
	s := compareSession(t)
	seed := uint64(1)
	results := compareResults("'Group 1'", 0)

	got := s.CompareResults(results, CompareOptions{Seed: &seed})
	if len(got) != 1 || !got[0].Holds() {
		t.Fatalf("a configuration without numberOfRuns = %+v, want one run", got)
	}
	lines := strings.Join(got[0].Lines, "\n")
	if !strings.Contains(lines, "2 stored run(s) in Results; 1 run(s) by OpenSysML") {
		t.Errorf("one run is not made:\n%s", lines)
	}
	if !strings.Contains(lines, "note: the configuration states no numberOfRuns, so one run is made, as its tool makes without one; -runs <number> makes more") {
		t.Errorf("the default is not noted:\n%s", lines)
	}

	got = s.CompareResults(results, CompareOptions{Seed: &seed, Runs: 5})
	lines = strings.Join(got[0].Lines, "\n")
	if !strings.Contains(lines, "5 run(s) by OpenSysML") || strings.Contains(lines, "states no numberOfRuns") {
		t.Errorf("-runs does not override the default:\n%s", lines)
	}
}

// An observable the tool summarised — a snapshot holding the mean of several
// runs, not one run's value — is compared by count and mean alone, pooled with
// the runs stored one by one; the statistics the tool did not keep stay blank,
// and each summary is noted with what it recorded.
func TestComparisonTablePoolsSummarisedResults(t *testing.T) {
	number := func(n float64) runtime.Value {
		return runtime.Value{Kind: runtime.ValConst, Const: semantics.Value{Kind: semantics.ValReal, Real: n}}
	}
	row := func(n float64) runtime.SweepRow {
		return runtime.SweepRow{Outputs: []runtime.CalcOutputValue{{Name: "target.total", Value: number(n)}}}
	}
	cfg := &compareResults("'Group 1'", 2).Configurations[0]
	cfg.Analysis = "total"
	cfg.Snapshots = append(cfg.Snapshots, simresults.Snapshot{
		ID: "_sum", Name: "analysis", Values: map[string]float64{"total": 10.0},
		Statistics: &simresults.Statistics{Observable: "total", Runs: 8, Mean: 10.0, Deviation: 1.5},
	})
	if runs := cfg.StoredRuns(); runs != 10 {
		t.Errorf("StoredRuns = %d, want 10: two stored one by one and eight summarised", runs)
	}
	if got := storedRuns(cfg); got != "10 stored run(s) over 3 snapshot(s)" {
		t.Errorf("storedRuns = %q", got)
	}

	table := runtime.SweepTable{Target: "Cfg::'Group 1'", Rows: []runtime.SweepRow{row(8), row(10)}}
	got := strings.Join(comparisonTable(cfg, table, nil), "\n")
	for _, want := range []string{
		"total      | tool                     | 10   |     | 9.0   |     |      |",
		"           | OpenSysML (target.total) | 2    | 8.0 | 9.0   | 8.0 | 10.0 | 10.0",
		"           | difference               |      |     | +0.0% |     |      |",
		`note: "analysis" summarises 8 run(s) of total: mean 10.0, deviation 1.5`,
		"note: target.total came to a deviation of 1.4142135623730951 over the 2 completed run(s)",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("the pooled table lacks %q:\n%s", want, got)
		}
	}

	cfg.Snapshots = cfg.Snapshots[2:]
	got = strings.Join(comparisonTable(cfg, table, nil), "\n")
	if !strings.Contains(got, "| tool                     | 8    |     | 10.0   |") || !strings.Contains(got, "| -10.0% |") {
		t.Errorf("a summary alone is not compared by count and mean:\n%s", got)
	}
}

// A simple name naming configurations of several packages compares none of them
// and says which it could name; an id or a qualified name compares its one.
func TestCompareRefusesAnAmbiguousName(t *testing.T) {
	s := compareSession(t)
	seed := uint64(1)
	results := compareResults("'Group 1'", 2)
	twin := results.Configurations[0]
	twin.ID, twin.Name = "_d", "Other::'Group 1'"
	results.Configurations = append(results.Configurations, twin)

	got := s.CompareResults(results, CompareOptions{Seed: &seed, Only: []string{"Group 1"}})
	if len(got) != 1 || got[0].Holds() || got[0].Subject != "compare Group 1" {
		t.Fatalf("an ambiguous name = %+v, want one refusal", got)
	}
	if lines := strings.Join(got[0].Lines, "\n"); !strings.Contains(lines, "2 configurations are named Group 1 (Cfg::'Group 1', Other::'Group 1')") {
		t.Errorf("the refusal does not list the configurations:\n%s", lines)
	}

	for name, subject := range map[string]string{"Cfg::'Group 1'": "compare Cfg::'Group 1'", "_d": "compare Other::'Group 1'"} {
		got := s.CompareResults(results, CompareOptions{Seed: &seed, Only: []string{name}})
		if len(got) != 1 || got[0].Subject != subject {
			t.Errorf("%s = %+v, want the one configuration %s", name, got, subject)
		}
	}
	one := results.Configurations[:1]
	got = s.CompareResults(&simresults.Results{Source: results.Source, Configurations: one}, CompareOptions{Seed: &seed, Only: []string{"Group 1", "'Group 1'"}})
	if len(got) != 1 || !got[0].Holds() {
		t.Errorf("a simple name borne by one configuration = %+v, want it compared once", got)
	}
}

func TestSameNameSplitsOutsideQuotes(t *testing.T) {
	for _, c := range []struct {
		name, qualified string
		want            bool
	}{
		{"A::B", "Cfg::'A::B'", true},
		{"'A::B'", "Cfg::'A::B'", true},
		{"B", "Cfg::'A::B'", false},
		{"B'", "Cfg::'A::B'", false},
		{"A::B", "Cfg::A::B", false},
		{"B", "Cfg::A::B", true},
		{`'it\'s'`, `Cfg::'it\'s'`, true},
		{"'it'", `Cfg::'it\'s'`, false},
		{"s'", `Cfg::'it\'s'`, false},
		{"Group 1", "'Cfg::Pkg'::'Group 1'", true},
		{"Pkg'::'Group 1'", "'Cfg::Pkg'::'Group 1'", false},
		{"Group 1", "Group 1", true},
		{"Group 1", "'Group 1'", true},
	} {
		if got := sameName(c.name, c.qualified); got != c.want {
			t.Errorf("sameName(%q, %q) = %t, want %t", c.name, c.qualified, got, c.want)
		}
	}

	s := compareSession(t)
	seed := uint64(1)
	results := compareResults("'Sub::Group'", 2)
	for _, name := range []string{"Sub::Group", "'Sub::Group'", "Cfg::'Sub::Group'"} {
		got := s.CompareResults(results, CompareOptions{Seed: &seed, Only: []string{name}})
		if len(got) != 1 || !got[0].Holds() || got[0].Subject != "compare Cfg::'Sub::Group'" {
			t.Errorf("-action %s = %+v, want the configuration named 'Sub::Group' compared", name, got)
		}
	}
	got := s.CompareResults(results, CompareOptions{Seed: &seed, Only: []string{"Group"}})
	if len(got) != 1 || got[0].Holds() || !strings.Contains(strings.Join(got[0].Lines, "\n"), "no configuration is named Group") {
		t.Errorf("-action Group over a configuration named 'Sub::Group' = %+v, want a refusal", got)
	}
}
