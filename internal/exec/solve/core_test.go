package solve

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// coreLabels names the assertions a core reported, for comparing against what a
// query asserts.
func coreLabels(core *Core) []string {
	out := make([]string, 0, len(core.Indices))
	for _, i := range core.Indices {
		out = append(out, CoreLabel(i))
	}
	return out
}

// coreConditions is what each core member was written as, in core order.
func coreConditions(core *Core) []string {
	out := make([]string, 0, len(core.Members))
	for _, m := range core.Members {
		out = append(out, m.From.Condition)
	}
	return out
}

// explained asks the discovered solver to explain a query, expecting a verdict.
func explained(t *testing.T, solver *Solver, q *Query, want Status) *Result {
	t.Helper()
	result, err := solver.Explain(context.Background(), q)
	if err != nil {
		t.Fatalf("explain: %v\nscript:\n%s", err, CoreScript(q, nil))
	}
	if result.Status != want {
		t.Fatalf("%s answered %s, want %s (reason %q)\nscript:\n%s",
			result.Solver, result.Status, want, result.Reason, CoreScript(q, nil))
	}
	return result
}

// unsatCoreOf explains a query the solver must find unsatisfiable, checking that
// the core it reports names assertions of that query.
func unsatCoreOf(t *testing.T, solver *Solver, q *Query) *Core {
	t.Helper()
	result := explained(t, solver, q, StatusUnsat)
	core := result.Core
	if core == nil {
		t.Fatal("an unsatisfiable verdict came with no core")
	}
	if len(core.Members) != len(core.Indices) {
		t.Fatalf("core names %d assertions but holds %d members", len(core.Indices), len(core.Members))
	}
	for n, i := range core.Indices {
		if i < 0 || i >= len(q.Assertions) {
			t.Fatalf("core names assertion %d, outside the query's %d", i, len(q.Assertions))
		}
		if core.Members[n].Term != q.Assertions[i].Term {
			t.Fatalf("core member %d is not the query's assertion %d", n, i)
		}
		if n > 0 && core.Indices[n-1] >= i {
			t.Fatalf("core indices %v are not in the query's assertion order", core.Indices)
		}
	}
	return core
}

// requirementQuery translates the named requirement of a source.
func requirementQuery(t *testing.T, src, name string) *Query {
	t.Helper()
	ctx, idx := fixture(t, "<test>", src)
	sym := symbolNamed(t, idx, name)
	q, err := Requirement(ctx, sym, sym.OwnerScope)
	if err != nil {
		t.Fatalf("translate %s: %v", name, err)
	}
	return q
}

// TestCoreScriptShape: the labelled script turns cores on, names each assertion,
// and asks for the core once the verdict is in.
func TestCoreScriptShape(t *testing.T) {
	i := &Var{Name: "test::C::i", Sort: Int, Location: "c.sysml:3:3"}
	q := &Query{
		Kind:    "constraint",
		Element: "C",
		Vars:    []*Var{i},
		Assertions: []Assertion{
			{
				Term: Binary(OpGt, Bool, VarTerm(i), IntTerm(5)),
				From: Provenance{Kind: "constraint", Element: "C", Condition: "i > 5", Role: RoleRequired, Location: "c.sysml:4:3"},
			},
			{
				Term: Binary(OpLt, Bool, VarTerm(i), IntTerm(2)),
				From: Provenance{Kind: "constraint", Element: "C", Condition: "i < 2", Role: RoleRequired, Location: "c.sysml:5:3"},
			},
		},
	}
	want := strings.Join([]string{
		"; OpenSysML SMT-LIB2 translation of constraint C",
		"; the runtime evaluator remains normative; solving is an optional extension",
		"; each assertion is named, so an unsat core names the conditions that conflict",
		"(set-option :produce-unsat-cores true)",
		"(set-logic QF_LIA)",
		"; test::C::i, declared at c.sysml:3:3",
		"(declare-const |test::C::i| Int)",
		"; required condition: i > 5 — constraint C, at c.sysml:4:3",
		"(assert (! (> |test::C::i| 5) :named sy!a0))",
		"; required condition: i < 2 — constraint C, at c.sysml:5:3",
		"(assert (! (< |test::C::i| 2) :named sy!a1))",
		"(check-sat)",
		"(get-unsat-core)",
		"",
	}, "\n")
	if got := CoreScript(q, nil); got != want {
		t.Errorf("core script is:\n%s\nwant:\n%s", got, want)
	}

	// A subset asserts only what it names, keeping each label with its assertion.
	subset := CoreScript(q, []int{1})
	if strings.Contains(subset, "sy!a0") || !strings.Contains(subset, ":named sy!a1") {
		t.Errorf("a subset script relabelled its assertions:\n%s", subset)
	}
	if empty := CoreScript(q, []int{}); strings.Contains(empty, "(assert") {
		t.Errorf("an empty subset asserted something:\n%s", empty)
	}
}

