package runtime

import (
	"errors"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/lower"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// instantiatePart instantiates the named part def, for a case about the
// connectors an object of it owns.
func instantiatePart(t *testing.T, name, src string) (*Instance, *Context) {
	t.Helper()
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))
	sym := findSymbolByName(idx.DocumentRoot("<test>"), name, ast.DefPart)
	if sym == nil {
		t.Fatalf("part def %s not found", name)
	}
	inst, err := ctx.Instantiate(sym)
	if err != nil {
		t.Fatalf("Instantiate %s: %v", name, err)
	}
	return inst, ctx
}

// featureValueInstance reads a feature value expected to hold one object and returns it.
func fvInstance(t *testing.T, ctx *Context, inst *Instance, path ...string) *Instance {
	t.Helper()
	cur := inst
	for _, name := range path {
		fv, err := cur.GetFeatureValue(ctx, name)
		if err != nil {
			t.Fatalf("GetFeatureValue %s: %v", name, err)
		}
		if fv.Value.Kind != ValInstance && fv.Value.Kind != ValVariant {
			t.Fatalf("feature value %s holds %s, want an object", name, fv.Value.Kind)
		}
		next, ok := ctx.Instance(fv.Value.Instance)
		if !ok {
			t.Fatalf("feature value %s names object %d, which the context does not hold", name, fv.Value.Instance)
		}
		cur = next
	}
	return cur
}

const twoPortSystem = `
	package test {
		private import ScalarValues::Real;
		port def P { attribute rate : Real = 3.0; }
		part def A { port p : P; }
		part def B { port q : P; }
		connection def Link {
			end source : P;
			end target : P;
		}
		part def Sys {
			part a : A;
			part b : B;
			connection link : Link connect a.p to b.q;
			interface iface connect a.p to b.q;
			connect a.p to b.q;
		}
	}
`

// A connector end is the feature it attaches to, not a copy of it: the object
// at `link.source` is the very object `a.p` holds (KerML 1.0 §7.4.6).
func TestConnectorEndsAreTheConnectedFeatures(t *testing.T) {
	inst, ctx := instantiatePart(t, "Sys", twoPortSystem)
	port := fvInstance(t, ctx, inst, "a", "p")
	peer := fvInstance(t, ctx, inst, "b", "q")

	link := fvInstance(t, ctx, inst, "link")
	if got := fvInstance(t, ctx, link, "source"); got.ID != port.ID {
		t.Errorf("link.source is object %d, want a.p (%d)", got.ID, port.ID)
	}
	if got := fvInstance(t, ctx, link, "target"); got.ID != peer.ID {
		t.Errorf("link.target is object %d, want b.q (%d)", got.ID, peer.ID)
	}
}

// Writing through a connected port is read through the end attached to it:
// sharing identity is what makes the connection observable.
func TestWritingAConnectedPortIsReadThroughTheEnd(t *testing.T) {
	inst, ctx := instantiatePart(t, "Sys", twoPortSystem)
	port := fvInstance(t, ctx, inst, "a", "p")
	fv, err := port.GetFeatureValue(ctx, "rate")
	if err != nil {
		t.Fatalf("GetFeatureValue rate: %v", err)
	}
	fv.Value = Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValReal, Real: 9.5}}

	end := fvInstance(t, ctx, fvInstance(t, ctx, inst, "link"), "source")
	read, err := end.GetFeatureValue(ctx, "rate")
	if err != nil {
		t.Fatalf("GetFeatureValue rate through the end: %v", err)
	}
	if read.Value.Const.Real != 9.5 {
		t.Errorf("link.source.rate = %v, want the 9.5 written on a.p", read.Value)
	}
}

// An optional or abstract connector links nothing of its own — it holds what a
// connector subsetting it holds — while a required one still links its ends.
func TestOptionalConnectorLinksNothingOfItsOwn(t *testing.T) {
	inst, ctx := instantiatePart(t, "Sys", `
		package test {
			port def P;
			part def A { port p : P; }
			part def B { port q : P; }
			connection def Link { end source : P; end target : P; }
			part def Sys {
				part a : A;
				part b : B;
				connection hitch : Link[0..1] connect a.p to b.q;
				interface spare[0..1] connect a.p to b.q;
				abstract connection links : Link[0..*];
				connection fitted : Link[0..1] connect a.p to b.q;
				connection tow : Link :> fitted connect a.p to b.q;
				connection link : Link connect a.p to b.q;
			}
		}`)
	before := len(ctx.instances)
	for _, name := range []string{"hitch", "spare"} {
		fv, err := inst.GetFeatureValue(ctx, name)
		if err != nil {
			t.Fatalf("sys.%s: %v", name, err)
		}
		if held := fv.HeldValue(); held.Kind != ValInvalid {
			t.Errorf("sys.%s = %s, want no link", name, FormatValue(held))
		}
		if read, err := fv.ReadValue(name); err != nil || elementCount(&read) != 0 {
			t.Errorf("sys.%s reads %s, %v; want the empty sequence", name, FormatValue(read), err)
		}
	}
	links, err := inst.GetFeatureValue(ctx, "links")
	if err != nil {
		t.Fatalf("sys.links: %v", err)
	}
	if held := links.HeldValue(); held.Kind != ValSequence || elementCount(&held) != 0 {
		t.Errorf("sys.links = %s, want an empty collection", FormatValue(held))
	}
	if n := len(ctx.instances); n != before {
		t.Errorf("reading the optional connectors made %d object(s), want none", n-before)
	}
	tow := fvInstance(t, ctx, inst, "tow")
	if got := fvInstance(t, ctx, inst, "fitted"); got.ID != tow.ID {
		t.Errorf("sys.fitted is object %d, want the tow that subsets it (%d)", got.ID, tow.ID)
	}
	port := fvInstance(t, ctx, inst, "a", "p")
	for _, name := range []string{"tow", "link"} {
		if got := fvInstance(t, ctx, fvInstance(t, ctx, inst, name), "source"); got.ID != port.ID {
			t.Errorf("%s.source is object %d, want a.p (%d)", name, got.ID, port.ID)
		}
	}
}

