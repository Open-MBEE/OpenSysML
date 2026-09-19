package queryexec

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// lampBody is a session's worth of state machines: two lamps whose machine has
// an `on` state of two orthogonal regions, a panel whose action draws a write
// order and sends, and a rock exhibiting none.
const lampBody = `
private import SI::*;
attribute def Toggle;
attribute def Boost;
attribute def Dim { attribute level : Integer; }
attribute def Report;
state def LampMachine {
	attribute brightness : Integer = 0;
	entry; then off;
	state off;
	choice pick;
	transition off_on first off accept Toggle then pick;
	transition first pick if brightness >= 0 then on;
	transition first pick if 1 / brightness > 0 then on;
	state on parallel {
		state light {
			entry; then run;
			state run;
			transition run_dim first run accept d : Dim do assign brightness := d.level then dim;
			transition run_boost first run accept Boost then dim;
			state dim {
				do action { assign brightness := brightness + 1; }
			}
		}
		state fan {
			entry; then slow;
			state slow;
			transition slow_fast first slow accept Boost then fast;
			state fast {
				entry send new Report() to panel;
			}
		}
	}
	transition on_off first on accept Toggle then off;
}
part def Lamp { exhibit state lp : LampMachine; }
part def Panel {
	attribute mark : Integer = 0;
	perform action marking {
		attribute rounds : Integer = 2;
		first start;
		then decide route;
			if mark == 0 then split;
			if 1 / mark > 0 then split;
		fork split;
		action low { assign mark := 1; }
		action high { assign mark := 2; }
		join sync;
		action report send new Report() to lamp1;
		done;
		succession first split then low;
		succession first split then high;
		succession first low then sync;
		succession first high then sync;
		succession first sync then report;
		succession first report then done;
	}
}
part lamp1 : Lamp;
part lamp2 : Lamp;
part panel : Panel;
part def Rock;
part rock : Rock;
`

const lampQueries = `
calc def CurrentStates :> Query {
	in root : Element;
	Project(source = States(source = root), properties = ("path", "machine", "name", "statePath", "region", "enclosing"))
}
calc def StateElements :> Query {
	in root : Element;
	Project(source = States(source = root), properties = ("object", "state"))
}
calc def LeavesNamed :> Query {
	in root : Element;
	in leaf : String;
	Project(
		source = WhereFeature(source = States(source = root), 'feature' = "name", operator = "=", value = leaf),
		properties = ("path", "statePath")
	)
}
calc def LeavesUnder :> Query {
	in root : Element;
	Project(
		source = WhereFeature(source = States(source = root), 'feature' = "region", operator = "=", value = "light"),
		properties = ("path", "name")
	)
}
calc def StatesByName :> Query {
	in root : Element;
	OrderBy(
		source = Project(source = States(source = root), properties = ("path", "name")),
		property = "name",
		direction = "ascending",
		missing = "last",
		multiple = "error"
	)
}
calc def StatesOfMachine :> Query {
	in root : Element;
	WhereName(source = States(source = root), operator = "=", value = "run")
}
calc def StateColumns :> Query {
	in root : Element;
	Project(
		source = States(source = root),
		properties = ("path"),
		columns = (
			Column(name = "label", expression = "lamp " + State::path),
			Column(name = "where", expression = State::machine + ":" + State::statePath)
		)
	)
}
calc def Running :> Query {
	Project(source = InState(name = "run"), properties = ("qualifiedName"))
}
calc def Lit :> Query {
	Project(source = InState(name = "on"), properties = ("qualifiedName"))
}
calc def Fanned :> Query {
	Project(source = InState(name = "on.slow"), properties = ("qualifiedName"))
}
calc def Nowhere :> Query {
	InState(name = "orbit")
}
calc def RunningStates :> Query {
	Project(source = States(source = InState(name = "run")), properties = ("path", "statePath"))
}
calc def StatesOfStates :> Query {
	in root : Element;
	States(source = States(source = root))
}
calc def StateDescendants :> Query {
	in root : Element;
	Descendants(source = States(source = root), maxDepth = 1)
}
`

type lampFixture struct {
	executionFixture
	ctx          *runtime.Context
	trace        *runtime.TraceRecorder
	lamp1, lamp2 *runtime.Instance
	rock, panel  *runtime.Instance
}

// loadLampFixture drives the lamps: lamp1 on at 0 s, dimmed at 1 s, boosted at
// 2.5 s; lamp2 on at 2 s, off at 2.5 s; the clock stands at 3.5 s.
func loadLampFixture(t *testing.T, queries string) lampFixture {
	t.Helper()
	return loadLampFixtureTracing(t, queries, runtime.NewTraceRecorder())
}