// TestPlainScriptHasNoCoreSupport: core support is opt-in, so the plain script a
// %check writes carries no labels and asks for no core.
func TestPlainScriptHasNoCoreSupport(t *testing.T) {
	q := constraintQuery(t, constraintSource(`
		in i : Integer;
		assert constraint { i > 5 }
	`), "test::C")
	script := Script(q)
	for _, unwanted := range []string{":named", "produce-unsat-cores", "get-unsat-core"} {
		if strings.Contains(script, unwanted) {
			t.Errorf("the plain script mentions %s:\n%s", unwanted, script)
		}
	}
}

// TestCoreLabelsRoundTrip: a label maps back to the assertion it was issued for,
// and anything else is refused rather than mapped to an assertion.
func TestCoreLabelsRoundTrip(t *testing.T) {
	for _, i := range []int{0, 1, 7, 128} {
		got, ok := coreLabelIndex(CoreLabel(i))
		if !ok || got != i {
			t.Errorf("label %s mapped to %d (ok %v), want %d", CoreLabel(i), got, ok, i)
		}
	}
	for _, label := range []string{"", "sy!a", "sy!a-1", "sy!a01", "sy!a1x", "sy!a 1", "a0", "|sy!a0|", "sy!!a0"} {
		if i, ok := coreLabelIndex(label); ok {
			t.Errorf("label %q mapped to assertion %d, want a refusal", label, i)
		}
	}
}

// TestCoreGolden: the labelled script of each fixture is locked as a golden, so
// the form a solver is asked for a core in cannot drift unnoticed.
func TestCoreGolden(t *testing.T) {
	cases := []struct {
		fixture string
		element string
		kind    string
		golden  string
	}{
		{"touchdown.sysml", "test::TouchdownRequirement", "requirement", filepath.Join("core", "touchdown.smt2")},
		{"mission_budget.sysml", "test::BudgetConstraint", "constraint", filepath.Join("core", "mission_budget.smt2")},
		{"safe_window.sysml", "test::rig::safeWindow", "constraint", filepath.Join("core", "safe_window_denied.smt2")},
	}
	for _, tc := range cases {
		t.Run(tc.golden, func(t *testing.T) {
			ctx, idx := fixtureFile(t, tc.fixture)
			sym := symbolNamed(t, idx, tc.element)
			query, err := translateElement(ctx, tc.kind, sym)
			if err != nil {
				t.Fatalf("translate %s: %v", tc.element, err)
			}
			compareGolden(t, tc.golden, CoreScript(query, nil))
		})
	}
}

// TestCoreGoldensAskForACore: every labelled golden turns cores on, labels each
// assertion, and asks for the core after the verdict.
func TestCoreGoldensAskForACore(t *testing.T) {
	entries, err := filepath.Glob(filepath.Join("testdata", "core", "*.smt2"))
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("no labelled golden scripts; run with -update")
	}
	for _, path := range entries {
		text, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		script := string(text)
		if !strings.Contains(script, "(set-option :produce-unsat-cores true)\n") {
			t.Errorf("%s does not turn unsat cores on", path)
		}
		if !strings.Contains(script, ":named "+CoreLabel(0)+"))\n") {
			t.Errorf("%s does not label its first assertion", path)
		}
		if !strings.HasSuffix(script, "(check-sat)\n(get-unsat-core)\n") {
			t.Errorf("%s does not ask for a core after the verdict", path)
		}
	}
}