// An untyped connector usage names no definition, so its type is implicit
// (SysML v2 §8.3.13): it materializes all the same, with the connected features
// at its ends, rather than reading as an unknown value.
func TestUntypedConnectorUsageMaterializes(t *testing.T) {
	inst, ctx := instantiatePart(t, "Sys", twoPortSystem)
	port := fvInstance(t, ctx, inst, "a", "p")
	peer := fvInstance(t, ctx, inst, "b", "q")

	iface := fvInstance(t, ctx, inst, "iface")
	if len(iface.Ends) != 2 {
		t.Fatalf("iface has %d ends, want 2", len(iface.Ends))
	}
	if got := fvInstance(t, ctx, iface, "source"); got.ID != port.ID {
		t.Errorf("iface.source is object %d, want a.p (%d)", got.ID, port.ID)
	}
	if got := fvInstance(t, ctx, iface, "target"); got.ID != peer.ID {
		t.Errorf("iface.target is object %d, want b.q (%d)", got.ID, peer.ID)
	}
}

// An anonymous `connect a.p to b.q` is a member of the object no feature names, and
// joins its ends exactly as a named connector does.
func TestAnonymousConnectorJoinsItsEnds(t *testing.T) {
	inst, ctx := instantiatePart(t, "Sys", twoPortSystem)
	port := fvInstance(t, ctx, inst, "a", "p")
	peer := fvInstance(t, ctx, inst, "b", "q")

	conns, err := inst.OwnedConnectors(ctx)
	if err != nil {
		t.Fatalf("OwnedConnectors: %v", err)
	}
	if len(conns) != 1 {
		t.Fatalf("object owns %d anonymous connectors, want 1", len(conns))
	}
	ends := conns[0].Ends
	if len(ends) != 2 {
		t.Fatalf("anonymous connector has %d ends, want 2", len(ends))
	}
	if ends[0].Value.Instance != port.ID || ends[1].Value.Instance != peer.ID {
		t.Errorf("anonymous connector joins %d and %d, want a.p (%d) and b.q (%d)",
			ends[0].Value.Instance, ends[1].Value.Instance, port.ID, peer.ID)
	}
}

// Reading the same anonymous connector twice reads the same object: it is one
// member of the object, materialized once.
func TestAnonymousConnectorIsMaterializedOnce(t *testing.T) {
	inst, ctx := instantiatePart(t, "Sys", twoPortSystem)
	first, err := inst.OwnedConnectors(ctx)
	if err != nil {
		t.Fatalf("OwnedConnectors: %v", err)
	}
	second, err := inst.OwnedConnectors(ctx)
	if err != nil {
		t.Fatalf("OwnedConnectors again: %v", err)
	}
	if len(first) != 1 || len(second) != 1 || first[0].ID != second[0].ID {
		t.Errorf("anonymous connector read as %v then %v, want one object both times", first, second)
	}
}

const nestedSystem = `
	package test {
		private import ScalarValues::Real;
		port def P { attribute rate : Real = 3.0; }
		part def Inner { port p : P; }
		part def A { part inner : Inner; }
		part def B { port q : P; }
		connection def Link { end source : P; end target : P; }
		connection def Link2 :> Link { end s2 : P; end t2 : P; }
		part def Sys {
			part a : A;
			part b : B;
			connection nested : Link connect a.inner.p to b.q;
			connection parts connect a to b;
			connection tri connect (a, b, a.inner);
			connection sub : Link2 connect a.inner.p to b.q;
		}
	}
`

// An end may name a feature reached through a chain: `a.inner.p` attaches the
// port of the nested part, not a port of `a`.
func TestConnectorEndFollowsAFeatureChain(t *testing.T) {
	inst, ctx := instantiatePart(t, "Sys", nestedSystem)
	port := fvInstance(t, ctx, inst, "a", "inner", "p")
	nested := fvInstance(t, ctx, inst, "nested")
	if got := fvInstance(t, ctx, nested, "source"); got.ID != port.ID {
		t.Errorf("nested.source is object %d, want a.inner.p (%d)", got.ID, port.ID)
	}
}

