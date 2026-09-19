package analysis

import (
	"context"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/check/passes"
	"github.com/Open-MBEE/OpenSysML/internal/core/libs"
	"github.com/Open-MBEE/OpenSysML/internal/exec/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// beaconModel is an action and an object's state machine on one clock: arming
// reads the beacon, which materializes it and starts its machine at t=0, so its
// timer and the action's wait are due together at t=5, and which runs first
// decides whether the action reads the lamp lit.
const beaconModel = `package test {
	private import SI::*;
	private import ScalarValues::*;
	part def Beacon {
		attribute lit : Boolean = false;
		exhibit state blinking {
			entry; then dark;
			state dark;
			accept after 5 [s] then shining;
			state shining { entry assign lit := true; }
		}
	}
	part beacon : Beacon;
	part def Pebble;
	action watcher {
		attribute armed : Boolean = false;
		attribute sawLit : Boolean = false;
		first start;
		then action arm assign armed := beacon.lit == false;
		then action wait accept after 5 [s];
		then action look assign sawLit := beacon.lit;
		then done;
	}
}`

// parseLibraryModel is the fixture over a model naming the standard library's elements.
func parseLibraryModel(t *testing.T, model string) *fixture {
	t.Helper()
	path := filepath.Join(t.TempDir(), "analysis.sysml")
	sf := source.New(path, []byte(model))
	p := parser.New(sf)
	file := p.ParseFile()
	if len(p.Diagnostics) > 0 {
		t.Fatalf("parse: %v", p.Diagnostics)
	}
	idx := libs.NewModelIndex()
	idx.AddDocument(path, file)
	idx.ExpandWildcardImports()
	resolver := resolve.New(idx)
	pkg, ok := idx.DocumentRoot(path).LookupLocal("test")
	if !ok || pkg.Scope == nil {
		t.Fatal("test package not indexed")
	}
	return &fixture{idx: idx, model: passes.NewTypedModel(resolver), resolver: resolver, pkg: pkg.Scope, source: sf}
}

// watcherRun is one run of the watcher: the action to its end, the beacon's
// machine keeping pace on the clock the action advances.
func watcherRun(t *testing.T, f *fixture) Linearization {
	t.Helper()
	watcher := f.symbol(t, "watcher")
	return func(ctx *runtime.Context) (runtime.Outcome, error) {
		outputs, err := ctx.ExecuteAction(watcher)
		if err != nil {
			return runtime.Outcome{}, err
		}
		return ctx.ActionOutcome(outputs), nil
	}
}

// Under all, explore referees check over behaviors on one clock as it does over an
// action: the tie's due order is the divergence check witnesses, explore's complete
// table has both outcomes, and neither disputes the other.
func TestExploreRefereesCheckOverBehaviorsOnOneClock(t *testing.T) {
	f := parseLibraryModel(t, beaconModel)
	watcher := f.checked(t, "watcher")
	q := questionOf(t, "test::watcher", Outcomes, &CheckAsk{Start: watcher.start})
	q.Linearize = watcherRun(t, f)
	plan, err := Default().AnswerWith(context.Background(), f.building(), q, Budget{}, All())
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Disagreements) != 0 || plan.Result.Engine != ExploreEngineName || plan.Result.Strength != Proved {
		t.Fatalf("plan %+v, want explore's proof standing with no disagreement", plan.Result)
	}
	var checked *Checked
	var exploration *runtime.Exploration
	for _, step := range plan.Steps {
		if step.Result == nil {
			continue
		}
		switch step.Engine {
		case CheckEngineName:
			checked = step.Result.Check()
			if step.Result.Claim != ClaimSensitive || step.Result.Strength != Witnessed {
				t.Fatalf("check %+v, want the tie's sensitivity witnessed beside the proof", step.Result)
			}
		case ExploreEngineName:
			exploration = step.Result.Exploration()
		}
	}
	if checked == nil || exploration == nil || !exploration.Complete() {
		t.Fatalf("steps %v, want check's report and explore's complete table", stepNames(plan))
	}
	got, want := finals(checked), explored(exploration)
	if strings.Join(got, ";") != strings.Join(want, ";") || len(got) != 2 {
		t.Fatalf("check finals %v, explore outcomes %v, want the two orders of the tie in both", got, want)
	}
	if len(checked.Report.Divergent) != 1 || checked.Report.Divergent[0].Feature != "sawLit" {
		t.Fatalf("divergent %+v, want sawLit alone", checked.Report.Divergent)
	}
}

