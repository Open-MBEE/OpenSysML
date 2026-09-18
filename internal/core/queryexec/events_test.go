package queryexec

import (
	"errors"
	"strconv"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
)

const eventQueries = `
calc def Happenings :> Query {
	in root : Element;
	Project(
		source = Events(source = root),
		properties = ("kind", "time", "path", "machine", "state", "from", "to", "event", "payload")
	)
}
calc def Accepted :> Query {
	in root : Element;
	in since : ScalarValue[0..1] = null;
	in before : ScalarValue[0..1] = null;
	Project(
		source = Events(source = root, kind = "accept", since = since, before = before),
		properties = ("time", "event", "payload")
	)
}
calc def Accepts :> Query {
	Project(source = Events(kind = "accept"), properties = ("time", "path", "event"))
}
calc def Moves :> Query {
	Project(source = Events(kind = "accept, transition"), properties = ("kind", "path", "name"))
}
calc def Sent :> Query {
	Project(source = Events(kind = "send"), properties = ("time", "path", "machine", "target", "event"))
}
calc def SenderState :> Query {
	Project(source = Events(kind = "send"), properties = ("path", "brightness", "rounds"))
}
calc def Steps :> Query {
	Project(source = Events(kind = "do, guard"), properties = ("kind", "time", "path", "machine", "state", "name"))
}
calc def Drawn :> Query {
	Project(source = Events(kind = "choice"), properties = ("time", "path", "machine", "name", "alternatives", "taken", "text"))
}
calc def DrawnBy :> Query {
	in root : Element;
	Project(source = Events(source = root, kind = "choice, guard"), properties = ("kind", "path", "machine", "name"))
}
calc def Between :> Query {
	in since : ScalarValue;
	in before : ScalarValue;
	Project(source = Events(kind = "accept", since = since, before = before), properties = ("time", "path", "event"))
}
calc def Since :> Query {
	Project(source = Events(kind = "accept", since = 2 [s]), properties = ("time", "path", "event"))
}
calc def Latest :> Query {
	OrderBy(
		source = Project(
			source = WhereFeature(source = Events(), 'feature' = "kind", operator = "=", value = "transition"),
			properties = ("path"),
			columns = (Column(name = "move", expression = Event::'from' + " -> " + Event::'to'))
		),
		property = "time",
		direction = "descending",
		missing = "last",
		multiple = "error"
	)
}
calc def NamedToggle :> Query {
	Project(source = WhereName(source = Events(kind = "accept"), operator = "=", value = "Toggle"), properties = ("time", "path"))
}
calc def Machines :> Query {
	Project(source = Events(kind = "transition"), properties = ("path", "qualifiedName"))
}
calc def Backwards :> Query {
	Events(since = 2 [s], before = 1 [s])
}
calc def Empty :> Query {
	Events(since = 1 [s], before = 1 [s])
}
calc def Lengths :> Query {
	Events(since = 1 [m])
}
calc def Unkind :> Query {
	Events(kind = "accept, wish")
}
calc def OfMachine :> Query {
	Events(source = LampMachine)
}
calc def EventsOfEvents :> Query {
	Events(source = Events())
}
calc def EventStates :> Query {
	States(source = Events())
}
`

func eventFixture(t *testing.T) lampFixture {
	t.Helper()
	return loadLampFixture(t, eventQueries)
}

// quantity folds a quantity expression in the fixture's package for a binding.
func (f lampFixture) quantity(t *testing.T, expr string) Value {
	t.Helper()
	p := parser.New(source.New("<expr>", []byte(expr)))
	node := p.ParseExpression()
	if node == nil || len(p.Diagnostics) != 0 {
		t.Fatalf("parse %q: %v", expr, p.Diagnostics)
	}
	quantity, ok := f.model.EvalQuantity(f.symbol(t, "lamp1").Scope, node)
	if !ok {
		t.Fatalf("%q does not fold to a quantity", expr)
	}
	return QuantityValue(quantity)
}

