package solve

import (
	"context"
	"sort"
	"strings"
	"testing"
)

// valuedQuery translates a constraint of the value-typed variation fixture.
func valuedQuery(t *testing.T, element string) (*Query, error) {
	t.Helper()
	ctx, idx := fixtureFile(t, "value_variants.sysml")
	sym := symbolNamed(t, idx, "test::gearFamily::"+element)
	return Constraint(ctx, sym, sym.OwnerScope)
}

// TestValueTypedVariationComparesTheValuesItsVariantsDeclare: `==` and `!=` on
// a variation point whose variants declare values compare the value the
// selection stands for, so a variant, a literal and a quantifier body all
// compare by value while the point stays a variation the query configures.
func TestValueTypedVariationComparesTheValuesItsVariantsDeclare(t *testing.T) {
	selected := "(ite (= |test::gearFamily::ratioChoice| |test::gearFamily::ratioChoice::low|) 3 (ite (= |test::gearFamily::ratioChoice| |test::gearFamily::ratioChoice::lowAgain|) 3 5))"
	cases := map[string]string{
		"isLow":   "(= " + selected + " 3)",
		"notHigh": "(distinct " + selected + " 5)",
		"isThree": "(= " + selected + " 3)",
		"eachLow": "(= " + selected + " 3)",
	}
	for element, want := range cases {
		q, err := valuedQuery(t, element)
		if err != nil {
			t.Fatalf("%s: %v", element, err)
		}
		if vs := q.Variations(); len(vs) != 1 || vs[0].Name != "test::gearFamily::ratioChoice" {
			t.Errorf("%s reads %v, want the ratio choice as a variation", element, vs)
		}
		if got := writeTerm(q.Assertions[0].Term); got != want {
			t.Errorf("%s asserts\n%s\nwant\n%s", element, got, want)
		}
	}
}

// TestTwoVariantsCompareAsTheChoice: two variants named outright compare as the
// choice itself, as the evaluator compares them, whatever values they declare.
func TestTwoVariantsCompareAsTheChoice(t *testing.T) {
	q, err := valuedQuery(t, "sameChoice")
	if err != nil {
		t.Fatalf("sameChoice: %v", err)
	}
	want := "(= |test::gearFamily::ratioChoice::low| |test::gearFamily::ratioChoice::lowAgain|)"
	if got := writeTerm(q.Assertions[0].Term); got != want {
		t.Errorf("sameChoice asserts %s, want %s", got, want)
	}
}

// TestValueTypedVariationPermitsEveryEqualValuedVariant: over a solver, a
// constraint selecting a variant by `==` permits every variant declaring the
// same value and no other, as validating an object of each selection answers.
func TestValueTypedVariationPermitsEveryEqualValuedVariant(t *testing.T) {
	q, err := valuedQuery(t, "isLow")
	if err != nil {
		t.Fatalf("isLow: %v", err)
	}
	names := permittedConfigurations(t, q)
	if got := strings.Join(names, ";"); got != "test::gearFamily::ratioChoice::low;test::gearFamily::ratioChoice::lowAgain" {
		t.Errorf("isLow permits %s, want low and lowAgain", got)
	}
}

// permittedConfigurations enumerates the selections a query over one variation
// point permits, as the values of that point in order.
func permittedConfigurations(t *testing.T, q *Query) []string {
	t.Helper()
	solver := requireSolver(t)
	result, err := solver.Configurations(context.Background(), q, 0)
	if err != nil {
		t.Fatalf("enumerate configurations: %v\nscript:\n%s", err, Script(q))
	}
	if result.Status != StatusSat || result.Truncated {
		t.Fatalf("status %s, truncated %v, want every selection reported", result.Status, result.Truncated)
	}
	var names []string
	for _, solution := range result.Solutions {
		if len(solution) != 1 {
			t.Fatalf("a selection assigns %d variation points, want one", len(solution))
		}
		names = append(names, solution[0].Value)
	}
	sort.Strings(names)
	return names
}
