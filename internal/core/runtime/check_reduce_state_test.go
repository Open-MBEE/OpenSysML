package runtime

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/lower"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// checkedInvocation is an invocation under the check policy, with the checker
// state the reduction reads footprints through.
type checkedInvocation struct {
	run *invocationRun
	c   *checker
}

func startCheckedInvocation(t *testing.T, m *exploreModel, actions, states []string) *checkedInvocation {
	t.Helper()
	ctx, err := m.fresh()
	if err != nil {
		t.Fatal(err)
	}
	mustSchedule(t, ctx, checkPolicy(&checkScript{due: -1}))
	var actionSyms, stateSyms []*symbols.Symbol
	for _, name := range actions {
		actionSyms = append(actionSyms, m.action(t, name))
	}
	for _, name := range states {
		stateSyms = append(stateSyms, m.state(t, name))
	}
	run, err := beginInvocation(ctx, invocationOf(actionSyms, stateSyms))
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	t.Cleanup(run.inv.Release)
	c := &checker{
		ctx:            ctx,
		inv:            run.inv,
		run:            run,
		opts:           reduced(),
		futures:        make(map[futureKey]lower.Footprint),
		machineFutures: make(map[*lower.StateGraph]lower.Footprint),
	}
	return &checkedInvocation{run: run, c: c}
}

// moveOf finds the one enabled move of the named executor, after settling.
func (ci *checkedInvocation) moveOf(t *testing.T, owner string) enabledMove {
	t.Helper()
	if err := ci.run.stabilize(); err != nil {
		t.Fatalf("stabilize: %v", err)
	}
	moves := ci.run.enabledMoves()
	var found []enabledMove
	for _, m := range moves {
		if m.Owner.dueLabel() == owner {
			found = append(found, m)
		}
	}
	if len(found) != 1 {
		t.Fatalf("%s has %d moves among %s", owner, len(found), moveLabels(moves))
	}
	return found[0]
}

func placeNames(places []lower.Place) string {
	names := make([]string, len(places))
	for i, p := range places {
		names[i] = p.String()
	}
	slices.Sort(names)
	return strings.Join(names, ",")
}

func TestStateDispatchFootprintsAreTheTransitionsOut(t *testing.T) {
	m := reductionModel(t, "por_two_machines", false)
	ci := startCheckedInvocation(t, m, nil, []string{"turner", "router", "loner"})
	turner := ci.moveOf(t, "state machine turner")
	router := ci.moveOf(t, "state machine router")
	loner := ci.moveOf(t, "state machine loner")
	for _, m := range []enabledMove{turner, router, loner} {
		if m.Kind != moveDispatch {
			t.Fatalf("%s is a %s, want a dispatch", m, m.Kind)
		}
	}
	writes := placeNames(ci.c.footprintOf(turner).Writes)
	if !strings.Contains(writes, "level") {
		t.Fatalf("turner's dispatch writes %q, want level", writes)
	}
	reads := placeNames(ci.c.footprintOf(router).Reads)
	if !strings.Contains(reads, "level") {
		t.Fatalf("router's dispatch reads %q, want level", reads)
	}
	if !ci.c.footprintOf(turner).Dependent(ci.c.footprintOf(router)) {
		t.Fatal("turner's effect and router's guard are independent")
	}
	for _, other := range []enabledMove{turner, router} {
		if ci.c.footprintOf(loner).Dependent(ci.c.footprintOf(other)) {
			t.Fatalf("loner depends on %s", other)
		}
		if ci.c.futureOf(loner).Dependent(ci.c.futureOf(other)) {
			t.Fatalf("loner's future depends on %s's", other)
		}
	}
	if !ci.c.futureOf(turner).Dependent(ci.c.futureOf(router)) {
		t.Fatal("turner's and router's futures are independent")
	}
}

func TestStateDoStepFootprintIsTheBehaviorPending(t *testing.T) {
	m := reductionModel(t, "por_state_do_write", false)
	ci := startCheckedInvocation(t, m, []string{"reader"}, []string{"counter"})
	entry := ci.moveOf(t, "state machine counter")
	if _, err := ci.run.makeMove(entry); err != nil {
		t.Fatalf("move %s: %v", entry, err)
	}
	step := ci.moveOf(t, "state machine counter")
	if step.Kind != moveDoStep {
		t.Fatalf("%s is a %s, want a do step", step, step.Kind)
	}
	fp := ci.c.footprintOf(step)
	if fp.Dynamic {
		t.Fatalf("the do step is dynamic: %s", fp)
	}
	if writes := placeNames(fp.Writes); !strings.Contains(writes, "count") {
		t.Fatalf("the do step writes %q, want count", writes)
	}
	if future := ci.c.futureOf(step); !strings.Contains(placeNames(future.Writes), "count") {
		t.Fatalf("the machine's future writes %q, want count", placeNames(future.Writes))
	}
}

func TestStateMoveUnitsAreTheirExecutor(t *testing.T) {
	m := reductionModel(t, "por_two_machines", false)
	ci := startCheckedInvocation(t, m, nil, []string{"turner", "router", "loner"})
	before := ci.moveOf(t, "state machine loner")
	if _, err := ci.run.makeMove(before); err != nil {
		t.Fatalf("move %s: %v", before, err)
	}
	after := ci.moveOf(t, "state machine loner")
	if before.unit() != after.unit() {
		t.Fatalf("loner's unit moved from %v to %v", before.unit(), after.unit())
	}
	if before.unit() == ci.moveOf(t, "state machine turner").unit() {
		t.Fatal("two machines share a unit")
	}
}

func TestFailingMoveFootprintIsDynamic(t *testing.T) {
	m := reductionModel(t, "por_two_machines", false)
	ci := startCheckedInvocation(t, m, nil, []string{"loner"})
	failing := ci.moveOf(t, "state machine loner")
	failing.Fails = errors.New("refused")
	if !ci.c.footprintOf(failing).Dynamic || !ci.c.futureOf(failing).Dynamic {
		t.Fatal("a failing move's footprint is not dynamic")
	}
}
