package solve

import (
	"errors"
	"strings"
	"testing"
)

// constraintQuery translates the named constraint of a source, failing the test
// when the translation refuses.
func constraintQuery(t *testing.T, src, name string) *Query {
	t.Helper()
	ctx, idx := fixture(t, "<test>", src)
	sym := symbolNamed(t, idx, name)
	q, err := Constraint(ctx, sym, sym.OwnerScope)
	if err != nil {
		t.Fatalf("translate %s: %v", name, err)
	}
	return q
}

// refusal translates the named constraint expecting a refusal, and returns it.
func refusal(t *testing.T, src, name string) *NotTranslatableError {
	t.Helper()
	ctx, idx := fixture(t, "<test>", src)
	sym := symbolNamed(t, idx, name)
	q, err := Constraint(ctx, sym, sym.OwnerScope)
	if err == nil {
		t.Fatalf("translating %s succeeded, want a refusal; script:\n%s", name, Script(q))
	}
	if q != nil {
		t.Errorf("a refused translation returned a query: %+v", q)
	}
	if !errors.Is(err, ErrNotTranslatable) {
		t.Fatalf("translating %s: %v, want ErrNotTranslatable", name, err)
	}
	var refused *NotTranslatableError
	if !errors.As(err, &refused) {
		t.Fatalf("translating %s: %v is not a *NotTranslatableError", name, err)
	}
	return refused
}

// constraintSource wraps a constraint body in a package importing the scalar
// value types, which is what most of these cases need.
func constraintSource(body string) string {
	return "package test {\n\tprivate import ScalarValues::*;\n\tconstraint def C {\n" + body + "\n\t}\n}\n"
}

// TestSorts: a feature's sort comes from its declared type, not from the shape of
// the condition reading it.
func TestSorts(t *testing.T) {
	q := constraintQuery(t, constraintSource(`
		in b : Boolean;
		in n : Natural;
		in i : Integer;
		in r : Real;
		in q : Rational;
		in s : String;
		assert constraint { b and n <= i and r >= q and s == "x" }
	`), "test::C")

	want := map[string]Sort{
		"test::C::b": Bool,
		"test::C::n": Int,
		"test::C::i": Int,
		"test::C::r": Real,
		"test::C::q": Real,
		"test::C::s": String,
	}
	got := map[string]Sort{}
	for _, v := range q.Vars {
		got[v.Name] = v.Sort
	}
	for name, sort := range want {
		if !got[name].Equal(sort) {
			t.Errorf("%s declared as %s, want %s", name, got[name].Name, sort.Name)
		}
	}
	if len(got) != len(want) {
		t.Errorf("declared %d variables, want %d: %v", len(got), len(want), got)
	}
}

// TestNaturalIsNotNegative: a Natural's declaration bounds its values, which the
// conditions do not say and a solver would otherwise ignore.
func TestNaturalIsNotNegative(t *testing.T) {
	q := constraintQuery(t, constraintSource(`
		in n : Natural;
		assert constraint { n <= 3 }
	`), "test::C")
	first := q.Assertions[0]
	if first.From.Role != RoleDomain {
		t.Fatalf("first assertion is %s, want a declared domain: %s", first.From.Role, Script(q))
	}
	if got := writeTerm(first.Term); got != "(>= |test::C::n| 0)" {
		t.Errorf("domain assertion is %s", got)
	}
}

// TestQuantityNormalization: magnitudes are scaled to the base units of their
// dimension, exactly — `5.4 [km/h]` is `1.5` metres per second, not 1.4999….
func TestQuantityNormalization(t *testing.T) {
	q := constraintQuery(t, `
		package test {
			public import SI::*;
			constraint def C {
				in speed : ISQSpaceTime::SpeedValue;
				assert constraint { speed <= 5.4 [km/h] }
			}
		}
	`, "test::C")
	script := Script(q)
	if !strings.Contains(script, "1.5") {
		t.Errorf("5.4 [km/h] was not normalized to 1.5 m/s:\n%s", script)
	}
}

// TestIncommensurableUnitsRefuse: comparing magnitudes of different dimensions is
// refused at translation, as the evaluator reports it as incommensurable.
func TestIncommensurableUnitsRefuse(t *testing.T) {
	refused := refusal(t, `
		package test {
			public import SI::*;
			constraint def C {
				in speed : ISQSpaceTime::SpeedValue;
				assert constraint { speed <= 5.4 [kg] }
			}
		}
	`, "test::C")
	if !strings.Contains(refused.Reason, "incommensurable") {
		t.Errorf("refusal reason is %q, want it to name incommensurable units", refused.Reason)
	}
}