// loadLampFixtureTracing drives the lamps as loadLampFixture does, into trace.
func loadLampFixtureTracing(t *testing.T, queries string, trace *runtime.TraceRecorder) lampFixture {
	t.Helper()
	fixture := loadExecutionFixture(t, lampBody+queries)
	ctx := runtime.NewContext(runtime.NewModel(fixture.model, fixture.resolver), runtime.DefaultMaxSteps)
	ctx.SetTrace(trace)
	instantiate := func(name string) *runtime.Instance {
		inst, err := ctx.Instantiate(fixture.symbol(t, name))
		if err != nil {
			t.Fatalf("Instantiate %s: %v", name, err)
		}
		return inst
	}
	f := lampFixture{executionFixture: fixture, ctx: ctx, trace: trace}
	f.lamp1, f.lamp2, f.rock, f.panel = instantiate("lamp1"), instantiate("lamp2"), instantiate("rock"), instantiate("panel")
	f.send(t, f.lamp1, "Toggle", nil)
	f.advance(t, 1)
	f.send(t, f.lamp1, "Dim", map[string]runtime.Value{"level": integerValue(3)})
	f.advance(t, 1)
	f.send(t, f.lamp2, "Toggle", nil)
	f.advance(t, 0.5)
	f.send(t, f.lamp1, "Boost", nil)
	f.send(t, f.lamp2, "Toggle", nil)
	f.advance(t, 1)
	return f
}

func integerValue(n int64) runtime.Value {
	return runtime.Value{Kind: runtime.ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: n}}
}

func (f lampFixture) send(t *testing.T, inst *runtime.Instance, signal string, args map[string]runtime.Value) {
	t.Helper()
	msg, err := f.ctx.SignalMessage(f.symbol(t, signal), args, inst)
	if err != nil {
		t.Fatalf("send %s: %v", signal, err)
	}
	f.ctx.PostMessage(msg)
}

func (f lampFixture) advance(t *testing.T, seconds float64) {
	t.Helper()
	if _, err := f.ctx.Advance(seconds); err != nil {
		t.Fatalf("advance %v: %v", seconds, err)
	}
}

func (f lampFixture) session() Context {
	return Context{
		Index: f.index, Resolver: f.resolver, Model: f.model, Runtime: f.ctx,
		Roots: []Root{{Label: "lamp1", Object: f.lamp1}, {Label: "lamp2", Object: f.lamp2}, {Label: "rock", Object: f.rock}, {Label: "panel", Object: f.panel}},
	}
}

func (f lampFixture) modelOnly() Context {
	return Context{Index: f.index, Resolver: f.resolver, Model: f.model}
}

func (f lampFixture) run(t *testing.T, context Context, name string, bindings Bindings) (*RowSet, error) {
	t.Helper()
	return Execute(f.program(t, name), context, bindings, Options{})
}

func (f lampFixture) rows(t *testing.T, name string, bindings Bindings) *RowSet {
	t.Helper()
	result, err := f.run(t, f.session(), name, bindings)
	if err != nil {
		t.Fatalf("execute %s: %v", name, err)
	}
	return result
}

func (f lampFixture) object(inst *runtime.Instance, label string) Bindings {
	return Bindings{"root": {ObjectValue(inst, label)}}
}

// rowTexts renders every cell of every row as `column=value+value`, one line per row.
func rowTexts(t *testing.T, result *RowSet) []string {
	t.Helper()
	var out []string
	for _, row := range result.Rows() {
		var cells []string
		for i, cell := range row.Cells() {
			var parts []string
			for _, value := range cell.Values() {
				parts = append(parts, valueText(t, value))
			}
			cells = append(cells, result.Columns()[i].Name()+"="+strings.Join(parts, "+"))
		}
		out = append(out, strings.Join(cells, " "))
	}
	return out
}

func valueText(t *testing.T, value Value) string {
	t.Helper()
	if text, ok := value.String(); ok {
		return text
	}
	if sym, ok := value.Element(); ok {
		return symbols.FQNOf(sym)
	}
	if _, label, ok := value.Object(); ok {
		return "object " + label
	}
	if quantity, ok := value.Quantity(); ok {
		return quantity.String()
	}
	if n, ok := value.Integer(); ok {
		return semantics.FormatReal(float64(n))
	}
	if r, ok := value.Real(); ok {
		return semantics.FormatReal(r)
	}
	t.Fatalf("value of kind %s has no text", value.Kind())
	return ""
}

func joinLines(lines []string) string {
	return strings.Join(lines, "\n")
}

