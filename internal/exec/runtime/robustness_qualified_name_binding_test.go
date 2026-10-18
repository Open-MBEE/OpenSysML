package runtime

import (
	"errors"
	"strings"
	"testing"
)

// TestRuntimeRobustnessQualifiedNameBinding exercises a declaration whose value reaches
// back to itself by qualified name: a direct self-binding, two declarations binding each
// other, a redefinition bound to the feature it redefines, the same cycles read from an
// object, and a qualified-name read that is not a cycle.
func TestRuntimeRobustnessQualifiedNameBinding(t *testing.T) {
	t.Run("declaration_bound_to_its_own_qualified_name_is_cyclic", testQualifiedNameSelfCycle)
	t.Run("declarations_bound_to_each_other_are_cyclic", testQualifiedNameMutualCycle)
	t.Run("redefinition_bound_to_its_own_qualified_name_is_cyclic", testQualifiedNameRedefinitionCycle)
	t.Run("cycle_read_through_a_chain_is_the_same_error", testQualifiedNameCycleThroughChain)
	t.Run("cycle_read_from_an_object_is_the_same_error", testQualifiedNameCycleOnObject)
	t.Run("qualified_name_read_of_a_sibling_is_not_cyclic", testQualifiedNameSiblingRead)
}

// qualifiedNameBindingFixture is a package whose usages bind values that name
// themselves, each other or a sibling by qualified name.
const qualifiedNameBindingFixture = `package test {
	private import ScalarValues::*;
	part def P { attribute a : Real; }
	part self {
		attribute own : Real = test::self::own;
		attribute left : Real = test::self::right;
		attribute right : Real = test::self::left;
		attribute base : Real = 2.0;
		attribute plusOne : Real = test::self::base + 1.0;
	}
	part redef : P {
		attribute :>> a = test::redef::a;
	}
}`

// wantQualifiedNameCycle checks that err is the cyclic feature value error naming the
// declaration the cycle closes on, within the step budget and without a panic.
func wantQualifiedNameCycle(t *testing.T, src string, err error, stated string) {
	t.Helper()
	if !errors.Is(err, ErrCyclicFeatureValue) {
		t.Fatalf("%s: err = %v; want ErrCyclicFeatureValue", src, err)
	}
	var cycle *cyclicBindingError
	if !errors.As(err, &cycle) {
		t.Fatalf("%s: err = %v; want a CyclicBindingError", src, err)
	}
	if cycle.Stated != stated {
		t.Errorf("%s: cycle stated as %q, want %q", src, cycle.Stated, stated)
	}
	if !strings.Contains(err.Error(), "cyclic feature value dependency: "+stated) {
		t.Errorf("%s: error text %q does not name the cycle", src, err)
	}
}

func testQualifiedNameSelfCycle(t *testing.T) {
	model, resolver, root := parseAndBuildModel(t, qualifiedNameBindingFixture)
	ctx := NewContext(typedModel(model, resolver), 1000)
	pkg, _ := root.LookupLocal("test")
	_, err := ctx.EvalWithScope(parseExpr(t, "test::self::own"), pkg.Scope)
	wantQualifiedNameCycle(t, "test::self::own", err, "test::self::own")
	if len(ctx.bindingStack) != 0 {
		t.Errorf("binding stack left holding %d declarations after the cycle", len(ctx.bindingStack))
	}
}

func testQualifiedNameMutualCycle(t *testing.T) {
	model, resolver, root := parseAndBuildModel(t, qualifiedNameBindingFixture)
	ctx := NewContext(typedModel(model, resolver), 1000)
	pkg, _ := root.LookupLocal("test")
	_, err := ctx.EvalWithScope(parseExpr(t, "test::self::left"), pkg.Scope)
	wantQualifiedNameCycle(t, "test::self::left", err, "test::self::left")
	_, err = ctx.EvalWithScope(parseExpr(t, "test::self::right"), pkg.Scope)
	wantQualifiedNameCycle(t, "test::self::right", err, "test::self::right")
}

func testQualifiedNameRedefinitionCycle(t *testing.T) {
	model, resolver, root := parseAndBuildModel(t, qualifiedNameBindingFixture)
	ctx := NewContext(typedModel(model, resolver), 1000)
	pkg, _ := root.LookupLocal("test")
	_, err := ctx.EvalWithScope(parseExpr(t, "test::redef::a"), pkg.Scope)
	wantQualifiedNameCycle(t, "test::redef::a", err, "test::redef::a")
}

func testQualifiedNameCycleThroughChain(t *testing.T) {
	model, resolver, root := parseAndBuildModel(t, qualifiedNameBindingFixture)
	ctx := NewContext(typedModel(model, resolver), 1000)
	pkg, _ := root.LookupLocal("test")
	_, err := ctx.EvalWithScope(parseExpr(t, "test::self.own"), pkg.Scope)
	wantQualifiedNameCycle(t, "test::self.own", err, "test::self::own")
	_, err = ctx.EvalWithScope(parseExpr(t, "redef.a"), pkg.Scope)
	wantQualifiedNameCycle(t, "redef.a", err, "test::redef::a")
}

func testQualifiedNameCycleOnObject(t *testing.T) {
	model, resolver, root := parseAndBuildModel(t, qualifiedNameBindingFixture)
	ctx := NewContext(typedModel(model, resolver), 1000)
	pkg, _ := root.LookupLocal("test")
	self, _ := pkg.Scope.LookupLocal("self")
	inst, err := ctx.Instantiate(self)
	if err != nil {
		t.Fatalf("instantiate self: %v", err)
	}
	for _, name := range []string{"own", "left", "right"} {
		_, err := inst.GetFeatureValue(ctx, name)
		wantQualifiedNameCycle(t, "self."+name, err, "test::self::"+name)
	}
	val, err := inst.GetFeatureValue(ctx, "plusOne")
	if err != nil {
		t.Fatalf("self.plusOne: %v", err)
	}
	if got := FormatValue(val.HeldValue()); got != "3.0" {
		t.Errorf("self.plusOne = %s, want 3.0", got)
	}
}

func testQualifiedNameSiblingRead(t *testing.T) {
	model, resolver, root := parseAndBuildModel(t, qualifiedNameBindingFixture)
	ctx := NewContext(typedModel(model, resolver), 1000)
	pkg, _ := root.LookupLocal("test")
	val, err := ctx.EvalWithScope(parseExpr(t, "test::self::plusOne"), pkg.Scope)
	if err != nil {
		t.Fatalf("test::self::plusOne: %v", err)
	}
	if got := FormatValue(val); got != "3.0" {
		t.Errorf("test::self::plusOne = %s, want 3.0", got)
	}
}