// TestQuantityScaleKeepsVariableComparable: a variable's magnitude is in the base
// units its dimension reduces to, which is what a converted literal is scaled to
// — a mass reduces to grams here, so `2.5 [kg]` is 2500.
func TestQuantityScaleKeepsVariableComparable(t *testing.T) {
	q := constraintQuery(t, `
		package test {
			public import SI::*;
			constraint def C {
				in mass : ISQ::MassValue;
				assert constraint { mass <= 2.5 [kg] }
			}
		}
	`, "test::C")
	if got := writeTerm(q.Assertions[len(q.Assertions)-1].Term); got != "(<= |test::C::mass| 2500.0)" {
		t.Errorf("assertion is %s, want the literal in base units", got)
	}
	var mass *Var
	for _, v := range q.Vars {
		if v.Name == "test::C::mass" {
			mass = v
		}
	}
	if mass == nil || mass.Dimension == "" {
		t.Fatalf("the variable records no dimension: %+v", mass)
	}
	if mass.Unit != "gram" {
		t.Errorf("the variable is expressed in %q, want the base unit its magnitudes are scaled to", mass.Unit)
	}
}

// TestRolesAndOrder: assumptions stay assumptions, required conditions stay
// required, and both keep the order the evaluator checks them in.
func TestRolesAndOrder(t *testing.T) {
	ctx, idx := fixtureFile(t, "touchdown.sysml")
	sym := symbolNamed(t, idx, "test::TouchdownRequirement")
	q, err := Requirement(ctx, sym, sym.OwnerScope)
	if err != nil {
		t.Fatalf("translate: %v", err)
	}
	var roles []Role
	for _, a := range q.Assertions {
		roles = append(roles, a.From.Role)
	}
	want := []Role{RoleAssumed, RoleRequired}
	if len(roles) != len(want) {
		t.Fatalf("roles are %v, want %v:\n%s", roles, want, Script(q))
	}
	for i, role := range want {
		if roles[i] != role {
			t.Errorf("assertion %d is %s, want %s", i, roles[i], role)
		}
	}
}

// TestProvenanceNamesWhatWasWritten: every assertion says which condition,
// element and source position it came from, which is what maps an answer back.
func TestProvenanceNamesWhatWasWritten(t *testing.T) {
	ctx, idx := fixtureFile(t, "touchdown.sysml")
	sym := symbolNamed(t, idx, "test::TouchdownRequirement")
	q, err := Requirement(ctx, sym, sym.OwnerScope)
	if err != nil {
		t.Fatalf("translate: %v", err)
	}
	for _, a := range q.Assertions {
		if a.From.Element != "TouchdownRequirement" {
			t.Errorf("assertion names element %q", a.From.Element)
		}
		if a.From.Condition == "" {
			t.Errorf("assertion records no condition text: %+v", a.From)
		}
		if !strings.HasPrefix(a.From.Location, "touchdown.sysml:") {
			t.Errorf("assertion records location %q", a.From.Location)
		}
		if a.From.Declared == nil {
			t.Errorf("assertion records no declaring element: %+v", a.From)
		}
	}
}

// TestInheritedConditionsAreTranslated: a usage's conditions include the ones its
// definition states, in the same order the evaluator checks them.
func TestInheritedConditionsAreTranslated(t *testing.T) {
	q := constraintQuery(t, `
		package test {
			private import ScalarValues::Integer;
			constraint def Base {
				in level : Integer;
				assert constraint { level >= 0 }
			}
			constraint def C :> Base {
				assert constraint { level <= 10 }
			}
		}
	`, "test::C")
	if len(q.Assertions) != 2 {
		t.Fatalf("translated %d assertions, want the inherited one too:\n%s", len(q.Assertions), Script(q))
	}
	if got := writeTerm(q.Assertions[0].Term); got != "(>= |test::Base::level| 0)" {
		t.Errorf("first assertion is %s, want the inherited condition first", got)
	}
}

// TestNegatedGroupNegatesTheConjunction: a negated body denies its conditions
// together — `not (a and b)`, not `not a and not b` (De Morgan).
func TestNegatedGroupNegatesTheConjunction(t *testing.T) {
	q := constraintQuery(t, constraintSource(`
		in level : Integer;
		assert not constraint {
			level > 10
			level < 20
		}
	`), "test::C")
	want := "(not (and (> |test::C::level| 10) (< |test::C::level| 20)))"
	if got := writeTerm(q.Assertions[0].Term); got != want {
		t.Errorf("assertion is %s, want %s", got, want)
	}
}