// An end may attach to a part rather than a port: what a connector relates is
// features, of whatever kind.
func TestConnectorEndAttachesToAPart(t *testing.T) {
	inst, ctx := instantiatePart(t, "Sys", nestedSystem)
	a := fvInstance(t, ctx, inst, "a")
	b := fvInstance(t, ctx, inst, "b")
	parts := fvInstance(t, ctx, inst, "parts")
	if got := fvInstance(t, ctx, parts, "source"); got.ID != a.ID {
		t.Errorf("parts.source is object %d, want a (%d)", got.ID, a.ID)
	}
	if got := fvInstance(t, ctx, parts, "target"); got.ID != b.ID {
		t.Errorf("parts.target is object %d, want b (%d)", got.ID, b.ID)
	}
}

// A connector with more than two ends keeps every one of them, in declaration
// order, in the participant feature a link holds its ends in.
func TestNaryConnectorKeepsEveryEnd(t *testing.T) {
	inst, ctx := instantiatePart(t, "Sys", nestedSystem)
	want := []int64{
		fvInstance(t, ctx, inst, "a").ID,
		fvInstance(t, ctx, inst, "b").ID,
		fvInstance(t, ctx, inst, "a", "inner").ID,
	}
	tri := fvInstance(t, ctx, inst, "tri")
	fv, err := tri.GetFeatureValue(ctx, participantEndName)
	if err != nil {
		t.Fatalf("GetFeatureValue participant: %v", err)
	}
	if fv.Values.Kind != ValSequence {
		t.Fatalf("participant holds %s, want a sequence of the ends", fv.Values.Kind)
	}
	got := fv.Values.Sequence().Elements()
	if len(got) != len(want) {
		t.Fatalf("participant holds %d ends, want %d", len(got), len(want))
	}
	for i, el := range got {
		if el.Instance != want[i] {
			t.Errorf("participant#%d is object %d, want %d", i+1, el.Instance, want[i])
		}
	}
}

// A connector declared in a specialization redefines the inherited ends by
// position (SysML v2 §8.3.13), so both names read the one end.
func TestRedefinedEndSharesTheInheritedFeatureValue(t *testing.T) {
	inst, ctx := instantiatePart(t, "Sys", nestedSystem)
	port := fvInstance(t, ctx, inst, "a", "inner", "p")
	peer := fvInstance(t, ctx, inst, "b", "q")
	sub := fvInstance(t, ctx, inst, "sub")
	for _, name := range []string{"s2", "source"} {
		if got := fvInstance(t, ctx, sub, name); got.ID != port.ID {
			t.Errorf("sub.%s is object %d, want a.inner.p (%d)", name, got.ID, port.ID)
		}
	}
	for _, name := range []string{"t2", "target"} {
		if got := fvInstance(t, ctx, sub, name); got.ID != peer.ID {
			t.Errorf("sub.%s is object %d, want b.q (%d)", name, got.ID, peer.ID)
		}
	}
}

const variationSystem = `
	package test {
		port def P;
		part def Part { port p1 : P; port p2 : P; port p3 : P; }
		part def Sys {
			part x : Part;
			variation interface link {
				variant interface direct connect x.p1 to x.p2;
				variant interface indirect connect x.p1 to x.p3;
			}
		}
		part chosenDirect : Sys { ref :>> link = link::direct; }
		part chosenIndirect : Sys { ref :>> link = link::indirect; }
	}
`

// The connection a selected `variant interface` declares is realized: the
// variation's feature value holds that variant's connector, with the features it connects
// at its ends. Selecting the other variant realizes the other connection.
func TestSelectedVariantInterfaceIsRealized(t *testing.T) {
	for _, tt := range []struct {
		usage  string
		target string // the port the selected variant connects x.p1 to
	}{
		{"chosenDirect", "p2"},
		{"chosenIndirect", "p3"},
	} {
		idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, variationSystem))
		sym := findSymbolByName(idx.DocumentRoot("<test>"), tt.usage, ast.DefPart)
		if sym == nil {
			t.Fatalf("part %s not found", tt.usage)
		}
		inst, err := ctx.Instantiate(sym)
		if err != nil {
			t.Fatalf("Instantiate %s: %v", tt.usage, err)
		}
		source := fvInstance(t, ctx, inst, "x", "p1")
		want := fvInstance(t, ctx, inst, "x", tt.target)
		other := "p3"
		if tt.target == "p3" {
			other = "p2"
		}
		unselected := fvInstance(t, ctx, inst, "x", other)

		link := fvInstance(t, ctx, inst, "link")
		if len(link.Ends) != 2 {
			t.Fatalf("%s: the realized connection has %d ends, want 2", tt.usage, len(link.Ends))
		}
		if got := fvInstance(t, ctx, link, "source"); got.ID != source.ID {
			t.Errorf("%s: link.source is object %d, want x.p1 (%d)", tt.usage, got.ID, source.ID)
		}
		got := fvInstance(t, ctx, link, "target")
		if got.ID != want.ID {
			t.Errorf("%s: link.target is object %d, want x.%s (%d)", tt.usage, got.ID, tt.target, want.ID)
		}
		if got.ID == unselected.ID {
			t.Errorf("%s: link.target is x.%s, which the unselected variant connects", tt.usage, other)
		}
	}
}

