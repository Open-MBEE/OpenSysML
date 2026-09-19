package solve

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

var update = flag.Bool("update", false, "rewrite the .smt2 golden files")

// TestGolden translates each fixture and compares the script it writes with the
// golden one, which locks both the translation and the writer's output.
func TestGolden(t *testing.T) {
	cases := []struct {
		fixture string
		element string // qualified name of the element to translate
		kind    string // constraint or requirement
		golden  string
	}{
		{"touchdown.sysml", "test::TouchdownRequirement", "requirement", "touchdown.smt2"},
		{"mission_budget.sysml", "test::BudgetConstraint", "constraint", "mission_budget.smt2"},
		{"ring_variants.sysml", "test::ringFamily::finishMatchesNesting", "constraint", "ring_variants.smt2"},
		{"safe_window.sysml", "test::SafeWindow", "constraint", "safe_window.smt2"},
		{"safe_window.sysml", "test::rig::safeWindow", "constraint", "safe_window_denied.smt2"},
		{"objectives.sysml", "test::MassBudget", "analysis", "objective_mass.smt2"},
		{"objectives.sysml", "test::CostThenMargin", "analysis", "objective_lexicographic.smt2"},
		{"objectives.sysml", "test::WheelChoice", "analysis", "objective_variants.smt2"},
		{"objectives.sysml", "test::GuardedRatio", "analysis", "objective_guarded.smt2"},
		// One per logic the feature set can select, so a change of logic shows here.
		{"logic_selection.sysml", "test::CratesPerPallet", "constraint", "crates_per_pallet.smt2"},
		{"logic_selection.sysml", "test::CratesPerRun", "constraint", "crates_per_run.smt2"},
		{"logic_selection.sysml", "test::MassAndCrates", "constraint", "mass_and_crates.smt2"},
		{"logic_selection.sysml", "test::MassPerCrate", "constraint", "mass_per_crate.smt2"},
	}
	for _, tc := range cases {
		t.Run(tc.golden, func(t *testing.T) {
			ctx, idx := fixtureFile(t, tc.fixture)
			sym := symbolNamed(t, idx, tc.element)
			query, err := translateElement(ctx, tc.kind, sym)
			if err != nil {
				t.Fatalf("translate %s: %v", tc.element, err)
			}
			compareGolden(t, tc.golden, Script(query))
		})
	}
}

// translateElement translates one element of the kind named.
func translateElement(ctx *runtime.Context, kind string, sym *symbols.Symbol) (*Query, error) {
	switch kind {
	case "requirement":
		return Requirement(ctx, sym, sym.OwnerScope)
	case "analysis":
		return Analysis(ctx, sym, sym.OwnerScope)
	}
	return Constraint(ctx, sym, sym.OwnerScope)
}

// compareGolden compares a script with its golden file, rewriting it under
// -update.
func compareGolden(t *testing.T, name, got string) {
	t.Helper()
	path := filepath.Join("testdata", name)
	if *update {
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatalf("write golden: %v", err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden (run with -update to create it): %v", err)
	}
	if got != string(want) {
		t.Errorf("script differs from %s:\n--- got ---\n%s\n--- want ---\n%s", name, got, want)
	}
}

// TestDeterministic: translating and writing the same element twice yields the
// same bytes, which is what makes a golden comparison meaningful.
func TestDeterministic(t *testing.T) {
	ctx, idx := fixtureFile(t, "ring_variants.sysml")
	sym := symbolNamed(t, idx, "test::ringFamily::finishMatchesNesting")

	first, err := Constraint(ctx, sym, sym.OwnerScope)
	if err != nil {
		t.Fatalf("first translation: %v", err)
	}
	second, err := Constraint(ctx, sym, sym.OwnerScope)
	if err != nil {
		t.Fatalf("second translation: %v", err)
	}
	if Script(first) != Script(second) {
		t.Errorf("two translations wrote different scripts:\n%s\n%s", Script(first), Script(second))
	}
}

// TestGoldenScriptsAreSelfDescribing: every golden script says what it came from
// and asks for an answer, so a solver run needs nothing added.
func TestGoldenScriptsAreSelfDescribing(t *testing.T) {
	entries, err := filepath.Glob(filepath.Join("testdata", "*.smt2"))
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("no golden scripts; run with -update")
	}
	for _, path := range entries {
		text, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		script := string(text)
		if !strings.HasPrefix(script, "; OpenSysML SMT-LIB2 translation of ") {
			t.Errorf("%s does not name what it was translated from", path)
		}
		if !strings.Contains(script, "(set-logic ") || !strings.HasSuffix(script, "(check-sat)\n") {
			t.Errorf("%s is not a complete script", path)
		}
	}
}
