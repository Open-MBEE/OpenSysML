package analysis

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/check/passes"
	"github.com/Open-MBEE/OpenSysML/internal/exec/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
)

// drawingModel draws a Real as its action starts, so any run of it needs a model seed.
const drawingModel = `
package test {
	private import ScalarValues::*;
	private import RandomFunctions::*;
	action draw {
		out attribute d : Real = uniform(0.0, 1.0);
		first start; then done;
	}
	calc def Twice { in x : Real; return : Real = x * uniform(1.0, 1.0); }
}`

// drawing is drawingModel indexed over the bundled libraries, built as a surface holding
// no context supplies a model, its source registered as a surface registers the files it read.
type drawing struct {
	idx    *symbols.Index
	pkg    *symbols.Scope
	source *source.SourceFile
}

func parseDrawing(t *testing.T) *drawing {
	t.Helper()
	return parseDrawingText(t, drawingModel)
}

// parseDrawingText is parseDrawing over another model text with a `test` package.
func parseDrawingText(t *testing.T, text string) *drawing {
	t.Helper()
	path := filepath.Join(t.TempDir(), "drawing.sysml")
	sf := source.New(path, []byte(text))
	p := parser.New(sf)
	file := p.ParseFile()
	if len(p.Diagnostics) > 0 {
		t.Fatalf("parse: %v", p.Diagnostics)
	}
	idx := libs.NewModelIndex()
	idx.AddDocument(path, file)
	idx.ExpandWildcardImports()
	pkg, ok := idx.DocumentRoot(path).LookupLocal("test")
	if !ok || pkg.Scope == nil {
		t.Fatal("test package not indexed")
	}
	return &drawing{idx: idx, pkg: pkg.Scope, source: sf}
}

func (d *drawing) semantics() (*runtime.Model, error) {
	resolver := resolve.New(d.idx)
	model := runtime.NewModel(passes.NewTypedModel(resolver), resolver)
	model.SetExpressionParser(parser.ParseOneExpression)
	model.RegisterSource(d.source)
	return model, nil
}

func (d *drawing) fresh(w *Worker) (*runtime.Context, error) {
	return runtime.NewContext(w.Model, fixtureSteps), nil
}

func (d *drawing) building() *Model { return &Model{Semantics: d.semantics, Fresh: d.fresh} }

func (d *drawing) symbol(t *testing.T, name string) *symbols.Symbol {
	t.Helper()
	sym, ok := d.pkg.LookupLocal(name)
	if !ok {
		t.Fatalf("%s not indexed", name)
	}
	return sym
}

// run is one run of the drawing action, its outcome the value drawn.
func (d *drawing) run(t *testing.T) Linearization {
	t.Helper()
	draw := d.symbol(t, "draw")
	return func(ctx *runtime.Context) (runtime.Outcome, error) {
		outputs, err := ctx.ExecuteAction(draw)
		if err != nil {
			return runtime.Outcome{}, err
		}
		return ctx.ActionOutcome(outputs), nil
	}
}

// start starts the drawing action as a check does.
func (d *drawing) start(t *testing.T) func(*runtime.Context) (*runtime.Invocation, error) {
	t.Helper()
	draw := d.symbol(t, "draw")
	return func(ctx *runtime.Context) (*runtime.Invocation, error) {
		exec, err := ctx.CreateActionExecutor(draw)
		if err != nil {
			return nil, err
		}
		return &runtime.Invocation{Actions: []*runtime.ActionExecutor{exec}}, nil
	}
}

// seeded is the request for subject under schedule with the model seed, when one is given.
func (d *drawing) seeded(subject string, schedule runtime.SchedulePolicy, seed ...uint64) Request {
	req := request(d.building(), subject, schedule)
	if len(seed) > 0 {
		req.ModelSeed = ModelSeed{Seed: seed[0], Set: true}
	}
	return req
}