// testUnattachableConnectorEnd: an end naming a feature no object reaches is
// reported, with where the end was written — a connector that cannot be attached
// is no connector, and must not read as an unknown value or fabricate an object
// of the end's declared type.
func testUnattachableConnectorEnd(t *testing.T) {
	inst, ctx := instantiatePart(t, "Sys", `
		package test {
			port def P;
			part def A { port p : P; }
			part def Sys {
				part a : A;
				connection link connect a.p to a.missing;
			}
		}
	`)
	_, err := inst.GetFeatureValue(ctx, "link")
	if !errors.Is(err, ErrConnectorEnd) {
		t.Fatalf("expected ErrConnectorEnd, got: %v", err)
	}
	var endErr *ConnectorEndError
	if !errors.As(err, &endErr) {
		t.Fatalf("expected a *ConnectorEndError, got %T", err)
	}
	if endErr.End != "a.missing" {
		t.Errorf("error names end %q, want a.missing", endErr.End)
	}
	if !strings.Contains(endErr.Location, "<test>") {
		t.Errorf("error is located at %q, want the file the end was written in", endErr.Location)
	}
	for _, id := range ctx.InstanceIDs() {
		if obj, _ := ctx.Instance(id); obj.Ends != nil || obj.Type.Name == "link" {
			t.Errorf("the connector that could not be attached was left behind as object %d", id)
		}
	}
}

// testUnattachableConnectorLeavesNoBehavior: a connector that cannot be attached
// runs nothing either — the behaviors its type binds, started when the connector
// was created, go with it, so a later run drains only surviving objects' work.
func testUnattachableConnectorLeavesNoBehavior(t *testing.T) {
	inst, ctx := instantiatePart(t, "Sys", `
		package test {
			port def P;
			part def A { port p : P; }
			connection def Link {
				attribute n : Integer = 0;
				exhibit state life { entry; then up; state up { entry action bump { assign n := 1; } } }
			}
			part def Sys {
				part a : A;
				connection link : Link connect a.p to a.missing;
				connection ok : Link connect a.p to a.p;
			}
		}
	`)
	if _, err := inst.GetFeatureValue(ctx, "link"); !errors.Is(err, ErrConnectorEnd) {
		t.Fatalf("expected ErrConnectorEnd, got: %v", err)
	}
	if got := len(ctx.objectBehaviors); got != 0 {
		t.Errorf("%d behavior(s) left attached by the connector that could not be attached", got)
	}
	if got := len(ctx.pendingBehaviors); got != 0 {
		t.Errorf("%d behavior(s) left queued by the connector that could not be attached", got)
	}

	ok := fvInstance(t, ctx, inst, "ok")
	if got := len(ctx.objectBehaviors); got != 1 {
		t.Errorf("%d behavior(s) attached, want the attached connector's alone", got)
	}
	if _, running := ok.ExhibitedState(); !running {
		t.Error("the connector that attached exhibits no machine")
	}
	if got := featureInt(t, ctx, ok, "n"); got != 1 {
		t.Errorf("n = %d, want 1 once the connector's machine entered up", got)
	}
}

// testUnattachableConnectorAbandonsWhatItsEndsMaterialized: an object an earlier
// end materialized goes with the connector a later end fails, its behaviors too.
// The part is a package-level occurrence, materialized when an end first names it.
func testUnattachableConnectorAbandonsWhatItsEndsMaterialized(t *testing.T) {
	model, resolver, root := parseAndBuildModel(t, `
		package test {
			port def P;
			part def Ticking {
				port p : P;
				exhibit state life { entry; then on; state on; }
			}
			part ticker : Ticking;
			part def Sys {
				connection link connect ticker.p to ticker.missing;
			}
		}
	`)
	pkg := resolveSymbol(t, root, "test")
	ctx := NewContext(model, resolver, 10000)
	inst, err := ctx.Instantiate(resolveSymbol(t, pkg.Scope, "Sys"))
	if err != nil {
		t.Fatalf("Instantiate Sys: %v", err)
	}
	before := len(ctx.InstanceIDs())
	if _, held := ctx.occurrences[resolveSymbol(t, pkg.Scope, "ticker")]; held {
		t.Fatal("ticker was materialized with Sys: the end naming it materializes it")
	}
	if _, err := inst.GetFeatureValue(ctx, "link"); !errors.Is(err, ErrConnectorEnd) {
		t.Fatalf("expected ErrConnectorEnd, got: %v", err)
	}
	if got := len(ctx.InstanceIDs()); got != before {
		t.Errorf("%d object(s) held, want %d: the part the first end materialized is gone with the connector", got, before)
	}
	if got := len(ctx.objectBehaviors); got != 0 {
		t.Errorf("%d behavior(s) left attached by the part the connector's first end materialized", got)
	}
	if got := len(ctx.pendingBehaviors); got != 0 {
		t.Errorf("%d behavior(s) left queued by the part the connector's first end materialized", got)
	}

	ticker, err := ctx.occurrenceOf(resolveSymbol(t, pkg.Scope, "ticker"))
	if err != nil {
		t.Fatalf("occurrenceOf ticker: %v", err)
	}
	assertCurrentState(t, machineOf(t, ticker, "life").State, "on")
	if got := len(ctx.objectBehaviors); got != 1 {
		t.Errorf("%d behavior(s) attached once ticker is read on its own, want its machine alone", got)
	}
}