// A contradiction inside one condition conflicts with itself: the core is that
// condition alone, and it is minimal without a further solver call.
func TestExplainedSingleConditionConflict(t *testing.T) {
	solver := requireSolver(t)
	q := constraintQuery(t, constraintSource(`
		in i : Integer;
		assert constraint { i > 5 and i < 2 }
	`), "test::C")
	core := unsatCoreOf(t, solver, q)
	if len(core.Members) != 1 || core.Members[0].From.Condition != "i > 5 and i < 2" {
		t.Fatalf("core is %v, want the single condition", coreConditions(core))
	}
	if !core.Minimal || core.Rounds != 0 {
		t.Errorf("a one-member core took %d rounds to call minimal (minimal %v)", core.Rounds, core.Minimal)
	}
}

// Two conditions that cannot hold together are both in the core, in the order the
// query asserts them, and each is needed.
func TestExplainedTwoConditionConflict(t *testing.T) {
	solver := requireSolver(t)
	q := constraintQuery(t, constraintSource(`
		in i : Integer;
		assert constraint { i > 5 }
		assert constraint { i < 2 }
		assert constraint { i != 100 }
	`), "test::C")
	core := unsatCoreOf(t, solver, q)
	if got := coreConditions(core); len(got) != 2 || got[0] != "i > 5" || got[1] != "i < 2" {
		t.Fatalf("core is %v, want the two conflicting conditions in assertion order", got)
	}
	if !core.Minimal {
		t.Errorf("core is not reported minimal: %s", core.Note)
	}
	for _, m := range core.Members {
		if m.From.Role != RoleRequired || m.From.Location == "" {
			t.Errorf("core member %+v does not carry its role and location", m.From)
		}
	}
	result := explained(t, solver, q, StatusUnsat)
	if result.Elapsed < result.Core.Elapsed {
		t.Errorf("explaining took %s, less than the %s it spent shrinking the core",
			result.Elapsed, result.Core.Elapsed)
	}
}

// A conflict that only exists because of a declared domain names that domain: the
// conditions alone are satisfiable, a Natural's non-negativity is what refuses.
func TestExplainedDomainOnlyConflict(t *testing.T) {
	solver := requireSolver(t)
	q := constraintQuery(t, constraintSource(`
		in n : Natural;
		assert constraint { n < 0 }
	`), "test::C")
	core := unsatCoreOf(t, solver, q)
	if len(core.Members) != 2 {
		t.Fatalf("core is %v, want the domain bound and the condition", coreConditions(core))
	}
	if core.Members[0].From.Role != RoleDomain {
		t.Errorf("core starts with %s, want the declared domain", core.Members[0].From.Role)
	}
	if core.Members[0].From.Declared == nil {
		t.Error("the domain member does not name the declaration it came from")
	}
	if core.Members[1].From.Role != RoleRequired {
		t.Errorf("core's second member is %s, want the required condition", core.Members[1].From.Role)
	}
}

// A divisor guard is a real core member: the conditions say the divisor is zero,
// which the guard the translation adds refuses.
func TestExplainedDivisorGuardConflict(t *testing.T) {
	solver := requireSolver(t)
	q := constraintQuery(t, constraintSource(`
		in a : Integer;
		in b : Integer;
		assert constraint { b == 0 }
		assert constraint { a / b == 0 }
	`), "test::C")
	core := unsatCoreOf(t, solver, q)
	roles := map[Role]bool{}
	for _, m := range core.Members {
		roles[m.From.Role] = true
	}
	if !roles[RoleDefined] || !roles[RoleRequired] {
		t.Fatalf("core is %v with roles %v, want the guard and the condition", coreConditions(core), roles)
	}
}