// Events reads the typed trace in recorded order: an accept, then the exits,
// entries and steps its transition made, each with instant, object, machine and payload.
func TestExecuteEventsReadsTheTraceInOrder(t *testing.T) {
	fixture := eventFixture(t)
	got := rowTexts(t, fixture.rows(t, "Happenings", fixture.object(fixture.lamp2, "lamp2")))
	want := []string{
		"kind=entry time=0.0 [s] path=lamp2 machine=lp state=off from= to= event= payload=",
		"kind=accept time=2.0 [s] path=lamp2 machine=lp state= from= to= event=Toggle payload=",
		"kind=exit time=2.0 [s] path=lamp2 machine=lp state=off from= to= event= payload=",
		"kind=guard time=2.0 [s] path=lamp2 machine=lp state= from= to= event= payload=",
		"kind=entry time=2.0 [s] path=lamp2 machine=lp state=on from= to= event= payload=",
		"kind=choice time=2.0 [s] path=lamp2 machine=lp state= from= to= event= payload=",
		"kind=entry time=2.0 [s] path=lamp2 machine=lp state=run from= to= event= payload=",
		"kind=entry time=2.0 [s] path=lamp2 machine=lp state=slow from= to= event= payload=",
		"kind=transition time=2.0 [s] path=lamp2 machine=lp state= from=off to=on event=accept Toggle payload=",
		"kind=accept time=2.5 [s] path=lamp2 machine=lp state= from= to= event=Toggle payload=",
		"kind=choice time=2.5 [s] path=lamp2 machine=lp state= from= to= event= payload=",
		"kind=exit time=2.5 [s] path=lamp2 machine=lp state=run from= to= event= payload=",
		"kind=exit time=2.5 [s] path=lamp2 machine=lp state=slow from= to= event= payload=",
		"kind=exit time=2.5 [s] path=lamp2 machine=lp state=on from= to= event= payload=",
		"kind=entry time=2.5 [s] path=lamp2 machine=lp state=off from= to= event= payload=",
		"kind=transition time=2.5 [s] path=lamp2 machine=lp state= from=on to=off event=accept Toggle payload=",
	}
	if joinLines(got) != joinLines(want) {
		t.Fatalf("lamp2 events:\n%s\nwant:\n%s", joinLines(got), joinLines(want))
	}

	// The payload of an accept is carried as `name = value`, the do step of a
	// state and a transition's effect follow the accept that fired it.
	got = rowTexts(t, fixture.rows(t, "Accepted", fixture.object(fixture.lamp1, "lamp1")))
	want = []string{
		"time=0.0 [s] event=Toggle payload=",
		"time=1.0 [s] event=Dim payload=level = 3",
		"time=2.5 [s] event=Boost payload=",
	}
	if joinLines(got) != joinLines(want) {
		t.Fatalf("lamp1 accepts:\n%s\nwant:\n%s", joinLines(got), joinLines(want))
	}
	// A type selects the population; an object with no machine has no records.
	if got := rowTexts(t, fixture.rows(t, "Accepted", Bindings{"root": {ElementValue(fixture.symbol(t, "Lamp"))}})); len(got) != 5 {
		t.Fatalf("Lamp accepts = %d rows, want 5:\n%s", len(got), joinLines(got))
	}
	if got := rowTexts(t, fixture.rows(t, "Happenings", fixture.object(fixture.rock, "rock"))); len(got) != 0 {
		t.Fatalf("rock events:\n%s", joinLines(got))
	}
}