// drawn is the drawing action's output d of one performance through the registry.
func (d *drawing) drawn(t *testing.T, req Request) (runtime.Value, error) {
	t.Helper()
	draw := d.symbol(t, "draw")
	out, _, err := Perform(context.Background(), Default(), req,
		func(ctx *runtime.Context) (map[string]runtime.Value, error) { return ctx.ExecuteAction(draw) },
		func(out map[string]runtime.Value, err error) Answer {
			if err != nil {
				return Answer{Err: err}
			}
			return Answer{Claim: ClaimValue, Values: ValuesOf(out)}
		})
	if err != nil {
		return runtime.Value{}, err
	}
	return out["d"], nil
}

// A request's model seed reaches the context the run engine builds: the same seed draws
// the same value under either schedule, another seed another value, and no seed the refusal.
func TestRunEngineDrawsFromTheRequestsModelSeed(t *testing.T) {
	d := parseDrawing(t)
	first, err := d.drawn(t, d.seeded("test::draw", runtime.DefaultSchedulePolicy, 7))
	if err != nil {
		t.Fatalf("seeded run: %v", err)
	}
	reversed, err := d.drawn(t, d.seeded("test::draw", policy(t, "reverse"), 7))
	if err != nil {
		t.Fatalf("seeded reverse run: %v", err)
	}
	if runtime.FormatValue(first) != runtime.FormatValue(reversed) {
		t.Errorf("seed 7 drew %s then %s under another schedule, want the model seed independent of it", runtime.FormatValue(first), runtime.FormatValue(reversed))
	}
	other, err := d.drawn(t, d.seeded("test::draw", runtime.DefaultSchedulePolicy, 8))
	if err != nil {
		t.Fatalf("run under seed 8: %v", err)
	}
	if runtime.FormatValue(other) == runtime.FormatValue(first) {
		t.Errorf("seeds 7 and 8 both drew %s, want distinct draws", runtime.FormatValue(first))
	}
	if _, err := d.drawn(t, d.seeded("test::draw", runtime.DefaultSchedulePolicy)); !errors.Is(err, runtime.ErrUnseededDraw) {
		t.Fatalf("unseeded run: %v, want ErrUnseededDraw", err)
	}
}

// The explorer and the checker seed every run of theirs from the request; without a seed
// a drawing run fails as unseeded, an outcome to the one and a violation to the other.
func TestExploreAndCheckDrawFromTheRequestsModelSeed(t *testing.T) {
	d := parseDrawing(t)
	plan, err := Default().Explore(context.Background(), d.seeded("test::draw", policy(t, "explore"), 7), d.run(t))
	if err != nil {
		t.Fatalf("seeded explore: %v", err)
	}
	if x := plan.Result.Exploration(); x == nil || !x.Complete() || len(x.Outcomes) != 1 {
		t.Fatalf("exploration %+v, want the one drawn outcome", plan.Result.Exploration())
	}
	plan, err = Default().Explore(context.Background(), d.seeded("test::draw", policy(t, "explore")), d.run(t))
	if err != nil {
		t.Fatalf("unseeded explore: %v", err)
	}
	if x := plan.Result.Exploration(); x == nil || len(x.Outcomes) != 1 || !errors.Is(x.Outcomes[0].Outcome.Err, runtime.ErrUnseededDraw) {
		t.Fatalf("unseeded exploration %+v, want its one run refused as unseeded", plan.Result.Exploration())
	}
	ask := &CheckAsk{Start: d.start(t)}
	plan, err = Default().Check(context.Background(), d.seeded("test::draw", policy(t, "explore"), 7), Holds, FreeSchedule, ask, nil, d.run(t))
	if err != nil {
		t.Fatalf("seeded check: %v", err)
	}
	if c := plan.Result.Check(); c == nil || c.Report == nil || c.Report.Verdict != runtime.CheckExhaustive || len(c.Violations) != 0 {
		t.Fatalf("check %+v, want an exhaustive search without violation", plan.Result.Check())
	}
	plan, err = Default().Check(context.Background(), d.seeded("test::draw", policy(t, "explore")), Holds, FreeSchedule, ask, nil, d.run(t))
	if err != nil {
		t.Fatalf("unseeded check: %v", err)
	}
	c := plan.Result.Check()
	if c == nil || c.Report == nil || c.Report.Verdict != runtime.CheckViolation || len(c.Report.Violations) != 1 || !errors.Is(c.Report.Violations[0].Err, runtime.ErrUnseededDraw) {
		t.Fatalf("unseeded check %+v, want the one run's failure as the unseeded refusal", c)
	}
}

