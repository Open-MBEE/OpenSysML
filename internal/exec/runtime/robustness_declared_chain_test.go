package runtime

import (
	"errors"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// TestRuntimeRobustnessDeclaredChain exercises the failure modes of a chain read
// through a package-level usage no object stands for: a member whose value reads
// its own chain, two members reading each other, a member whose bound value does
// not evaluate, a member whose count the model leaves open, and one whose lower
// bound of 0 nothing declared meets.
func TestRuntimeRobustnessDeclaredChain(t *testing.T) {
	t.Run("member_reading_its_own_chain_is_cyclic", testDeclaredChainSelfCycle)
	t.Run("members_reading_each_other_are_cyclic", testDeclaredChainMutualCycle)
	t.Run("member_reading_a_sibling_chain_is_not_cyclic", testDeclaredChainSiblingRead)
	t.Run("broken_member_value_reports_its_error", testDeclaredChainBrokenValue)
	t.Run("open_member_count_stays_undetermined", testDeclaredChainOpenCount)
	t.Run("optional_member_count_is_closed_over_the_model", testDeclaredChainOptionalCount)
	t.Run("namespace_usages_subsetting_each_other_report_the_cycle", testDeclaredChainNamespaceSubsetCycle)
	t.Run("optional_subsetter_keeps_its_binding_and_its_multiplicity", testDeclaredChainOptionalSubsetter)
}

// declaredChainFixture is a package-level requirement usage whose members bind
// values that read the usage back, fail to evaluate, or leave their count open.
const declaredChainFixture = `package test {
	private import ScalarValues::*;
	attribute def Details { attribute limit : Real default = 4.0; }
	action def Act { in x : Real[0..1]; }
	port def Pt { in attribute v : Real[0..1]; }
	requirement def Limits {
		attribute own : Real = test::req.own;
		attribute left : Real = test::req.right;
		attribute right : Real = test::req.left;
		attribute base : Real = 2.0;
		attribute plusOne : Real = test::req.base + 1.0;
		attribute broken : Real = 1.0 / 0.0;
		attribute some : Details[1..*];
		attribute any : Details[0..*];
		attribute one : Details;
		action act : Act;
		port pt : Pt;
	}
	part def W;
	part l1 : W[0..*];
	part l2 : W[0..*] :> l1;
	part leaf : W :> l2;
	part c1 : W[0..*] :> c2;
	part c2 : W[0..*] :> c1;
	part d1 : W[1..*] :> d2;
	part d2 : W[1..*] :> d1;
	part bp : W[2..*];
	part bc : W[0..1] :> bp;
	part bw : W;
	bind bc = bw;
	part op : W[0..*];
	part oc : W[0..1] :> op;
	part og : W[2] :> oc;
	requirement req : Limits;
}`

func declaredChainContext(t *testing.T) (*Context, *symbols.Scope) {
	t.Helper()
	model, resolver, root := parseAndBuildModel(t, declaredChainFixture)
	ctx := NewContext(typedModel(model, resolver), 10000)
	pkg, _ := root.LookupLocal("test")
	return ctx, pkg.Scope
}

func testDeclaredChainSelfCycle(t *testing.T) {
	ctx, scope := declaredChainContext(t)
	_, err := ctx.EvalWithScope(parseExpr(t, "req.own"), scope)
	if !errors.Is(err, ErrCyclicFeatureValue) {
		t.Fatalf("req.own: err = %v; want ErrCyclicFeatureValue", err)
	}
}

func testDeclaredChainMutualCycle(t *testing.T) {
	ctx, scope := declaredChainContext(t)
	_, err := ctx.EvalWithScope(parseExpr(t, "req.left"), scope)
	if !errors.Is(err, ErrCyclicFeatureValue) {
		t.Fatalf("req.left: err = %v; want ErrCyclicFeatureValue", err)
	}
}

func testDeclaredChainSiblingRead(t *testing.T) {
	ctx, scope := declaredChainContext(t)
	val, err := ctx.EvalWithScope(parseExpr(t, "req.plusOne"), scope)
	if err != nil {
		t.Fatalf("req.plusOne: %v", err)
	}
	if got := FormatValue(val); got != "3.0" {
		t.Fatalf("req.plusOne = %s, want 3.0", got)
	}
}

func testDeclaredChainBrokenValue(t *testing.T) {
	ctx, scope := declaredChainContext(t)
	_, err := ctx.EvalWithScope(parseExpr(t, "req.broken"), scope)
	if !errors.Is(err, ErrDivisionByZero) {
		t.Fatalf("req.broken: err = %v; want ErrDivisionByZero", err)
	}
}

func testDeclaredChainOpenCount(t *testing.T) {
	ctx, scope := declaredChainContext(t)
	for src, count := range map[string]string{
		"req.some.limit": "[1..*]",
		"req.some":       "[1..*]",
	} {
		val, err := ctx.EvalWithScope(parseExpr(t, src), scope)
		wantUndetermined(t, src, val, err, count)
	}
}

// A member whose lower bound is 0 and which nothing values holds nothing: the read
// is the empty sequence, not an error and not an undetermined count, and a chain
// through it is empty too. A parameter of an un-invoked action usage is an unbound
// input whatever its lower bound; a port's directed feature is not. Subsetting is transitive.
func testDeclaredChainOptionalCount(t *testing.T) {
	ctx, scope := declaredChainContext(t)
	for src, want := range map[string]string{"req.any.limit": "[]", "req.any": "[]"} {
		val, err := ctx.EvalWithScope(parseExpr(t, src), scope)
		wantFormatted(t, src, val, err, want)
	}
	val, err := ctx.EvalWithScope(parseExpr(t, "req.act.x"), scope)
	wantUndetermined(t, "req.act.x", val, err, "[0..1]")
	val, err = ctx.EvalWithScope(parseExpr(t, "req.pt.v"), scope)
	wantFormatted(t, "req.pt.v", val, err, "[]")
	val, err = ctx.EvalWithScope(parseExpr(t, "l1"), scope)
	if err != nil || len(heldObjects(val)) != 1 {
		t.Fatalf("l1 = %s, %v; want the one object leaf contributes through l2", FormatValue(val), err)
	}
	val, err = ctx.EvalWithScope(parseExpr(t, "req.one.limit"), scope)
	if err != nil {
		t.Fatalf("req.one.limit: %v", err)
	}
	if got := FormatValue(val); got != "4.0" {
		t.Fatalf("req.one.limit = %s, want 4.0", got)
	}
}

func testDeclaredChainNamespaceSubsetCycle(t *testing.T) {
	ctx, scope := declaredChainContext(t)
	for _, src := range []string{"c1", "c2", "d1", "d2"} {
		_, err := ctx.EvalWithScope(parseExpr(t, src), scope)
		if !errors.Is(err, ErrCyclicFeatureValue) {
			t.Fatalf("%s: err = %v; want ErrCyclicFeatureValue", src, err)
		}
	}
}

func testDeclaredChainOptionalSubsetter(t *testing.T) {
	ctx, scope := declaredChainContext(t)
	bw, err := ctx.EvalWithScope(parseExpr(t, "bw"), scope)
	if err != nil {
		t.Fatalf("bw: %v", err)
	}
	val, err := ctx.EvalWithScope(parseExpr(t, "bp"), scope)
	wantUndetermined(t, "bp", val, err, "[2..*]")
	if known := heldObjects(sequenceOf(knownElementsOf(val))); len(known) != 1 || known[0] != heldObjects(bw)[0] {
		t.Fatalf("bp certainly holds %v; want the one object bound to bc", known)
	}
	if _, err := ctx.EvalWithScope(parseExpr(t, "op"), scope); !errors.Is(err, ErrMultiplicityViolation) {
		t.Fatalf("op: err = %v; want ErrMultiplicityViolation from oc[0..1] holding og's two", err)
	}
}
