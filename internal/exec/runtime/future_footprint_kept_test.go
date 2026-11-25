package runtime

import (
	"reflect"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/ir/lower"
)

// gatedFutures reads every executor's future through a gate of its own, keeping none.
func gatedFutures(ctx *Context) map[string]lower.Footprint {
	fresh := newFutureFootprints(ctx)
	fresh.gate()
	fresh.forget()
	futures := make(map[string]lower.Footprint)
	for _, w := range ctx.clock.waiters {
		futures[w.dueLabel()] = fresh.executorFuture(w)
	}
	return futures
}

func TestGateKeepsEachExecutorsFutureUntilAskedAgain(t *testing.T) {
	m := reductionModel(t, "por_two_machines", false)
	ci := startCheckedInvocation(t, m, nil, []string{"turner", "router", "loner"})
	ci.moveOf(t, "state machine turner")
	defer ci.run.enter()()
	ctx := ci.c.ctx

	if futures := ctx.futureFootprints(); futures.executors != nil {
		t.Fatalf("futures hold %d executors before a gate, want none", len(futures.executors))
	}
	for _, w := range ctx.clock.waiters {
		ctx.futures.executorFuture(w)
	}
	if ctx.futures.executors != nil {
		t.Fatalf("an ungated read kept %d futures, want none", len(ctx.futures.executors))
	}

	if !ctx.gateFutures() {
		t.Fatal("the check policy gates no futures")
	}
	want := gatedFutures(ctx)
	if len(want) < 3 {
		t.Fatalf("%d executors on the clock, want the three machines", len(want))
	}
	for range 2 {
		for _, w := range ctx.clock.waiters {
			if got := ctx.futures.executorFuture(w); !reflect.DeepEqual(got, want[w.dueLabel()]) {
				t.Fatalf("%s: kept future %+v, want %+v", w.dueLabel(), got, want[w.dueLabel()])
			}
		}
	}
	if kept := len(ctx.futures.executors); kept != len(want) {
		t.Fatalf("gate kept %d futures, want one for each of %d executors", kept, len(want))
	}

	if ctx.futureFootprints().executors != nil {
		t.Fatal("asking for the footprints again kept the last gate's futures")
	}
}

func TestKeptFutureIsNotReadAfterAMove(t *testing.T) {
	m := reductionModel(t, "por_two_machines", false)
	ci := startCheckedInvocation(t, m, nil, []string{"turner", "router", "loner"})
	ctx := ci.c.ctx

	gate := func() map[string]lower.Footprint {
		defer ci.run.enter()()
		if !ctx.gateFutures() {
			t.Fatal("the check policy gates no futures")
		}
		got := make(map[string]lower.Footprint)
		for _, w := range ctx.clock.waiters {
			got[w.dueLabel()] = ctx.futures.executorFuture(w)
		}
		if want := gatedFutures(ctx); !reflect.DeepEqual(got, want) {
			t.Fatalf("gated futures %+v, want %+v", got, want)
		}
		return got
	}

	const turner = "state machine turner"
	move := ci.moveOf(t, turner)
	before := gate()
	if !containsPlace(before[turner].Writes, "level") {
		t.Fatalf("turner's future writes %s before its transition, want level", placeNames(before[turner].Writes))
	}
	if _, err := ci.run.makeMove(move); err != nil {
		t.Fatalf("move %s: %v", move, err)
	}
	if after := gate(); containsPlace(after[turner].Writes, "level") {
		t.Fatalf("turner's future still writes %s once turned", placeNames(after[turner].Writes))
	}
}

func containsPlace(places []lower.Place, name string) bool {
	for _, p := range places {
		if p.Name == name {
			return true
		}
	}
	return false
}

func TestAddFootprintIsTheUnionAndLeavesItsArgumentAlone(t *testing.T) {
	a := lower.Footprint{
		Reads:   []lower.Place{{Name: "a"}},
		Writes:  []lower.Place{{Name: "b"}},
		Sends:   []lower.Channel{{Port: "out"}},
		Flows:   []lower.Flow{{Reads: []lower.Place{{Name: "a"}}, Writes: []lower.Place{{Name: "b"}}}},
		Creates: true,
	}
	b := lower.Footprint{
		Reads:      make([]lower.Place, 1, 8),
		Accepts:    []lower.Channel{{Port: "in"}},
		Completion: true,
		Routes:     true,
	}
	b.Reads[0] = lower.Place{Name: "c"}
	c := lower.Footprint{Writes: []lower.Place{{Name: "d"}}, Dynamic: true}

	var sum lower.Footprint
	for _, fp := range []lower.Footprint{b, a, c} {
		addFootprint(&sum, fp)
	}
	if want := unionFootprints(unionFootprints(b, a), c); !reflect.DeepEqual(sum, want) {
		t.Fatalf("added %+v, want the union %+v", sum, want)
	}
	if spare := b.Reads[:2][1]; spare.Name != "" {
		t.Fatalf("adding wrote %q into the spare room of a footprint added from", spare.Name)
	}
}