// kind keeps the kinds named, one or comma-separated; every recorded kind is
// reachable with its target, machine, alternatives and the one taken.
func TestExecuteEventsByKind(t *testing.T) {
	fixture := eventFixture(t)
	got := rowTexts(t, fixture.rows(t, "Accepts", nil))
	want := []string{
		"time=0.0 [s] path=lamp1 event=Toggle",
		"time=1.0 [s] path=lamp1 event=Dim",
		"time=2.0 [s] path=lamp2 event=Toggle",
		"time=2.5 [s] path=lamp2 event=Toggle",
		"time=2.5 [s] path=lamp1 event=Boost",
	}
	if joinLines(got) != joinLines(want) {
		t.Fatalf("accepts:\n%s\nwant:\n%s", joinLines(got), joinLines(want))
	}
	got = rowTexts(t, fixture.rows(t, "Moves", nil))
	want = []string{
		"kind=accept path=lamp1 name=Toggle",
		"kind=transition path=lamp1 name=on",
		"kind=accept path=lamp1 name=Dim",
		"kind=transition path=lamp1 name=dim",
		"kind=accept path=lamp2 name=Toggle",
		"kind=transition path=lamp2 name=on",
		"kind=accept path=lamp2 name=Toggle",
		"kind=transition path=lamp2 name=off",
		"kind=accept path=lamp1 name=Boost",
		"kind=transition path=lamp1 name=fast",
	}
	if joinLines(got) != joinLines(want) {
		t.Fatalf("accepts and transitions:\n%s\nwant:\n%s", joinLines(got), joinLines(want))
	}
	got = rowTexts(t, fixture.rows(t, "Sent", nil))
	want = []string{
		"time=0.0 [s] path=panel machine=marking target=object lamp1 event=Report",
		"time=0.0 [s] path= machine= target=object lamp1 event=Toggle",
		"time=1.0 [s] path= machine= target=object lamp1 event=Dim",
		"time=2.0 [s] path= machine= target=object lamp2 event=Toggle",
		"time=2.5 [s] path= machine= target=object lamp1 event=Boost",
		"time=2.5 [s] path= machine= target=object lamp2 event=Toggle",
		"time=2.5 [s] path=lamp1 machine=lp target=object panel event=Report",
	}
	if joinLines(got) != joinLines(want) {
		t.Fatalf("sends:\n%s\nwant:\n%s", joinLines(got), joinLines(want))
	}
	// A send row reads the sending behavior's declared attributes; a message
	// posted from outside the run has no behavior to read.
	got = rowTexts(t, fixture.rows(t, "SenderState", nil))
	want = []string{
		"path=panel brightness= rounds=2.0",
		"path= brightness= rounds=",
		"path= brightness= rounds=",
		"path= brightness= rounds=",
		"path= brightness= rounds=",
		"path= brightness= rounds=",
		"path=lamp1 brightness=0.0 rounds=",
	}
	if joinLines(got) != joinLines(want) {
		t.Fatalf("sender attributes:\n%s\nwant:\n%s", joinLines(got), joinLines(want))
	}
	got = rowTexts(t, fixture.rows(t, "Steps", nil))
	want = []string{
		"kind=guard time=0.0 [s] path=panel machine=marking state= name=2->split",
		"kind=guard time=0.0 [s] path=lamp1 machine=lp state= name=2->on",
		"kind=do time=1.0 [s] path=lamp1 machine=lp state=dim name=dim",
		"kind=guard time=2.0 [s] path=lamp2 machine=lp state= name=2->on",
	}
	if joinLines(got) != joinLines(want) {
		t.Fatalf("do steps and guards:\n%s\nwant:\n%s", joinLines(got), joinLines(want))
	}
	got = rowTexts(t, fixture.rows(t, "Drawn", nil))
	want = []string{
		"time=0.0 [s] path=panel machine=marking name=write order " +
			"alternatives=mark of object #6 := 1 by token 2+mark of object #6 := 2 by token 3 " +
			"taken=mark of object #6 := 1 by token 2 " +
			"text=choice step 4: writes mark of object #6 := 1 by token 2, mark of object #6 := 2 by token 3 (unordered; mark of object #6 := 1 by token 2 stood)",
		"time=0.0 [s] path=panel machine=marking name=token order " +
			"alternatives=2@low+3@high taken=3@high " +
			"text=choice step 4: tokens 2@low, 3@high (unordered; took 3@high first)",
		"time=0.0 [s] path=lamp1 machine=lp name=entry order " +
			"alternatives=run(entry)+slow(entry) taken=run(entry) " +
			"text=choice entering on: next run(entry), slow(entry) (unordered; took run(entry) first)",
		"time=2.0 [s] path=lamp2 machine=lp name=entry order " +
			"alternatives=run(entry)+slow(entry) taken=run(entry) " +
			"text=choice entering on: next run(entry), slow(entry) (unordered; took run(entry) first)",
		"time=2.5 [s] path= machine= name=due order " +
			"alternatives=state machine LampMachine of object #1+state machine LampMachine of object #3 " +
			"taken=state machine LampMachine of object #3 " +
			"text=choice at t=2.5: due state machine LampMachine of object #1, state machine LampMachine of object #3 (unordered; ran state machine LampMachine of object #3 first)",
		"time=2.5 [s] path=lamp2 machine=lp name=exit order " +
			"alternatives=run(exit)+slow(exit) taken=run(exit) " +
			"text=choice exiting on: next run(exit), slow(exit) (unordered; took run(exit) first)",
	}
	if joinLines(got) != joinLines(want) {
		t.Fatalf("choices:\n%s\nwant:\n%s", joinLines(got), joinLines(want))
	}
	// An action's choices and guards are the performer's: source reaches them.
	got = rowTexts(t, fixture.rows(t, "DrawnBy", fixture.object(fixture.panel, "panel")))
	want = []string{
		"kind=guard path=panel machine=marking name=2->split",
		"kind=choice path=panel machine=marking name=write order",
		"kind=choice path=panel machine=marking name=token order",
	}
	if joinLines(got) != joinLines(want) {
		t.Fatalf("panel choices and guards:\n%s\nwant:\n%s", joinLines(got), joinLines(want))
	}
}

