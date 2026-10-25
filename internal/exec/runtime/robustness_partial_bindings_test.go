package runtime

import (
	"errors"
	"strings"
	"testing"
)

// A binding whose multiplicities link fewer values than its ends hold identifies some
// value of one end with some value of the other (KerML 1.0 §7.4.6, §8.3.4.5.2): it
// determines neither end and is checked as a verdict when the object is validated.
func TestRuntimeRobustnessPartialBindings(t *testing.T) {
	t.Run("bound_collection_holds_its_own_declaration", testBoundCollectionHoldsItsOwnDeclaration)
	t.Run("ends_sharing_no_value_are_a_violated_verdict", testEndsSharingNoValueAreAViolatedVerdict)
	t.Run("whole_binding_of_unlike_types_is_a_typed_error", testWholeBindingOfUnlikeTypesIsATypedError)
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
	idx, _, ctx := buildRuntime(t, "<partial-binding-verdict>", parseAndBuild(t, partialBindingMesh))
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
