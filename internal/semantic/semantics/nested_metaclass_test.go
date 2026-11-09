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
		state def Mode { state on; entry action e; do action d; exit action x; transition t first on then on; }
		state def Dual parallel { state a; state b; }
		part def Machine { state m parallel { state x; state y; } state n { state z; } }
		action def Run { accept a : Signal; send 1 to 2; terminate; event occurrence ev; action nested; }
		verification def Check { objective { verify requirement r; } subject s; }
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
				case *ast.Usage, *ast.AssumeMember, *ast.RequireMember, *ast.SubstateMember, *ast.TransitionMember, *ast.SendStatement:
					checked++
					want := m.nestedSubsettingFQN(sym)
					membership, kind := nestedMembership(sym)
					ownerMembership, _ := nestedMembership(owner)
					got := m.ImpliedSubsettingFQNForMetaclass(NestedMetaclasses{
						Metaclass: nestedMetaclassName(m, sym), OwnerMetaclass: nestedMetaclassName(m, owner),
						MembershipMetaclass: membership, MembershipKind: kind, OwnerMembershipMetaclass: ownerMembership,
						OwnerParallel: nestedParallel(owner), Composite: nestedComposite(m, sym), Portion: nestedPortion(sym),
					}, owner)
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
		case d.IsVerifiedRequirement():
			return "RequirementVerificationMembership", ""
		case d.Kind == ast.UsageObjective:
			return "ObjectiveMembership", ""
		case d.Kind == ast.UsageActor:
			return "ActorMembership", ""
		case d.Kind == ast.UsageStakeholder:
			return "StakeholderMembership", ""
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

// nestedParallel is the `parallel` the graph states for an owning state (isParallel).
func nestedParallel(owner *symbols.Symbol) bool {
	switch d := owner.Decl.(type) {
	case *ast.Usage:
		return d.IsParallel
	case *ast.Definition:
		return d.IsParallel
	}
	return false
}

func nestedComposite(m *Model, sym *symbols.Symbol) bool {
	u, ok := sym.Decl.(*ast.Usage)
	if !ok {
		// The constraint an `assume`/`require` member owns and the member forms
		// standing in for a usage are composite (nestedSubsettingFQN).
		return true
	}
	if !m.isSysMLUsage(sym) {
		return u.IsComposite
	}
	return usageIsComposite(u)
}

// The graph path has to reconstruct the roles and the owner's isParallel the
// declaration rules read off the syntax, else the role collections are lost.
func TestImpliedSubsettingFQNForMetaclassRolesAndParallelOwner(t *testing.T) {
	m, _ := buildModelWithStdlib(t, `package P {}`)
	action := func(metaclass string) NestedMetaclasses {
		return NestedMetaclasses{Metaclass: metaclass, OwnerMetaclass: "ActionDefinition", MembershipMetaclass: "FeatureMembership", Composite: true}
	}
	cases := []struct {
		name string
		n    NestedMetaclasses
		want string
	}{
		{"accept", action("AcceptActionUsage"), "Actions::Action::acceptSubactions"},
		{"send", action("SendActionUsage"), "Actions::Action::sendSubactions"},
		{"terminate", action("TerminateActionUsage"), "Actions::Action::terminateSubactions"},
		{"action", action("ActionUsage"), "Actions::Action::subactions"},
		{"event", NestedMetaclasses{Metaclass: "EventOccurrenceUsage", OwnerMetaclass: "PartDefinition", MembershipMetaclass: "FeatureMembership"}, "Occurrences::Occurrence::timeEnclosedOccurrences"},
		{"verify under objective", NestedMetaclasses{Metaclass: "SatisfyRequirementUsage", OwnerMetaclass: "RequirementUsage", MembershipMetaclass: "RequirementVerificationMembership", OwnerMembershipMetaclass: "ObjectiveMembership", Composite: true}, "VerificationCases::VerificationCase::obj::requirementVerifications"},
		{"verify under plain requirement", NestedMetaclasses{Metaclass: "SatisfyRequirementUsage", OwnerMetaclass: "RequirementUsage", MembershipMetaclass: "RequirementVerificationMembership", Composite: true}, "Requirements::RequirementCheck::subrequirements"},
		{"objective", NestedMetaclasses{Metaclass: "RequirementUsage", OwnerMetaclass: "VerificationCaseDefinition", MembershipMetaclass: "ObjectiveMembership", Composite: true}, ""},
		{"parallel state owner", NestedMetaclasses{Metaclass: "StateUsage", OwnerMetaclass: "StateUsage", MembershipMetaclass: "FeatureMembership", OwnerParallel: true, Composite: true}, "States::StateAction::substates"},
		{"nonparallel state owner", NestedMetaclasses{Metaclass: "StateUsage", OwnerMetaclass: "StateUsage", MembershipMetaclass: "FeatureMembership", Composite: true}, "States::StateAction::exclusiveStates"},
		{"parallel state def owner", NestedMetaclasses{Metaclass: "StateUsage", OwnerMetaclass: "StateDefinition", MembershipMetaclass: "FeatureMembership", OwnerParallel: true, Composite: true}, "States::StateAction::substates"},
	}
	for _, tc := range cases {
		if got := m.ImpliedSubsettingFQNForMetaclass(tc.n, nil); got != tc.want {
			t.Errorf("%s: got %q, want %q", tc.name, got, tc.want)
		}
	}
}