// The interval is [since, before): kept at since, dropped at before; bounds are
// durations in any unit of time or bare clock seconds, either left open.
func TestExecuteEventsIntervalIsClosedOpen(t *testing.T) {
	fixture := eventFixture(t)
	lamp1 := fixture.object(fixture.lamp1, "lamp1")
	bounds := func(since, before Value) Bindings {
		return Bindings{"root": lamp1["root"], "since": {since}, "before": {before}}
	}
	second := func(seconds string) Value {
		return fixture.quantity(t, seconds+" [s]")
	}
	want := "time=1.0 [s] event=Dim payload=level = 3"
	for name, b := range map[string]Bindings{
		"seconds":      bounds(second("1"), second("2.5")),
		"bare numbers": bounds(IntegerValue(1), RealValue(2.5)),
	} {
		got := rowTexts(t, fixture.rows(t, "Accepted", b))
		if joinLines(got) != want {
			t.Fatalf("%s: accepts in [1 s, 2.5 s):\n%s", name, joinLines(got))
		}
	}
	got := rowTexts(t, fixture.rows(t, "Accepted", bounds(fixture.quantity(t, "0 [min]"), fixture.quantity(t, "1 [min]"))))
	if len(got) != 3 {
		t.Fatalf("accepts in [0 min, 1 min):\n%s", joinLines(got))
	}
	got = rowTexts(t, fixture.rows(t, "Between", Bindings{"since": {second("1")}, "before": {second("2.5")}}))
	if joinLines(got) != joinLines([]string{"time=1.0 [s] path=lamp1 event=Dim", "time=2.0 [s] path=lamp2 event=Toggle"}) {
		t.Fatalf("accepts of every object in [1 s, 2.5 s):\n%s", joinLines(got))
	}
	// Nudging before past 2.5 s admits both accepts at that instant.
	got = rowTexts(t, fixture.rows(t, "Between", Bindings{"since": {second("2.5")}, "before": {second("2.6")}}))
	if joinLines(got) != joinLines([]string{"time=2.5 [s] path=lamp2 event=Toggle", "time=2.5 [s] path=lamp1 event=Boost"}) {
		t.Fatalf("accepts in [2.5 s, 2.6 s):\n%s", joinLines(got))
	}
	got = rowTexts(t, fixture.rows(t, "Since", nil))
	if len(got) != 3 || got[0] != "time=2.0 [s] path=lamp2 event=Toggle" {
		t.Fatalf("accepts since 2 s:\n%s", joinLines(got))
	}
	got = rowTexts(t, fixture.rows(t, "Accepted", Bindings{"root": lamp1["root"], "before": {second("1")}}))
	if joinLines(got) != "time=0.0 [s] event=Toggle payload=" {
		t.Fatalf("lamp1 accepts before 1 s:\n%s", joinLines(got))
	}
}

// Event rows take the row operations object rows do: WhereFeature, WhereName,
// OrderBy over the instant, Column over the row's properties and machine metadata.
func TestExecuteEventRowsThroughRowOperations(t *testing.T) {
	fixture := eventFixture(t)
	got := rowTexts(t, fixture.rows(t, "Latest", nil))
	want := []string{
		"path=lamp2 move=on -> off",
		"path=lamp1 move=slow -> fast",
		"path=lamp2 move=off -> on",
		"path=lamp1 move=run -> dim",
		"path=lamp1 move=off -> on",
	}
	if joinLines(got) != joinLines(want) {
		t.Fatalf("transitions, latest first:\n%s\nwant:\n%s", joinLines(got), joinLines(want))
	}
	got = rowTexts(t, fixture.rows(t, "NamedToggle", nil))
	if joinLines(got) != joinLines([]string{"time=0.0 [s] path=lamp1", "time=2.0 [s] path=lamp2", "time=2.5 [s] path=lamp2"}) {
		t.Fatalf("accepts named Toggle:\n%s", joinLines(got))
	}
	got = rowTexts(t, fixture.rows(t, "Machines", nil))
	if len(got) != 5 || got[0] != "path=lamp1 qualifiedName=Observatory::LampMachine" {
		t.Fatalf("machines of transitions:\n%s", joinLines(got))
	}
}

