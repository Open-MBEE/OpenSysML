package semantics

import (
	"math/big"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// An expression owns one `in` parameter per operand, each binding its operand
// through a FeatureValue, and the `out` result parameter when its evaluation
// yields a value (KerML 8.4.4.9), as the conversion writes them.
func TestExpressionOwnsItsParametersAndResult(t *testing.T) {
	m, root := buildModelWithStdlib(t, `
		package P {
			part def D {
				attribute a : ScalarValues::Real;
				attribute b : ScalarValues::Real;
				attribute x = a + b * 2;
				attribute y = a as ScalarValues::Integer;
				attribute z = 1;
			}
		}
	`)
	pkg, _ := root.LookupLocal("P")
	d := member(t, m, pkg, "D")
	names := func(syms []*symbols.Symbol) []string {
		var out []string
		for _, sym := range syms {
			if mc := m.MetaclassOf(sym); mc != nil {
				out = append(out, mc.Name)
			} else {
				out = append(out, "?")
			}
		}
		return out
	}
	equal := func(got, want []string) bool {
		if len(got) != len(want) {
			return false
		}
		for i := range got {
			if got[i] != want[i] {
				return false
			}
		}
		return true
	}

	x := m.ValueExpression(member(t, m, d, "x"))
	if x == nil {
		t.Fatal("x: no value expression")
	}
	params := m.ExpressionParameters(x)
	if got := names(params); !equal(got, []string{"Feature", "Feature"}) {
		t.Fatalf("x parameters: %v", got)
	}
	if again := m.ExpressionParameters(x); len(again) != 2 || again[0] != params[0] || again[1] != params[1] {
		t.Error("x parameters are not stable across reads")
	}
	result := m.ExpressionResult(x)
	if result == nil || m.MetaclassOf(result).Name != "Feature" {
		t.Fatal("x: no result parameter")
	}
	for i, want := range []ast.FeatureDirection{ast.DirIn, ast.DirIn} {
		if got, ok := ReflectiveDirection(params[i]); !ok || got != want {
			t.Errorf("in%d direction: %v %v", i, got, ok)
		}
	}
	if got, ok := ReflectiveDirection(result); !ok || got != ast.DirOut {
		t.Errorf("out direction: %v %v", got, ok)
	}
	if got, ok := ReflectiveDirection(x); !ok || got != ast.DirNone {
		t.Errorf("expression direction: %v %v", got, ok)
	}
	if got := names(m.OwnedMemberships(x)); !equal(got, []string{"ParameterMembership", "ParameterMembership", "ReturnParameterMembership"}) {
		t.Errorf("x owned memberships: %v", got)
	}
	for feature, want := range map[string][]*symbols.Symbol{
		"input":           params,
		"parameter":       {params[0], params[1], result},
		"ownedFeature":    {params[0], params[1], result},
		"directedFeature": {params[0], params[1], result},
		"ownedElement":    {params[0], params[1], result},
		"output":          {result},
		"result":          {result},
	} {
		got, ok := m.ReflectiveElements(x, feature)
		if !ok || len(got) != len(want) {
			t.Errorf("x.%s: %v (%v), want %d", feature, names(got), ok, len(want))
			continue
		}
		for i := range want {
			if got[i] != want[i] {
				t.Errorf("x.%s[%d] is not the synthesized part", feature, i)
			}
		}
	}
	args := m.ExpressionArguments(x)
	if got := names(args); !equal(got, []string{"FeatureReferenceExpression", "OperatorExpression"}) {
		t.Fatalf("x arguments: %v", got)
	}
	if got, ok := m.ReflectiveElements(x, "argument"); !ok || len(got) != 2 || got[0] != args[0] || got[1] != args[1] {
		t.Error("x.argument is not the operands")
	}
	for i := range args {
		if got, ok := m.ReflectiveElements(args[i], "owner"); !ok || len(got) != 1 || got[0] != params[i] {
			t.Errorf("a%d.owner is not in%d", i, i)
		}
		if got := m.ValueExpression(params[i]); got != args[i] {
			t.Errorf("in%d binds another value", i)
		}
		fv := m.OwnedMemberships(params[i])
		if got := names(fv); !equal(got, []string{"FeatureValue"}) {
			t.Errorf("in%d owned memberships: %v", i, got)
			continue
		}
		if got, ok := m.ReflectiveElements(fv[0], "value"); !ok || len(got) != 1 || got[0] != args[i] {
			t.Errorf("in%d FeatureValue.value is not a%d", i, i)
		}
		if got, ok := m.ReflectiveElements(fv[0], "featureWithValue"); !ok || len(got) != 1 || got[0] != params[i] {
			t.Errorf("in%d FeatureValue.featureWithValue is not in%d", i, i)
		}
		if got, ok := m.ReflectiveElements(args[i], "owningMembership"); !ok || len(got) != 1 || got[0] != fv[0] {
			t.Errorf("a%d.owningMembership is not its FeatureValue", i)
		}
	}
	if got, ok := m.ReflectiveElements(params[0], "owner"); !ok || len(got) != 1 || got[0] != x {
		t.Error("in0.owner is not the expression")
	}
	if got, ok := m.ReflectiveElements(result, "owner"); !ok || len(got) != 1 || got[0] != x {
		t.Error("out.owner is not the expression")
	}
	// The nested operand `b * 2` owns parts of its own; its literal owns none.
	nested := args[1]
	if got := names(m.ExpressionParameters(nested)); !equal(got, []string{"Feature", "Feature"}) {
		t.Errorf("b*2 parameters: %v", got)
	}
	if m.ExpressionResult(nested) == nil {
		t.Error("b*2: no result parameter")
	}
	literal := m.ExpressionArguments(nested)[1]
	if m.MetaclassOf(literal).Name != "LiteralInteger" || len(m.ExpressionParameters(literal)) != 0 ||
		m.ExpressionResult(literal) != nil || len(m.OwnedMemberships(literal)) != 0 {
		t.Error("the literal 2 owns parts")
	}
	if got, ok := m.ReflectiveElements(literal, "ownedElement"); !ok || len(got) != 0 {
		t.Errorf("literal ownedElement: %v", names(got))
	}

	// `a as T` owns a parameter for its operand and one for the type
	// argument, unbound, after it; and its result.
	y := m.ValueExpression(member(t, m, d, "y"))
	yParams := m.ExpressionParameters(y)
	if got := names(yParams); !equal(got, []string{"Feature", "Feature"}) {
		t.Fatalf("y parameters: %v", got)
	}
	if got := names(m.ExpressionArguments(y)); !equal(got, []string{"FeatureReferenceExpression"}) {
		t.Errorf("y arguments: %v", got)
	}
	if yParams[1].Expr.Index != 1 || m.ValueExpression(yParams[1]) != nil || len(m.OwnedMemberships(yParams[1])) != 0 {
		t.Error("y's type argument parameter binds a value")
	}
	if got := names(m.OwnedMemberships(y)); !equal(got, []string{"ParameterMembership", "ParameterMembership", "ReturnParameterMembership"}) {
		t.Errorf("y owned memberships: %v", got)
	}

	// A literal value owns nothing.
	z := m.ValueExpression(member(t, m, d, "z"))
	if len(m.ExpressionParameters(z)) != 0 || m.ExpressionResult(z) != nil || len(m.OwnedMemberships(z)) != 0 {
		t.Error("the literal value of z owns parts")
	}
}

// A real literal's value is the exact rational its notation states, exponent
// notation included (KerML LiteralRational::value), as the evaluator reads it.
func TestLiteralRealValueReadsExponentNotation(t *testing.T) {
	m, root := buildModelWithStdlib(t, `
		package P {
			part def D {
				attribute e = 1e3;
				attribute f = 2.5E-1;
				attribute g = 0.75;
			}
		}
	`)
	pkg, _ := root.LookupLocal("P")
	d := member(t, m, pkg, "D")
	for name, want := range map[string]*big.Rat{"e": big.NewRat(1000, 1), "f": big.NewRat(1, 4), "g": big.NewRat(3, 4)} {
		expr := m.ValueExpression(member(t, m, d, name))
		if expr == nil || m.MetaclassOf(expr).Name != "LiteralRational" {
			t.Fatalf("%s: value expression %v", name, expr)
		}
		got, ok := m.reflectiveFeatureValue(expr, "value")
		if !ok || got.Kind != symbols.FilterValueRational || got.Rat.Cmp(want) != 0 {
			t.Errorf("%s.value = %v (%v), want %v", name, got, ok, want)
		}
	}
}

// Feature::valuation is the feature's FeatureValue; a feature without a value
// part has none. A parameter shares its expression's node but states none of
// the expression's own values.
func TestFeatureValuationAndParameterValues(t *testing.T) {
	m, root := buildModelWithStdlib(t, `
		package P {
			part def D {
				attribute a : ScalarValues::Real;
				attribute x = a + 2;
			}
		}
	`)
	pkg, _ := root.LookupLocal("P")
	d := member(t, m, pkg, "D")
	x, a := member(t, m, d, "x"), member(t, m, d, "a")
	valuation, ok := m.ReflectiveElements(x, "valuation")
	if !ok || len(valuation) != 1 || m.MetaclassOf(valuation[0]).Name != "FeatureValue" || valuation[0] != m.OwnedMemberships(x)[0] {
		t.Errorf("x.valuation = %v (%v), want x's FeatureValue", valuation, ok)
	}
	if none, ok := m.ReflectiveElements(a, "valuation"); !ok || len(none) != 0 {
		t.Errorf("a.valuation = %v (%v), want none", none, ok)
	}
	if _, ok := m.ReflectiveElements(d, "valuation"); ok {
		t.Error("a part definition derives valuation")
	}
	expr := m.ValueExpression(x)
	if got, ok := m.reflectiveFeatureValue(expr, "operator"); !ok || got.Str != "+" {
		t.Errorf("x.value.operator = %v (%v)", got, ok)
	}
	for _, part := range append(m.ExpressionParameters(expr), m.ExpressionResult(expr)) {
		if got, ok := m.reflectiveFeatureValue(part, "operator"); ok {
			t.Errorf("a parameter of x's expression states operator %v", got)
		}
		if got, ok := m.reflectiveFeatureValue(part, "value"); ok {
			t.Errorf("a parameter of x's expression states value %v", got)
		}
	}
}

// A valued symbol without a body scope still owns its FeatureValue.
func TestScopelessValuedSymbolOwnsItsFeatureValue(t *testing.T) {
	m, root := buildModelWithStdlib(t, `package P { part def D { attribute x = 1; } }`)
	pkg, _ := root.LookupLocal("P")
	x := member(t, m, member(t, m, pkg, "D"), "x")
	scopeless := *x
	scopeless.Scope = nil
	memberships := m.OwnedMemberships(&scopeless)
	if len(memberships) != 1 || m.MetaclassOf(memberships[0]).Name != "FeatureValue" {
		t.Fatalf("OwnedMemberships(scope-less x) = %d, want the FeatureValue", len(memberships))
	}
	if m.ValueExpression(&scopeless) != memberships[0].Implicit.Member {
		t.Error("the FeatureValue owns another expression")
	}
}

// An OperatorExpression's `operator` reads as the conversion writes it for
// every notation form: a sequence's `,`, a cast's `as`, an index's `[` or `#`.
func TestOperatorExpressionOperatorForEveryNotationForm(t *testing.T) {
	m, root := buildModelWithStdlib(t, `
		package P {
			part def D {
				attribute items : ScalarValues::Real[*];
				attribute seq = (1, 2);
				attribute cast = items as ScalarValues::Integer;
				attribute classified = (as ScalarValues::Integer);
				attribute bracketed = items[1];
				attribute hashed = items#(1);
				attribute sum = 1 + 2;
			}
		}
	`)
	pkg, _ := root.LookupLocal("P")
	d := member(t, m, pkg, "D")
	for name, want := range map[string]string{"seq": ",", "cast": "as", "classified": "as", "bracketed": "[", "hashed": "#", "sum": "+"} {
		expr := m.ValueExpression(member(t, m, d, name))
		if expr == nil {
			t.Fatalf("%s: no value expression", name)
		}
		got, ok := m.reflectiveFeatureValue(expr, "operator")
		if !ok || got.Kind != symbols.FilterValueString || got.Str != want {
			t.Errorf("%s.operator = %q (%v), want %q", name, got.Str, ok, want)
		}
	}
}

// An invocation or constructor owns one `in` parameter per argument (receiver,
// positional, named), each binding it through a FeatureValue, and its result.
func TestInvocationOwnsItsArgumentParameters(t *testing.T) {
	m, root := buildModelWithStdlib(t, `
		package P {
			calc def F { in a : ScalarValues::Real; in b : ScalarValues::Real; return r : ScalarValues::Real = a + b; }
			part def Pt { attribute px : ScalarValues::Real; attribute py : ScalarValues::Real; }
			part def D {
				attribute items : ScalarValues::Real[*];
				attribute positional = F(1, 2);
				attribute named = F(a = 1, b = 2);
				attribute arrow = items->F(3);
				attribute built = new Pt(1, 2);
			}
		}
	`)
	pkg, _ := root.LookupLocal("P")
	d := member(t, m, pkg, "D")
	cases := []struct {
		name      string
		metaclass string
		args      []string
	}{
		{"positional", "InvocationExpression", []string{"LiteralInteger", "LiteralInteger"}},
		{"named", "InvocationExpression", []string{"LiteralInteger", "LiteralInteger"}},
		{"arrow", "InvocationExpression", []string{"FeatureReferenceExpression", "LiteralInteger"}},
		{"built", "ConstructorExpression", []string{"LiteralInteger", "LiteralInteger"}},
	}
	for _, c := range cases {
		expr := m.ValueExpression(member(t, m, d, c.name))
		if expr == nil || m.MetaclassOf(expr).Name != c.metaclass {
			t.Fatalf("%s: value expression %v", c.name, expr)
		}
		params, args := m.ExpressionParameters(expr), m.ExpressionArguments(expr)
		if len(params) != len(c.args) || len(args) != len(c.args) {
			t.Errorf("%s: %d parameters, %d arguments, want %d", c.name, len(params), len(args), len(c.args))
			continue
		}
		for i, want := range c.args {
			if got := m.MetaclassOf(args[i]).Name; got != want {
				t.Errorf("%s argument %d is %s, want %s", c.name, i, got, want)
			}
			if params[i].Expr.Index != i || m.ValueExpression(params[i]) != args[i] {
				t.Errorf("%s: in%d does not bind argument %d", c.name, params[i].Expr.Index, i)
			}
			if fv := m.OwnedMemberships(params[i]); len(fv) != 1 || m.MetaclassOf(fv[0]).Name != "FeatureValue" {
				t.Errorf("%s: in%d owns %d memberships", c.name, i, len(fv))
			}
		}
		if m.ExpressionResult(expr) == nil {
			t.Errorf("%s: no result parameter", c.name)
		}
		// The Membership to the function, the parameters, the result.
		if got := len(m.OwnedMemberships(expr)); got != len(c.args)+2 {
			t.Errorf("%s owns %d memberships, want %d", c.name, got, len(c.args)+2)
		}
	}
}

// An expression owns a plain Membership to the element it names — a reference's
// referent, an invocation's function, a chain's target — as the conversion writes it.
func TestExpressionOwnsTheMembershipToWhatItNames(t *testing.T) {
	m, root := buildModelWithStdlib(t, `
		package P {
			calc def F { in a : ScalarValues::Real; return r : ScalarValues::Real = a; }
			part def Pt { attribute q : ScalarValues::Real; }
			part def D {
				attribute a : ScalarValues::Real;
				part p : Pt;
				attribute ref = a;
				attribute call = F(1);
				attribute chain = p.q;
				attribute dangling = nothere;
				attribute meta = Pt.metadata;
			}
		}
	`)
	pkg, _ := root.LookupLocal("P")
	d := member(t, m, pkg, "D")
	f, pt := member(t, m, pkg, "F"), member(t, m, pkg, "Pt")
	a, q := member(t, m, d, "a"), member(t, m, pt, "q")
	names := func(syms []*symbols.Symbol) []string {
		var out []string
		for _, sym := range syms {
			out = append(out, m.MetaclassOf(sym).Name)
		}
		return out
	}
	for _, c := range []struct {
		name, slot string
		target     *symbols.Symbol
		order      []string
	}{
		{"ref", "referent", a, []string{"Membership", "ReturnParameterMembership"}},
		{"call", "function", f, []string{"Membership", "ParameterMembership", "ReturnParameterMembership"}},
		{"chain", "targetFeature", q, []string{"ParameterMembership", "Membership", "ReturnParameterMembership"}},
	} {
		expr := m.ValueExpression(member(t, m, d, c.name))
		memberships := m.OwnedMemberships(expr)
		got := names(memberships)
		if len(got) != len(c.order) {
			t.Errorf("%s owns %v, want %v", c.name, got, c.order)
			continue
		}
		for i := range c.order {
			if got[i] != c.order[i] {
				t.Errorf("%s owns %v, want %v", c.name, got, c.order)
				break
			}
			if memberships[i].Implicit.Ordinal != i {
				t.Errorf("%s membership %d has ordinal %d", c.name, i, memberships[i].Implicit.Ordinal)
			}
		}
		elements, ok := m.ReflectiveElements(expr, c.slot)
		if !ok || len(elements) != 1 || elements[0] != c.target {
			t.Errorf("%s.%s = %v (%v), want the named element", c.name, c.slot, names(elements), ok)
		}
		for _, membership := range memberships {
			rel := membership.Implicit
			if rel.Form != symbols.ImplicitReferenceMembership {
				continue
			}
			if rel.Member != c.target || rel.Slot != c.slot {
				t.Errorf("%s: the Membership names %v at %q", c.name, rel.Member, rel.Slot)
			}
			if got, ok := m.ReflectiveElements(membership, "memberElement"); !ok || len(got) != 1 || got[0] != c.target {
				t.Errorf("%s: Membership.memberElement = %v", c.name, names(got))
			}
			if got, ok := m.ReflectiveElements(membership, "owner"); !ok || len(got) != 1 || got[0] != expr {
				t.Errorf("%s: Membership.owner is not the expression", c.name)
			}
			if _, ok := m.ReflectiveElements(membership, "ownedMemberElement"); ok {
				t.Errorf("%s: a plain Membership derives ownedMemberElement", c.name)
			}
			if got, ok := m.ReflectiveElements(membership, "ownedRelatedElement"); !ok || len(got) != 0 {
				t.Errorf("%s: Membership.ownedRelatedElement = %v (%v), want none", c.name, names(got), ok)
			}
			if got, ok := m.reflectiveFeatureValue(membership, "visibility"); !ok || !strings.HasSuffix(got.RefFQN, "::public") {
				t.Errorf("%s: Membership.visibility = %v (%v), want the public literal", c.name, got, ok)
			}
			if got, ok := m.reflectiveFeatureValue(membership, "memberName"); !ok || got.Str != c.target.Name {
				t.Errorf("%s: Membership.memberName = %v (%v)", c.name, got, ok)
			}
		}
		if again := m.OwnedMemberships(expr); len(again) != len(memberships) || again[0] != memberships[0] {
			t.Errorf("%s: memberships are not stable across reads", c.name)
		}
	}
	meta := m.ValueExpression(member(t, m, d, "meta"))
	if got := names(m.OwnedMemberships(meta)); len(got) != 1 || got[0] != "Membership" {
		t.Errorf("a metadata access owns %v, want its Membership", got)
	}
	if got, ok := m.ReflectiveElements(meta, "referencedElement"); !ok || len(got) != 1 || got[0] != pt {
		t.Errorf("metadata access referencedElement = %v (%v), want Pt", names(got), ok)
	}
	if _, ok := m.ReflectiveElements(meta, "function"); ok {
		t.Error("a metadata access derives a function")
	}
	for _, feature := range []string{"owningRelationship", "ownedAnnotation", "ownedMembership"} {
		if got, ok := m.ReflectiveElements(m.OwnedMemberships(meta)[0], feature); !ok || len(got) != 0 {
			t.Errorf("Membership.%s = %v (%v), want empty", feature, names(got), ok)
		}
	}
	dangling := m.ValueExpression(member(t, m, d, "dangling"))
	if got := names(m.OwnedMemberships(dangling)); len(got) != 1 || got[0] != "ReturnParameterMembership" {
		t.Errorf("an unresolved reference owns %v, want its result only", got)
	}
	if got, ok := m.ReflectiveElements(dangling, "referent"); !ok || len(got) != 0 {
		t.Errorf("an unresolved reference's referent = %v (%v)", names(got), ok)
	}
}

// A change trigger (`accept when a > 1`) binds its `in` parameter to a
// FeatureReferenceExpression whose referent is the written condition, as the
// conversion writes it: the wrapper owns the condition through a
// FeatureMembership and its own result; the condition keeps its own parts.
func TestChangeTriggerOwnsItsConditionThroughAWrapper(t *testing.T) {
	m, root := buildModelWithStdlib(t, `
		package P {
			action def A {
				attribute a : ScalarValues::Real;
				action acc accept e when a > 1;
			}
		}
	`)
	var payload *symbols.Symbol
	var walk func(scope *symbols.Scope)
	walk = func(scope *symbols.Scope) {
		visit := func(sym *symbols.Symbol) bool {
			if usage, ok := sym.Decl.(*ast.Usage); ok {
				if _, ok := usage.Value.(*ast.ChangeEvent); ok {
					payload = sym
				}
			}
			return true
		}
		scope.ForEachMember(visit)
		scope.ForEachAnonymousMember(visit)
		for _, child := range scope.Children() {
			walk(child)
		}
	}
	walk(root)
	if payload == nil {
		t.Fatal("no payload declares a change trigger")
	}
	trigger := m.ValueExpression(payload)
	if trigger == nil || m.MetaclassOf(trigger).Name != "TriggerInvocationExpression" {
		t.Fatalf("payload value = %v", trigger)
	}
	params := m.ExpressionParameters(trigger)
	args := m.ExpressionArguments(trigger)
	if len(params) != 1 || len(args) != 1 || m.MetaclassOf(args[0]).Name != "FeatureReferenceExpression" {
		t.Fatalf("trigger binds %d params, %d args (%v)", len(params), len(args), args)
	}
	if result := m.ExpressionResult(trigger); result == nil || m.ownerOf(result) != trigger {
		t.Errorf("the trigger's result = %v", result)
	}
	if owned := m.OwnedMemberships(trigger); len(owned) != 2 || m.MetaclassOf(owned[1]).Name != "ReturnParameterMembership" {
		t.Errorf("the trigger owns %v", owned)
	}
	wrapper := args[0]
	if m.ValueExpression(params[0]) != wrapper || m.ownerOf(wrapper) != params[0] {
		t.Errorf("in0 does not bind the wrapper")
	}
	memberships := m.OwnedMemberships(wrapper)
	if len(memberships) != 2 || m.MetaclassOf(memberships[0]).Name != "FeatureMembership" ||
		m.MetaclassOf(memberships[1]).Name != "ReturnParameterMembership" {
		t.Fatalf("wrapper owns %v", memberships)
	}
	condition := memberships[0].Implicit.Member
	if m.MetaclassOf(condition).Name != "OperatorExpression" || m.ownerOf(condition) != wrapper {
		t.Errorf("condition = %s owned by %v", m.MetaclassOf(condition).Name, m.ownerOf(condition))
	}
	if referent, _ := m.ReflectiveElements(wrapper, "referent"); len(referent) != 1 || referent[0] != condition {
		t.Errorf("wrapper referent = %v", referent)
	}
	if got, ok := m.reflectiveFeatureValue(wrapper, "operator"); ok {
		t.Errorf("wrapper answers operator %v", got)
	}
	for _, feature := range []string{"ownedElement", "ownedFeature"} {
		if owned, _ := m.ReflectiveElements(wrapper, feature); len(owned) != 2 || owned[0] != condition || owned[1] != memberships[1].Implicit.Member {
			t.Errorf("wrapper %s = %v, want the condition then the result", feature, owned)
		}
	}
	for _, feature := range []string{"parameter", "directedFeature"} {
		if owned, _ := m.ReflectiveElements(wrapper, feature); len(owned) != 1 || owned[0] != memberships[1].Implicit.Member {
			t.Errorf("wrapper %s = %v, want the result alone", feature, owned)
		}
	}
	if value, ok := m.reflectiveFeatureValue(condition, "operator"); !ok || value.Str != ">" {
		t.Errorf("condition operator = %v, %v", value, ok)
	}
	if len(m.ExpressionParameters(condition)) != 2 {
		t.Errorf("condition binds %d params", len(m.ExpressionParameters(condition)))
	}
	if again := m.ExpressionArguments(trigger); again[0] != wrapper || m.OwnedMemberships(wrapper)[0].Implicit.Member != condition {
		t.Error("a second read synthesizes different identities")
	}
}
