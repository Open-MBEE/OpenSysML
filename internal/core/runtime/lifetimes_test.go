package runtime

import (
	"errors"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
	"strings"
	"testing"
)

// lifetimeFixture instantiates test::<name> of src and answers the object and a
// way to invoke test::<calc> over values, for the lifetime tests below.
func lifetimeFixture(t *testing.T, src string) (instantiate func(name string) *Instance, invoke func(calc string, args ...Value) (Value, error), ctx *Context) {
	t.Helper()
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
	instantiate = func(name string) *Instance {
		matches := idx.LookupQualified("test::" + name)
		if len(matches) != 1 {
			t.Fatalf("test::%s: %d matching symbols, want 1", name, len(matches))
		}
		inst, err := ctx.Instantiate(matches[0])
		if err != nil {
			t.Fatalf("Instantiate(%s): %v", name, err)
		}
		return inst
	}
	invoke = func(calc string, args ...Value) (Value, error) {
		sym, scope := calcByName(t, idx.DocumentRoot("<test>"), "test", calc)
		return ctx.InvokeCalc(sym, args, scope)
	}
	return instantiate, invoke, ctx
}

func objectValue(inst *Instance) Value { return Value{Kind: ValInstance, Instance: inst.ID} }

const lifetimeModel = `
	package test {
		private import ScalarValues::*;
		private import OccurrenceFunctions::*;
		part def Widget { attribute n : Integer = 1; }
		part def Bench { part w : Widget; part spare : Widget; }
		part def Rover {
			exhibit state modes { entry; then idle; state idle; }
		}
		calc def Destroy { in b : Bench[0..1]; return : Bench[0..1] = destroy(b); }
		calc def DestroyRover { in r : Rover; return : Rover = destroy(r); }
		calc def During { in o : Bench; return : Boolean = isDuring(o); }
		calc def WidgetDuring { in w : Widget; return : Boolean = isDuring(w); }
		calc def Create { in b : Bench; return : Bench = create(b); }
		calc def CreateSpare { in b : Bench; return : Widget = create(b.spare); }
		calc def AddAt { in g : Widget[0..*] ordered nonunique; in w : Widget; in i : Positive; return : Widget[0..*] = addNewAt(g, w, i); }
		calc def AddSpare { in b : Bench; in g : Widget[0..*] ordered nonunique; return : Widget[0..*] = addNew(g, b.spare); }
	}
`

// TestOccurrenceLifeBeginsWithWhole: an instantiated object is alive from its
// materialization, and a part of it began when its whole did.
func TestOccurrenceLifeBeginsWithWhole(t *testing.T) {
	instantiate, invoke, ctx := lifetimeFixture(t, lifetimeModel)
	bench := instantiate("Bench")
	whole, ok := ctx.OccurrenceLife(bench.ID)
	if !ok || !whole.Alive() {
		t.Fatalf("OccurrenceLife(bench) = %v, %v; want alive", whole, ok)
	}
	fv, err := bench.GetFeatureValue(ctx, "w")
	if err != nil {
		t.Fatal(err)
	}
	id, _ := fv.HeldValue().Object()
	part, ok := ctx.OccurrenceLife(id)
	if !ok || part.Began != whole.Began || part.Ended != 0 {
		t.Errorf("OccurrenceLife(w) = %v, %v; want began at %d, alive", part, ok, whole.Began)
	}
	if got, err := invoke("During", objectValue(bench)); err != nil || !got.Const.Bool {
		t.Errorf("isDuring(bench) = %s, %v; want true", FormatValue(got), err)
	}
	if _, ok := ctx.OccurrenceLife(bench.ID + 100); ok {
		t.Error("OccurrenceLife of an unregistered identity answered one")
	}
}