// TestNegatedSingleConditionIsInverted: a negated body of one condition inverts
// that condition, as a conjunction of one is that one.
func TestNegatedSingleConditionIsInverted(t *testing.T) {
	q := constraintQuery(t, constraintSource(`
		in level : Integer;
		assert not constraint { level > 10 }
	`), "test::C")
	want := "(not (> |test::C::level| 10))"
	if got := writeTerm(q.Assertions[0].Term); got != want {
		t.Errorf("assertion is %s, want %s", got, want)
	}
}

// TestNegatedElementDeniesItsRequiredConditions: `assert not constraint c : C`
// holds when one required condition fails, so the query denies the conjunction.
func TestNegatedElementDeniesItsRequiredConditions(t *testing.T) {
	ctx, idx := fixtureFile(t, "safe_window.sysml")
	sym := symbolNamed(t, idx, "test::rig::safeWindow")
	q, err := Constraint(ctx, sym, sym.OwnerScope)
	if err != nil {
		t.Fatalf("translate: %v", err)
	}
	if !q.Negated {
		t.Error("the query does not record that the element denies its conditions")
	}
	if len(q.Assertions) != 1 || q.Assertions[0].From.Role != RoleDenied {
		t.Fatalf("a negated element asserted %d terms:\n%s", len(q.Assertions), Script(q))
	}
	want := "(not (and (>= |test::SafeWindow::level| 0) (not (and (> |test::SafeWindow::level| 10) (< |test::SafeWindow::level| 20)))))"
	if got := writeTerm(q.Assertions[0].Term); got != want {
		t.Errorf("denial is %s, want %s", got, want)
	}
}

// TestNegatedElementWithOnlyAssumptions: an assumption is trusted rather than
// required, so a negated element stating only assumptions denies nothing.
func TestNegatedElementWithOnlyAssumptions(t *testing.T) {
	ctx, idx := fixture(t, "<test>", `
		package test {
			private import ScalarValues::Integer;
			part def Rig {
				attribute level : Integer;
				assert not constraint cn { assume constraint { level > 1 } }
			}
		}
	`)
	sym := symbolNamed(t, idx, "test::Rig::cn")
	if _, err := Constraint(ctx, sym, sym.OwnerScope); !errors.Is(err, ErrNoConditions) {
		t.Fatalf("translate: %v, want ErrNoConditions", err)
	}
}

// TestNoConditions: an element stating no condition has nothing to translate, as
// evaluating it reports the same.
func TestNoConditions(t *testing.T) {
	ctx, idx := fixture(t, "<test>", `
		package test {
			constraint def C;
		}
	`)
	sym := symbolNamed(t, idx, "test::C")
	if _, err := Constraint(ctx, sym, sym.OwnerScope); !errors.Is(err, ErrNoConditions) {
		t.Fatalf("translate: %v, want ErrNoConditions", err)
	}
}

// TestAllOrNothing: one condition outside the subset fails the whole
// translation, since a script missing a conjunct would answer about conditions it
// does not hold.
func TestAllOrNothing(t *testing.T) {
	refused := refusal(t, constraintSource(`
		in level : Integer;
		assert constraint { level >= 0 }
		assert constraint { level ** 2 <= 100 }
	`), "test::C")
	if !strings.Contains(refused.Construct, "**") {
		t.Errorf("refusal names %q, want the unsupported operator", refused.Construct)
	}
	if refused.Condition == "" {
		t.Error("the refusal does not name the condition it appeared in")
	}
	if !strings.HasPrefix(refused.Location, "<test>:") {
		t.Errorf("the refusal records location %q", refused.Location)
	}
}