// An inherited condition names the supertype that declared it, which is what
// makes a conflict in a specialized requirement traceable.
func TestExplainedInheritedConditionNamesItsDeclarer(t *testing.T) {
	solver := requireSolver(t)
	q := requirementQuery(t, `
		package test {
			private import ScalarValues::Integer;
			requirement def Base {
				subject s;
				attribute x : Integer;
				require constraint { x > 10 }
			}
			requirement def Derived :> Base {
				require constraint { x < 1 }
			}
		}`, "test::Derived")
	core := unsatCoreOf(t, solver, q)
	declarers := map[string]*symbols.Symbol{}
	for _, m := range core.Members {
		if m.From.Declared == nil {
			t.Fatalf("core member %q names no declaring element", m.From.Condition)
		}
		declarers[m.From.Condition] = m.From.Declared
	}
	if sym := declarers["x > 10"]; sym == nil || sym.Name != "Base" {
		t.Errorf("the inherited condition is declared by %v, want Base", sym)
	}
	if sym := declarers["x < 1"]; sym == nil || sym.Name != "Derived" {
		t.Errorf("the condition written on Derived is declared by %v, want Derived", sym)
	}
}

// A negated element asserts that its conditions do not all hold, so a tautology
// makes the denial itself the conflict.
func TestExplainedNegatedElement(t *testing.T) {
	solver := requireSolver(t)
	ctx, idx := fixture(t, "<test>", `
		package test {
			private import ScalarValues::Integer;
			constraint def Always {
				in i : Integer;
				assert constraint { i == i }
			}
			part rig {
				assert not constraint always : Always;
			}
		}`)
	sym := symbolNamed(t, idx, "test::rig::always")
	q, err := Constraint(ctx, sym, sym.OwnerScope)
	if err != nil {
		t.Fatalf("translate: %v", err)
	}
	core := unsatCoreOf(t, solver, q)
	if len(core.Members) != 1 || core.Members[0].From.Role != RoleDenied {
		t.Fatalf("core is %v, want the denied conditions", coreConditions(core))
	}
}

// A satisfiable element has no conflict to report, and none is invented for it.
func TestExplainedSatisfiableElementHasNoCore(t *testing.T) {
	solver := requireSolver(t)
	q := constraintQuery(t, constraintSource(`
		in i : Integer;
		assert constraint { i > 5 }
	`), "test::C")
	if result := explained(t, solver, q, StatusSat); result.Core != nil {
		t.Fatalf("a satisfiable query came with a core: %v", coreConditions(result.Core))
	}
}

// Reduction drops the members a conflict does not need: this solver answers unsat
// only while the first assertion is present, so the core shrinks to it.
func TestExplainReducesANonMinimalCore(t *testing.T) {
	q := intQuery(t)
	q.Assertions = append(q.Assertions, q.Assertions[0], q.Assertions[0])
	solver := fakeSolver(t, "core-needs-first")
	result, err := solver.Explain(context.Background(), q)
	if err != nil {
		t.Fatalf("explain: %v", err)
	}
	if result.Status != StatusUnsat {
		t.Fatalf("status %s, want unsat", result.Status)
	}
	core := result.Core
	if got := coreLabels(core); len(got) != 1 || got[0] != CoreLabel(0) {
		t.Fatalf("core is %v, want the one assertion the conflict needs", got)
	}
	if !core.Minimal || core.Note != "" {
		t.Errorf("a shrunk core is reported as %v (%q)", core.Minimal, core.Note)
	}
	if core.Rounds != len(q.Assertions) {
		t.Errorf("shrinking took %d rounds, want one per candidate dropped", core.Rounds)
	}
}