// TestBindingRefusesADestroyedEnd: a binding neither reads a destroyed object's
// feature into a live one nor writes a live value into it.
func TestBindingRefusesADestroyedEnd(t *testing.T) {
	instantiate, invoke, ctx := lifetimeFixture(t, `
		package test {
			private import ScalarValues::*;
			private import OccurrenceFunctions::*;
			part def Widget { attribute n : Integer = 1; attribute m : Integer; }
			part def Rig {
				part w : Widget;
				attribute shown : Integer;
				bind shown = w.n;
				attribute knob : Integer = 9;
				bind w.m = knob;
			}
			calc def DestroyWidget { in w : Widget; return : Widget = destroy(w); }
		}
	`)
	rig := instantiate("Rig")
	if fv, err := rig.GetFeatureValue(ctx, "shown"); err != nil || !valueIdentical(fv.HeldValue(), constInt(1)) {
		t.Fatalf("shown before destroy = %v, %v; want 1", fv, err)
	}
	fv, err := rig.GetFeatureValue(ctx, "w")
	if err != nil {
		t.Fatal(err)
	}
	id, _ := fv.HeldValue().Object()
	w, _ := ctx.getInstance(id)
	if _, err := invoke("DestroyWidget", objectValue(w)); err != nil {
		t.Fatalf("destroy(w): %v", err)
	}

	if _, err := rig.GetFeatureValue(ctx, "shown"); !errors.Is(err, ErrOccurrenceDestroyed) {
		t.Errorf("shown bound to a destroyed end: %v; want %v", err, ErrOccurrenceDestroyed)
	}
	if _, err := rig.GetFeatureValue(ctx, "knob"); !errors.Is(err, ErrOccurrenceDestroyed) {
		t.Errorf("knob bound into a destroyed end: %v; want %v", err, ErrOccurrenceDestroyed)
	}
	if m := w.FeatureValues["m"]; m.Materialized || m.HeldValue().Kind != ValInvalid {
		t.Errorf("destroyed w.m = %s; want it left unwritten", FormatValue(m.HeldValue()))
	}
}

// TestDestroyedObjectPerformsNothing: a destroyed object performs no behavior of
// its type, not even one that touches none of its features, and sends nothing.
func TestDestroyedObjectPerformsNothing(t *testing.T) {
	instantiate, invoke, ctx := lifetimeFixture(t, `
		package test {
			private import ScalarValues::*;
			private import OccurrenceFunctions::*;
			item def Ping;
			part def Beacon {
				action ping { first start; action fire { send Ping to tower; } succession first start then fire; }
				calc def Seven { return : Integer = 7; }
				constraint def Sure { true }
				state def Blink { entry; then on; state on; }
			}
			calc def DestroyBeacon { in b : Beacon; return : Beacon = destroy(b); }
		}
	`)
	beacon := instantiate("Beacon")
	if _, err := ctx.InvokeOperation(beacon, "ping", nil); err != nil {
		t.Fatalf("ping before destroy: %v", err)
	}
	if sent := len(ctx.PendingMessages()); sent != 1 {
		t.Fatalf("messages before destroy = %d, want 1", sent)
	}
	if _, err := invoke("DestroyBeacon", objectValue(beacon)); err != nil {
		t.Fatalf("destroy(beacon): %v", err)
	}
	for _, op := range []string{"ping", "Seven", "Sure", "missing"} {
		if _, err := ctx.InvokeOperation(beacon, op, nil); !errors.Is(err, ErrOccurrenceDestroyed) {
			t.Errorf("invoke %s on a destroyed object = %v; want %v", op, err, ErrOccurrenceDestroyed)
		}
	}
	if sent := len(ctx.PendingMessages()); sent != 1 {
		t.Errorf("messages after destroy = %d, want the 1 sent before it", sent)
	}
	members := map[string]*symbols.Symbol{}
	for _, member := range ctx.model.semantics.MembersOf(beacon.Type) {
		members[member.Name] = member
	}
	if _, err := ctx.ExecuteActionPerformedBy(members["ping"], beacon, nil); !errors.Is(err, ErrOccurrenceDestroyed) {
		t.Errorf("ExecuteActionPerformedBy a destroyed object = %v; want %v", err, ErrOccurrenceDestroyed)
	}
	if _, _, err := ctx.ExecuteStatePerformedBy(members["Blink"], beacon, nil); !errors.Is(err, ErrOccurrenceDestroyed) {
		t.Errorf("ExecuteStatePerformedBy a destroyed object = %v; want %v", err, ErrOccurrenceDestroyed)
	}
	if _, err := ctx.CreateActionExecutorFor(members["ping"], beacon); !errors.Is(err, ErrOccurrenceDestroyed) {
		t.Errorf("CreateActionExecutorFor a destroyed object = %v; want %v", err, ErrOccurrenceDestroyed)
	}
	if _, err := ctx.CreateStateExecutorFor(members["Blink"], beacon); !errors.Is(err, ErrOccurrenceDestroyed) {
		t.Errorf("CreateStateExecutorFor a destroyed object = %v; want %v", err, ErrOccurrenceDestroyed)
	}
}