// TestConflictingResultExpressionRefusal: a second result expression, stated over
// an inherited one or inherited twice, refuses translation and the refusal names
// the document and position of the offending body or declaration, not that of
// the supertype it inherits from.
func TestConflictingResultExpressionRefusal(t *testing.T) {
	ctx, idx := fixtureDocuments(t,
		document{"base.sysml", `
			package base {
				private import ScalarValues::*;
				constraint def Base { in x : Real; x > 0.0 }
				constraint def Other { in x : Real; x > 2.0 }
			}
		`},
		document{"sub.sysml", `
			package sub {
				private import base::*;
				constraint def Stated :> Base { x > 1.0 }
				constraint def Inherited :> Base, Other;
			}
		`})
	cases := []struct {
		name     string
		location string
	}{
		{"sub::Stated", "sub.sysml:4:37"},
		{"sub::Inherited", "sub.sysml:5:5"},
	}
	for _, tc := range cases {
		sym := symbolNamed(t, idx, tc.name)
		q, err := Constraint(ctx, sym, sym.OwnerScope)
		if q != nil || err == nil {
			t.Fatalf("translating %s: query %v, err %v; want a refusal", tc.name, q, err)
		}
		var refused *NotTranslatableError
		if !errors.As(err, &refused) {
			t.Fatalf("translating %s: %v is not a *NotTranslatableError", tc.name, err)
		}
		if refused.Construct != "conflicting result expression" {
			t.Errorf("%s: refusal names %q", tc.name, refused.Construct)
		}
		if refused.File != "sub.sysml" || refused.Location != tc.location {
			t.Errorf("%s: refusal records file %q at %q, want sub.sysml at %s", tc.name, refused.File, refused.Location, tc.location)
		}
	}
}

// TestRefusals: each construct outside the subset refuses rather than being
// silently dropped, naming what refused.
func TestRefusals(t *testing.T) {
	cases := []struct {
		name string
		body string
		want string // substring of the refusal's message
	}{
		{"exponentiation", `in i : Integer; assert constraint { i ** 2 == 4 }`, "exponentiation"},
		{"division by a literal zero", `in i : Integer; assert constraint { i / 0 == 1 }`, "division by zero"},
		{"remainder by a literal zero", `in i : Integer; assert constraint { i % 0 == 1 }`, "division by zero"},
		{"real division by a literal zero", `in x : Real; assert constraint { x / 0.0 == 1.0 }`, "division by zero"},
		{"real remainder", `in x : Real; assert constraint { x % 2.0 == 0.0 }`, "floating point"},
		{"range", `in i : Integer; assert constraint { i == 1..3 }`, "collection"},
		{"sequence index", `in i : Integer; assert constraint { i#(1) == 1 }`, "sequence"},
		{"classification", `in i : Integer; assert constraint { i istype Integer }`, "classification"},
		{"identity", `in i : Integer; assert constraint { i === i }`, "identity"},
		{"null coalescing", `in i : Integer; assert constraint { (i ?? 0) == 0 }`, "null"},
		{"null literal", `in i : Integer; assert constraint { i == null }`, "`null`"},
		{"metadata access", `in i : Integer; assert constraint { C::i.metadata == i }`, "not translatable"},
		{"collection feature", `in xs : Integer[*]; assert constraint { xs == 1 }`, "not translatable"},
		{"unbounded quantifier", `in xs : Integer[*]; assert constraint { xs->forAll{in x; x > 0} }`, "not translatable"},
		{"select expression", `in xs : Integer[*]; assert constraint { xs->select{in x; x > 0} == xs }`, "not translatable"},
		{"calc invocation", `
			in i : Integer;
			calc def Double { in x : Integer; return : Integer = x * 2; }
			assert constraint { Double(i) == 4 }`, "invocation"},
		{"string ordering", `in s : String; assert constraint { s < "x" }`, "rather than numbers"},
		{"non-boolean condition", `in i : Integer; assert constraint { i }`, "rather than a boolean"},
		{"non-scalar feature", `
			part def Tank;
			in tank : Tank;
			assert constraint { tank == tank }`, "no scalar sort"},
		{"unresolved name", `assert constraint { missing > 0 }`, "not translatable"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			refused := refusal(t, constraintSource("\t\t"+tc.body), "test::C")
			if !strings.Contains(refused.Error(), tc.want) {
				t.Errorf("refusal is %q, want it to mention %q", refused.Error(), tc.want)
			}
		})
	}
}

// TestFeatureChainGrounds: a chain ending in a scalar feature is one variable per
// path, so two chains reaching the same feature differently do not share one.
func TestFeatureChainGrounds(t *testing.T) {
	q := constraintQuery(t, `
		package test {
			private import ScalarValues::Real;
			part def Tank { attribute pressure : Real; }
			constraint def C {
				in a : Tank;
				in b : Tank;
				assert constraint { a.pressure < b.pressure }
			}
		}
	`, "test::C")
	if len(q.Vars) != 2 {
		t.Fatalf("declared %d variables, want one per chain:\n%s", len(q.Vars), Script(q))
	}
	for _, v := range q.Vars {
		if !v.Sort.Equal(Real) {
			t.Errorf("%s is %s, want Real", v.Name, v.Sort.Name)
		}
	}
}