// testUnattachableConnectorTouchesNoOtherObject: a connector that cannot attach
// has sent no message and written nothing on the objects that survive it.
func testUnattachableConnectorTouchesNoOtherObject(t *testing.T) {
	model, resolver, root := parseAndBuildModel(t, `
		package test {
			item def Ping;
			port def P;
			part def A { port p : P; }
			part def Listener {
				attribute heard : Integer = 0;
				exhibit state listening {
					entry; then waiting;
					state waiting;
					accept Ping then noted;
					state noted { entry action note { assign heard := heard + 1; } }
				}
			}
			part good : Listener;
			connection def Link {
				exhibit state life { entry; then up; state up { entry send Ping() to good; } }
			}
			part def Sys {
				part a : A;
				connection link : Link connect a.p to a.missing;
				connection ok : Link connect a.p to a.p;
			}
		}
	`)
	pkg := resolveSymbol(t, root, "test")
	ctx := NewContext(model, resolver, 10000)
	good, err := ctx.occurrenceOf(resolveSymbol(t, pkg.Scope, "good"))
	if err != nil {
		t.Fatalf("occurrenceOf good: %v", err)
	}
	inst, err := ctx.Instantiate(resolveSymbol(t, pkg.Scope, "Sys"))
	if err != nil {
		t.Fatalf("Instantiate Sys: %v", err)
	}
	if _, err := inst.GetFeatureValue(ctx, "link"); !errors.Is(err, ErrConnectorEnd) {
		t.Fatalf("expected ErrConnectorEnd, got: %v", err)
	}
	assertCurrentState(t, machineOf(t, good, "listening").State, "waiting")
	if got := featureInt(t, ctx, good, "heard"); got != 0 {
		t.Errorf("heard = %d after the connector that could not be attached, want 0: its behavior never ran", got)
	}
	if got := len(ctx.messages); got != 0 {
		t.Errorf("%d message(s) left on the bus by the connector that could not be attached", got)
	}

	fvInstance(t, ctx, inst, "ok")
	assertCurrentState(t, machineOf(t, good, "listening").State, "noted")
	if got := featureInt(t, ctx, good, "heard"); got != 1 {
		t.Errorf("heard = %d once the connector that attached ran, want 1", got)
	}
}

// testConnectorWhoseStartFailsLeavesNoTrace: a connector whose own behavior
// writes to and signals an older object before failing is undone whole — the
// write, the signal and what the older object would have done on it — and a
// connector whose start succeeds is answered by the older object once kept.
func testConnectorWhoseStartFailsLeavesNoTrace(t *testing.T) {
	model, resolver, root := parseAndBuildModel(t, `
		package test {
			item def Ping;
			port def P;
			part def Listener {
				attribute heard : Integer = 0;
				exhibit state listening {
					entry; then waiting;
					state waiting;
					accept Ping then noted;
					state noted { entry action note { assign heard := heard + 1; } }
				}
			}
			part good : Listener;
			part def A { port p : P; }
			connection def Faulty {
				exhibit state life {
					entry; then poke;
					state poke { entry action bump { assign good.heard := good.heard + 10; } }
					transition first poke then ping;
					state ping { entry send Ping() to good; }
					transition first ping then fail;
					state fail { entry action bad { assign good.nosuch := 1; } }
				}
			}
			connection def Fine {
				exhibit state life {
					entry; then poke;
					state poke { entry action bump { assign good.heard := good.heard + 10; } }
					transition first poke then ping;
					state ping { entry send Ping() to good; }
				}
			}
			part def Sys {
				part a : A;
				connection faulty : Faulty connect a.p to a.p;
				connection fine : Fine connect a.p to a.p;
			}
		}
	`)
	pkg := resolveSymbol(t, root, "test")
	ctx := NewContext(model, resolver, 10000)
	good, err := ctx.occurrenceOf(resolveSymbol(t, pkg.Scope, "good"))
	if err != nil {
		t.Fatalf("occurrenceOf good: %v", err)
	}
	inst, err := ctx.Instantiate(resolveSymbol(t, pkg.Scope, "Sys"))
	if err != nil {
		t.Fatalf("Instantiate Sys: %v", err)
	}
	if _, err := inst.GetFeatureValue(ctx, "faulty"); !errors.Is(err, ErrNoSuchFeature) {
		t.Fatalf("expected ErrNoSuchFeature from the connector's behavior, got: %v", err)
	}
	if fv := inst.FeatureValues["faulty"]; fv != nil && fv.Materialized {
		t.Errorf("the connector whose start failed is held: %+v", fv)
	}
	assertCurrentState(t, machineOf(t, good, "listening").State, "waiting")
	if got := featureInt(t, ctx, good, "heard"); got != 0 {
		t.Errorf("heard = %d after the connector whose start failed, want 0: its write was undone", got)
	}
	if got := len(ctx.messages); got != 0 {
		t.Errorf("%d message(s) left on the bus by the connector whose start failed", got)
	}
	if got := len(ctx.objectBehaviors) + len(ctx.pendingBehaviors); got != 1 {
		t.Errorf("%d behavior(s) attached or pending besides good's machine", got-1)
	}

	fine := fvInstance(t, ctx, inst, "fine")
	assertCurrentState(t, machineOf(t, fine, "life").State, "ping")
	assertCurrentState(t, machineOf(t, good, "listening").State, "noted")
	if got := featureInt(t, ctx, good, "heard"); got != 11 {
		t.Errorf("heard = %d once a connector's start succeeded, want 11: its write kept and its signal answered", got)
	}
	if got := len(ctx.messages); got != 0 {
		t.Errorf("%d message(s) left on the bus once the connector was kept and answered", got)
	}
}