// TestDestroyEndsPortionsAndRefusesReads: `destroy` ends the object and the parts
// it holds, after which no feature of theirs is read or written, and none is
// destroyed again.
func TestDestroyEndsPortionsAndRefusesReads(t *testing.T) {
	instantiate, invoke, ctx := lifetimeFixture(t, lifetimeModel)
	bench := instantiate("Bench")
	fv, err := bench.GetFeatureValue(ctx, "w")
	if err != nil {
		t.Fatal(err)
	}
	partID, _ := fv.HeldValue().Object()
	part, _ := ctx.getInstance(partID)

	got, err := invoke("Destroy", objectValue(bench))
	if err != nil || !valueIdentical(got, objectValue(bench)) {
		t.Fatalf("destroy(bench) = %s, %v; want bench", FormatValue(got), err)
	}
	whole, _ := ctx.OccurrenceLife(bench.ID)
	for _, inst := range []*Instance{bench, part} {
		l, ok := ctx.OccurrenceLife(inst.ID)
		if !ok || !l.Destroyed || l.Alive() {
			t.Errorf("OccurrenceLife(#%d) = %v; want destroyed", inst.ID, l)
		}
		if l.Ended != whole.Ended {
			t.Errorf("OccurrenceLife(#%d) ended at %d; want %d, with its whole", inst.ID, l.Ended, whole.Ended)
		}
	}
	if _, err := bench.GetFeatureValue(ctx, "w"); !errors.Is(err, ErrOccurrenceDestroyed) {
		t.Errorf("read of a destroyed object: %v, want %v", err, ErrOccurrenceDestroyed)
	}
	if _, err := part.GetFeatureValue(ctx, "n"); !errors.Is(err, ErrOccurrenceDestroyed) {
		t.Errorf("read of a destroyed portion: %v, want %v", err, ErrOccurrenceDestroyed)
	}
	if err := part.SetFeatureValue(ctx, "n", constInt(2)); !errors.Is(err, ErrOccurrenceDestroyed) {
		t.Errorf("write to a destroyed portion: %v, want %v", err, ErrOccurrenceDestroyed)
	}
	if _, err := part.GetFeatureValue(ctx, "nowhere"); !errors.Is(err, ErrOccurrenceDestroyed) {
		t.Errorf("read of no feature of a destroyed object: %v, want %v", err, ErrOccurrenceDestroyed)
	}
	if err := part.SetFeatureValue(ctx, "nowhere", constInt(2)); !errors.Is(err, ErrOccurrenceDestroyed) {
		t.Errorf("write to no feature of a destroyed object: %v, want %v", err, ErrOccurrenceDestroyed)
	}
	if got, err := invoke("WidgetDuring", objectValue(part)); err != nil || got.Const.Bool {
		t.Errorf("isDuring(destroyed part) = %s, %v; want false", FormatValue(got), err)
	}
	if _, err := invoke("Destroy", objectValue(bench)); !errors.Is(err, ErrOccurrenceDestroyed) {
		t.Errorf("second destroy: %v, want %v", err, ErrOccurrenceDestroyed)
	}
	if got, err := invoke("Destroy", nullValue()); err != nil || !isEmptyValue(got) {
		t.Errorf("destroy() = %s, %v; want nothing", FormatValue(got), err)
	}
}

// TestDestroyRefusedWhilePerforming: an object whose exhibited state machine has
// not completed cannot end; the refusal names the behavior under way.
func TestDestroyRefusedWhilePerforming(t *testing.T) {
	instantiate, invoke, ctx := lifetimeFixture(t, lifetimeModel)
	rover := instantiate("Rover")
	_, err := invoke("DestroyRover", objectValue(rover))
	if !errors.Is(err, ErrOccurrenceLifetime) || !strings.Contains(err.Error(), "under way") {
		t.Fatalf("destroy(rover) = %v; want %v naming the behavior under way", err, ErrOccurrenceLifetime)
	}
	if l, _ := ctx.OccurrenceLife(rover.ID); !l.Alive() {
		t.Errorf("OccurrenceLife(rover) = %v after the refusal; want alive", l)
	}
}