// A violation reached through draws alone is witnessed by them: the result's witness
// carries the draws where it has no choice, its standing counts them, and its
// schedule replays them.
func TestCheckWitnessCarriesTheDraws(t *testing.T) {
	d := parseDrawing(t)
	never := runtime.CheckProperty{Name: "never", Holds: func(*runtime.Context, *runtime.Invocation) (bool, error) { return false, nil }}
	ask := &CheckAsk{Start: d.start(t), Properties: []runtime.CheckProperty{never}}
	plan, err := Default().Check(context.Background(), d.seeded("test::draw", policy(t, "explore"), 7), Holds, FreeSchedule, ask, nil, d.run(t))
	if err != nil {
		t.Fatalf("seeded check: %v", err)
	}
	result := plan.Result
	if result.Claim != ClaimViolated || result.Witness == nil {
		t.Fatalf("result %+v, want the violation witnessed", result)
	}
	w := result.Witness
	if len(w.Draws) != 1 || len(w.Choices) != 0 {
		t.Fatalf("witness draws %v choices %v, want the one draw and no choice", w.Draws, w.Choices)
	}
	if replay, ok := w.Schedule.Replay(); !ok || len(replay) != 0 {
		t.Fatalf("witness schedule %s, want a replay of no choice", w.Schedule)
	}
	if got := result.Standing(); !strings.Contains(got, "witness of 1 draw replayed") {
		t.Errorf("standing %q, want the draw counted", got)
	}
}

// The sweep engine seeds the rows' contexts from the request; a request without a seed
// leaves a drawing row refused as any unseeded run is.
func TestSweepDrawsFromTheRequestsModelSeed(t *testing.T) {
	d := parseDrawing(t)
	twice := d.symbol(t, "Twice")
	model, err := d.semantics()
	if err != nil {
		t.Fatal(err)
	}
	ctx := runtime.NewContext(model, fixtureSteps)
	plan := runtime.SweepPlan{Ranges: []runtime.SweepRange{{Param: "x", From: intOf(1), To: intOf(2)}}}
	resolved, err := ctx.ResolveSweepPlan(twice, plan, 0, nil)
	if err != nil {
		t.Fatalf("resolve the plan: %v", err)
	}
	row := func(ctx *runtime.Context, bindings []runtime.SweepBinding) (runtime.SweepRunResult, error) {
		bound := make(map[string]runtime.Value, len(bindings))
		for _, b := range bindings {
			bound[b.Param] = b.Value
		}
		value, err := ctx.InvokeCalcWith(twice, nil, bound, d.pkg)
		if err != nil {
			return runtime.SweepRunResult{}, err
		}
		return runtime.SweepRunResult{Outputs: []runtime.CalcOutputValue{{Name: "result", Value: value}}}, nil
	}
	answered, err := Default().Sweep(context.Background(), d.seeded("test::Twice", ctx.Schedule(), 7), resolved, row)
	if err != nil {
		t.Fatalf("seeded sweep: %v", err)
	}
	table := answered.Result.Table()
	if len(table.Rows) != 2 {
		t.Fatalf("table %+v, want 2 rows", table)
	}
	for _, r := range table.Rows {
		if r.Err != nil {
			t.Errorf("row %v: %v, want it drawn under the request's seed", r.Bindings, r.Err)
		}
	}
	answered, err = Default().Sweep(context.Background(), d.seeded("test::Twice", ctx.Schedule()), resolved, row)
	if err != nil {
		t.Fatalf("unseeded sweep: %v", err)
	}
	for _, r := range answered.Result.Table().Rows {
		if !errors.Is(r.Err, runtime.ErrUnseededDraw) {
			t.Errorf("row %v: %v, want ErrUnseededDraw", r.Bindings, r.Err)
		}
	}
}