// TestChainThroughACollectionRefuses: a chain reading through a feature holding
// many values reads one value per element, which no single variable stands for.
func TestChainThroughACollectionRefuses(t *testing.T) {
	refused := refusal(t, `
		package test {
			private import ScalarValues::Real;
			part def Tank { attribute pressure : Real; }
			constraint def C {
				in tanks : Tank[*];
				assert constraint { tanks.pressure < 3.0 }
			}
		}
	`, "test::C")
	if !strings.Contains(refused.Error(), "more than one value") {
		t.Errorf("refusal is %q, want it to name the multiplicity", refused.Error())
	}
}

// TestRedefinedFeatureIsOneVariable: a subtype redefining a feature reads the
// value the supertype's conditions read, so both constrain one variable — the
// evaluator resolves the name through one masked feature.
func TestRedefinedFeatureIsOneVariable(t *testing.T) {
	q := constraintQuery(t, `
		package test {
			private import ScalarValues::Integer;
			constraint def Base {
				in level : Integer;
				assert constraint { level >= 5 }
			}
			constraint def C :> Base {
				in :>> level;
				assert constraint { level <= 3 }
			}
		}
	`, "test::C")
	if len(q.Vars) != 1 {
		t.Fatalf("declared %d variables, want the redefined feature once:\n%s", len(q.Vars), Script(q))
	}
	name := q.Vars[0].Name
	for i, a := range q.Assertions {
		if !strings.Contains(writeTerm(a.Term), smtSymbol(name)) {
			t.Errorf("assertion %d is %s, want it to read %s", i, writeTerm(a.Term), name)
		}
	}
}

// TestBoundSubjectReadsTheSubjectsFeatures: a usage binding the subject role
// keeps the definition's subject type, so conditions chaining through it translate.
func TestBoundSubjectReadsTheSubjectsFeatures(t *testing.T) {
	q := requirementQuery(t, `
		package test {
			private import ScalarValues::Real;
			part def Truck {
				attribute payload : Real;
				attribute payloadLimit : Real;
			}
			requirement def PayloadReq {
				subject truck : Truck;
				require constraint { truck.payload <= truck.payloadLimit }
			}
			part loaded : Truck;
			requirement payloadHolds : PayloadReq {
				subject truck = loaded;
			}
		}
	`, "test::payloadHolds")
	if len(q.Vars) != 2 {
		t.Fatalf("declared %d variables, want one per chain through the subject:\n%s", len(q.Vars), Script(q))
	}
	for _, v := range q.Vars {
		if !v.Sort.Equal(Real) {
			t.Errorf("%s is %s, want Real", v.Name, v.Sort.Name)
		}
	}
}

// TestEnumerationIsAFiniteSort: an enumeration-typed feature ranges over the
// literals its definition declares, encoded as a datatype rather than a number.
func TestEnumerationIsAFiniteSort(t *testing.T) {
	q := constraintQuery(t, `
		package test {
			enum def Finish { enum polished; enum brushed; }
			constraint def C {
				in f : Finish;
				assert constraint { f == Finish::polished }
			}
		}
	`, "test::C")
	if len(q.Sorts) != 1 {
		t.Fatalf("declared %d datatype sorts, want one:\n%s", len(q.Sorts), Script(q))
	}
	sort := q.Sorts[0]
	if sort.Kind != SortDatatype || len(sort.Values) != 2 {
		t.Fatalf("sort is %+v, want the two literals", sort)
	}
	if !strings.Contains(Script(q), "(declare-datatypes ((|test::Finish| 0)) (((|test::Finish::polished|) (|test::Finish::brushed|))))") {
		t.Errorf("the script does not declare the enumeration:\n%s", Script(q))
	}
	// The enumeration's values are its variants, yet a feature holding one is a
	// value to pin rather than a variation point to configure.
	if sort.Variation || len(q.Variations()) != 0 {
		t.Errorf("an enumeration-typed feature is not a variation point: %+v", sort)
	}
}

