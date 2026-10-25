package runtime

import (
	"errors"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// TestRuntimeRobustnessPartialBinding covers a binding that does not determine an end
// whole — the connector's own `binding [n]` bounding the links included: it identifies
// some value of one end with some value of the other (KerML 1.0 §7.4.6, §8.3.4.5.2), so
// each end holds what its own declaration states and the binding is a verdict when the
// object is validated; never a panic or a hang.
func TestRuntimeRobustnessPartialBinding(t *testing.T) {
	t.Run("underdetermined end holds its own declaration", testPartialBindingUnderdeterminedEnd)
	t.Run("lower bound above the link count", testPartialBindingLowerBoundAboveLinks)
	t.Run("mutually partial bindings terminate", testPartialBindingMutual)
	t.Run("reading through the partially bound end", testPartialBindingReadThrough)
	t.Run("instantiating reports no undetermined end", testPartialBindingInstantiatesClean)
	t.Run("bound_collection_holds_its_own_declaration", testBoundCollectionHoldsItsOwnDeclaration)
	t.Run("ends_sharing_no_value_are_a_violated_verdict", testEndsSharingNoValueAreAViolatedVerdict)
	t.Run("whole_binding_of_unlike_types_is_a_typed_error", testWholeBindingOfUnlikeTypesIsATypedError)
	t.Run("verdicts_over_empty_undetermined_and_linkless_ends", testPartialBindingVerdictEdges)
}

// rigScope parses a model around `part def Rig` and returns the scope of package
// test so `rig.<feature>` expressions evaluate against the rig instance.
func rigScope(t *testing.T, rig string) (*Context, *symbols.Scope) {
	t.Helper()
	ctx, idx := libraryShapeContext(t, `package test {
		part def Thing { attribute mass : Real; }
		`+rig+`
		part rig : Rig;
	}`)
	pkg, ok := idx.DocumentRoot("<test>").LookupLocal("test")
	if !ok || pkg.Scope == nil {
		t.Fatal("test package not indexed")
	}
	return ctx, pkg.Scope
}

// `binding [1]` links one value of each end, so the end whose feature must hold
// two is not bound whole: it holds the two objects its own declaration states.
func testPartialBindingUnderdeterminedEnd(t *testing.T) {
	ctx, scope := rigScope(t, `part def Rig {
		part a { part xs : Thing [1]; }
		part ys : Thing [2];
		binding [1] bind [0..*] a.xs = [0..*] ys;
	}`)
	if val, err := evalIn(t, ctx, scope, "rig.ys"); err != nil || len(elementsOf(val)) != 2 {
		t.Fatalf("rig.ys = %s, %v; want its own two objects", FormatValue(val), err)
	}
}

// A feature declaring a lower bound above the connector's link count can never
// be linked whole: its open `[3..*]` count is undetermined, and the other end, some
// unspecified value of it, keeps its own one object.
func testPartialBindingLowerBoundAboveLinks(t *testing.T) {
	ctx, scope := rigScope(t, `part def Rig {
		part a { part xs : Thing [1]; }
		part ys3 : Thing [3..*];
		binding [1] bind [0..*] a.xs = [0..*] ys3;
	}`)
	if val, err := evalIn(t, ctx, scope, "rig.ys3"); err != nil || val.Kind != ValUndetermined {
		t.Fatalf("rig.ys3 = %s, %v; want undetermined", FormatValue(val), err)
	}
	if val, err := evalIn(t, ctx, scope, "rig.a.xs"); err != nil || val.Kind != ValInstance {
		t.Fatalf("rig.a.xs = %s, %v; want its own object", FormatValue(val), err)
	}
}

// Two partial bindings each relating the ends to one another determine neither:
// each end holds its own objects, never a hang.
func testPartialBindingMutual(t *testing.T) {
	ctx, scope := rigScope(t, `part def Rig {
		part ys : Thing [2];
		part zs : Thing [2];
		binding [1] bind [0..*] ys = [0..*] zs;
		binding [1] bind [0..*] zs = [0..*] ys;
	}`)
	for _, expr := range []string{"rig.ys", "rig.zs"} {
		if val, err := evalIn(t, ctx, scope, expr); err != nil || len(elementsOf(val)) != 2 {
			t.Fatalf("%s = %s, %v; want its own two objects", expr, FormatValue(val), err)
		}
	}
}

// Reading a member through a partially bound end reads the end's own objects: a
// `mass` no object states is undetermined, not a binding error.
func testPartialBindingReadThrough(t *testing.T) {
	ctx, scope := rigScope(t, `part def Rig {
		part a { part xs : Thing [1]; }
		part ys : Thing [2];
		binding [1] bind [0..*] a.xs = [0..*] ys;
	}`)
	if val, err := evalIn(t, ctx, scope, "rig.ys.mass"); err != nil || val.Kind != ValUndetermined {
		t.Fatalf("rig.ys.mass = %s, %v; want undetermined", FormatValue(val), err)
	}
}

// Instantiating an object whose feature is related only by `[0..1]` bindings reports
// nothing for that feature, and every other end still answers; the feature itself
// holds the one object its declaration states, not a value picked from an end.
func testPartialBindingInstantiatesClean(t *testing.T) {
	ctx, scope := rigScope(t, `part def Rig {
		part a { part x1 : Thing; part x2 : Thing; part xs : Thing [2] = (x1, x2); }
		part b { part y1 : Thing; part y2 : Thing; part ys : Thing [2] = (y1, y2); }
		part shared : Thing [1];
		binding [1] bind [0..1] a.xs = [0..1] shared;
		binding [1] bind [0..1] b.ys = [0..1] shared;
	}`)
	rig, err := evalIn(t, ctx, scope, "rig")
	if err != nil {
		t.Fatalf("rig = %v", err)
	}
	id, ok := rig.Object()
	if !ok {
		t.Fatalf("rig = %v, want an object", rig)
	}
	inst, ok := ctx.Instance(id)
	if !ok {
		t.Fatalf("object %d is not an instance", id)
	}
	errs, bounded := ctx.MaterializationErrors(inst)
	if len(errs) != 0 || bounded {
		t.Fatalf("MaterializationErrors = %v, bounded %v; want none, read in full", errs, bounded)
	}
	if v, err := evalIn(t, ctx, scope, "rig.a.xs"); err != nil || v.Sequence() == nil || len(v.Sequence().Elements()) != 2 {
		t.Fatalf("rig.a.xs = %v, %v; want two objects", v, err)
	}
	if v, err := evalIn(t, ctx, scope, "rig.shared"); err != nil || v.Kind != ValInstance {
		t.Fatalf("rig.shared = %v, %v; want its own object", v, err)
	}
}

const partialBindingMesh = `package P {
	private import ScalarValues::*;
	item def Node;
	part def Mesh {
		item nodes : Node [4];
		item left : Node [2] :> nodes;
		item right : Node [2] :> nodes;
		attribute counts : Integer [2] = (1, 2);
		binding [1] bind [0..1] left = [0..1] right;
		binding [1] bind [0..1] left = [0..1] counts;
	}
}`

// partialBindingDisjoint binds two collections that subset nothing: each holds the objects its
// own declaration makes, so the bindings link values neither end holds.
const partialBindingDisjoint = `package P {
	private import ScalarValues::*;
	item def Node;
	part def Mesh {
		item left : Node [2];
		item right : Node [2];
		attribute counts : Integer [2] = (1, 2);
		binding [1] bind [0..1] left = [0..1] right;
		binding [1] bind [0..1] left = [0..1] counts;
	}
}`

func testBoundCollectionHoldsItsOwnDeclaration(t *testing.T) {
	idx, _, ctx := buildRuntime(t, "<partial-binding-own-declaration>", parseAndBuild(t, partialBindingMesh))
	inst, err := ctx.Instantiate(oneSymbol(t, idx, "P::Mesh"))
	if err != nil {
		t.Fatalf("instantiate: %v", err)
	}
	for _, name := range []string{"left", "right"} {
		fv, err := inst.GetFeatureValue(ctx, name)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if got := len(elementsOf(fv.HeldValue())); got != 2 || fv.BindingDerived {
			t.Errorf("%s = %s (binding-derived %v), want the two objects its declaration states", name, FormatValue(fv.HeldValue()), fv.BindingDerived)
		}
	}
	fv, err := inst.GetFeatureValue(ctx, "nodes")
	if err != nil || len(elementsOf(fv.HeldValue())) != 4 {
		t.Errorf("nodes = %s, %v; want the four objects left and right hold", FormatValue(fv.HeldValue()), err)
	}
}

func testEndsSharingNoValueAreAViolatedVerdict(t *testing.T) {
	idx, _, ctx := buildRuntime(t, "<partial-binding-verdict>", parseAndBuild(t, partialBindingDisjoint))
	inst, err := ctx.Instantiate(oneSymbol(t, idx, "P::Mesh"))
	if err != nil {
		t.Fatalf("instantiate: %v", err)
	}
	report, err := ctx.ValidateObject(inst, nil)
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if len(report.Verdicts) != 2 {
		t.Fatalf("%d verdict(s), want one per binding: %+v", len(report.Verdicts), report.Verdicts)
	}
	for i, want := range []string{"left and right share 0 value(s) where the binding links 1", "left and counts share 0 value(s) where the binding links 1"} {
		v := report.Verdicts[i]
		if v.Kind != AssertionBinding || v.Status != ValidationViolated || !errors.Is(v.Err, ErrViolated) {
			t.Errorf("verdict %d = %s %s %v, want a violated binding", i+1, v.Kind, v.Status, v.Err)
		}
		if v.Err == nil || !strings.Contains(v.Err.Error(), want) {
			t.Errorf("verdict %d error %v does not name %q", i+1, v.Err, want)
		}
	}
	if report.Valid() {
		t.Error("an object violating its bindings is reported valid")
	}
}

func testWholeBindingOfUnlikeTypesIsATypedError(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<whole-binding-unlike-types>", parseAndBuild(t, `package P {
		private import ScalarValues::*;
		item def Node;
		part def Whole {
			item n : Node [1];
			attribute k : Integer = 3;
			bind n = k;
		}
		part w : Whole;
	}`))
	pkg, ok := idx.DocumentRoot("<whole-binding-unlike-types>").LookupLocal("P")
	if !ok {
		t.Fatal("package P not found")
	}
	_, err := evalIn(t, ctx, pkg.Scope, "w.n")
	if !errors.Is(err, ErrTypeMismatch) {
		t.Fatalf("n = %v, want ErrTypeMismatch", err)
	}
	if !strings.Contains(err.Error(), "cannot write 3 (an Integer) to a feature typed by Node") {
		t.Errorf("error %q does not name the unlike types", err)
	}
}

// A stated link count is a count of shared values, a repeated value counted as often as both
// ends hold it: two empty ends share none under `binding [1]`, `(1, 1, 2)` and `(1, 1, 3)` share
// two, `(1, 1, 2)` and `(1, 3, 4)` one; `binding [0]` and an end reaching no object state nothing.
func testPartialBindingVerdictEdges(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<partial-binding-verdict-edges>", parseAndBuild(t, `package P {
		private import ScalarValues::*;
		part def G { attribute item : Integer = 1; }
		part def Rig {
			attribute a : Integer [*];
			attribute b : Integer [*];
			binding [1] bind [0..1] a = [0..1] b;
			attribute z : Integer = 1;
			attribute w : Integer = 2;
			binding [0] bind z = w;
			attribute twice : Integer [3] nonunique = (1, 1, 2);
			attribute again : Integer [3] nonunique = (1, 1, 3);
			binding [2] bind [0..*] twice = [0..*] again;
			attribute once : Integer [3] nonunique = (1, 3, 4);
			binding [2] bind [0..*] twice = [0..*] once;
			part groups : G [2];
			part optional : G [0..1];
			binding [1] bind [0..*] groups.item = [0..*] optional.item;
		}
	}`))
	inst, err := ctx.Instantiate(oneSymbol(t, idx, "P::Rig"))
	if err != nil {
		t.Fatalf("instantiate: %v", err)
	}
	report, err := ctx.ValidateObject(inst, nil)
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	want := map[string]ValidationStatus{
		"binding [1] bind [0..1] a = [0..1] b":         ValidationViolated,
		"binding [2] bind [0..*] twice = [0..*] again": ValidationHolds,
		"binding [2] bind [0..*] twice = [0..*] once":  ValidationViolated,
	}
	if len(report.Verdicts) != len(want) {
		t.Fatalf("%d verdict(s), want %d: %+v", len(report.Verdicts), len(want), report.Verdicts)
	}
	for _, v := range report.Verdicts {
		status, ok := want[v.Text]
		if !ok || v.Kind != AssertionBinding || v.Status != status {
			t.Errorf("verdict %q = %s %s (%v), want %s", v.Text, v.Kind, v.Status, v.Err, status)
		}
	}
	if report.Valid() {
		t.Error("an object violating a binding is reported valid")
	}
}
