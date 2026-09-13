package lower

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
)

// ClassifierBehaviorKind is how a type binds a behavior to its objects.
type ClassifierBehaviorKind int

const (
	// ExhibitedState is a state machine the type exhibits: `exhibit state m { … }`,
	// `exhibit state m : M;`, `exhibit m;` (SysML v2 §7.16.6, Part::exhibitedStates).
	ExhibitedState ClassifierBehaviorKind = iota
	// PerformedAction is an action the type performs: `perform action a { … }`,
	// `perform action a : A;`, `perform a;` (Part::performedActions).
	PerformedAction
)

// String names the kind in diagnostics.
func (k ClassifierBehaviorKind) String() string {
	switch k {
	case ExhibitedState:
		return "exhibited state machine"
	case PerformedAction:
		return "performed action"
	default:
		return "classifier behavior"
	}
}

// ClassifierBehavior is a behavior every object of a type runs because the type
// exhibits or performs it: the declaration that binds it, whether that
// declaration states a body of its own or names an element holding one, and the
// values it binds to the behavior's parameters.
type ClassifierBehavior struct {
	Kind ClassifierBehaviorKind
	// Name is the name the behavior answers to on the object, which is the
	// effective name of the declaration binding it.
	Name string
	// Decl is the `exhibit`/`perform` declaration itself.
	Decl *ast.Usage
	// StatesBody reports whether Decl states the behavior's body. A declaration
	// that states none names the element that holds one instead.
	StatesBody bool
	// Arguments are the values the declaration binds to the behavior's
	// parameters (`exhibit m { in controller = vehicleController; }`), in
	// declaration order.
	Arguments []Attribute
}

// ClassifierBehaviorOf reports the behavior a type's member binds to every
// object of that type, and false for a member that binds none: a state or action
// a type merely owns is not one its objects run.
func ClassifierBehaviorOf(member ast.Node) (ClassifierBehavior, bool) {
	usage, ok := unwrapMembership(member).(*ast.Usage)
	if !ok {
		return ClassifierBehavior{}, false
	}
	kind, ok := classifierBehaviorKind(usage)
	if !ok {
		return ClassifierBehavior{}, false
	}
	name, _ := ast.EffectiveName(usage)
	return ClassifierBehavior{
		Kind:       kind,
		Name:       name,
		Decl:       usage,
		StatesBody: StatesBehaviorBody(usage.Members),
		Arguments:  behaviorArguments(usage.Members),
	}, true
}

// ClassifierBehaviorsOf reports the behaviors the given members bind, in
// declaration order.
func ClassifierBehaviorsOf(members []ast.Node) []ClassifierBehavior {
	var out []ClassifierBehavior
	for _, member := range members {
		if behavior, ok := ClassifierBehaviorOf(member); ok {
			out = append(out, behavior)
		}
	}
	return out
}

// classifierBehaviorKind reports which behavior an `exhibit` or `perform`
// declaration binds, by the keyword it was written with.
func classifierBehaviorKind(usage *ast.Usage) (ClassifierBehaviorKind, bool) {
	switch {
	case usage.Kind == ast.UsageState && (usage.Keyword == "exhibit" || usage.Keyword == "exhibit state"):
		return ExhibitedState, true
	case usage.IsPerformedAction():
		return PerformedAction, true
	default:
		return 0, false
	}
}

// BehaviorMembers are the members of a behavior declaration, and an error for a
// node that declares no behavior.
func BehaviorMembers(decl ast.Node) ([]ast.Node, error) {
	return machineMembers(decl)
}

// StatesBehaviorBody reports whether members state a behavior body rather than
// only annotating the declaration or binding the behavior's parameters
// (`exhibit m { in x = y; }`, `exhibit m : M { doc /* … */ }`).
func StatesBehaviorBody(members []ast.Node) bool {
	for _, member := range members {
		if statesBehaviorBodyMember(unwrapMembership(member)) {
			return true
		}
	}
	return false
}

// statesBehaviorBodyMember reports whether one member of an `exhibit`/`perform`
// declaration is body syntax: annotations describe the declaration and
// directional usages bind the behavior's parameters, so neither is.
func statesBehaviorBodyMember(member ast.Node) bool {
	switch member := member.(type) {
	case *ast.Comment, *ast.Documentation, *ast.TextualRepresentation:
		return false
	case *ast.Usage:
		if member.Direction != ast.DirNone || member.Kind == ast.UsageMetadata {
			return false
		}
		if member.Kind == ast.UsageAttribute {
			redefinitions, _ := ast.SplitRedefinitions(member.Relationships)
			return len(redefinitions) == 0
		}
		return true
	case *ast.Definition:
		return member.Kind != ast.DefMetadata
	default:
		return true
	}
}

// behaviorArguments are the values a binding member supplies to the behavior's
// parameters, keyed by the parameter name they bind.
func behaviorArguments(members []ast.Node) []Attribute {
	var args []Attribute
	for _, member := range members {
		usage, ok := unwrapMembership(member).(*ast.Usage)
		if !ok || !isBehaviorArgument(usage) || usage.Value == nil {
			continue
		}
		name, _ := ast.EffectiveName(usage)
		if name == "" {
			continue
		}
		args = append(args, Attribute{Name: name, Direction: usage.Direction, IsResult: usage.IsResult, Type: TypeText(usage), Value: usage.Value, Node: usage})
	}
	return args
}

// isBehaviorArgument reports whether a member of an `exhibit`/`perform`
// declaration supplies a value the behavior reads. An `out` member declares
// what it answers: no argument, though no body either.
func isBehaviorArgument(usage *ast.Usage) bool {
	return usage.Direction == ast.DirIn || usage.Direction == ast.DirInOut
}
