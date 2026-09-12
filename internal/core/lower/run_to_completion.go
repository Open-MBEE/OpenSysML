package lower

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// The Occurrences::Occurrence features the executor implements at their library
// defaults only: every state machine runs to completion, scoped to the whole machine.
const (
	isRunToCompletionFeature    = "isRunToCompletion"
	runToCompletionScopeFeature = "runToCompletionScope"
)

// runToCompletionFeatureFQNs are the library declarations of the two features:
// Occurrence's, and StatePerformance's redefinition of each.
var runToCompletionFeatureFQNs = map[string]string{
	"Occurrences::Occurrence::isRunToCompletion":                isRunToCompletionFeature,
	"Occurrences::Occurrence::runToCompletionScope":             runToCompletionScopeFeature,
	"StatePerformances::StatePerformance::isRunToCompletion":    isRunToCompletionFeature,
	"StatePerformances::StatePerformance::runToCompletionScope": runToCompletionScopeFeature,
}

// StateRedefinitionResolver resolves the feature a redefinition written in a state
// body targets, an alias followed to what it names.
type StateRedefinitionResolver interface {
	RedefinitionTarget(scope *symbols.Scope, decl ast.Node, target ast.Node) (*symbols.Symbol, bool)
}

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

// redefinedRunToCompletionFeature names the library run-to-completion feature
// usage, declared in scope, redefines — directly, through an alias or through a
// feature that itself redefines it — or "" when it redefines neither. A target
// that does not resolve (no resolver, or the library out of reach) is read by its
// spelling, so a name spelled like the feature is refused rather than run under
// the default.
func (g *StateGraph) redefinedRunToCompletionFeature(usage *ast.Usage, scope *symbols.Scope) string {
	resolver, resolved := g.endpoints.(StateRedefinitionResolver)
	for _, rel := range usage.Relationships {
		if rel == nil || rel.Kind != ast.RelRedefines {
			continue
		}
		if resolved {
			if target, ok := resolver.RedefinitionTarget(scope, usage, rel.Target); ok {
				if name := libraryRunToCompletionFeature(resolver, target, map[*symbols.Symbol]bool{}); name != "" {
					return name
				}
				continue
			}
		}
		switch name, _ := ast.TargetName(rel.Target); name {
		case isRunToCompletionFeature, runToCompletionScopeFeature:
			return name
		}
	}
	return ""
}

// libraryRunToCompletionFeature names the library run-to-completion feature sym is,
// or reaches through the redefinitions its own declaration writes.
func libraryRunToCompletionFeature(resolver StateRedefinitionResolver, sym *symbols.Symbol, seen map[*symbols.Symbol]bool) string {
	if sym == nil || seen[sym] {
		return ""
	}
	seen[sym] = true
	if name, ok := runToCompletionFeatureFQNs[symbols.FQNOf(sym)]; ok {
		return name
	}
	usage, ok := sym.Decl.(*ast.Usage)
	if !ok {
		return ""
	}
	for _, rel := range usage.Relationships {
		if rel == nil || rel.Kind != ast.RelRedefines {
			continue
		}
		if target, ok := resolver.RedefinitionTarget(sym.OwnerScope, usage, rel.Target); ok {
			if name := libraryRunToCompletionFeature(resolver, target, seen); name != "" {
				return name
			}
		}
	}
	return ""
}

// refuseRunToCompletionRedefinition refuses usage, declared in scope, when it
// redefines a run-to-completion feature to anything but the library default. A
// redefinition without a value keeps the inherited default. `self` restates the
// scope only in the machine's own body: written in a substate, it narrows the
// scope to that state.
func (g *StateGraph) refuseRunToCompletionRedefinition(usage *ast.Usage, scope *symbols.Scope, owner string, machine bool) error {
	feature := g.redefinedRunToCompletionFeature(usage, scope)
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
func (g *StateGraph) refuseRunToCompletionRedefinitions(body []inheritedMember, machine ast.Node) error {
	for _, member := range body {
		usage, ok := unwrapMembership(member.node).(*ast.Usage)
		if !ok {
			continue
		}
		owner := DescribeMember(machine)
		if member.owner != nil {
			owner = DescribeMember(member.owner) + ", inherited by " + DescribeMember(machine) + ","
		}
		if err := g.refuseRunToCompletionRedefinition(usage, member.scope, owner, true); err != nil {
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
