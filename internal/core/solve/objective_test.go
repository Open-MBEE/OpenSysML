package solve

import (
	"errors"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// analysisQuery translates one analysis case of the shared objectives fixture.
func analysisQuery(t *testing.T, name string) *Query {
	t.Helper()
	q, err := analysisOf(t, name)
	if err != nil {
		t.Fatalf("translate %s: %v", name, err)
	}
	return q
}

// analysisOf translates an analysis case of the fixture, refusals included.
func analysisOf(t *testing.T, name string) (*Query, error) {
	t.Helper()
	ctx, idx := fixtureFile(t, "objectives.sysml")
	sym := symbolNamed(t, idx, "test::"+name)
	return Analysis(ctx, sym, sym.OwnerScope)
}

// TestObjectiveDirectionAndValue: an objective's direction comes from the
// trade-study definition typing it and its term from the value it states as its
// best, with provenance back to the objective element.
func TestObjectiveDirectionAndValue(t *testing.T) {
	q := analysisQuery(t, "MassBudget")
	if q.Kind != "analysis" || q.Element != "MassBudget" {
		t.Errorf("query is about %s %q, want analysis MassBudget", q.Kind, q.Element)
	}
	if !q.Optimizes() || len(q.Objectives) != 1 {
		t.Fatalf("query states %d objectives, want 1", len(q.Objectives))
	}
	obj := q.Objectives[0]
	if obj.Direction != Minimize || obj.Name != "lightest" {
		t.Errorf("objective is %s %q, want minimize lightest", obj.Direction, obj.Name)
	}
	if got := writeTerm(obj.Term); got != "|test::MassBudget::mass|" {
		t.Errorf("objective term is %s", got)
	}
	if obj.Expression != "mass" {
		t.Errorf("objective expression is %q, want mass", obj.Expression)
	}
	// A quantity-valued objective carries the base units its magnitude is in, so
	// its optimum reads as the notation does.
	if obj.Unit != "gram" || obj.Dimension == "" {
		t.Errorf("objective is in units %q of dimension %q, want grams", obj.Unit, obj.Dimension)
	}
	if obj.Symbol == nil || obj.Symbol.Name != "lightest" {
		t.Errorf("objective has no provenance to the objective element: %+v", obj.Symbol)
	}
	if !strings.HasPrefix(obj.Location, "objectives.sysml:") {
		t.Errorf("objective was written at %q", obj.Location)
	}
}

// TestObjectiveOwnConditions: what is feasible is the case's conditions together
// with each objective's own, and an objective's variables are the query's.
func TestObjectiveOwnConditions(t *testing.T) {
	q := analysisQuery(t, "CrewSizing")
	var roles []string
	for _, a := range q.Assertions {
		if a.From.Role == RoleDomain || a.From.Role == RoleDefined {
			continue
		}
		roles = append(roles, a.From.Role.String()+": "+writeTerm(a.Term))
	}
	want := []string{
		"assumed condition: (<= |test::CrewSizing::crew| 7)",
		"required condition: (>= |test::CrewSizing::crew| 2)",
	}
	if strings.Join(roles, "\n") != strings.Join(want, "\n") {
		t.Errorf("conditions are:\n%s\nwant:\n%s", strings.Join(roles, "\n"), strings.Join(want, "\n"))
	}
	if len(q.Vars) != 1 || q.Vars[0].Name != "test::CrewSizing::crew" {
		t.Errorf("query declares %d variables: %+v", len(q.Vars), q.Vars)
	}
}

// TestObjectiveConditionsFromItsDefinition: a condition a model states on its own
// objective definition bounds the values its objectives improve, the objective's
// inherited `best` being bound to the value its `eval` returns, as the library
// derives it.
func TestObjectiveConditionsFromItsDefinition(t *testing.T) {
	q := analysisQuery(t, "BoundedByItsDefinition")
	script := Script(q)
	best := "|test::BoundedByItsDefinition::lightest.best|"
	mass := "|test::BoundedByItsDefinition::mass|"
	for _, want := range []string{
		"(assert (= " + best + " " + mass + "))",
		"(assert (>= " + best + " 2))",
	} {
		if !strings.Contains(script, want) {
			t.Errorf("script does not assert %s:\n%s", want, script)
		}
	}
}

// TestObjectiveBestShadowsTheCases: a `best` the objective's definition
// conditions is the objective's own even when the case declares a feature of
// that name, which the case's own conditions keep naming.
func TestObjectiveBestShadowsTheCases(t *testing.T) {
	q := analysisQuery(t, "ShadowedBest")
	script := Script(q)
	objBest := "|test::ShadowedBest::lightest.best|"
	caseBest := "|test::ShadowedBest::best|"
	mass := "|test::ShadowedBest::mass|"
	for _, want := range []string{
		"(assert (= " + objBest + " " + mass + "))",
		"(assert (>= " + objBest + " 2))",
		"(<= " + caseBest + " 1)",
	} {
		if !strings.Contains(script, want) {
			t.Errorf("script does not assert %s:\n%s", want, script)
		}
	}
	if strings.Contains(script, "(>= "+caseBest+" 2)") {
		t.Errorf("the definition's condition binds the case's best:\n%s", script)
	}
}

// TestObjectiveConditionLocalIsNotShadowed: a `best` a condition nested in the
// objective declares for itself is that condition's, not the objective's nor the
// case's; a sibling condition without one still reads the objective's.
func TestObjectiveConditionLocalIsNotShadowed(t *testing.T) {
	q := analysisQuery(t, "LocalBest")
	script := Script(q)
	objBest := "|test::LocalBest::lightest.best|"
	localBest := "|test::LocalBest::lightest::tight::best|"
	for _, want := range []string{
		"(assert (>= " + localBest + " 6))",
		"(assert (>= " + objBest + " 3))",
		"(assert (>= " + objBest + " 2))",
		"(assert (= " + objBest + " |test::LocalBest::mass|))",
	} {
		if !strings.Contains(script, want) {
			t.Errorf("script does not assert %s:\n%s", want, script)
		}
	}
	for _, wrong := range []string{"(>= " + objBest + " 6)", "(>= |test::LocalBest::best| 6)"} {
		if strings.Contains(script, wrong) {
			t.Errorf("the condition's own best is read as another's %s:\n%s", wrong, script)
		}
	}
}

// TestObjectiveEvalReadsTheObjectivesOwnMember: the evaluation's `weight` is the
// objective's own attribute, which shadows the case's of the same name.
func TestObjectiveEvalReadsTheObjectivesOwnMember(t *testing.T) {
	q := analysisQuery(t, "OwnWeight")
	script := Script(q)
	own := "|test::OwnWeight::lightest.weight|"
	if want := "(minimize " + own + ")"; !strings.Contains(script, want) {
		t.Errorf("script does not %s:\n%s", want, script)
	}
	if wrong := "(minimize |test::OwnWeight::weight|)"; strings.Contains(script, wrong) {
		t.Errorf("the objective improves the case's weight %s:\n%s", wrong, script)
	}
	if want := "(assert (= " + own + " (+ |test::OwnWeight::mass| 1)))"; !strings.Contains(script, want) {
		t.Errorf("script does not assert %s:\n%s", want, script)
	}
}

// TestObjectivesInDeclarationOrder: objectives are optimized in the order
// declared, which is what makes them lexicographic.
func TestObjectivesInDeclarationOrder(t *testing.T) {
	q := analysisQuery(t, "CostThenMargin")
	var got []string
	for _, obj := range q.Objectives {
		got = append(got, obj.Direction.String()+" "+obj.Name)
	}
	want := "minimize cheapest, maximize widestMargin"
	if strings.Join(got, ", ") != want {
		t.Errorf("objectives are %q, want %q", strings.Join(got, ", "), want)
	}
}

// TestObjectiveVariablesAreDeclared: a variable only an objective mentions is
// declared, bounded by its domain and reachable through the query's logic.
func TestObjectiveVariablesAreDeclared(t *testing.T) {
	ctx, idx := fixture(t, "objective_only.sysml", `
		package test {
			private import ScalarValues::*;
			private import TradeStudies::*;
			analysis def SpareRoom {
				attribute used : Integer;
				attribute spare : Natural;
				require constraint { used <= 3 }
				objective roomiest : MaximizeObjective {
					subject :>> selectedAlternative;
					in calc :>> eval { spare }
				}
			}
		}`)
	sym := symbolNamed(t, idx, "test::SpareRoom")
	q, err := Analysis(ctx, sym, sym.OwnerScope)
	if err != nil {
		t.Fatalf("translate: %v", err)
	}
	var names []string
	for _, v := range q.Vars {
		names = append(names, v.Name)
	}
	if strings.Join(names, ", ") != "test::SpareRoom::spare, test::SpareRoom::used" {
		t.Errorf("query declares %q", strings.Join(names, ", "))
	}
	// The Natural's domain is asserted although only the objective mentions it.
	script := Script(q)
	if !strings.Contains(script, "(>= |test::SpareRoom::spare| 0)") {
		t.Errorf("the objective's variable is unbounded below:\n%s", script)
	}
	if !strings.Contains(script, "(declare-const |test::SpareRoom::spare| Int)") {
		t.Errorf("the objective's variable is not declared:\n%s", script)
	}
}

// TestObjectiveDecidesLogic: an objective's term counts towards the logic the
// script declares, as an assertion's does.
func TestObjectiveDecidesLogic(t *testing.T) {
	ctx, idx := fixture(t, "objective_logic.sysml", `
		package test {
			private import ScalarValues::*;
			private import TradeStudies::*;
			analysis def MixedSorts {
				attribute count : Integer;
				attribute ratio : Real;
				require constraint { count >= 2 }
				objective finest : MinimizeObjective {
					subject :>> selectedAlternative;
					in calc :>> eval { ratio }
				}
			}
		}`)
	sym := symbolNamed(t, idx, "test::MixedSorts")
	q, err := Analysis(ctx, sym, sym.OwnerScope)
	if err != nil {
		t.Fatalf("translate: %v", err)
	}
	// Without the objective the query is integer arithmetic; the real-valued
	// objective is what makes it mixed.
	if got := q.Logic(); got != "AUFLIRA" {
		t.Errorf("logic is %s, want AUFLIRA", got)
	}
}

// TestQuantityObjectiveIsNormalized: a quantity-valued objective is optimized
// over the same base magnitudes the conditions are translated to.
func TestQuantityObjectiveIsNormalized(t *testing.T) {
	q := analysisQuery(t, "MassBudget")
	script := Script(q)
	if !strings.Contains(script, "(minimize |test::MassBudget::mass|)") {
		t.Errorf("objective is not over the mass magnitude:\n%s", script)
	}
	// The conditions are in grams, so the objective's optimum is too.
	if !strings.Contains(script, "10000") || !strings.Contains(script, "90000") {
		t.Errorf("conditions are not normalized to grams:\n%s", script)
	}
}

// TestObjectiveScriptIsExplicitlyLexicographic: the script says how several
// objectives relate rather than leaving it to the backend's default, and says it
// once, before the objectives it governs.
func TestObjectiveScriptIsExplicitlyLexicographic(t *testing.T) {
	script := Script(analysisQuery(t, "CostThenMargin"))
	option := "(set-option :opt.priority lex)"
	if strings.Count(script, option) != 1 {
		t.Fatalf("the script states the objective priority %d times:\n%s", strings.Count(script, option), script)
	}
	if strings.Index(script, option) > strings.Index(script, "(minimize ") {
		t.Errorf("the priority is stated after the objectives:\n%s", script)
	}
	// Declaration order is optimization order.
	if strings.Index(script, "(minimize ") > strings.Index(script, "(maximize ") {
		t.Errorf("objectives are not emitted in declaration order:\n%s", script)
	}
}

// TestSingleObjectiveScriptStatesPriority: one objective states the priority too,
// so a second one added to the model does not change how the first is read.
func TestSingleObjectiveScriptStatesPriority(t *testing.T) {
	if script := Script(analysisQuery(t, "MassBudget")); !strings.Contains(script, "(set-option :opt.priority lex)") {
		t.Errorf("a single objective's script states no priority:\n%s", script)
	}
}

// TestObjectiveFreeScriptsAreUnchanged: a query stating no objective writes no
// optimization commands, so the scripts of every other question are as they were.
func TestObjectiveFreeScriptsAreUnchanged(t *testing.T) {
	ctx, idx := fixtureFile(t, "mission_budget.sysml")
	sym := symbolNamed(t, idx, "test::BudgetConstraint")
	q, err := Constraint(ctx, sym, sym.OwnerScope)
	if err != nil {
		t.Fatalf("translate: %v", err)
	}
	if q.Optimizes() {
		t.Fatal("a constraint query states objectives")
	}
	script := Script(q)
	for _, command := range []string{"opt.priority", "(minimize ", "(maximize "} {
		if strings.Contains(script, command) {
			t.Errorf("an objective-free script writes %s:\n%s", command, script)
		}
	}
}

// TestObjectiveOverVariantSelection: an objective may improve a value a variant
// selection decides, which makes the optimum name a configuration.
func TestObjectiveOverVariantSelection(t *testing.T) {
	q := analysisQuery(t, "WheelChoice")
	if len(q.Objectives) != 1 {
		t.Fatalf("query states %d objectives, want 1", len(q.Objectives))
	}
	term := writeTerm(q.Objectives[0].Term)
	if !strings.Contains(term, "ite ") || !strings.Contains(term, "rim") {
		t.Errorf("objective term is %s, want one deciding on the rim variant", term)
	}
}

// TestObjectiveWithGuardedDivision: a case whose conditions divide by a computed
// divisor keeps its divisor guards, the objective itself staying linear.
func TestObjectiveWithGuardedDivision(t *testing.T) {
	q := analysisQuery(t, "GuardedRatio")
	script := Script(q)
	if !strings.Contains(script, "(distinct |test::GuardedRatio::parts| 0)") {
		t.Errorf("the divisor is not guarded:\n%s", script)
	}
	if got := writeTerm(q.Objectives[0].Term); got != "|test::GuardedRatio::parts|" {
		t.Errorf("objective term is %s", got)
	}
	if !q.Nonlinear {
		t.Error("a query dividing by a variable is not marked nonlinear")
	}
}

// TestDerivedAnalysisObjectivesRedefineByPosition: an analysis definition or
// usage restating its general's several objectives without naming them takes
// each one's direction at the same position, and the restatement stands in for
// the objective it redefines rather than beside it (which, being valueless,
// would refuse the whole analysis). A general restating only its first objective
// still hands its second one down at that position.
func TestDerivedAnalysisObjectivesRedefineByPosition(t *testing.T) {
	ctx, idx := fixture(t, "derived_objectives.sysml", `package test {
		private import ScalarValues::*;
		private import TradeStudies::*;
		analysis def Base {
			attribute cost : Integer;
			attribute margin : Integer;
			require constraint { cost >= 1 and cost <= 9 }
			require constraint { margin >= 0 and margin <= cost }
			objective cheapest : MinimizeObjective;
			objective widestMargin : MaximizeObjective;
		}
		analysis def Derived :> Base {
			objective { in calc :>> eval { cost + 1 } }
			objective { in calc :>> eval { margin } }
		}
		analysis d : Base {
			objective { in calc :>> eval { cost + 1 } }
			objective { in calc :>> eval { margin } }
		}
		analysis def Mid :> Base { objective { in calc :>> eval { cost } } }
		analysis def ViaMid :> Mid {
			objective { in calc :>> eval { cost + 1 } }
			objective { in calc :>> eval { margin } }
		}
		analysis viaMid : Mid {
			objective { in calc :>> eval { cost + 1 } }
			objective { in calc :>> eval { margin } }
		}
	}`)
	for _, name := range []string{"test::Derived", "test::d", "test::ViaMid", "test::viaMid"} {
		sym := symbolNamed(t, idx, name)
		objectives := ctx.ObjectivesOf(sym, sym.OwnerScope)
		if len(objectives) != 2 {
			t.Fatalf("%s states %d objectives, want its 2 restatements: %+v", name, len(objectives), objectives)
		}
		want := []struct {
			direction runtime.ObjectiveDirection
			typ, text string
		}{{runtime.Minimize, "MinimizeObjective", "cost + 1"}, {runtime.Maximize, "MaximizeObjective", "margin"}}
		for i, w := range want {
			got := objectives[i]
			if got.Direction != w.direction || got.Type == nil || got.Type.Name != w.typ || got.Text() != w.text {
				t.Errorf("%s objective %d = %v %v %q, want %v %s %q", name, i, got.Direction, got.Type, got.Text(), w.direction, w.typ, w.text)
			}
		}
		q, err := Analysis(ctx, sym, sym.OwnerScope)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		script := Script(q)
		for _, line := range []string{"(minimize (+ |test::Base::cost| 1))", "(maximize |test::Base::margin|)"} {
			if strings.Count(script, line) != 1 {
				t.Errorf("%s script states %q %d times, want once:\n%s", name, line, strings.Count(script, line), script)
			}
		}
	}
}

// TestPositionalObjectivesInheritValues: a positional restatement stating no
// `eval` of its own keeps the inherited value, also through a chain of them,
// so the analysis solves instead of being refused as valueless.
func TestPositionalObjectivesInheritValues(t *testing.T) {
	ctx, idx := fixture(t, "positional_objectives.sysml", `package test {
		private import ScalarValues::*;
		private import TradeStudies::*;
		analysis def Base {
			attribute cost : Integer;
			attribute margin : Integer;
			require constraint { cost >= 1 and cost <= 9 }
			require constraint { margin >= 0 and margin <= cost }
			objective cheapest : MinimizeObjective { in calc :>> eval { cost } }
			objective widestMargin : MaximizeObjective { in calc :>> eval { margin } }
		}
		analysis def Derived :> Base { objective; }
		analysis def Twice :> Derived {
			objective;
			objective { in calc :>> eval { margin - 1 } }
		}
		analysis d : Twice { objective : MinimizeObjective; }
	}`)
	for name, want := range map[string][]string{
		"test::Derived": {"(minimize |test::Base::cost|)", "(maximize |test::Base::margin|)"},
		"test::Twice":   {"(minimize |test::Base::cost|)", "(maximize (- |test::Base::margin| 1))"},
		"test::d":       {"(minimize |test::Base::cost|)", "(maximize (- |test::Base::margin| 1))"},
	} {
		sym := symbolNamed(t, idx, name)
		if objectives := ctx.ObjectivesOf(sym, sym.OwnerScope); len(objectives) != 2 || objectives[0].Name != "cheapest" || objectives[1].Name != "widestMargin" {
			t.Fatalf("%s states %+v, want cheapest then widestMargin", name, objectives)
		}
		q, err := Analysis(ctx, sym, sym.OwnerScope)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		script := Script(q)
		for _, line := range want {
			if strings.Count(script, line) != 1 {
				t.Errorf("%s script states %q %d times, want once:\n%s", name, line, strings.Count(script, line), script)
			}
		}
	}
}

// TestObjectiveRefusals: every objective outside the translatable subset refuses
// with a typed error naming why and where, rather than being skipped.
func TestObjectiveRefusals(t *testing.T) {
	cases := []struct {
		element  string
		sentinel error
		says     []string
	}{
		{"NonlinearGain", ErrNotOptimizable, []string{"objective bestGain", "nonlinear", "objectives.sysml:"}},
		// The nonlinearity is judged of the objective, not of the conditions it
		// is optimized within, which are nonlinear here too.
		{"GuardedNonlinearGain", ErrNotOptimizable, []string{"objective bestGain", "nonlinear"}},
		{"UndirectedGoal", ErrNotOptimizable, []string{"objective goal", "no direction",
			"TradeStudies::MinimizeObjective"}},
		{"ValuelessGoal", ErrNotOptimizable, []string{"objective goal", "no value", "in calc :>> eval { expression }"}},
		// The old spelling is a validation error, so it is refused rather than
		// read as the value; the refusal points at the spelling that is not.
		{"ReboundBest", ErrNotOptimizable, []string{"objective goal", "bound `best`", "validation rejects",
			"in calc :>> eval { expression }"}},
		{"StepwiseGoal", ErrNotOptimizable, []string{"objective goal", "in steps", "in calc :>> eval { expression }"}},
		// A trade study over listed alternatives is the analysis run's question:
		// the refusal points there rather than guessing a continuous domain.
		{"ListedAlternatives", ErrNotOptimizable, []string{"objective tradeStudyObjective",
			"`evaluationFunction` to each alternative", "listed alternatives", "%analysis"}},
		{"NoGoal", ErrNoObjective, []string{"NoGoal", "no objective"}},
	}
	for _, tc := range cases {
		t.Run(tc.element, func(t *testing.T) {
			q, err := analysisOf(t, tc.element)
			if err == nil {
				t.Fatalf("translated an untranslatable objective:\n%s", Script(q))
			}
			if !errors.Is(err, tc.sentinel) {
				t.Errorf("error is %v, want one of %v", err, tc.sentinel)
			}
			for _, want := range tc.says {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("error %q does not say %q", err, want)
				}
			}
		})
	}
}