// States answers one row per active leaf: lamp1 stands in both regions of `on`,
// lamp2 in `off`; the composite `on` encloses both, each in its own region.
func TestExecuteStatesListsEveryActiveLeaf(t *testing.T) {
	fixture := loadLampFixture(t, lampQueries)
	got := rowTexts(t, fixture.rows(t, "CurrentStates", fixture.object(fixture.lamp1, "lamp1")))
	want := []string{
		"path=lamp1 machine=lp name=dim statePath=on.dim region=light enclosing=on",
		"path=lamp1 machine=lp name=fast statePath=on.fast region=fan enclosing=on",
	}
	if joinLines(got) != joinLines(want) {
		t.Fatalf("lamp1 states:\n%s\nwant:\n%s", joinLines(got), joinLines(want))
	}
	got = rowTexts(t, fixture.rows(t, "CurrentStates", fixture.object(fixture.lamp2, "lamp2")))
	if joinLines(got) != "path=lamp2 machine=lp name=off statePath=off region= enclosing=" {
		t.Fatalf("lamp2 states:\n%s", joinLines(got))
	}

	// A type selects the population: every lamp the session holds, in session order.
	got = rowTexts(t, fixture.rows(t, "CurrentStates", Bindings{"root": {ElementValue(fixture.symbol(t, "Lamp"))}}))
	if len(got) != 3 || got[0] != want[0] || got[1] != want[1] || !strings.HasPrefix(got[2], "path=lamp2 ") {
		t.Fatalf("Lamp states:\n%s", joinLines(got))
	}

	// The object column is the object row; the state column is the declaration.
	got = rowTexts(t, fixture.rows(t, "StateElements", fixture.object(fixture.lamp2, "lamp2")))
	if joinLines(got) != "object=object lamp2 state=Observatory::LampMachine::off" {
		t.Fatalf("state elements:\n%s", joinLines(got))
	}
}

// A leaf nested below a region's own state still stands in that region: the row
// names the innermost region an enclosing state is declared in.
func TestExecuteStatesNameTheRegionOfANestedLeaf(t *testing.T) {
	fixture := loadExecutionFixture(t, `
state def Dome {
	entry; then open;
	state open parallel {
		state shutter {
			entry; then ajar;
			state ajar {
				entry; then widening;
				state widening;
			}
		}
		state drive {
			entry; then idle;
			state idle;
		}
	}
}
part def Housing { exhibit state control : Dome; }
part housing : Housing;
calc def NestedStates :> Query {
	in root : Element;
	Project(source = States(source = root), properties = ("statePath", "region", "enclosing"))
}
`)
	ctx := runtime.NewContext(runtime.NewModel(fixture.model, fixture.resolver), runtime.DefaultMaxSteps)
	housing, err := ctx.Instantiate(fixture.symbol(t, "housing"))
	if err != nil {
		t.Fatalf("Instantiate housing: %v", err)
	}
	context := Context{Index: fixture.index, Resolver: fixture.resolver, Model: fixture.model, Runtime: ctx, Roots: []Root{{Label: "housing", Object: housing}}}
	result, err := Execute(fixture.program(t, "NestedStates"), context, Bindings{"root": {ObjectValue(housing, "housing")}}, Options{})
	if err != nil {
		t.Fatalf("execute NestedStates: %v", err)
	}
	want := []string{
		"statePath=open.ajar.widening region=shutter enclosing=open+ajar",
		"statePath=open.idle region=drive enclosing=open",
	}
	if got := rowTexts(t, result); joinLines(got) != joinLines(want) {
		t.Fatalf("nested leaves:\n%s\nwant:\n%s", joinLines(got), joinLines(want))
	}
}

// State rows take the row operations the object rows do: WhereFeature over the
// leaf's name or its region, WhereName, OrderBy, and Column expressions.
func TestExecuteStateRowsThroughRowOperations(t *testing.T) {
	fixture := loadLampFixture(t, lampQueries)
	lamps := Bindings{"root": {ElementValue(fixture.symbol(t, "Lamp"))}}

	got := rowTexts(t, fixture.rows(t, "LeavesNamed", Bindings{"root": lamps["root"], "leaf": {StringValue("fast")}}))
	if joinLines(got) != "path=lamp1 statePath=on.fast" {
		t.Fatalf("leaves named fast:\n%s", joinLines(got))
	}
	got = rowTexts(t, fixture.rows(t, "LeavesUnder", lamps))
	if joinLines(got) != "path=lamp1 name=dim" {
		t.Fatalf("leaves in region light:\n%s", joinLines(got))
	}
	got = rowTexts(t, fixture.rows(t, "StatesByName", lamps))
	if joinLines(got) != joinLines([]string{"path=lamp1 name=dim", "path=lamp1 name=fast", "path=lamp2 name=off"}) {
		t.Fatalf("states by name:\n%s", joinLines(got))
	}
	if result := fixture.rows(t, "StatesOfMachine", lamps); len(result.Rows()) != 0 {
		t.Fatalf("states named run = %d rows, want none", len(result.Rows()))
	}
	got = rowTexts(t, fixture.rows(t, "StateColumns", fixture.object(fixture.lamp1, "lamp1")))
	if joinLines(got) != joinLines([]string{"path=lamp1 label=lamp lamp1 where=lp:on.dim", "path=lamp1 label=lamp lamp1 where=lp:on.fast"}) {
		t.Fatalf("state columns:\n%s", joinLines(got))
	}
}

