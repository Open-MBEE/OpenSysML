package runtime

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// labelsOf renders the conditions collected, with their role, so one comparison
// covers order, negation, grouping and whether each is required.
func labelsOf(conds []Condition) string {
	parts := make([]string, 0, len(conds))
	for _, cond := range conds {
		role := "assumed"
		if cond.Required {
			role = "required"
		}
		parts = append(parts, role+" "+cond.Label())
	}
	return strings.Join(parts, "; ")
}

// ConditionsOf hands out the conditions the evaluator checks, in the evaluator's
// order: inherited first, each with the role its declaration gave it.
func TestConditionsOfInheritedOrderAndRoles(t *testing.T) {
	ctx, scope := conditionFixture(t, `
		package test {
			requirement def Base {
				attribute a;
				attribute b;
				assume constraint { a > 0 }
				require constraint { a <= b }
			}
			requirement touchdown : Base {
				attribute :>> a = 1;
				attribute :>> b = 2;
				require constraint { b < 10 }
			}
		}
	`)
	sym := requirementNamed(t, scope, "touchdown")
	got := labelsOf(ctx.ConditionsOf(sym, sym.OwnerScope))
	want := "assumed a > 0; required a <= b; required b < 10"
	if got != want {
		t.Errorf("conditions are [%s], want [%s]", got, want)
	}
}

// A negated element's conditions keep the roles they were declared with: the
// negation is the element's assertion, not a property of each condition.
func TestConditionsOfNegatedElementKeepsRoles(t *testing.T) {
	ctx, scope := conditionFixture(t, `
		package test {
			constraint def SafeWindow {
				attribute level;
				assert not constraint { level > 10 }
			}
			part rig {
				attribute level = 4;
				assert constraint safe : SafeWindow { attribute :>> level = 4; }
			}
		}
	`)
	sym := requirementNamed(t, scope, "SafeWindow")
	if got, want := labelsOf(ctx.ConditionsOf(sym, sym.OwnerScope)), "required not level > 10"; got != want {
		t.Errorf("conditions are [%s], want [%s]", got, want)
	}
}

// A negated body states one condition denying the conjunction of its conditions —
// De Morgan — rather than denying each of them.
func TestConditionsOfNegatedBodyIsOneGroup(t *testing.T) {
	ctx, scope := conditionFixture(t, `
		package test {
			constraint def Window {
				attribute level;
				assert not constraint {
					level > 10
					level < 20
				}
			}
		}
	`)
	sym := requirementNamed(t, scope, "Window")
	conds := ctx.ConditionsOf(sym, sym.OwnerScope)
	if len(conds) != 1 {
		t.Fatalf("conditions are [%s], want one group", labelsOf(conds))
	}
	if len(conds[0].Group) != 2 || !conds[0].Negated {
		t.Fatalf("condition is %s, want a negated group of two", conds[0].Label())
	}
	if got, want := labelsOf(conds), "required not { level > 10; level < 20 }"; got != want {
		t.Errorf("conditions are [%s], want [%s]", got, want)
	}
}

// A negated body of one condition is that condition negated, since a conjunction
// of one is that one.
func TestConditionsOfNegatedSingletonBodyIsOneCondition(t *testing.T) {
	ctx, scope := conditionFixture(t, `
		package test {
			constraint def Window {
				attribute level;
				assert not constraint { level > 10 }
			}
		}
	`)
	sym := requirementNamed(t, scope, "Window")
	conds := ctx.ConditionsOf(sym, sym.OwnerScope)
	if len(conds) != 1 || conds[0].Group != nil || !conds[0].Negated {
		t.Fatalf("conditions are [%s], want one negated condition", labelsOf(conds))
	}
}

// Each condition knows the scope its names resolve in and the element declaring
// it, which is what a caller reporting about a condition needs.
func TestConditionsOfCarryScopeAndOwner(t *testing.T) {
	ctx, scope := conditionFixture(t, `
		package test {
			requirement def Base {
				attribute a;
				require constraint { a > 0 }
			}
			requirement touchdown : Base { attribute :>> a = 1; }
		}
	`)
	sym := requirementNamed(t, scope, "touchdown")
	conds := ctx.ConditionsOf(sym, sym.OwnerScope)
	if len(conds) != 1 {
		t.Fatalf("conditions are [%s], want one", labelsOf(conds))
	}
	if conds[0].Scope == nil {
		t.Fatal("the condition resolves its names in no scope")
	}
	owner := conds[0].Owner()
	if owner == nil || owner.Name != "Base" {
		t.Fatalf("the condition is declared by %v, want Base", owner)
	}
}

// A bodiless constraint referencing another (constraint kept ::> c) checks the
// referenced conditions, once even when a diamond reaches the same constraint twice.
func TestConditionsOfReferencedConstraint(t *testing.T) {
	ctx, scope := conditionFixture(t, `
		package test {
			constraint def Pos {
				attribute y : Integer = 3;
				y > 0
			}
			constraint c : Pos;
			constraint kept ::> c;
			requirement def R { require constraint d ::> c; }
			constraint diamond :> kept ::> c;
		}
	`)
	for _, name := range []string{"c", "kept", "diamond"} {
		sym := requirementNamed(t, scope, name)
		if got, want := labelsOf(ctx.ConditionsOf(sym, sym.OwnerScope)), "required y > 0"; got != want {
			t.Errorf("%s: conditions are [%s], want [%s]", name, got, want)
		}
		holds, err := ctx.EvaluateConstraint(sym, sym.OwnerScope)
		if err != nil || !holds {
			t.Errorf("%s: holds=%v, err=%v; want the inherited condition to hold", name, holds, err)
		}
	}
	req := requirementNamed(t, scope, "R")
	if got, want := labelsOf(ctx.ConditionsOf(req, req.OwnerScope)), "required y > 0"; got != want {
		t.Errorf("R: conditions are [%s], want [%s]", got, want)
	}
}

// A symbol stating no condition, and no symbol at all, collect nothing.
func TestConditionsOfWithoutConditions(t *testing.T) {
	ctx, scope := conditionFixture(t, `
		package test {
			requirement def Documented { doc /* prose only */ }
		}
	`)
	sym := requirementNamed(t, scope, "Documented")
	if conds := ctx.ConditionsOf(sym, nil); len(conds) != 0 {
		t.Errorf("conditions are [%s], want none", labelsOf(conds))
	}
	if conds := ctx.ConditionsOf(nil, scope); conds != nil {
		t.Errorf("a nil symbol states [%s], want nothing", labelsOf(conds))
	}
	var missing *symbols.Symbol
	if conds := ctx.ConditionsOf(missing, nil); conds != nil {
		t.Errorf("a nil symbol states [%s], want nothing", labelsOf(conds))
	}
}