func TestGateReadsNoFutureTheLastGateKept(t *testing.T) {
	m := reductionModel(t, "por_two_machines", false)
	ci := startCheckedInvocation(t, m, nil, []string{"turner", "router", "loner"})
	ci.moveOf(t, "state machine turner")
	defer ci.run.enter()()
	ctx := ci.c.ctx

	futures := ctx.futureFootprints()
	futures.gate()
	if len(futures.senders) != 0 || futures.anySender {
		t.Fatalf("machines sending nothing gate %d senders, any %t", len(futures.senders), futures.anySender)
	}
	for _, w := range ctx.clock.waiters {
		futures.executors[w] = lower.Footprint{Sends: []lower.Channel{{Port: "stale"}}}
	}
	futures.gate()
	if len(futures.senders) != 0 {
		t.Fatalf("a gate read %d sends out of the futures the last one kept", len(futures.senders))
	}
}

func TestSumFootprintsIsTheUnionInOrderWithNoSpareRoom(t *testing.T) {
	a := lower.Footprint{
		Reads:   []lower.Place{{Name: "a"}},
		Writes:  []lower.Place{{Name: "b"}},
		Sends:   []lower.Channel{{Port: "out"}},
		Flows:   []lower.Flow{{Reads: []lower.Place{{Name: "a"}}, Writes: []lower.Place{{Name: "b"}}}},
		Creates: true,
	}
	b := lower.Footprint{Reads: []lower.Place{{Name: "c"}}, Accepts: []lower.Channel{{Port: "in"}}, Routes: true}
	c := lower.Footprint{Writes: []lower.Place{{Name: "d"}}, Dynamic: true, Completion: true}

	parts := []lower.Footprint{b, {}, a, c}
	sum := sumFootprints(parts)
	if want := unionFootprints(unionFootprints(b, a), c); !reflect.DeepEqual(sum, want) {
		t.Fatalf("summed %+v, want the union %+v", sum, want)
	}
	if spare := cap(sum.Reads) - len(sum.Reads) + cap(sum.Writes) - len(sum.Writes); spare != 0 {
		t.Fatalf("the sum keeps room for %d more places, want none", spare)
	}
	if got := sumFootprints(nil); !reflect.DeepEqual(got, lower.Footprint{}) {
		t.Fatalf("the sum of no footprints is %+v, want the zero footprint", got)
	}
}

func TestMachineFutureIsTheUnionOfWhatItsMovesTouch(t *testing.T) {
	m := reductionModel(t, "por_two_machines", false)
	ci := startCheckedInvocation(t, m, nil, []string{"turner", "router", "loner"})
	ci.moveOf(t, "state machine turner")
	defer ci.run.enter()()
	ctx := ci.c.ctx

	for _, w := range ctx.clock.waiters {
		e, ok := w.(*StateExecutor)
		if !ok {
			continue
		}
		futures := newFutureFootprints(ctx)
		got := futures.machineFuture(e)
		parts, stepped := futures.machineParts(e, nil)
		if !stepped || len(parts) == 0 {
			t.Fatalf("%s: a started machine lists %d parts, stepped %t", w.dueLabel(), len(parts), stepped)
		}
		var want lower.Footprint
		for _, part := range parts {
			want = unionFootprints(want, part)
		}
		if want = withDrop(e, want); !reflect.DeepEqual(got, want) {
			t.Fatalf("%s: future %+v, want the union of its parts %+v", w.dueLabel(), got, want)
		}
		if again := futures.machineFuture(e); !reflect.DeepEqual(again, got) {
			t.Fatalf("%s: a second read %+v differs from the first %+v", w.dueLabel(), again, got)
		}
	}
}

func TestMachineFutureKeepsItsListOfPartsAndNothingInIt(t *testing.T) {
	m := reductionModel(t, "por_two_machines", false)
	ci := startCheckedInvocation(t, m, nil, []string{"turner", "router", "loner"})
	ci.moveOf(t, "state machine turner")
	defer ci.run.enter()()
	ctx := ci.c.ctx

	shared := newFutureFootprints(ctx)
	machines := 0
	for _, w := range ctx.clock.waiters {
		e, ok := w.(*StateExecutor)
		if !ok {
			continue
		}
		machines++
		got := shared.machineFuture(e)
		if want := newFutureFootprints(ctx).machineFuture(e); !reflect.DeepEqual(got, want) {
			t.Fatalf("%s: future %+v over a kept list, want %+v", w.dueLabel(), got, want)
		}
		if cap(shared.parts) == 0 {
			t.Fatalf("%s: the list of parts was not kept", w.dueLabel())
		}
		for i, part := range shared.parts[:cap(shared.parts)] {
			if !reflect.DeepEqual(part, lower.Footprint{}) {
				t.Fatalf("%s: the kept list still holds part %d: %+v", w.dueLabel(), i, part)
			}
		}
	}
	if machines < 2 {
		t.Fatalf("%d machines read, want the list kept from one to the next", machines)
	}
}
