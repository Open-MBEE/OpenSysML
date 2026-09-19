package runtime

import (
	"errors"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// analysisFixture builds a runtime over the standard library, which is what makes
// the trade-study objective definitions resolve, and returns the package scope.
func analysisFixture(t *testing.T, src string) (*Context, *symbols.Scope) {
	t.Helper()
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
	pkg, ok := idx.DocumentRoot("<test>").LookupLocal("test")
	if !ok || pkg.Scope == nil {
		t.Fatal("test package not indexed")
	}
	return ctx, pkg.Scope
}

// objectivesOfCase returns the objectives of the named analysis case.
func objectivesOfCase(t *testing.T, ctx *Context, scope *symbols.Scope, name string) []Objective {
	t.Helper()
	sym := requirementNamed(t, scope, name)
	if err := RequireAnalysis(sym); err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	return ctx.ObjectivesOf(sym, sym.OwnerScope)
}

// visibleIn reports whether the name is declared in the scope or one enclosing it.
func visibleIn(scope *symbols.Scope, name string) bool {
	for ; scope != nil; scope = scope.Parent() {
		if _, ok := scope.LookupLocal(name); ok {
			return true
		}
	}
	return false
}

// objectiveLabels renders each objective as its direction, name and value, so one
// comparison covers order, direction and what is improved.
func objectiveLabels(objs []Objective) string {
	parts := make([]string, 0, len(objs))
	for _, obj := range objs {
		parts = append(parts, obj.Direction.String()+" "+obj.Name+" = "+obj.Text())
	}
	return strings.Join(parts, "; ")
}

// An objective's direction comes from the trade-study definition typing it, and
// the value it improves from the expression its redefinition of the library's
// `eval` calculation returns. Objectives stand in declaration order.
func TestObjectivesOfDirectionValueAndOrder(t *testing.T) {
	ctx, scope := analysisFixture(t, `
		package test {
			private import ScalarValues::*;
			private import TradeStudies::*;
			analysis def Trade {
				attribute cost : Integer;
				attribute margin : Integer;
				objective cheapest : MinimizeObjective {
					subject :>> selectedAlternative;
					in calc :>> eval { cost }
				}
				objective widest : MaximizeObjective {
					subject :>> selectedAlternative;
					in calc :>> eval { margin }
				}
			}
		}
	`)
	got := objectiveLabels(objectivesOfCase(t, ctx, scope, "Trade"))
	want := "minimize cheapest = cost; maximize widest = margin"
	if got != want {
		t.Errorf("objectives are [%s], want [%s]", got, want)
	}
}

// A definition specializing MinimizeObjective states a direction too, so a
// project's own objective definitions are read as the library's are.
func TestObjectivesOfDirectionThroughSpecialization(t *testing.T) {
	ctx, scope := analysisFixture(t, `
		package test {
			private import ScalarValues::*;
			private import TradeStudies::*;
			requirement def LeastMass :> MinimizeObjective;
			analysis def Trade {
				attribute mass : Integer;
				objective lightest : LeastMass {
					subject :>> selectedAlternative;
					in calc :>> eval { mass }
				}
			}
		}
	`)
	objs := objectivesOfCase(t, ctx, scope, "Trade")
	if len(objs) != 1 || objs[0].Direction != Minimize {
		t.Fatalf("objectives are [%s], want a minimizing one", objectiveLabels(objs))
	}
	if objs[0].Type == nil || objs[0].Type.Name != "LeastMass" {
		t.Errorf("objective is typed by %v, want LeastMass", objs[0].Type)
	}
}

// An objective typed by neither Minimize nor MaximizeObjective states no
// direction: the caller refuses rather than guessing one.
func TestObjectivesOfWithoutDirection(t *testing.T) {
	ctx, scope := analysisFixture(t, `
		package test {
			private import ScalarValues::*;
			private import TradeStudies::*;
			requirement def FitsWell :> TradeStudyObjective;
			analysis def Trade {
				attribute size : Integer;
				objective goal : FitsWell {
					subject :>> selectedAlternative;
					in calc :>> eval { size }
				}
			}
		}
	`)
	objs := objectivesOfCase(t, ctx, scope, "Trade")
	if len(objs) != 1 || objs[0].Direction != NoDirection {
		t.Fatalf("objectives are [%s], want one without a direction", objectiveLabels(objs))
	}
	if objs[0].Direction.String() != "unstated" {
		t.Errorf("the direction reads %q", objs[0].Direction.String())
	}
}

// An objective stating no value has none: nothing is invented for it.
func TestObjectivesOfWithoutValue(t *testing.T) {
	ctx, scope := analysisFixture(t, `
		package test {
			private import ScalarValues::*;
			private import TradeStudies::*;
			analysis def Trade {
				attribute size : Integer;
				objective goal : MinimizeObjective;
			}
		}
	`)
	objs := objectivesOfCase(t, ctx, scope, "Trade")
	if len(objs) != 1 || objs[0].Value != nil || objs[0].Text() != "" {
		t.Fatalf("objectives are [%s], want one stating no value", objectiveLabels(objs))
	}
}

// The expression `eval` returns is the objective's value, and its names resolve
// in the scope stating it, which sees the case's attributes.
func TestObjectivesOfValueScope(t *testing.T) {
	ctx, scope := analysisFixture(t, `
		package test {
			private import ScalarValues::*;
			private import TradeStudies::*;
			analysis def Trade {
				attribute total : Integer;
				objective cheapest : MinimizeObjective {
					subject :>> selectedAlternative;
					in calc :>> eval { total + 1 }
				}
			}
		}
	`)
	objs := objectivesOfCase(t, ctx, scope, "Trade")
	if len(objs) != 1 || objs[0].Text() != "total + 1" {
		t.Fatalf("objectives are [%s], want one improving total + 1", objectiveLabels(objs))
	}
	if objs[0].Scope == nil {
		t.Fatal("the objective's value resolves its names in no scope")
	}
	if !visibleIn(objs[0].Scope, "total") {
		t.Error("the objective's value does not see the case's attributes")
	}
}

// An `eval` naming its result explicitly states the same value as a bare body.
func TestObjectivesOfExplicitResult(t *testing.T) {
	ctx, scope := analysisFixture(t, `
		package test {
			private import ScalarValues::*;
			private import TradeStudies::*;
			analysis def Trade {
				attribute total : Integer;
				objective cheapest : MinimizeObjective {
					subject :>> selectedAlternative;
					in calc :>> eval { return :>> result = total * 2; }
				}
			}
		}
	`)
	objs := objectivesOfCase(t, ctx, scope, "Trade")
	if len(objs) != 1 || objs[0].Text() != "total * 2" || objs[0].Eval == nil {
		t.Fatalf("objectives are [%s], want one improving total * 2 through eval", objectiveLabels(objs))
	}
}

// An `eval` computing in steps states no single expression: the objective is
// recorded as stepwise and given no value, rather than a guessed one.
func TestObjectivesOfStepwiseEval(t *testing.T) {
	ctx, scope := analysisFixture(t, `
		package test {
			private import ScalarValues::*;
			private import TradeStudies::*;
			analysis def Trade {
				attribute total : Integer;
				objective cheapest : MinimizeObjective {
					subject :>> selectedAlternative;
					in calc :>> eval {
						attribute doubled : Integer = total * 2;
						return :>> result = doubled;
					}
				}
			}
		}
	`)
	objs := objectivesOfCase(t, ctx, scope, "Trade")
	if len(objs) != 1 || objs[0].Value != nil || !objs[0].StepwiseEval || objs[0].Eval == nil {
		t.Fatalf("objectives are [%s], want one stepwise eval stating no value", objectiveLabels(objs))
	}
}

// An objective giving the library's bound `best` a value of its own is the
// spelling validation rejects: it is recorded as such and never read as the value.
func TestObjectivesOfReboundBest(t *testing.T) {
	ctx, scope := analysisFixture(t, `
		package test {
			private import ScalarValues::*;
			private import TradeStudies::*;
			analysis def Trade {
				attribute total : Integer;
				objective cheapest : MinimizeObjective {
					attribute :>> best = total;
				}
			}
		}
	`)
	objs := objectivesOfCase(t, ctx, scope, "Trade")
	if len(objs) != 1 || objs[0].Value != nil || objs[0].ReboundBest == nil {
		t.Fatalf("objectives are [%s], want one rebinding best and stating no value", objectiveLabels(objs))
	}
	if objs[0].Best == nil || objs[0].Best != objs[0].ReboundBest {
		t.Errorf("the rebound feature %v is not the objective's best %v", objs[0].ReboundBest, objs[0].Best)
	}
}

// An objective's own conditions are its own: the library conditions it inherits
// are about choosing among alternatives, not about which values are feasible.
func TestObjectivesOfOwnConditions(t *testing.T) {
	ctx, scope := analysisFixture(t, `
		package test {
			private import ScalarValues::*;
			private import TradeStudies::*;
			analysis def Trade {
				attribute crew : Integer;
				objective largest : MaximizeObjective {
					subject :>> selectedAlternative;
					in calc :>> eval { crew }
					require constraint { crew >= 2 }
					assume constraint { crew <= 7 }
				}
			}
		}
	`)
	objs := objectivesOfCase(t, ctx, scope, "Trade")
	if len(objs) != 1 {
		t.Fatalf("objectives are [%s], want one", objectiveLabels(objs))
	}
	if got, want := labelsOf(objs[0].Conditions), "required crew >= 2; assumed crew <= 7"; got != want {
		t.Errorf("the objective's own conditions are [%s], want [%s]", got, want)
	}
}

// A condition a model states on its own objective definition is the model's, so
// an objective typed by that definition states it too: only the library's own
// trade-study conditions are about choosing among alternatives.
func TestObjectivesOfInheritedProjectConditions(t *testing.T) {
	ctx, scope := analysisFixture(t, `
		package test {
			private import ScalarValues::*;
			private import TradeStudies::*;
			requirement def LeastMass :> MinimizeObjective {
				attribute :>> best;
				require constraint { best > 0 }
			}
			analysis def Trade {
				attribute mass : Integer;
				objective lightest : LeastMass {
					subject :>> selectedAlternative;
					in calc :>> eval { mass }
					require constraint { mass <= 90 }
				}
			}
		}
	`)
	objs := objectivesOfCase(t, ctx, scope, "Trade")
	if len(objs) != 1 {
		t.Fatalf("objectives are [%s], want one", objectiveLabels(objs))
	}
	if got, want := labelsOf(objs[0].Conditions), "required best > 0; required mass <= 90"; got != want {
		t.Errorf("the objective's conditions are [%s], want [%s]", got, want)
	}
}

// An objective restating an inherited one, by redefinition or by name, is the same
// objective declared again: it keeps the inherited place in the lexicographic
// order and takes the value it states, whichever general it is inherited through.
// Its `eval` binds the inherited result rather than stating a second result
// expression, which the pilot rejects.
func TestObjectivesOfRedeclared(t *testing.T) {
	ctx, scope := analysisFixture(t, `
		package test {
			private import ScalarValues::*;
			private import TradeStudies::*;
			analysis def Base {
				attribute cost : Integer;
				attribute margin : Integer;
				objective cheapest : MinimizeObjective {
					subject :>> selectedAlternative;
					in calc :>> eval { cost }
				}
				objective widest : MaximizeObjective {
					subject :>> selectedAlternative;
					in calc :>> eval { margin }
				}
			}
			analysis def Refined :> Base {
				objective :>> cheapest : MinimizeObjective {
					subject :>> selectedAlternative;
					in calc :>> eval { return :>> result = cost + 1; }
				}
			}
			analysis def Renamed :> Base {
				objective cheapest : MinimizeObjective {
					subject :>> selectedAlternative;
					in calc :>> eval { return :>> result = cost + 2; }
				}
			}
			analysis def Other :> Base;
			analysis def Diamond :> Other, Refined;
			analysis def Reversed :> Refined, Other;
			analysis def Positional :> Base {
				objective;
			}
			analysis def Twice :> Positional {
				objective { subject :>> selectedAlternative; }
			}
			analysis def Narrowed :> Refined {
				objective : MinimizeObjective;
			}
		}
	`)
	for _, tc := range []struct{ name, want string }{
		{"Refined", "minimize cheapest = cost + 1; maximize widest = margin"},
		{"Renamed", "minimize cheapest = cost + 2; maximize widest = margin"},
		{"Diamond", "minimize cheapest = cost + 1; maximize widest = margin"},
		{"Reversed", "minimize cheapest = cost + 1; maximize widest = margin"},
		// A positional restatement stating no `eval` keeps the inherited value.
		{"Positional", "minimize cheapest = cost; maximize widest = margin"},
		{"Twice", "minimize cheapest = cost; maximize widest = margin"},
		{"Narrowed", "minimize cheapest = cost + 1; maximize widest = margin"},
	} {
		got := objectiveLabels(objectivesOfCase(t, ctx, scope, tc.name))
		if got != tc.want {
			t.Errorf("%s: objectives are [%s], want [%s]", tc.name, got, tc.want)
		}
	}
}

// The conditions a case holds true of its parameters are its own members' and
// those of the constraints it requires, assumes or asserts, in evaluator order.
func TestCaseConditionsOf(t *testing.T) {
	ctx, scope := analysisFixture(t, `
		package test {
			private import ScalarValues::*;
			private import TradeStudies::*;
			constraint def Positive {
				attribute size : Integer;
				assert constraint { size > 0 }
			}
			analysis def Trade {
				attribute size : Integer;
				attribute spare : Integer;
				assume constraint { size <= 9 }
				require constraint { spare >= 1 }
				require limit : Positive { attribute :>> size = size; }
				constraint unchecked { spare < 100 }
			}
		}
	`)
	sym := requirementNamed(t, scope, "Trade")
	got := labelsOf(ctx.CaseConditionsOf(sym, sym.OwnerScope))
	// The constraint the case merely declares states nothing it checks.
	want := "assumed size <= 9; required spare >= 1; required size > 0"
	if got != want {
		t.Errorf("conditions are [%s], want [%s]", got, want)
	}
}

// A case inheriting conditions states them too, inherited ones first, as the
// evaluator checks them.
func TestCaseConditionsOfInherited(t *testing.T) {
	ctx, scope := analysisFixture(t, `
		package test {
			private import ScalarValues::*;
			private import TradeStudies::*;
			analysis def Base {
				attribute size : Integer;
				require constraint { size >= 1 }
			}
			analysis def Refined :> Base {
				require constraint { size <= 4 }
			}
		}
	`)
	sym := requirementNamed(t, scope, "Refined")
	if got, want := labelsOf(ctx.CaseConditionsOf(sym, sym.OwnerScope)),
		"required size >= 1; required size <= 4"; got != want {
		t.Errorf("conditions are [%s], want [%s]", got, want)
	}
}

// A case stating a result expression over an inherited one, or inheriting two,
// records the conflict ahead of its required conditions; inheriting one does not.
func TestCaseConditionsOfConflict(t *testing.T) {
	ctx, scope := analysisFixture(t, `
		package test {
			private import ScalarValues::*;
			private import TradeStudies::*;
			analysis def Base {
				attribute size : Integer;
				require constraint { size >= 1 }
				size > 0
			}
			analysis def Other {
				attribute size : Integer;
				size <= 9
			}
			analysis def Stated :> Base { size <= 4 }
			analysis def Inherited :> Base, Other;
			analysis def Kept :> Base;
		}
	`)
	for _, tc := range []struct{ name, want string }{
		{"Stated", "required conflicting result expression; required size >= 1"},
		{"Inherited", "required conflicting result expression; required size >= 1"},
		{"Kept", "required size >= 1"},
	} {
		sym := requirementNamed(t, scope, tc.name)
		conds := ctx.CaseConditionsOf(sym, sym.OwnerScope)
		if got := labelsOf(conds); got != tc.want {
			t.Errorf("%s: conditions are [%s], want [%s]", tc.name, got, tc.want)
		}
		conflict := conflictingResultExpression(conds)
		if tc.name == "Kept" {
			if conflict != nil {
				t.Errorf("Kept: records a conflict %+v, want none", conflict)
			}
			continue
		}
		if conflict == nil || conflict.Node == nil {
			t.Fatalf("%s: records no conflict node", tc.name)
		}
		if conds[0].Scope == nil || conds[0].Scope.Owner() != sym {
			t.Errorf("%s: the conflict's scope is not the case's own", tc.name)
		}
	}
}

// Asking about no symbol collects nothing rather than failing.
func TestAnalysisAccessorsWithoutSymbol(t *testing.T) {
	ctx, _ := analysisFixture(t, `package test { }`)
	if objs := ctx.ObjectivesOf(nil, nil); objs != nil {
		t.Errorf("a nil symbol states [%s], want nothing", objectiveLabels(objs))
	}
	if conds := ctx.CaseConditionsOf(nil, nil); conds != nil {
		t.Errorf("a nil symbol states [%s], want nothing", labelsOf(conds))
	}
}

// RequireAnalysis settles the kind before objectives are asked for: only an
// analysis case states them, and anything else is a typed refusal.
func TestRequireAnalysis(t *testing.T) {
	ctx, scope := analysisFixture(t, `
		package test {
			private import ScalarValues::*;
			private import TradeStudies::*;
			analysis def Trade { attribute size : Integer; }
			analysis trade : Trade;
			part def Wheel;
			constraint def Small { attribute size : Integer; }
		}
	`)
	_ = ctx
	for _, name := range []string{"Trade", "trade"} {
		if err := RequireAnalysis(requirementNamed(t, scope, name)); err != nil {
			t.Errorf("%s is not read as an analysis case: %v", name, err)
		}
	}
	for _, name := range []string{"Wheel", "Small"} {
		err := RequireAnalysis(requirementNamed(t, scope, name))
		if !errors.Is(err, ErrNotAnAnalysis) {
			t.Errorf("%s: error is %v, want one saying it is no analysis case", name, err)
		}
		if !strings.Contains(err.Error(), name) {
			t.Errorf("error %q does not name %s", err, name)
		}
		if !strings.Contains(err.Error(), "an analysis case") {
			t.Errorf("error %q does not read \"an analysis case\"", err)
		}
	}
}

// An analysis usage states the objectives of the definition it is typed by, which
// is what makes `%optimize` work on a usage as on a definition.
func TestObjectivesOfAnalysisUsage(t *testing.T) {
	ctx, scope := analysisFixture(t, `
		package test {
			private import ScalarValues::*;
			private import TradeStudies::*;
			analysis def Trade {
				attribute size : Integer;
				require constraint { size >= 2 }
				objective smallest : MinimizeObjective {
					subject :>> selectedAlternative;
					in calc :>> eval { size }
				}
			}
			analysis trade : Trade;
		}
	`)
	got := objectiveLabels(objectivesOfCase(t, ctx, scope, "trade"))
	if got != "minimize smallest = size" {
		t.Errorf("objectives are [%s], want [minimize smallest = size]", got)
	}
	sym := requirementNamed(t, scope, "trade")
	if conds := labelsOf(ctx.CaseConditionsOf(sym, sym.OwnerScope)); conds != "required size >= 2" {
		t.Errorf("conditions are [%s], want [required size >= 2]", conds)
	}
}

// A restatement deeper in one branch of a diamond stands for the common
// ancestor's objective seen through the other branch, in whichever order the
// generals are written and traversed, and goes by the name it inherits.
func TestObjectivesOfUnevenDiamond(t *testing.T) {
	ctx, scope := analysisFixture(t, `
		package test {
			private import ScalarValues::*;
			private import TradeStudies::*;
			analysis def Base {
				attribute cost : Integer;
				attribute margin : Integer;
				objective cheapest : MinimizeObjective {
					subject :>> selectedAlternative;
					in calc :>> eval { cost }
				}
			}
			analysis def A :> Base;
			analysis def Mid :> Base {
				objective : MinimizeObjective {
					subject :>> selectedAlternative;
					in calc :>> eval { return :>> result = cost + margin; }
				}
			}
			analysis def B :> Mid;
			analysis def D :> A, B;
			analysis def Reversed :> B, A;
		}
	`)
	for _, name := range []string{"D", "Reversed"} {
		got := objectiveLabels(objectivesOfCase(t, ctx, scope, name))
		if want := "minimize cheapest = cost + margin"; got != want {
			t.Errorf("%s: objectives are [%s], want [%s]", name, got, want)
		}
	}
}