// TestAnalysisRefusesANonAnalysis: only an analysis case states objectives.
func TestAnalysisRefusesANonAnalysis(t *testing.T) {
	ctx, idx := fixtureFile(t, "objectives.sysml")
	sym := symbolNamed(t, idx, "test::Wheel")
	if _, err := Analysis(ctx, sym, sym.OwnerScope); err == nil {
		t.Fatal("translated a part definition as an analysis case")
	} else if !strings.Contains(err.Error(), "analysis") {
		t.Errorf("error is %v", err)
	}
}

// TestAnalysisWithoutContext: no context is a refusal rather than a panic.
func TestAnalysisWithoutContext(t *testing.T) {
	if _, err := Analysis(nil, &symbols.Symbol{Name: "A"}, nil); err == nil {
		t.Fatal("translated an analysis case without a runtime context")
	}
}

// TestAnalysisWithPins: values already fixed bound what the optimum may be, so an
// optimization can ask for the best value consistent with a configuration.
func TestAnalysisWithPins(t *testing.T) {
	ctx, idx := fixtureFile(t, "objectives.sysml")
	sym := symbolNamed(t, idx, "test::CostThenMargin")
	q, err := Analysis(ctx, sym, sym.OwnerScope)
	if err != nil {
		t.Fatalf("translate: %v", err)
	}
	pinned, err := AnalysisWith(ctx, sym, sym.OwnerScope, []Pin{{
		Feature: q.Vars[0].Symbol,
		Name:    q.Vars[0].Name,
		Value:   runtime.Value{Kind: runtime.ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: 5}},
		Source:  PinChosen,
	}})
	if err != nil {
		t.Fatalf("translate with pins: %v", err)
	}
	if len(pinned.Assertions) != len(q.Assertions)+1 {
		t.Errorf("pinning asserted %d terms, want %d", len(pinned.Assertions), len(q.Assertions)+1)
	}
	if len(pinned.Objectives) != len(q.Objectives) {
		t.Errorf("pinning changed the objectives: %+v", pinned.Objectives)
	}
}

