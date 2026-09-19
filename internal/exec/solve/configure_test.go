package solve

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// variantQuery translates a constraint of the variant fixture.
func variantQuery(t *testing.T, element string) (*Query, *symbols.Index) {
	t.Helper()
	ctx, idx := fixtureFile(t, "nested_variants.sysml")
	sym := symbolNamed(t, idx, element)
	q, err := Constraint(ctx, sym, sym.OwnerScope)
	if err != nil {
		t.Fatalf("translate %s: %v", element, err)
	}
	return q, idx
}

// TestVariationsAreTheVariationPointsRead: the variables a configuration assigns
// are the variation points, not every finite-valued feature; an enumeration is
// finite too and is not a choice of variant.
func TestVariationsAreTheVariationPointsRead(t *testing.T) {
	q, _ := variantQuery(t, "test::ringFamily::variantsAgree")
	names := []string{}
	for _, v := range q.Variations() {
		names = append(names, v.Name)
	}
	if strings.Join(names, ",") != "test::ringFamily::nesting,test::ringFamily::trim" {
		t.Errorf("variation points %v, want the nesting and the trim", names)
	}

	other, _ := variantQuery(t, "test::ringFamily::gildedIsPolished")
	for _, v := range other.Variations() {
		if v.Name == "test::ringFamily::finish" {
			t.Error("finish is an enumeration, not a variation point")
		}
	}
}

// TestFixValueChecksWhatItIsGiven: choosing a variant is refused unless the
// query reads the variation point and the name is one of its variants, and the
// refusal names both.
func TestFixValueChecksWhatItIsGiven(t *testing.T) {
	q, _ := variantQuery(t, "test::ringFamily::variantsAgree")
	nesting := q.Variations()[0]
	before := len(q.Assertions)

	if err := q.FixValue(nil, "whatever", PinChosen); !errors.Is(err, ErrNotPinnable) {
		t.Errorf("error %v, want a refusal to fix nothing", err)
	}
	if err := q.FixValue(nesting, "test::ringFamily::nesting::none", PinChosen); !errors.Is(err, ErrNotPinnable) {
		t.Errorf("error %v, want a refusal of a name that is no variant", err)
	}
	if err := q.FixValue(&Var{Name: "elsewhere", Sort: nesting.Sort}, "x", PinChosen); !errors.Is(err, ErrNotPinnable) {
		t.Errorf("error %v, want a refusal of a variable the query does not read", err)
	}
	if len(q.Assertions) != before {
		t.Errorf("a refused choice left %d assertions behind", len(q.Assertions)-before)
	}

	if err := q.FixValue(nesting, "test::ringFamily::nesting::nestingTrue", PinChosen); err != nil {
		t.Fatalf("fix a variant the query reads: %v", err)
	}
	if len(q.Pinned) != 1 || q.Assertions[q.Pinned[0].Index].From.Role != RolePinned {
		t.Errorf("the chosen variant is not recorded with the assertion fixing it: %+v", q.Pinned)
	}
	if !strings.Contains(Script(q), "(assert (= |test::ringFamily::nesting| |test::ringFamily::nesting::nestingTrue|))") {
		t.Errorf("the script does not fix the chosen variant:\n%s", Script(q))
	}
}

// TestFixValueRefusesAFeatureWithNoValuesToName: a number has no names of
// values, so no name of a variant fixes it.
func TestFixValueRefusesAFeatureWithNoValuesToName(t *testing.T) {
	ctx, idx := fixtureFile(t, "panel_pins.sysml")
	sym := symbolNamed(t, idx, "test::Panel::fits")
	q, err := Constraint(ctx, sym, sym.OwnerScope)
	if err != nil {
		t.Fatalf("translate: %v", err)
	}
	if err := q.FixValue(q.Vars[0], "wide", PinChosen); !errors.Is(err, ErrNotPinnable) {
		t.Errorf("error %v, want a refusal to name the value of a number", err)
	}
}

