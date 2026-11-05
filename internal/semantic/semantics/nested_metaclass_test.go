package semantics

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// The metaclass-keyed rules a graph element is judged by agree with the
// declaration-keyed rules its source would be judged by.
func TestImpliedSubsettingFQNForMetaclassAgreesWithTheDeclarationRules(t *testing.T) {
	m, root := buildModelWithStdlib(t, `package P {
		part def Vehicle {
			part eng;
			item cargo;
			port p;
			action go;
			perform action drive;
			exhibit state running;
			ref part spare;
			occurrence o;
			snapshot s;
			timeslice ts;
			attribute mass;
		}
		item def Box { part lid; item content; }
		action def Drive { action step; item fuel; }
		state def Mode { state on; }
		use case def Trip { include use case stop; actor driver; }
		requirement def Safe { require constraint c; assume constraint a; }
		part v : Vehicle { part wheel; action spin; }
		occurrence def O { part p; }
	}`)
	var checked int
	var walk func(scope *symbols.Scope)
	walk = func(scope *symbols.Scope) {
		scope.ForEachMember(func(sym *symbols.Symbol) bool {
			if sym.Kind == symbols.SymbolAlias {
				return true
			}
			if owner := scope.Owner(); owner != nil {
				if _, isUsage := sym.Decl.(*ast.Usage); isUsage {
					checked++
					want := m.nestedSubsettingFQN(sym)
					got := m.ImpliedSubsettingFQNForMetaclass(nestedMetaclassName(m, sym), nestedMetaclassName(m, owner), nestedComposite(m, sym), sym.Decl.(*ast.Usage).Portion, owner)
					if got != want {
						t.Errorf("%s (%s in %s): metaclass rules %q, declaration rules %q", sym.Name, nestedMetaclassName(m, sym), nestedMetaclassName(m, owner), got, want)
					}
				}
			}
			if sym.Scope != nil {
				walk(sym.Scope)
			}
			return true
		})
	}
	walk(root)
	if checked < 20 {
		t.Fatalf("checked only %d nested usages", checked)
	}
}

// nestedMetaclassName is the metaclass the graph states for sym: the perform,
// exhibit and include forms are metaclasses of their own there.
func nestedMetaclassName(m *Model, sym *symbols.Symbol) string {
	if u, ok := sym.Decl.(*ast.Usage); ok {
		switch {
		case u.IsPerformedAction():
			return "PerformActionUsage"
		case u.IsExhibitedState():
			return "ExhibitStateUsage"
		case u.IsIncludedUseCase():
			return "IncludeUseCaseUsage"
		}
	}
	if mc := m.MetaclassOf(sym); mc != nil {
		return mc.Name
	}
	return ""
}

func nestedComposite(m *Model, sym *symbols.Symbol) bool {
	u, ok := sym.Decl.(*ast.Usage)
	if !ok {
		return false
	}
	if !m.isSysMLUsage(sym) {
		return u.IsComposite
	}
	return usageIsComposite(u)
}