// TestAnalysisRefusesConflictingResultExpressions: an analysis stating a result
// expression over an inherited one, or inheriting two, is refused where validation
// rejects it, whether or not it states an objective; a goalless case whose
// condition is merely untranslatable still lacks an objective first.
func TestAnalysisRefusesConflictingResultExpressions(t *testing.T) {
	ctx, idx := fixtureDocuments(t,
		document{"base.sysml", `
			package base {
				private import ScalarValues::*;
				private import TradeStudies::*;
				analysis def Base {
					attribute size : Integer;
					objective roomiest : MaximizeObjective {
						subject :>> selectedAlternative;
						in calc :>> eval { size }
					}
					size >= 1
				}
				analysis def Other { attribute size : Integer; size <= 9 }
				analysis def Goalless { attribute size : Integer; size >= 1 }
			}
		`},
		document{"sub.sysml", `
			package sub {
				private import base::*;
				analysis def Stated :> Base { size <= 4 }
				analysis def Inherited :> Base, Other;
				analysis def Kept :> Base;
				analysis def GoallessStated :> Goalless { size <= 4 }
				analysis def GoallessFlowed {
					attribute size : Integer;
					require constraint { action a; size <= 3 }
				}
			}
		`})
	sym := symbolNamed(t, idx, "sub::Kept")
	if _, err := Analysis(ctx, sym, sym.OwnerScope); err != nil {
		t.Fatalf("translating sub::Kept: %v", err)
	}
	sym = symbolNamed(t, idx, "sub::GoallessFlowed")
	if _, err := Analysis(ctx, sym, sym.OwnerScope); !errors.Is(err, ErrNoObjective) {
		t.Errorf("translating sub::GoallessFlowed: %v, want no objective", err)
	}
	for _, tc := range []struct{ name, location string }{
		{"sub::Stated", "sub.sysml:4:35"},
		{"sub::Inherited", "sub.sysml:5:5"},
		{"sub::GoallessStated", "sub.sysml:7:47"},
	} {
		sym := symbolNamed(t, idx, tc.name)
		q, err := Analysis(ctx, sym, sym.OwnerScope)
		var refused *NotTranslatableError
		if q != nil || !errors.As(err, &refused) {
			t.Fatalf("translating %s: query %v, err %v; want a refusal", tc.name, q, err)
		}
		if refused.Construct != "conflicting result expression" {
			t.Errorf("%s: refusal names %q", tc.name, refused.Construct)
		}
		if refused.File != "sub.sysml" || refused.Location != tc.location {
			t.Errorf("%s: refusal records file %q at %q, want sub.sysml at %s", tc.name, refused.File, refused.Location, tc.location)
		}
	}
}