const noFlowModel = `
	package test {
		private import ScalarValues::*;
		private import OccurrenceFunctions::*;
		action def Report;
		part def Camera { perform action report : Report; }
		calc def ReportDuring { in c : Camera; return : Boolean = isDuring(c.report); }
		calc def DestroyCamera { in c : Camera; return : Camera = destroy(c); }
	}
`

// TestPerformedActionWithoutAFlowCompletes: an action stating no flow is
// performed at once, so nothing happens during it and its performer can end.
func TestPerformedActionWithoutAFlowCompletes(t *testing.T) {
	instantiate, invoke, ctx := lifetimeFixture(t, noFlowModel)
	camera := instantiate("Camera")
	behavior, ok := camera.Behavior("report")
	if !ok || behavior.Action == nil || behavior.Action.State() != StateCompleted {
		t.Fatalf("report = %v, %v; want a completed action", behavior, ok)
	}
	report := behavior.Action.occurrence
	if report == nil {
		t.Fatal("the performed action stands for no occurrence")
	}
	if l, ok := ctx.OccurrenceLife(report.ID); !ok || l.Alive() || l.Destroyed || l.Began > l.Ended {
		t.Errorf("OccurrenceLife(report) = %v, %v; want ended after it began, not destroyed", l, ok)
	}
	if got, err := invoke("ReportDuring", objectValue(camera)); err != nil || got.Const.Bool {
		t.Errorf("isDuring(camera.report) = %s, %v; want false", FormatValue(got), err)
	}
	if _, err := invoke("DestroyCamera", objectValue(camera)); err != nil {
		t.Errorf("destroy(camera) = %v; want the performer to end", err)
	}
	if l, _ := ctx.OccurrenceLife(camera.ID); !l.Destroyed {
		t.Errorf("OccurrenceLife(camera) = %v; want destroyed", l)
	}
}

const noFlowInputModel = `
	package test {
		private import ScalarValues::*;
		action def Report { in n : Integer; attribute twice : Integer = n * 2; }
		part def Camera { attribute level : Integer = 21; perform action report : Report { in n = level; } }
		calc def Twice { in c : Camera; return : Integer = c.report.twice; }
	}
`

// TestPerformedActionWithoutAFlowTakesItsInputs: an action stating no flow
// completes holding the inputs bound to it and the attributes read from them.
func TestPerformedActionWithoutAFlowTakesItsInputs(t *testing.T) {
	instantiate, invoke, ctx := lifetimeFixture(t, noFlowInputModel)
	camera := instantiate("Camera")
	behavior, ok := camera.Behavior("report")
	if !ok || behavior.Action == nil || behavior.Action.State() != StateCompleted {
		t.Fatalf("report = %v, %v; want a completed action", behavior, ok)
	}
	fv, err := behavior.Action.occurrence.GetFeatureValue(ctx, "n")
	if err != nil || !valueIdentical(fv.HeldValue(), constInt(21)) {
		t.Errorf("report.n = %s, %v; want 21", FormatValue(fv.HeldValue()), err)
	}
	if got, err := invoke("Twice", objectValue(camera)); err != nil || !valueIdentical(got, constInt(42)) {
		t.Errorf("camera.report.twice = %s, %v; want 42", FormatValue(got), err)
	}
}

const createBehavingModel = `
	package test {
		private import ScalarValues::*;
		private import OccurrenceFunctions::*;
		action def Tick;
		part def Sensor { attribute n : Integer = 1; }
		part def Probe {
			part sensor : Sensor;
			perform action tick : Tick;
			exhibit state modes { entry; then on; state on; }
		}
		part probe : Probe;
		calc def CreateProbe { return : Probe = create(probe); }
	}
`