// TestMaxConfigurationsFromEnv: the bound is read from the environment, and an
// unusable override falls back to the default rather than removing the bound.
func TestMaxConfigurationsFromEnv(t *testing.T) {
	cases := map[string]int{
		"":     DefaultMaxConfigurations,
		"4":    4,
		" 7 ":  7,
		"0":    DefaultMaxConfigurations,
		"-2":   DefaultMaxConfigurations,
		"lots": DefaultMaxConfigurations,
	}
	for text, want := range cases {
		t.Setenv(MaxConfigurationsEnv, text)
		if got := MaxConfigurationsFromEnv(); got != want {
			t.Errorf("%s=%q gave %d, want %d", MaxConfigurationsEnv, text, got, want)
		}
	}
}

// TestConfiguringWithoutVariationsIsTyped: an element reading no variation point
// has no configuration to report, which is a typed error naming the element
// rather than an empty answer.
func TestConfiguringWithoutVariationsIsTyped(t *testing.T) {
	solver := requireSolver(t)
	ctx, idx := fixtureFile(t, "panel_pins.sysml")
	sym := symbolNamed(t, idx, "test::Panel::fits")
	q, err := Constraint(ctx, sym, sym.OwnerScope)
	if err != nil {
		t.Fatalf("translate: %v", err)
	}
	_, err = solver.Configurations(context.Background(), q, 0)
	var absent *NoVariationsError
	if !errors.As(err, &absent) || !errors.Is(err, ErrNoVariations) {
		t.Fatalf("error %v, want a typed report that no variation point is read", err)
	}
	if !strings.Contains(absent.Error(), "fits") {
		t.Errorf("error %q does not name the element", absent.Error())
	}
}

// TestConfigurationsEnumeratesEveryConsistentSelection: every combination the
// constraints permit is reported once, each assigning every variation point, and
// the result says the enumeration was not cut short.
func TestConfigurationsEnumeratesEveryConsistentSelection(t *testing.T) {
	solver := requireSolver(t)
	q, _ := variantQuery(t, "test::ringFamily::variantsAgree")
	result, err := solver.Configurations(context.Background(), q, 0)
	if err != nil {
		t.Fatalf("enumerate configurations: %v", err)
	}
	if result.Status != StatusSat || result.Truncated {
		t.Fatalf("status %s, truncated %v, want every selection reported", result.Status, result.Truncated)
	}
	// nestingTrue forbids plain, so three of the four combinations remain.
	if len(result.Solutions) != 3 {
		t.Fatalf("reported %d selections, want 3: %+v", len(result.Solutions), result.Solutions)
	}
	seen := map[string]bool{}
	for _, solution := range result.Solutions {
		if len(solution) != 2 {
			t.Fatalf("a selection assigns %d variation points, want both", len(solution))
		}
		chosen := map[string]string{}
		for _, a := range solution {
			chosen[a.Var.Name] = a.Value
		}
		key := chosen["test::ringFamily::nesting"] + "/" + chosen["test::ringFamily::trim"]
		if seen[key] {
			t.Errorf("the selection %s was reported twice", key)
		}
		seen[key] = true
		if strings.HasSuffix(chosen["test::ringFamily::nesting"], "nestingTrue") &&
			!strings.HasSuffix(chosen["test::ringFamily::trim"], "gilded") {
			t.Errorf("the selection %s does not satisfy the constraint", key)
		}
	}
}

// TestConfigurationsStopAtTheirBound: an enumeration asked for fewer than there
// are reports that many and says it was cut short, never implying it showed all
// of them.
func TestConfigurationsStopAtTheirBound(t *testing.T) {
	solver := requireSolver(t)
	q, _ := variantQuery(t, "test::ringFamily::variantsAgree")
	result, err := solver.Configurations(context.Background(), q, 2)
	if err != nil {
		t.Fatalf("enumerate configurations: %v", err)
	}
	if result.Status != StatusSat || !result.Truncated || len(result.Solutions) != 2 {
		t.Fatalf("status %s, truncated %v, %d selections, want 2 and truncated",
			result.Status, result.Truncated, len(result.Solutions))
	}
	// Why it stopped is recorded, so a bound is never reported as a solver that
	// stopped deciding, whether or not the solver would say why.
	if !result.AtBound || result.Undecided {
		t.Fatalf("at bound %v, undecided %v, want the bound as the reason",
			result.AtBound, result.Undecided)
	}
}