// TestCaseStepsAreNotBodyStatements: a case's own action steps are its procedure,
// so an analysis stating them still translates; a step inside a required
// constraint's body is refused as any body statement is.
func TestCaseStepsAreNotBodyStatements(t *testing.T) {
	ctx, idx := fixture(t, "case_steps.sysml", `
		package test {
			private import ScalarValues::*;
			private import TradeStudies::*;
			analysis def Stepped {
				attribute used : Integer;
				attribute spare : Natural;
				action measure; action compare;
				first measure then compare;
				perform action tally : Tally;
				require constraint { used <= 3 }
				objective roomiest : MaximizeObjective {
					subject :>> selectedAlternative;
					in calc :>> eval { spare }
				}
			}
			action def Tally;
			analysis def Flowed {
				attribute used : Integer;
				require constraint { action a; used <= 3 }
				objective roomiest : MaximizeObjective {
					subject :>> selectedAlternative;
					in calc :>> eval { used }
				}
			}
		}`)
	sym := symbolNamed(t, idx, "test::Stepped")
	if _, err := Analysis(ctx, sym, sym.OwnerScope); err != nil {
		t.Fatalf("translate: %v", err)
	}
	sym = symbolNamed(t, idx, "test::Flowed")
	_, err := Analysis(ctx, sym, sym.OwnerScope)
	var refused *NotTranslatableError
	if !errors.As(err, &refused) {
		t.Fatalf("translate: %v, want a refusal", err)
	}
	if refused.Construct != "body statement" || refused.Condition != "`action` statement" {
		t.Errorf("refused %q (%q), want the body statement's action", refused.Construct, refused.Condition)
	}
}