// InState answers the objects whose machine stands in the named state, once
// each: a leaf, a composite state enclosing one, or a dotted path.
func TestExecuteInStateFindsObjectsByLeafOrEnclosingState(t *testing.T) {
	fixture := loadLampFixture(t, lampQueries)
	if result := fixture.rows(t, "Running", nil); len(result.Rows()) != 0 {
		t.Fatalf("objects in run = %v, want none", cellTexts(t, result, 0))
	}
	if got := cellTexts(t, fixture.rows(t, "Lit", nil), 0); strings.Join(got, ",") != "lamp1" {
		t.Fatalf("objects in on = %v", got)
	}
	if result := fixture.rows(t, "Fanned", nil); len(result.Rows()) != 0 {
		t.Fatalf("objects in on.slow = %v, want none", cellTexts(t, result, 0))
	}
	fixture.send(t, fixture.lamp2, "Toggle", nil)
	fixture.advance(t, 0)
	if got := cellTexts(t, fixture.rows(t, "Fanned", nil), 0); strings.Join(got, ",") != "lamp2" {
		t.Fatalf("objects in on.slow after lamp2 switched on = %v", got)
	}
	if got := cellTexts(t, fixture.rows(t, "Lit", nil), 0); strings.Join(got, ",") != "lamp1,lamp2" {
		t.Fatalf("objects in on = %v", got)
	}
	// The inverse composes with States: the rows of the objects it selected.
	got := rowTexts(t, fixture.rows(t, "RunningStates", nil))
	if joinLines(got) != joinLines([]string{"path=lamp2 statePath=on.run", "path=lamp2 statePath=on.slow"}) {
		t.Fatalf("states of the objects in run:\n%s", joinLines(got))
	}
}

// Every unsupported path is a typed error: no session, no state machine, no object
// held, a state no machine declares, a state row read as an element or object.
func TestExecuteStatesRefusals(t *testing.T) {
	fixture := loadLampFixture(t, lampQueries)
	lamp1 := fixture.object(fixture.lamp1, "lamp1")

	_, err := fixture.run(t, fixture.modelOnly(), "Running", nil)
	executionError(t, err, ErrorNoRuntime)
	_, err = fixture.run(t, fixture.modelOnly(), "CurrentStates", Bindings{"root": {ElementValue(fixture.symbol(t, "Lamp"))}})
	executionError(t, err, ErrorNoRuntime)

	_, err = fixture.run(t, fixture.session(), "CurrentStates", fixture.object(fixture.rock, "rock"))
	if got := executionError(t, err, ErrorNoStateMachine); got.Target != "rock" || !strings.Contains(got.Error(), "rock, which exhibits no state machine") {
		t.Fatalf("no-state-machine error = %v", got)
	}
	_, err = fixture.run(t, fixture.session(), "CurrentStates", Bindings{"root": {ElementValue(fixture.symbol(t, "LampMachine"))}})
	if got := executionError(t, err, ErrorNotHeld); got.Target != "Observatory::LampMachine" {
		t.Fatalf("not-held error = %v", got)
	}
	_, err = fixture.run(t, fixture.session(), "Nowhere", nil)
	if got := executionError(t, err, ErrorUnknownState); got.Actual != "orbit" || !strings.Contains(got.Error(), "names state orbit") {
		t.Fatalf("unknown-state error = %v", got)
	}
	_, err = fixture.run(t, fixture.session(), "StatesOfStates", lamp1)
	if got := executionError(t, err, ErrorStateRow); got.Target != "lamp1.lp in on.dim" {
		t.Fatalf("state-row error = %v", got)
	}
	_, err = fixture.run(t, fixture.session(), "StateDescendants", lamp1)
	executionError(t, err, ErrorStateRow)
}
