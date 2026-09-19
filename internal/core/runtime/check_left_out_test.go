package runtime

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/lower"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// leftOutRun drives the looping-do machine move by move, first enabled move
// each, to the first state its moves leave the do round before a dispatch out at.
func leftOutRun(t *testing.T, m *exploreModel) (*checker, *StateExecutor) {
	t.Helper()
	ctx, err := m.fresh()
	if err != nil {
		t.Fatal(err)
	}
	mustSchedule(t, ctx, checkPolicy(&checkScript{due: -1}))
	c := &checker{
		ctx:            ctx,
		visited:        make(map[stateKey]*visitedState),
		onStack:        make(map[stateKey]int),
		finals:         make(map[string]int),
		futures:        make(map[futureKey]lower.Footprint),
		machineFutures: make(map[*lower.StateGraph]lower.Footprint),
		budgets:        ctx.Budgets(),
	}
	run, err := beginInvocation(ctx, stateStarterOf(m.state(t, "Machine"), HorizonAt(5)))
	if err != nil {
		t.Fatal(err)
	}
	if err := run.inv.started(ctx); err != nil {
		t.Fatal(err)
	}
	c.inv, c.run = run.inv, run
	for moves := 0; ; moves++ {
		if err := run.stabilize(); err != nil {
			t.Fatalf("settling after %d moves: %v", moves, err)
		}
		if len(run.leftOut()) > 0 {
			return c, run.inv.States[0]
		}
		if run.terminal() {
			t.Fatalf("the run ended after %d moves without leaving a do round out", moves)
		}
		if _, err := run.makeMove(run.enabledMoves()[0]); err != nil {
			t.Fatalf("move %d: %v", moves+1, err)
		}
	}
}

// A machine whose do round was left standing before its dispatch is a state of
// its own: reached first with each ready token moved, then with one left standing,
// the second visit is new and records the round the moves leave out.
func TestCheckTellsAMachineLeavingItsDoRoundStandingFromOneThatDidNot(t *testing.T) {
	c, machine := leftOutRun(t, loopingDoModel(t, false))
	standing := func(left bool) {
		for _, act := range machine.doActions {
			if act.run != nil {
				act.run.host.flow.leftStanding = left
			}
		}
	}
	standing(false)
	if names := c.run.leftOut(); len(names) != 0 {
		t.Fatalf("each token moved: leaves out %v, want nothing", names)
	}
	_, moved, _, visited, err := c.visit(0)
	if err != nil || visited || len(c.notEnumerated) != 0 {
		t.Fatalf("first visit: visited %v, not enumerated %v, err %v; want a new state leaving nothing out", visited, c.notEnumerated, err)
	}
	standing(true)
	_, left, _, visited, err := c.visit(0)
	if err != nil || visited || left == moved {
		t.Fatalf("visit left standing: visited %v, same key %v, err %v; want a new state of its own", visited, left == moved, err)
	}
	if !slices.Equal(c.notEnumerated, []string{NotEnumeratedDoRound}) {
		t.Fatalf("visit left standing: not enumerated %v, want the do round before the dispatch", c.notEnumerated)
	}
}

// enablingBranchModel loads the conformance case whose do body's `setter` branch
// writes the feature the `watcher` branch's change wait blocks on, under a timed exit.
func enablingBranchModel(t *testing.T) *exploreModel {
	t.Helper()
	text, err := os.ReadFile(filepath.Join("testdata", "conformance", "state_do_action_branch_enables_other_before_exit.sysml"))
	if err != nil {
		t.Fatal(err)
	}
	return parseLibraryModel(t, string(text))
}

// A token another token's move enables is left standing too: `raise` moves alone and
// frees `watch`, which a fixed policy's sweep takes before the dispatch the check makes.
func TestCheckTellsADoRoundStandingWhenOneBranchEnablesAnother(t *testing.T) {
	m := enablingBranchModel(t)
	report, err := Check(context.Background(), m.fresh, stateStarterOf(m.state(t, "Machine"), HorizonAt(3)), CheckBudget{}, unreduced(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if report.Verdict != CheckWithinBounds || len(report.BoundsHit) != 0 || !slices.Equal(report.NotEnumerated, []string{NotEnumeratedDoRound}) {
		t.Fatalf("%s, want no violation within bounds, the do round before the dispatch alone not enumerated", report.Status())
	}
	if len(report.Divergent) != 0 || len(report.Violations) != 0 {
		t.Fatalf("divergent %v, violations %v; want the one outcome the moves reach", report.Divergent, report.Violations)
	}
}

// tiedCallModel is the looping-do machine beside an action whose call is tied on
// its arguments' types: byResult.result is the performance's feature, byResult.mark
// the untaken overload's.
func tiedCallModel(t *testing.T) *exploreModel {
	t.Helper()
	return parseLibraryModel(t, loopingDoText(t, `
	package A {
		action def tag { in x : Integer; in y : Real; out mark : Integer; first step; action step { assign mark := 1; } }
	}
	package B {
		action def tag { in x : Real; in y : Integer; out result : Integer; first step; action step { assign result := 2; } }
	}
	calc def same { in v; v }
	action outer {
		private import A::*;
		private import B::*;
		attribute p = same(1.5);
		attribute q = same(2);
		first start;
		then action byResult = tag(x = p, y = q);
		then done;
	}
`))
}

// jointStarterOf starts the action beside the machine, up to the horizon.
func jointStarterOf(action, machine *symbols.Symbol, horizon Horizon) Starter {
	return func(ctx *Context) (*Invocation, error) {
		act, err := ctx.CreateActionExecutor(action)
		if err != nil {
			return nil, err
		}
		exec, err := ctx.CreateStateExecutor(machine)
		if err != nil {
			return nil, err
		}
		return &Invocation{Actions: []*ActionExecutor{act}, States: []*StateExecutor{exec}, Horizon: horizon}, nil
	}
}

// A path under a tied call no performance held keeps its bounded report when the
// search left a do round out, as it does under a bound hit: the round not run may
// hold the performance. The action alone, searched exhaustively, fails the check.
func TestCheckKeepsABoundedReportWhenADoRoundLeftOutMayHoldAPath(t *testing.T) {
	m := tiedCallModel(t)
	start := jointStarterOf(m.action(t, "outer"), m.state(t, "Machine"), HorizonAt(5))
	for _, name := range []string{"outer.byResult.result", "outer.byResult.mark"} {
		report, err := Check(context.Background(), m.fresh, start, CheckBudget{}, CheckOptions{Reduce: true, Diverge: []string{name}}, nil)
		if err != nil {
			t.Fatalf("Diverge %s: %v, want a bounded report", name, err)
		}
		if report.Verdict != CheckWithinBounds || len(report.BoundsHit) != 0 || !slices.Equal(report.NotEnumerated, []string{NotEnumeratedDoRound}) {
			t.Fatalf("Diverge %s: %s, want no violation within bounds, the do round before the dispatch alone not enumerated", name, report.Status())
		}
	}
	_, err := Check(context.Background(), m.fresh, starterOf(m.action(t, "outer")), CheckBudget{}, CheckOptions{Reduce: true, Diverge: []string{"byResult.mark"}}, nil)
	var unknown *UnknownCheckFeatureError
	if !errors.As(err, &unknown) || unknown.Name != "byResult.mark" {
		t.Fatalf("the action alone, exhaustive: %v, want ErrUnknownCheckFeature naming byResult.mark", err)
	}
}