// TestVariationPointIsAFiniteSort: the variant a variation point selects ranges
// over its variants, which is a finite choice like an enumeration.
func TestVariationPointIsAFiniteSort(t *testing.T) {
	ctx, idx := fixtureFile(t, "ring_variants.sysml")
	sym := symbolNamed(t, idx, "test::ringFamily::finishMatchesNesting")
	q, err := Constraint(ctx, sym, sym.OwnerScope)
	if err != nil {
		t.Fatalf("translate: %v", err)
	}
	if len(q.Sorts) != 2 {
		t.Fatalf("declared %d datatype sorts, want the enumeration and the variation:\n%s", len(q.Sorts), Script(q))
	}
	for _, s := range q.Sorts {
		if len(s.Values) != 2 {
			t.Errorf("sort %s has values %v", s.Name, s.Values)
		}
	}
}

// TestVariationSortIsShared: a usage selecting a variant reads the same finite
// sort as the variation declaring it, so comparing the two translates.
func TestVariationSortIsShared(t *testing.T) {
	q := constraintQuery(t, `
		package test {
			attribute def Nesting { attribute cost : ScalarValues::Real; }
			part def Ring { attribute nesting : Nesting; }
			abstract part ringFamily : Ring {
				variation attribute :>> nesting {
					variant attribute nestingTrue { :>> cost = 100.0; }
					variant attribute nestingFalse { :>> cost = 200.0; }
				}
			}
			part chosen :> ringFamily {
				attribute :>> nesting = nesting::nestingTrue;
				assert constraint selected { nesting == nesting::nestingTrue }
			}
		}
	`, "test::chosen::selected")
	if len(q.Sorts) != 1 {
		t.Fatalf("declared %d datatype sorts, want the one variation:\n%s", len(q.Sorts), Script(q))
	}
	if len(q.Sorts[0].Values) != 2 {
		t.Errorf("sort %s has values %v, want the two variants", q.Sorts[0].Name, q.Sorts[0].Values)
	}
}

// TestBodyStatementRefuses: a statement in a constraint body is not executed, so
// the translation refuses rather than solving the conditions as if it ran.
func TestBodyStatementRefuses(t *testing.T) {
	refused := refusal(t, constraintSource(`
		attribute y : Integer = 1;
		assign y := 10;
		y > 5
	`), "test::C")
	if refused.Construct != "body statement" {
		t.Errorf("refused construct is %q, want the body statement", refused.Construct)
	}
	if !strings.Contains(refused.Reason, "does not execute") {
		t.Errorf("refusal reason is %q, want it to say the statement is not executed", refused.Reason)
	}
	if refused.Condition != "`assign` statement" {
		t.Errorf("refused condition is %q, want the assign statement", refused.Condition)
	}
}

// TestPerformedActionRefuses: a performed action is a usage rather than a
// statement node, and translation refuses it the same way, nested or not.
func TestPerformedActionRefuses(t *testing.T) {
	src := `
		package test {
			private import ScalarValues::*;
			action def Bump;
			constraint def C {
				attribute y : Integer = 1;
				perform action bump : Bump;
				y > 5
			}
			part def Rig {
				attribute z : Integer = 1;
				action bump : Bump;
				constraint nested { assert constraint { perform bump; z > 5 } }
			}
		}
	`
	for _, name := range []string{"test::C", "test::Rig::nested"} {
		refused := refusal(t, src, name)
		if refused.Construct != "body statement" {
			t.Errorf("%s: refused construct is %q, want the body statement", name, refused.Construct)
		}
		if refused.Condition != "`perform` statement" {
			t.Errorf("%s: refused condition is %q, want the perform statement", name, refused.Condition)
		}
	}
}

// TestActionFlowRefuses: action nodes and the successions between them are steps
// of a constraint body that translation refuses rather than drops.
func TestActionFlowRefuses(t *testing.T) {
	src := `
		package test {
			private import ScalarValues::*;
			constraint def C {
				attribute y : Integer = 1;
				action a; action b;
				first a then b;
				y > 5
			}
			part def Rig {
				attribute z : Integer = 1;
				action a; action b;
				constraint edge { first a then b; z > 5 }
				constraint node { action c; fork f; first c then f; z > 5 }
				constraint nested { assert constraint { action c; z > 5 } }
			}
		}
	`
	for name, want := range map[string]string{
		"test::C":           "`action` statement",
		"test::Rig::edge":   "`first` statement",
		"test::Rig::node":   "`action` statement",
		"test::Rig::nested": "`action` statement",
	} {
		refused := refusal(t, src, name)
		if refused.Construct != "body statement" {
			t.Errorf("%s: refused construct is %q, want the body statement", name, refused.Construct)
		}
		if refused.Condition != want {
			t.Errorf("%s: refused condition is %q, want %s", name, refused.Condition, want)
		}
	}
}
