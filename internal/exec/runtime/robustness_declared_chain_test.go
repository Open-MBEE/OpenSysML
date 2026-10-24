package runtime

import (
	"errors"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// TestRuntimeRobustnessDeclaredChain exercises the failure modes of a chain read
// through a package-level usage no object stands for: a member whose value reads
// its own chain, two members reading each other, a member whose bound value does
// not evaluate, and a member whose count the model leaves open.
func TestRuntimeRobustnessDeclaredChain(t *testing.T) {
	t.Run("member_reading_its_own_chain_is_cyclic", testDeclaredChainSelfCycle)
	t.Run("members_reading_each_other_are_cyclic", testDeclaredChainMutualCycle)
	t.Run("member_reading_a_sibling_chain_is_not_cyclic", testDeclaredChainSiblingRead)
	t.Run("broken_member_value_reports_its_error", testDeclaredChainBrokenValue)
	t.Run("open_member_count_stays_undetermined", testDeclaredChainOpenCount)
}

// declaredChainFixture is a package-level requirement usage whose members bind
// values that read the usage back, fail to evaluate, or leave their count open.
const declaredChainFixture = `package test {
	private import ScalarValues::*;
	attribute def Details { attribute limit : Real default = 4.0; }
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
	}
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
		"req.any.limit":  "[0..*]",
		"req.some":       "[1..*]",
	} {
		val, err := ctx.EvalWithScope(parseExpr(t, src), scope)
		wantUndetermined(t, src, val, err, count)
	}
	val, err := ctx.EvalWithScope(parseExpr(t, "req.one.limit"), scope)
	if err != nil {
		t.Fatalf("req.one.limit: %v", err)
	}
	if got := FormatValue(val); got != "4.0" {
		t.Fatalf("req.one.limit = %s, want 4.0", got)
	}
}