// testConnectorAnsweredByAFailingBehaviorIsKept: a connector created whole is
// kept when an older object's behavior fails answering it — its write kept, its
// object held under its feature, its behavior attached — and the failure is that
// behavior's, reported once: reading the connector again neither fails nor reruns it.
func testConnectorAnsweredByAFailingBehaviorIsKept(t *testing.T) {
	model, resolver, root := parseAndBuildModel(t, `
		package test {
			item def Ping;
			port def P;
			part def Listener {
				attribute heard : Integer = 0;
				exhibit state listening {
					entry; then waiting;
					state waiting;
					accept Ping then noted;
					state noted { entry action note { assign heard := heard + nosuch; } }
				}
			}
			part good : Listener;
			part other : Listener;
			part def A { port p : P; }
			connection def Fine {
				attribute pokes : Integer = 0;
				exhibit state life {
					entry; then poke;
					state poke { entry action bump { assign good.heard := good.heard + 10; assign pokes := pokes + 1; } }
					transition first poke then ping;
					state ping { entry send Ping() to good; }
				}
			}
			connection def Other {
				exhibit state life { entry; then ping; state ping { entry send Ping() to other; } }
			}
			part def Sys {
				part a : A;
				connection fine : Fine connect a.p to a.p;
				connection : Other connect a.p to a.p;
			}
		}
	`)
	pkg := resolveSymbol(t, root, "test")
	ctx := NewContext(model, resolver, 10000)
	good, err := ctx.occurrenceOf(resolveSymbol(t, pkg.Scope, "good"))
	if err != nil {
		t.Fatalf("occurrenceOf good: %v", err)
	}
	inst, err := ctx.Instantiate(resolveSymbol(t, pkg.Scope, "Sys"))
	if err != nil {
		t.Fatalf("Instantiate Sys: %v", err)
	}
	if _, err := inst.GetFeatureValue(ctx, "fine"); !errors.Is(err, ErrUnresolvedReference) {
		t.Fatalf("expected the listener's behavior to fail on its unresolved reference, got: %v", err)
	}
	fv := inst.FeatureValues["fine"]
	if fv == nil || !fv.Materialized || fv.Value.Kind != ValInstance {
		t.Fatalf("the connector the listener failed answering is not held: %+v", fv)
	}
	fine, live := ctx.Instance(fv.Value.Instance)
	if !live {
		t.Fatalf("the context does not hold connector %d", fv.Value.Instance)
	}
	if len(fine.Ends) != 2 {
		t.Fatalf("the kept connector has %d end(s), want 2", len(fine.Ends))
	}
	for _, end := range fine.Ends {
		if _, held := ctx.Instance(end.Value.Instance); end.Value.Kind != ValInstance || !held {
			t.Errorf("end %q of the kept connector is not attached to a held object: %+v", end.Name, end.Value)
		}
	}
	assertCurrentState(t, machineOf(t, fine, "life").State, "ping")
	if got := featureInt(t, ctx, good, "heard"); got != 10 {
		t.Errorf("heard = %d, want 10: the kept connector's write stays with the connector", got)
	}
	if got := featureInt(t, ctx, fine, "pokes"); got != 1 {
		t.Errorf("pokes = %d, want 1: the connector's own write is kept", got)
	}
	if got := len(ctx.messages); got != 0 {
		t.Errorf("%d message(s) left on the bus: the listener took the signal before failing", got)
	}

	again, err := inst.GetFeatureValue(ctx, "fine")
	if err != nil {
		t.Fatalf("reading the kept connector again: %v", err)
	}
	if again.Value.Instance != fine.ID {
		t.Errorf("reading again holds connector %d, want %d: the kept one", again.Value.Instance, fine.ID)
	}
	if got := featureInt(t, ctx, fine, "pokes"); got != 1 {
		t.Errorf("pokes = %d after reading again, want 1: the kept connector does not run again", got)
	}

	// An anonymous connector is kept the same way, in its slot.
	if _, err := ctx.occurrenceOf(resolveSymbol(t, pkg.Scope, "other")); err != nil {
		t.Fatalf("occurrenceOf other: %v", err)
	}
	if _, err := inst.OwnedConnectors(ctx); !errors.Is(err, ErrUnresolvedReference) {
		t.Fatalf("expected the other listener's behavior to fail on its unresolved reference, got: %v", err)
	}
	if got := len(ctx.messages); got != 0 {
		t.Errorf("%d message(s) left on the bus: the other listener took the signal before failing", got)
	}
	conns := inst.MaterializedConnectors(ctx)
	if len(conns) != 1 {
		t.Fatalf("the object holds %d anonymous connectors, want 1: the one the other listener failed answering", len(conns))
	}
	assertCurrentState(t, machineOf(t, conns[0], "life").State, "ping")
	if again, err := inst.OwnedConnectors(ctx); err != nil || len(again) != 1 || again[0].ID != conns[0].ID {
		t.Errorf("OwnedConnectors again = %v, %v; want the kept connector %d and no error", again, err, conns[0].ID)
	}
}