// Every unsupported path is a typed error: no session, no trace, an empty or backwards
// interval, a bound not measuring time, an unknown kind, no object held, a row misread.
func TestExecuteEventsRefusals(t *testing.T) {
	fixture := eventFixture(t)

	_, err := fixture.run(t, fixture.modelOnly(), "Accepts", nil)
	executionError(t, err, ErrorNoRuntime)

	silent := runtime.NewContext(runtime.NewModel(fixture.model, fixture.resolver), runtime.DefaultMaxSteps)
	_, err = fixture.run(t, Context{Index: fixture.index, Resolver: fixture.resolver, Model: fixture.model, Runtime: silent}, "Accepts", nil)
	if got := executionError(t, err, ErrorNoTrace); !strings.Contains(got.Error(), "records none") {
		t.Fatalf("no-trace error = %v", got)
	}

	_, err = fixture.run(t, fixture.session(), "Backwards", nil)
	if got := executionError(t, err, ErrorInvalidInterval); got.Actual != "before = 1 is not after since = 2" {
		t.Fatalf("backwards error = %v", got)
	}
	_, err = fixture.run(t, fixture.session(), "Empty", nil)
	executionError(t, err, ErrorInvalidInterval)

	_, err = fixture.run(t, fixture.session(), "Lengths", nil)
	got := executionError(t, err, ErrorInvalidInterval)
	if got.Parameter != "since" || !errors.Is(got, runtime.ErrIncommensurableUnits) || !strings.Contains(got.Error(), "m does not measure a duration") {
		t.Fatalf("length error = %v", got)
	}

	_, err = fixture.run(t, fixture.session(), "Unkind", nil)
	if got := executionError(t, err, ErrorInvalidArgument); got.Parameter != "kind" || got.Actual != "accept, wish" {
		t.Fatalf("kind error = %v", got)
	}

	_, err = fixture.run(t, fixture.session(), "OfMachine", nil)
	if got := executionError(t, err, ErrorNotHeld); got.Target != "Observatory::LampMachine" {
		t.Fatalf("not-held error = %v", got)
	}
	_, err = fixture.run(t, fixture.session(), "EventsOfEvents", nil)
	if got := executionError(t, err, ErrorEventRow); got.Target != "lamp1.lp: enter: off" {
		t.Fatalf("event-row error = %v", got)
	}
	_, err = fixture.run(t, fixture.session(), "EventStates", nil)
	executionError(t, err, ErrorEventRow)
}

// A recorder bounded to the most recent records answers an interval that starts after
// the last record it dropped, and refuses one reaching what it no longer holds.
func TestExecuteEventsOverATruncatedTrace(t *testing.T) {
	fixture := loadLampFixtureTracing(t, eventQueries, runtime.NewEventRecorder(4))
	second := func(seconds string) Value { return fixture.quantity(t, seconds+" [s]") }
	dropped, upTo := fixture.trace.Dropped()
	if dropped == 0 || upTo != 2.5 {
		t.Fatalf("dropped %d records up to t = %v, want the run before 2.5 s dropped", dropped, upTo)
	}

	got := rowTexts(t, fixture.rows(t, "Between", Bindings{"since": {second("2.6")}, "before": {second("3.5")}}))
	if len(got) != 0 {
		t.Fatalf("accepts in [2.6, 3.5):\n%s", joinLines(got))
	}

	for name, bindings := range map[string]Bindings{
		"Accepts": nil,
		"Between": {"since": {second("2.5")}, "before": {second("2.6")}},
		"Since":   nil,
	} {
		_, err := fixture.run(t, fixture.session(), name, bindings)
		if got := executionError(t, err, ErrorTraceTruncated); got.Actual != strconv.Itoa(dropped)+" records up to t = 2.5 dropped" {
			t.Fatalf("%s over a truncated trace = %v", name, got)
		}
	}
}
