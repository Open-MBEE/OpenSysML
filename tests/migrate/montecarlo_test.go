package migrate_test

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/migrate"
	"github.com/Open-MBEE/OpenSysML/internal/translate/simresults"
)

// montecarlo.xmi is a block inheriting the MagicDraw customization's
// MonteCarloAnalysis, whose binding connector wires its t to the pattern's Mean,
// with a result package holding one snapshot of a run and several of the
// pattern's statistics: a whole one, one holding the Mean as another feature — which
// is that feature's value and no statistic of t, the numbers' equality being no
// evidence of the run — one without a Mean, one recording no Deviation, one with a
// fractional N, one holding no t, one whose Mean is a string and Deviation NaN, and one
// the tool left blank. A second configuration stores nothing, a third targets an
// analysis binding no feature.
func TestMonteCarloAnalysisSnapshotsAreSummaries(t *testing.T) {
	r := migrateFixtureFile(t, "montecarlo")
	if r.Results == nil || len(r.Results.Configurations) != 3 {
		t.Fatalf("results = %+v, want three configurations", r.Results)
	}
	want := []simresults.ConfigurationResults{{
		ID: "_g0", Name: "'Group 0'", Runs: 3, Draws: "average",
		Target: "target", Behavior: "run", Location: "Results", Analysis: "t",
		Observables: []string{"p", "t", "u"},
		Snapshots: []simresults.Snapshot{
			{ID: "_raw", Name: "run 1", Values: map[string]float64{"p": 0.5, "t": 2}},
			{ID: "_sum", Name: "analysis of 4 runs", Values: map[string]float64{"t": 3.5},
				Statistics: &simresults.Statistics{Observable: "t", Runs: 4, Mean: 3.5, Deviation: simresults.Real(0.5), OutOfSpec: simresults.Count(1)}},
			{ID: "_foreign", Name: "analysis of u", Values: map[string]float64{"u": 9}},
			{ID: "_half", Name: "half an analysis", Values: map[string]float64{"t": 6}},
			{ID: "_lean", Name: "analysis without a deviation", Values: map[string]float64{"t": 7},
				Statistics: &simresults.Statistics{Observable: "t", Runs: 2, Mean: 7}},
			{ID: "_odd", Name: "analysis of two and a half runs", Values: map[string]float64{"t": 4}},
			{ID: "_vast", Name: "analysis of more runs than a count holds", Values: map[string]float64{"t": 4}},
			{ID: "_stray", Name: "analysis of nothing held", Values: map[string]float64{}},
			{ID: "_garbled", Name: "analysis with a garbled mean", Values: map[string]float64{"t": 3}},
			{ID: "_blank", Name: "analysis left blank", Values: map[string]float64{"t": 5}},
		},
		Notes: []string{
			"the slot of MonteCarloAnalysis::Deviation holds \"NaN\", which is no finite number in 1 snapshot(s), so it is not among the results",
			"the slot of MonteCarloAnalysis::Mean holds a LiteralString, which is no number in 1 snapshot(s), so it is not among the results",
			"2 snapshot(s) record MonteCarloAnalysis statistics whose Mean no value of t holds, though the analysis binds the two, so the statistics are not read",
			"1 snapshot(s) record a MonteCarloAnalysis statistic that is no number, so they hold no statistics",
			"1 snapshot(s) record a MonteCarloAnalysis::N of 2.5, which is no count of runs, so they hold no statistics",
			"1 snapshot(s) record a MonteCarloAnalysis::N of 9.223372036854776e+18, which is more runs than a count holds, so they hold no statistics",
			"1 snapshot(s) record no MonteCarloAnalysis::N and Mean together, so they hold no statistics",
		},
	}, {
		ID: "_g5", Name: "'Group 1'", Draws: "average",
		Target: "target", Behavior: "run", Location: "Empty", Analysis: "t",
		Observables: []string{}, Snapshots: []simresults.Snapshot{},
		Notes: []string{"the result location Empty holds no snapshot of the target's classifier"},
	}, {
		ID: "_g6", Name: "'Group 2'", Runs: 1, Draws: "average",
		Target: "target", Behavior: "run", Location: "Unbound Results",
		Observables: []string{"t"},
		Snapshots:   []simresults.Snapshot{{ID: "_ub1", Name: "run 1", Values: map[string]float64{"t": 1}}},
		Notes: []string{
			"'Unbound Analysis' inherits MonteCarloAnalysis but binds its Mean to no feature, so its statistics summarise no observable",
			"the slot of MonteCarloAnalysis::N holds a statistic of no observable the target analyses in 1 snapshot(s), so it is not among the results",
		},
	}}
	for i, cfg := range r.Results.Configurations {
		if cfg.Observables == nil {
			cfg.Observables = []string{}
		}
		if !reflect.DeepEqual(cfg, want[i]) {
			gotJSON, _ := json.MarshalIndent(cfg, "", "  ")
			wantJSON, _ := json.MarshalIndent(want[i], "", "  ")
			t.Errorf("configuration %d:\n%s\nwant:\n%s", i, gotJSON, wantJSON)
		}
	}
	group0 := r.Results.Configurations[0]
	if values := group0.Values("t"); !reflect.DeepEqual(values, []float64{2, 6, 4, 4, 3, 5}) {
		t.Errorf("Values(t) = %v, want the runs stored one by one, without the summary's mean", values)
	}
	if runs := group0.StoredRuns(); runs != 14 {
		t.Errorf("StoredRuns = %d, want 6 summarised and 8 stored one by one", runs)
	}
	if got, want := r.Results.Summary(), "results of 3 run configuration(s): 2 with 11 stored snapshot(s) standing for 15 run(s)"; got != want {
		t.Errorf("Summary = %q, want %q", got, want)
	}

	wantLine(t, r.Notation, "/* results of the simulation tool: 10 snapshot(s) in Results standing for 14 run(s) analysing t holding p, t, u */")
	wantLine(t, r.Notation, "/* results of the simulation tool: 0 snapshot(s) in Empty analysing t */")
	wantLine(t, r.Notation, "/* results of the simulation tool: 1 snapshot(s) in Unbound Results holding t */")
	wantNote(t, r, "_analysis", migrate.Approximated, "generalization of the simulation tool's MonteCarloAnalysis is not written: v2 has no analysis pattern for the statistics it computes over the runs, which the migration results read from the result snapshots")
	wantNote(t, r, "_bind", migrate.Unmapped, "the connector binds t to the simulation tool's MonteCarloAnalysis::Mean, the statistic it computes of t over the runs, which v2 has no analysis pattern for; the migration results read the statistic from the result snapshots")
	wantNote(t, r, "_sumMean", migrate.Unmapped, "the slot holds the simulation tool's MonteCarloAnalysis::Mean statistic of the runs, which is no value of the instance; the migration results read it")
	wantNote(t, r, "_sumT", migrate.Mapped, "")
	wantNote(t, r, "_rawN", migrate.Unmapped, "the slot holds the simulation tool's MonteCarloAnalysis::N statistic of the runs, which is no value of the instance; the migration results read it")
	wantClean(t, "montecarlo.sysml", r)
}
