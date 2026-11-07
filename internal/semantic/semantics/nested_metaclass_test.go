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
		state def Mode { state on; entry action e; do action d; exit action x; }
		use case def Trip { include use case stop; actor driver; }
		requirement def Safe { require constraint c; assume constraint a; ref constraint r; }
		part v : Vehicle { part wheel; action spin; }
		occurrence def O { part p; }
		variation part def Kit { variant part big; }
	}`)
	var checked int
	var walk func(scope *symbols.Scope)
	walk = func(scope *symbols.Scope) {
		scope.ForEachMember(func(sym *symbols.Symbol) bool {
			if sym.Kind == symbols.SymbolAlias {
				return true
			}
			if owner := scope.Owner(); owner != nil {
				switch sym.Decl.(type) {
				case *ast.Usage, *ast.AssumeMember, *ast.RequireMember:
					checked++
					want := m.nestedSubsettingFQN(sym)
					membership, kind := nestedMembership(sym)
					got := m.ImpliedSubsettingFQNForMetaclass(nestedMetaclassName(m, sym), nestedMetaclassName(m, owner), membership, kind, nestedComposite(m, sym), nestedPortion(sym), owner)
					if got != want {
						t.Errorf("%s (%s in %s, %s %s): metaclass rules %q, declaration rules %q", sym.Name, nestedMetaclassName(m, sym), nestedMetaclassName(m, owner), membership, kind, got, want)
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

// nestedMembership is the metaclass and kind the graph states for the membership
// owning sym: the requirement-constraint, state-subaction and variant forms.
func nestedMembership(sym *symbols.Symbol) (metaclass, kind string) {
	switch d := sym.Decl.(type) {
	case *ast.AssumeMember:
		return "RequirementConstraintMembership", "assumption"
	case *ast.RequireMember:
		return "RequirementConstraintMembership", "requirement"
	case *ast.Usage:
		switch {
		case d.IsVariant:
			return "VariantMembership", ""
		case d.IsAssumedConstraint():
			return "RequirementConstraintMembership", "assumption"
		case d.IsRequirementConstraint():
			return "RequirementConstraintMembership", "requirement"
		case d.IsStateAction():
			if _, ok := stateActionFQNs[d.Keyword]; ok {
				return "StateSubactionMembership", d.Keyword
			}
			return "StateSubactionMembership", d.PrefixKeyword
		}
	}
	return "FeatureMembership", ""
}

func nestedPortion(sym *symbols.Symbol) ast.PortionKind {
	if u, ok := sym.Decl.(*ast.Usage); ok {
		return u.Portion
	}
	return ast.PortionNone
}

func nestedComposite(m *Model, sym *symbols.Symbol) bool {
	u, ok := sym.Decl.(*ast.Usage)
	if !ok {
		// The constraint an `assume`/`require` member owns is composite (nestedSubsettingFQN).
		_, assume := sym.Decl.(*ast.AssumeMember)
		_, require := sym.Decl.(*ast.RequireMember)
		return assume || require
	}
	if !m.isSysMLUsage(sym) {
		return u.IsComposite
	}
	return usageIsComposite(u)
}