// A core reported as it came says so rather than claiming minimality: a budget
// too small to shrink it leaves it unreduced and noted.
func TestExplainReportsAnUnreducedCoreHonestly(t *testing.T) {
	q := intQuery(t)
	q.Assertions = append(q.Assertions, q.Assertions[0])
	cases := map[string]func(*Solver){
		"budget":  func(s *Solver) { s.CoreBudget = time.Nanosecond },
		"members": func(s *Solver) { s.MaxCoreMembers = 1 },
	}
	for name, configure := range cases {
		t.Run(name, func(t *testing.T) {
			solver := fakeSolver(t, "core-needs-first")
			configure(solver)
			result, err := solver.Explain(context.Background(), q)
			if err != nil {
				t.Fatalf("explain: %v", err)
			}
			core := result.Core
			if len(core.Members) != 2 {
				t.Fatalf("core is %v, want the solver's own two members", coreLabels(core))
			}
			if core.Minimal {
				t.Error("an unreduced core is reported as minimal")
			}
			if core.Note == "" {
				t.Error("an unreduced core does not say why it was not shrunk")
			}
		})
	}
}

// A solver that will not name what conflicts is a failure, never an empty or
// invented core.
func TestExplainCoreFailures(t *testing.T) {
	q := intQuery(t)
	cases := []struct {
		name  string
		reply string
		want  string
	}{
		{"refused", `(error "unsat core is not available")`, "would not report an unsat core"},
		{"not a list", "sy!a0", "rather than a list of assertion names"},
		{"nested", "((sy!a0))", "which the query did not assert"},
		{"unissued label", "(sy!a7)", "which the query did not assert"},
		{"foreign label", "(whatever)", "which the query did not assert"},
		{"repeated label", "(sy!a0 sy!a0)", "names sy!a0 twice"},
		{"empty", "()", "core is empty"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			solver := fakeSolver(t, "unsat")
			solver.Env = append(solver.Env, coreEnv+"="+tc.reply)
			result, err := solver.Explain(context.Background(), q)
			if err == nil {
				t.Fatalf("explain answered %s with core %v, want a failure", result.Status, result.Core)
			}
			if !errors.Is(err, ErrNoCore) || !errors.Is(err, ErrSolverProcess) {
				t.Fatalf("error %v is not an ErrNoCore and an ErrSolverProcess", err)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error %q does not mention %q", err.Error(), tc.want)
			}
		})
	}
}

// Explaining a query no verdict was reached about carries no core, and an
// undecided verdict stays undecided.
func TestExplainWithoutAnUnsatVerdict(t *testing.T) {
	q := intQuery(t)
	cases := map[string]Status{"sat": StatusSat, "unknown": StatusUnknown}
	for scenario, want := range cases {
		t.Run(scenario, func(t *testing.T) {
			solver := fakeSolver(t, scenario)
			result, err := solver.Explain(context.Background(), q)
			if err != nil {
				t.Fatalf("explain: %v", err)
			}
			if result.Status != want || result.Core != nil {
				t.Fatalf("status %s with core %v, want %s and no core", result.Status, result.Core, want)
			}
		})
	}
}

// A solver that fails while a core is being shrunk is reported as the failure it
// is, rather than as a core it did not stand behind.
func TestExplainReportsAFailureWhileShrinking(t *testing.T) {
	q := intQuery(t)
	q.Assertions = append(q.Assertions, q.Assertions[0])
	solver := fakeSolver(t, "core-then-crash")
	if _, err := solver.Explain(context.Background(), q); err == nil {
		t.Fatal("explain answered despite the solver failing mid-reduction")
	} else if !errors.Is(err, ErrSolverProcess) {
		t.Fatalf("error %v is not an ErrSolverProcess", err)
	}
}

// TestCoreBudgetFromEnv: the budget override is read, and a nonsensical one falls
// back to the default rather than leaving reduction unbounded.
func TestCoreBudgetFromEnv(t *testing.T) {
	cases := map[string]time.Duration{
		"":      DefaultCoreBudget,
		"250ms": 250 * time.Millisecond,
		"1m":    time.Minute,
		"-1s":   DefaultCoreBudget,
		"0s":    DefaultCoreBudget,
		"lots":  DefaultCoreBudget,
		" 2s":   2 * time.Second,
	}
	for text, want := range cases {
		t.Setenv(CoreBudgetEnv, text)
		if got := coreBudgetFromEnv(); got != want {
			t.Fatalf("%s=%q gave %s, want %s", CoreBudgetEnv, text, got, want)
		}
	}
}
