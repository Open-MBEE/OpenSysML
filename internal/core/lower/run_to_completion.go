package lower

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
)

// The Occurrences::Occurrence features the executor implements at their library
// defaults only: every state machine runs to completion, scoped to the whole machine.
const (
	isRunToCompletionFeature    = "isRunToCompletion"
	runToCompletionScopeFeature = "runToCompletionScope"
)

// RunToCompletionRedefinition reports a redefinition of isRunToCompletion or
// runToCompletionScope the executor cannot honor: a value other than the
// library default, or an expression lowering cannot decide restates it.
type RunToCompletionRedefinition struct {
	// Feature is isRunToCompletion or runToCompletionScope.
	Feature string
	// Decl is the redefining declaration.
	Decl *ast.Usage
	// Owner describes the state or definition whose body declares it.
	Owner string
	// Written is the value as written.
	Written string
	// Unverified marks a value lowering cannot decide rather than one it read
	// as departing from the default.
	Unverified bool
}

func (e *RunToCompletionRedefinition) Error() string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "%s: %s redefines %s = %s, which the runtime ", ErrUnsupportedStateContent, e.Owner, e.Feature, e.Written)
	if e.Unverified {
		sb.WriteString("cannot verify restates the library default: ")
	} else {
		sb.WriteString("cannot honor: ")
	}
	switch e.Feature {
	case isRunToCompletionFeature:
		sb.WriteString("every state machine runs to completion (Occurrences::Occurrence::isRunToCompletion default true)")
	default:
		sb.WriteString("the whole state machine is the run-to-completion scope (Occurrences::Occurrence::runToCompletionScope default self)")
	}
	return sb.String()
}

// Unwrap makes the refusal match ErrUnsupportedStateContent, the error every
// other state content lowering cannot represent reports.
func (e *RunToCompletionRedefinition) Unwrap() error { return ErrUnsupportedStateContent }

// redefinedRunToCompletionFeature names the run-to-completion feature usage
// redefines, or "" when it redefines neither.
func redefinedRunToCompletionFeature(usage *ast.Usage) string {
	for _, rel := range usage.Relationships {
		if rel == nil || rel.Kind != ast.RelRedefines {
			continue
		}
		switch name, _ := ast.TargetName(rel.Target); name {
		case isRunToCompletionFeature, runToCompletionScopeFeature:
			return name
		}
	}
	return ""
}

// refuseRunToCompletionRedefinition refuses usage when it redefines a
// run-to-completion feature to anything but the library default. A redefinition
// without a value keeps the inherited default. `self` restates the scope only in
// the machine's own body: written in a substate, it narrows the scope to that state.
func refuseRunToCompletionRedefinition(usage *ast.Usage, owner string, machine bool) error {
	feature := redefinedRunToCompletionFeature(usage)
	if feature == "" || usage.Value == nil {
		return nil
	}
	refusal := &RunToCompletionRedefinition{Feature: feature, Decl: usage, Owner: owner, Written: writtenValue(usage.Value)}
	switch feature {
	case isRunToCompletionFeature:
		lit, ok := usage.Value.(*ast.LiteralBool)
		switch {
		case ok && lit.Value:
			return nil
		case ok:
			return refusal
		}
	case runToCompletionScopeFeature:
		switch {
		case FeaturePath(usage.Value) != "self":
		case machine:
			return nil
		default:
			refusal.Written += ", narrowing the scope to that state"
			return refusal
		}
	}
	refusal.Unverified = true
	return refusal
}

// refuseRunToCompletionRedefinitions applies refuseRunToCompletionRedefinition
// to a machine's body; owner describes the machine itself for members written in it.
func refuseRunToCompletionRedefinitions(body []inheritedMember, machine ast.Node) error {
	for _, member := range body {
		usage, ok := unwrapMembership(member.node).(*ast.Usage)
		if !ok {
			continue
		}
		owner := DescribeMember(machine)
		if member.owner != nil {
			owner = DescribeMember(member.owner) + ", inherited by " + DescribeMember(machine) + ","
		}
		if err := refuseRunToCompletionRedefinition(usage, owner, true); err != nil {
			return err
		}
	}
	return nil
}

// writtenValue renders a feature value for a message, as far as the notation
// lowering reads: literals, names and operator expressions.
func writtenValue(node ast.Node) string {
	switch n := node.(type) {
	case *ast.LiteralBool:
		return strconv.FormatBool(n.Value)
	case *ast.LiteralInteger:
		return n.Value
	case *ast.LiteralReal:
		return n.Value
	case *ast.LiteralString:
		return strconv.Quote(n.Value)
	case *ast.NullExpr:
		return "null"
	case *ast.OperatorExpr:
		parts := make([]string, len(n.Operands))
		for i, operand := range n.Operands {
			parts[i] = writtenValue(operand)
		}
		if len(parts) == 1 {
			return n.Operator.String() + " " + parts[0]
		}
		return strings.Join(parts, " "+n.Operator.String()+" ")
	}
	if path := FeaturePath(node); path != "" {
		return path
	}
	return "an expression"
}