// Two check plans over behaviors on one clock, on two goroutines, each build
// workers of their own; run under -race, this is the isolation test for the
// shared-clock search.
func TestChecksOfBehaviorsOnOneClockHaveWorkersOfTheirOwn(t *testing.T) {
	f := parseLibraryModel(t, beaconModel)
	w := &workers{}
	model := f.recording(w)
	watcher := f.checked(t, "watcher")
	q := questionOf(t, "test::watcher", Outcomes, &CheckAsk{Start: watcher.start, Diverge: []string{"sawLit"}})
	results := make([]Result, 2)
	errs := make([]error, 2)
	var wg sync.WaitGroup
	for i := range results {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			plan, err := Default().AnswerWith(context.Background(), model, q, Budget{Jobs: 2}, Only(CheckEngineName))
			results[i], errs[i] = plan.Result, err
		}(i)
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Fatalf("plan %d: %v", i, err)
		}
		if results[i].Claim != ClaimSensitive || len(results[i].Check().Report.Divergent) != 1 || len(results[i].Check().Report.Divergent[0].Values) != 2 {
			t.Fatalf("plan %d: %+v, want sawLit divergent over 2 values", i, results[i])
		}
	}
	if resolvers, models := w.distinct(); resolvers < 2 || models < 2 {
		t.Fatalf("%d resolvers, %d models across two plans, want each plan's own", resolvers, models)
	}
}

// The witness a check returns replays in a context of its own, whatever numbers
// that context gives the objects the moves name: the schedule carries the
// bindings of the check's run, so a beacon numbered differently is still found.
func TestCheckWitnessesReplayOverObjectsNumberedAfresh(t *testing.T) {
	f := parseLibraryModel(t, beaconModel)
	watcher := f.checked(t, "watcher")
	q := questionOf(t, "test::watcher", Outcomes, &CheckAsk{Start: watcher.start, Diverge: []string{"sawLit"}})
	result := answered(t, Default(), f.building(), q, Budget{}).Result
	if result.Claim != ClaimSensitive || result.Witness == nil || result.Contrast == nil {
		t.Fatalf("result %+v, want sawLit's two values witnessed", result)
	}
	values := result.Check().Report.Divergent[0].Values
	for i, w := range []*Witness{result.Witness, result.Contrast} {
		bound, ok := w.Schedule.Witness()
		if !ok || len(bound.Objects) == 0 || !strings.Contains(bound.Objects[0].Path, "beacon") {
			t.Fatalf("witness %d: schedule %s binds %v, want the beacon the moves name", i, w.Schedule, bound.Objects)
		}
		ctx := f.context(t)
		if _, err := ctx.Instantiate(f.symbol(t, "Pebble")); err != nil {
			t.Fatal(err)
		}
		if err := ctx.SetSchedule(w.Schedule); err != nil {
			t.Fatal(err)
		}
		outcome, err := watcherRun(t, f)(ctx)
		if err != nil {
			t.Fatalf("witness %d: replay over a renumbered beacon: %v", i, err)
		}
		if err := ctx.Unfollowed(); err != nil {
			t.Fatalf("witness %d: %v", i, err)
		}
		if got := runtime.FormatValue(outcome.Outputs["sawLit"]); got != values[i].Value {
			t.Fatalf("witness %d: sawLit %s, want %s as the check saw it", i, got, values[i].Value)
		}
	}
}