// TestCreateBeginsBeforeWhatItReachesWith: a usage `create` first reaches begins
// where the call reached it, ahead of the part reached with it and of the action
// and state it performs, none of which predates its performer.
func TestCreateBeginsBeforeWhatItReachesWith(t *testing.T) {
	_, invoke, ctx := lifetimeFixture(t, createBehavingModel)
	got, err := invoke("CreateProbe")
	if err != nil {
		t.Fatalf("create(probe): %v", err)
	}
	probeID, _ := got.Object()
	probe, _ := ctx.getInstance(probeID)
	life, _ := ctx.OccurrenceLife(probeID)
	if !life.Alive() {
		t.Fatalf("OccurrenceLife(probe) = %v; want alive", life)
	}
	fv, err := probe.GetFeatureValue(ctx, "sensor")
	if err != nil {
		t.Fatal(err)
	}
	sensorID, _ := fv.HeldValue().Object()
	if l, _ := ctx.OccurrenceLife(sensorID); l.Began != life.Began {
		t.Errorf("OccurrenceLife(sensor) = %v; want begun with the probe at %d", l, life.Began)
	}
	for _, name := range []string{"tick", "modes"} {
		behavior, ok := probe.Behavior(name)
		if !ok {
			t.Fatalf("probe performs no %s", name)
		}
		var occurrence *Instance
		switch {
		case behavior.Action != nil:
			occurrence = behavior.Action.occurrence
		case behavior.State != nil:
			occurrence = behavior.State.occurrence
		}
		if occurrence == nil {
			t.Fatalf("%s stands for no occurrence", name)
		}
		if l, _ := ctx.OccurrenceLife(occurrence.ID); l.Began < life.Began {
			t.Errorf("OccurrenceLife(%s) = %v; want begun no earlier than its performer at %d", name, l, life.Began)
		}
	}
}

const aliasedPartModel = `
	package test {
		private import ScalarValues::*;
		private import OccurrenceFunctions::*;
		part def Widget { attribute n : Integer = 1; }
		part def Base { part p : Widget; }
		part def Derived :> Base { part q :>> p; }
		calc def DestroyDerived { in d : Derived; return : Derived = destroy(d); }
	}
`

// TestDestroyEndsAnAliasedPartOnce: a part two names of one redefined feature
// hold is one portion, ended once with one trace event.
func TestDestroyEndsAnAliasedPartOnce(t *testing.T) {
	instantiate, invoke, ctx := lifetimeFixture(t, aliasedPartModel)
	trace := NewTraceRecorder()
	ctx.SetTrace(trace)
	derived := instantiate("Derived")
	p, err := derived.GetFeatureValue(ctx, "p")
	if err != nil {
		t.Fatal(err)
	}
	q, err := derived.GetFeatureValue(ctx, "q")
	if err != nil {
		t.Fatal(err)
	}
	if p != q {
		t.Fatalf("p and q are different feature values; the fixture aliases nothing")
	}
	if portions := ctx.portionsOf(derived); len(portions) != 2 {
		t.Errorf("portionsOf(derived) lists %d, want the object and its one part", len(portions))
	}
	if _, err := invoke("DestroyDerived", objectValue(derived)); err != nil {
		t.Fatalf("destroy(derived): %v", err)
	}
	var destroyed []string
	for _, entry := range trace.Entries() {
		if strings.HasPrefix(entry, "destroy: ") {
			destroyed = append(destroyed, entry)
		}
	}
	if len(destroyed) != 2 {
		t.Errorf("trace records %d destructions %q; want one for the object and one for its part", len(destroyed), destroyed)
	}
}

// TestCreateOnlyWhatTheCallReaches: `create` begins an object the call first
// reaches and refuses one that began before it; `addNewAt` refuses an index past
// one beyond the group's end before it creates anything.
func TestCreateOnlyWhatTheCallReaches(t *testing.T) {
	instantiate, invoke, ctx := lifetimeFixture(t, lifetimeModel)
	bench := instantiate("Bench")
	if _, err := invoke("Create", objectValue(bench)); !errors.Is(err, ErrOccurrenceLifetime) {
		t.Errorf("create(bench) = %v; want %v", err, ErrOccurrenceLifetime)
	}
	whole, _ := ctx.OccurrenceLife(bench.ID)
	got, err := invoke("CreateSpare", objectValue(bench))
	if err != nil {
		t.Fatalf("create(bench.spare): %v", err)
	}
	spareID, _ := got.Object()
	spare, _ := ctx.OccurrenceLife(spareID)
	if !spare.Alive() || spare.Began <= whole.Began {
		t.Errorf("OccurrenceLife(spare) = %v; want begun after the bench at %d", spare, whole.Began)
	}
	if _, err := invoke("CreateSpare", objectValue(bench)); !errors.Is(err, ErrOccurrenceLifetime) {
		t.Errorf("second create(bench.spare) = %v; want %v", err, ErrOccurrenceLifetime)
	}

	widget := instantiate("Widget")
	before, _ := ctx.OccurrenceLife(widget.ID)
	_, err = invoke("AddAt", nullValue(), objectValue(widget), constInt(2))
	if !errors.Is(err, ErrIndexOutOfRange) {
		t.Errorf("addNewAt((), w, 2) = %v; want %v", err, ErrIndexOutOfRange)
	}
	if after, _ := ctx.OccurrenceLife(widget.ID); after != before {
		t.Errorf("OccurrenceLife(w) = %v after the refusal; want %v unchanged", after, before)
	}
}