// testUnattachableConnectorEndsRunNothingEarly: the behaviors of an object an
// end materializes run only once every end is attached, so a later end's failure
// leaves the objects that survive it exactly as they were.
func testUnattachableConnectorEndsRunNothingEarly(t *testing.T) {
	model, resolver, root := parseAndBuildModel(t, `
		package test {
			item def Ping;
			port def P;
			part def Listener {
				attribute heard : Integer = 0;
				exhibit state listening {
					entry; then waiting;
					state waiting;
					accept Ping then noted;
					state noted { entry action note { assign heard := heard + 1; } }
				}
			}
			part good : Listener;
			part def Pinger {
				port p : P;
				exhibit state life { entry; then up; state up { entry send Ping() to good; } }
			}
			part pinger : Pinger;
			part def Sys {
				connection link connect pinger.p to pinger.missing;
				connection ok connect pinger.p to pinger.p;
			}
		}
	`)
	pkg := resolveSymbol(t, root, "test")
	ctx := NewContext(model, resolver, 10000)
	good, err := ctx.occurrenceOf(resolveSymbol(t, pkg.Scope, "good"))
	if err != nil {
		t.Fatalf("occurrenceOf good: %v", err)
	}
	inst, err := ctx.Instantiate(resolveSymbol(t, pkg.Scope, "Sys"))
	if err != nil {
		t.Fatalf("Instantiate Sys: %v", err)
	}
	before := len(ctx.InstanceIDs())
	if _, held := ctx.occurrences[resolveSymbol(t, pkg.Scope, "pinger")]; held {
		t.Fatal("pinger was materialized with Sys: the end naming it materializes it")
	}
	if _, err := inst.GetFeatureValue(ctx, "link"); !errors.Is(err, ErrConnectorEnd) {
		t.Fatalf("expected ErrConnectorEnd, got: %v", err)
	}
	if got := len(ctx.InstanceIDs()); got != before {
		t.Errorf("%d object(s) held, want %d: the part the first end materialized is gone with the connector", got, before)
	}
	assertCurrentState(t, machineOf(t, good, "listening").State, "waiting")
	if got := featureInt(t, ctx, good, "heard"); got != 0 {
		t.Errorf("heard = %d after the connector that could not be attached, want 0: the part its first end materialized never ran", got)
	}
	if got := len(ctx.messages); got != 0 {
		t.Errorf("%d message(s) left on the bus by the part the connector's first end materialized", got)
	}
	if got := len(ctx.objectBehaviors) + len(ctx.pendingBehaviors); got != 1 {
		t.Errorf("%d behavior(s) attached or pending besides good's machine", got-1)
	}

	fvInstance(t, ctx, inst, "ok")
	pinger, err := ctx.occurrenceOf(resolveSymbol(t, pkg.Scope, "pinger"))
	if err != nil {
		t.Fatalf("occurrenceOf pinger: %v", err)
	}
	assertCurrentState(t, machineOf(t, pinger, "life").State, "up")
	assertCurrentState(t, machineOf(t, good, "listening").State, "noted")
	if got := featureInt(t, ctx, good, "heard"); got != 1 {
		t.Errorf("heard = %d once the connector that attached ran the part it materialized, want 1", got)
	}
}

// testMultiplicityOnAConnector: a connector usage holding more than one
// connector is not one connection, so there is no set of ends to attach — that
// is reported rather than filled with objects of the connector's type.
func testMultiplicityOnAConnector(t *testing.T) {
	inst, ctx := instantiatePart(t, "Sys", `
		package test {
			port def P;
			part def A { port p : P; }
			part def B { port q : P; }
			part def Sys {
				part a : A;
				part b : B;
				connection links[2] connect a.p to b.q;
			}
		}
	`)
	_, err := inst.GetFeatureValue(ctx, "links")
	if !errors.Is(err, ErrConnectorEnd) {
		t.Fatalf("expected ErrConnectorEnd, got: %v", err)
	}
	var endErr *ConnectorEndError
	if !errors.As(err, &endErr) {
		t.Fatalf("expected a *ConnectorEndError, got %T", err)
	}
	if !strings.Contains(endErr.Location, "<test>") {
		t.Errorf("error is located at %q, want the file the connector was written in", endErr.Location)
	}
}

// testConnectorAttachedToItself: an end naming the connector it belongs to would
// attach ends forever, so the cycle is reported rather than recursed into.
func testConnectorAttachedToItself(t *testing.T) {
	inst, ctx := instantiatePart(t, "Sys", `
		package test {
			port def P;
			part def A { port p : P; }
			part def Sys {
				part a : A;
				connection link connect link to a.p;
			}
		}
	`)
	if _, err := inst.GetFeatureValue(ctx, "link"); !errors.Is(err, ErrCyclicFeatureValue) {
		t.Fatalf("expected ErrCyclicFeatureValue, got: %v", err)
	}
}