// TestConfigurationsKeepWhatTheyFoundWhenTimeRunsOut: a deadline part way
// through an enumeration reports the configurations already found, as truncated
// and undecided, rather than discarding them for a bare `unknown`.
func TestConfigurationsKeepWhatTheyFoundWhenTimeRunsOut(t *testing.T) {
	q, _ := variantQuery(t, "test::ringFamily::variantsAgree")
	solver := fakeSolver(t, "sat-then-hang")
	solver.Timeout = 200 * time.Millisecond
	solver.Env = append(solver.Env, "OPENSYSML_TEST_SOLVER_MODEL="+
		"((|test::ringFamily::nesting| |test::ringFamily::nesting::nestingTrue|)"+
		" (|test::ringFamily::trim| |test::ringFamily::trim::gilded|))")

	result, err := solver.Configurations(context.Background(), q, 0)
	if err != nil {
		t.Fatalf("enumerate configurations: %v", err)
	}
	if result.Status != StatusSat || len(result.Solutions) != 1 {
		t.Fatalf("status %s, %d selections, want the one reported before the deadline",
			result.Status, len(result.Solutions))
	}
	if !result.Truncated || !result.Undecided || !result.TimedOut {
		t.Fatalf("truncated %v, undecided %v, timed out %v, want all three",
			result.Truncated, result.Undecided, result.TimedOut)
	}
	if !strings.Contains(result.Reason, "ran out of time") {
		t.Errorf("reason %q does not say the solver ran out of time", result.Reason)
	}
	if got := result.Solutions[0][0].Value; got != "test::ringFamily::nesting::nestingTrue" {
		t.Errorf("the selection reported %s, want the nesting the solver gave", got)
	}
}

// TestConfigurationsOfAConstrainedVariant: a variant constrained through another
// feature narrows the selections, and the enumeration respects that.
func TestConfigurationsOfAConstrainedVariant(t *testing.T) {
	solver := requireSolver(t)
	ctx, idx := fixtureFile(t, "nested_variants.sysml")
	sym := symbolNamed(t, idx, "test::ringFamily::gildedIsPolished")
	q, err := Constraint(ctx, sym, sym.OwnerScope)
	if err != nil {
		t.Fatalf("translate: %v", err)
	}
	trim := q.Variations()
	if len(trim) != 1 {
		t.Fatalf("variation points %+v, want the trim alone", trim)
	}
	if err := q.FixValue(trim[0], "test::ringFamily::trim::gilded", PinChosen); err != nil {
		t.Fatalf("choose the gilded trim: %v", err)
	}
	result, err := solver.Configurations(context.Background(), q, 0)
	if err != nil {
		t.Fatalf("enumerate configurations: %v", err)
	}
	if result.Status != StatusSat || len(result.Solutions) != 1 {
		t.Fatalf("status %s, %d selections, want the one chosen trim",
			result.Status, len(result.Solutions))
	}
	if got := result.Solutions[0][0].Value; got != "test::ringFamily::trim::gilded" {
		t.Errorf("the selection reported %s, want the trim chosen", got)
	}
}

// TestChosenSelectionCanConflict: a combination the constraints forbid is unsat,
// and the core names the choices that conflict rather than the whole query.
func TestChosenSelectionCanConflict(t *testing.T) {
	solver := requireSolver(t)
	q, _ := variantQuery(t, "test::ringFamily::variantsAgree")
	vars := q.Variations()
	if err := q.FixValue(vars[0], "test::ringFamily::nesting::nestingTrue", PinChosen); err != nil {
		t.Fatalf("choose the nesting: %v", err)
	}
	if err := q.FixValue(vars[1], "test::ringFamily::trim::plain", PinChosen); err != nil {
		t.Fatalf("choose the trim: %v", err)
	}
	result, err := solver.Explain(context.Background(), q)
	if err != nil {
		t.Fatalf("explain the chosen selection: %v", err)
	}
	if result.Status != StatusUnsat {
		t.Fatalf("status %s, want the chosen selection to conflict", result.Status)
	}
	if result.Core == nil {
		t.Skip("this solver produced no core")
	}
	fixed := 0
	for _, member := range result.Core.Members {
		if member.From.Role == RolePinned {
			fixed++
		}
	}
	if fixed == 0 {
		t.Errorf("the core names no chosen variant: %+v", result.Core.Members)
	}
}