// TestAddNewCreatesNothingWhenTheGroupCannotGrow: an `addNew` the element budget
// refuses leaves the occurrence it would have created as it was.
func TestAddNewCreatesNothingWhenTheGroupCannotGrow(t *testing.T) {
	instantiate, invoke, ctx := lifetimeFixture(t, lifetimeModel)
	spareOf := func(bench *Instance) OccurrenceLife {
		fv, err := bench.GetFeatureValue(ctx, "spare")
		if err != nil {
			t.Fatal(err)
		}
		id, _ := fv.HeldValue().Object()
		l, _ := ctx.OccurrenceLife(id)
		return l
	}
	// The call first reaches the spare, so within budget it creates it.
	bench := instantiate("Bench")
	whole, _ := ctx.OccurrenceLife(bench.ID)
	if _, err := invoke("AddSpare", objectValue(bench), nullValue()); err != nil {
		t.Fatalf("addNew((), spare) within budget: %v", err)
	}
	if l := spareOf(bench); !l.Alive() || l.Began <= whole.Began {
		t.Errorf("OccurrenceLife(spare) = %v; want begun after the bench at %d", l, whole.Began)
	}

	// A group already holding one cannot grow to two under a budget of one.
	held := objectValue(instantiate("Widget"))
	tight := DefaultBudgets()
	tight.MaxElements = 1
	if err := ctx.SetBudgets(tight); err != nil {
		t.Fatal(err)
	}
	other := instantiate("Bench")
	whole, _ = ctx.OccurrenceLife(other.ID)
	if _, err := invoke("AddSpare", objectValue(other), held); !errors.Is(err, ErrElementLimitExceeded) {
		t.Fatalf("addNew((w), spare) over budget = %v; want %v", err, ErrElementLimitExceeded)
	}
	if l := spareOf(other); !l.Alive() || l.Began != whole.Began {
		t.Errorf("OccurrenceLife(spare) = %v after the refusal; want begun with the bench at %d", l, whole.Began)
	}
}

// TestLifetimeChangesRollBackWithTheJournal: a destruction inside a journal that
// rolls back leaves the object alive, as the feature values it restores are.
func TestLifetimeChangesRollBackWithTheJournal(t *testing.T) {
	instantiate, invoke, ctx := lifetimeFixture(t, lifetimeModel)
	bench := instantiate("Bench")
	before, _ := ctx.OccurrenceLife(bench.ID)
	_, rollback := ctx.beginJournal()
	if _, err := invoke("Destroy", objectValue(bench)); err != nil {
		t.Fatal(err)
	}
	if l, _ := ctx.OccurrenceLife(bench.ID); !l.Destroyed {
		t.Fatalf("OccurrenceLife(bench) = %v inside the journal; want destroyed", l)
	}
	rollback()
	if after, _ := ctx.OccurrenceLife(bench.ID); after != before {
		t.Errorf("OccurrenceLife(bench) = %v after rollback; want %v", after, before)
	}
	if _, err := bench.GetFeatureValue(ctx, "w"); err != nil {
		t.Errorf("read after rollback: %v", err)
	}
}

// TestAbandonedObjectsLoseTheirLives: an object a failed creation abandons
// leaves no lifetime behind.
func TestAbandonedObjectsLoseTheirLives(t *testing.T) {
	instantiate, _, ctx := lifetimeFixture(t, lifetimeModel)
	mark := len(ctx.created)
	widget := instantiate("Widget")
	ctx.abandonInstancesSince(mark)
	if _, ok := ctx.OccurrenceLife(widget.ID); ok {
		t.Error("OccurrenceLife of an abandoned object answered one")
	}
}