// testMutuallyAttachedConnectors: two connectors each naming the other as an end
// are a cycle across feature values, reported the same way as a self-attached one.
func testMutuallyAttachedConnectors(t *testing.T) {
	inst, ctx := instantiatePart(t, "Sys", `
		package test {
			port def P;
			part def A { port p : P; }
			part def Sys {
				part a : A;
				connection here connect there to a.p;
				connection there connect here to a.p;
			}
		}
	`)
	if _, err := inst.GetFeatureValue(ctx, "here"); !errors.Is(err, ErrCyclicFeatureValue) {
		t.Fatalf("expected ErrCyclicFeatureValue, got: %v", err)
	}
}

// An anonymous succession or transition carries ends too, but relates them in
// time rather than joining them, so it is no connector to materialize and must
// not be reported as one when the connectors of an object are read.
func TestAnonymousSuccessionIsNoConnector(t *testing.T) {
	inst, ctx := instantiatePart(t, "Sys", `
		package test {
			port def P;
			part def A { port p : P; }
			part def B { port q : P; }
			action def Step;
			part def Sys {
				part a : A;
				part b : B;
				action one : Step;
				action two : Step;
				succession one then two;
				connect a.p to b.q;
			}
		}
	`)
	conns, err := inst.OwnedConnectors(ctx)
	if err != nil {
		t.Fatalf("OwnedConnectors: %v", err)
	}
	if len(conns) != 1 {
		t.Fatalf("the object owns %d anonymous connectors, want only the `connect`", len(conns))
	}
	if len(conns[0].Ends) != 2 {
		t.Errorf("the connector has %d ends, want 2", len(conns[0].Ends))
	}
}

// A variation point belongs to the object that selected it: two objects of one
// type each selecting a variant must record their own selection, so neither
// overwrites the other's.
func TestVariantSelectionIsPerOwner(t *testing.T) {
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, `
	package test {
		port def P;
		part def Engine { port p : P; }
		abstract part family {
			variation part engine : Engine {
				variant part electric : Engine;
				variant part petrol : Engine;
			}
		}
		part sedan :> family { part :>> engine = engine::electric; }
		part coupe :> family { part :>> engine = engine::petrol; }
	}`))
	owners := map[string]*Instance{}
	for usage, want := range map[string]string{"test::sedan": "electric", "test::coupe": "petrol"} {
		inst, err := ctx.Instantiate(oneSymbol(t, idx, usage))
		if err != nil {
			t.Fatalf("%s: %v", usage, err)
		}
		if _, err := inst.GetFeatureValue(ctx, "engine"); err != nil {
			t.Fatalf("%s.engine: %v", usage, err)
		}
		owners[want] = inst
		if got := ctx.selectedVariants[variantSelection{owner: inst.ID, variation: "engine"}]; got != want {
			t.Errorf("%s selected %q, want %q", usage, got, want)
		}
	}
	// A connection of an object is governed by that object's own selection, so
	// each object resolves the variation to the variant it selected.
	conn := lower.Connection{Variation: "engine", Owner: lower.OwnerObject}
	for want, inst := range owners {
		if got := ctx.selectedVariant(conn, inst); got != want {
			t.Errorf("routing resolved engine to %q for the object selecting %q", got, want)
		}
	}
}

// A connector usage is not a part: instantiating the object it denotes must not
// go through ordinary composite materialization, which would build an object of
// each end's declared type instead of attaching the connected features.
func TestConnectorUsageIsRecognized(t *testing.T) {
	idx, model, _ := buildRuntime(t, "<test>", parseAndBuild(t, twoPortSystem))
	scope := idx.DocumentRoot("<test>")
	sys := findSymbolByName(scope, "Sys", ast.DefPart)
	if sys == nil || sys.Scope == nil {
		t.Fatal("Sys part def not found")
	}
	for name, want := range map[string]bool{"link": true, "iface": true, "a": false, "b": false} {
		var sym *symbols.Symbol
		if found, ok := sys.Scope.LookupLocal(name); ok {
			sym = found
		}
		if sym == nil {
			t.Fatalf("member %s not found", name)
		}
		if got := model.IsConnectorUsage(sym); got != want {
			t.Errorf("IsConnectorUsage(%s) = %v, want %v", name, got, want)
		}
	}
}

// Every connector-like usage that states ends with a connect clause attaches the
// same way, whatever keyword declares it: an allocation states its ends with
// `allocate` (SysML v2 §8.3.19) and a KerML `connector` with `connect`. Both
// take their implicit type from the library when they name no definition.
func TestEveryConnectorKindAttachesItsEnds(t *testing.T) {
	inst, ctx := instantiatePart(t, "Sys", `
		package test {
			port def P;
			part def A { port p : P; }
			part def B { port q : P; }
			part def Sys {
				part a : A;
				part b : B;
				allocation alloc allocate a.p to b.q;
				connector wire connect a.p to b.q;
			}
		}
	`)
	port := fvInstance(t, ctx, inst, "a", "p")
	peer := fvInstance(t, ctx, inst, "b", "q")
	for _, name := range []string{"alloc", "wire"} {
		conn := fvInstance(t, ctx, inst, name)
		if got := fvInstance(t, ctx, conn, "source"); got.ID != port.ID {
			t.Errorf("%s.source is object %d, want a.p (%d)", name, got.ID, port.ID)
		}
		if got := fvInstance(t, ctx, conn, "target"); got.ID != peer.ID {
			t.Errorf("%s.target is object %d, want b.q (%d)", name, got.ID, peer.ID)
		}
	}
}
