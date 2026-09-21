package lower

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
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
// body targets, an alias followed to what it names, and tells the library's own
// declarations from a model's spelled like them.
type StateRedefinitionResolver interface {
	RedefinitionTarget(scope *symbols.Scope, decl ast.Node, target ast.Node) (*symbols.Symbol, bool)
	LibraryFeature(sym *symbols.Symbol) bool
}

// RunToCompletionRedefinition reports a redefinition of runToCompletionScope the
// graph cannot resolve to this machine or an enclosing state.
type RunToCompletionRedefinition struct {
	// Feature is the redefined run-to-completion feature.
	Feature string
	// Decl is the redefinition declaration.
	Decl *ast.Usage
	// Owner describes the body containing the declaration.
	Owner string
	// Written is the value as written.
	Written string
	// NotAncestor reports a resolved state outside the current state ancestry.
	NotAncestor bool
}

func (e *RunToCompletionRedefinition) Error() string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "%s: %s redefines %s = %s, which ", ErrUnsupportedStateContent, e.Owner, e.Feature, e.Written)
	if e.NotAncestor {
		sb.WriteString("is neither the state itself nor a state enclosing it: ")
	} else {
		sb.WriteString("names no occurrence of this state machine: ")
	}
	sb.WriteString("a run-to-completion scope is the state itself or a state enclosing it (Occurrences::Occurrence::runToCompletionScope default self)")
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
	resolver, _ := g.endpoints.(StateRedefinitionResolver)
	return redefinedRunToCompletionFeatureIn(resolver, usage, scope, map[*symbols.Symbol]bool{})
}

// redefinedRunToCompletionFeatureIn is redefinedRunToCompletionFeature over the
// given resolver (nil for none), seen guarding the walk against a cycle.
func redefinedRunToCompletionFeatureIn(resolver StateRedefinitionResolver, usage *ast.Usage, scope *symbols.Scope, seen map[*symbols.Symbol]bool) string {
	for _, rel := range usage.Relationships {
		if rel == nil || rel.Kind != ast.RelRedefines {
			continue
		}
		if resolver != nil {
			if target, ok := resolver.RedefinitionTarget(scope, usage, rel.Target); ok {
				if name := libraryRunToCompletionFeature(resolver, target, seen); name != "" {
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
// or reaches through the redefinitions its own declaration writes. A model's own
// declaration under the library's name is not the library's.
func libraryRunToCompletionFeature(resolver StateRedefinitionResolver, sym *symbols.Symbol, seen map[*symbols.Symbol]bool) string {
	if sym == nil || seen[sym] {
		return ""
	}
	seen[sym] = true
	if name, ok := runToCompletionFeatureFQNs[symbols.FQNOf(sym)]; ok && resolver.LibraryFeature(sym) {
		return name
	}
	usage, ok := sym.Decl.(*ast.Usage)
	if !ok {
		return ""
	}
	return redefinedRunToCompletionFeatureIn(resolver, usage, sym.OwnerScope, seen)
}

// runToCompletionDecl records one effective RTC declaration and its scope.
type runToCompletionDecl struct {
	// usage is the declaration carrying the effective value.
	usage *ast.Usage
	// scope resolves names written in usage.
	scope *symbols.Scope
	// owner describes the body containing usage.
	owner string
}

// RunToCompletion is the effective run-to-completion configuration of a body.
type RunToCompletion struct {
	// Value is the isRunToCompletion expression, nil for the default true.
	Value ast.Node
	// ValueScope resolves names in Value.
	ValueScope *symbols.Scope
	// ValueOwner owns Value's state data; nil denotes the machine.
	ValueOwner *ast.StateNode
	// Scope is the state named by runToCompletionScope; nil denotes the machine.
	Scope *ast.StateNode
}

// recordRunToCompletion records the effective valued members of one body.
func (g *StateGraph) recordRunToCompletion(state *ast.StateNode, body []inheritedMember, self string) {
	effective := map[string]inheritedMember{}
	for _, member := range body {
		usage, ok := unwrapMembership(member.node).(*ast.Usage)
		if !ok || usage.Value == nil {
			continue
		}
		if feature := g.redefinedRunToCompletionFeature(usage, member.scope); feature != "" {
			effective[feature] = member
		}
	}
	if g.runToCompletionDecls[state] == nil {
		g.runToCompletionDecls[state] = make(map[string]runToCompletionDecl)
	}
	for _, feature := range []string{isRunToCompletionFeature, runToCompletionScopeFeature} {
		member, ok := effective[feature]
		if !ok {
			continue
		}
		owner := self
		if member.owner != nil {
			owner = DescribeMember(member.owner) + ", inherited by " + self + ","
		}
		usage := unwrapMembership(member.node).(*ast.Usage)
		g.runToCompletionDecls[state][feature] = runToCompletionDecl{
			usage: usage,
			scope: member.scope,
			owner: owner,
		}
	}
}

// RunToCompletionOf returns the effective configuration for state, or the
// machine when state is nil.
func (g *StateGraph) RunToCompletionOf(state *ast.StateNode) RunToCompletion {
	return g.RunToCompletion[state]
}

// resolveRunToCompletion resolves recorded RTC declarations after graph vertices exist.
func (g *StateGraph) resolveRunToCompletion() error {
	done := map[*ast.StateNode]bool{}
	var resolve func(*ast.StateNode) error
	resolve = func(state *ast.StateNode) error {
		if done[state] {
			return nil
		}
		if parent := g.ParentState[state]; parent != nil {
			if err := resolve(parent); err != nil {
				return err
			}
		}
		r := RunToCompletion{}
		if state != nil {
			r = g.RunToCompletion[g.ParentState[state]]
		}
		if decl := g.runToCompletionDecls[state][isRunToCompletionFeature]; decl.usage != nil {
			r.Value = decl.usage.Value
			r.ValueScope = decl.scope
			r.ValueOwner = state
		}
		if decl := g.runToCompletionDecls[state][runToCompletionScopeFeature]; decl.usage != nil {
			if FeaturePath(decl.usage.Value) == "self" {
				r.Scope = state
			} else {
				var err error
				r.Scope, err = g.scopeState(state, decl)
				if err != nil {
					return err
				}
				if r.Scope != nil {
					ancestor := false
					for current := state; current != nil; current = g.ParentState[current] {
						if current == r.Scope {
							ancestor = true
							break
						}
					}
					if !ancestor {
						return decl.refusal(true)
					}
				}
			}
		}
		g.RunToCompletion[state] = r
		done[state] = true
		return nil
	}
	if err := resolve(nil); err != nil {
		return err
	}
	for _, state := range g.States {
		if err := resolve(state); err != nil {
			return err
		}
	}
	return nil
}

// scopeState resolves a scope declaration to the machine or a state vertex.
func (g *StateGraph) scopeState(state *ast.StateNode, decl runToCompletionDecl) (*ast.StateNode, error) {
	if named, ok := g.endpoints.Endpoint(decl.scope, decl.usage.Value); ok && named == g.machineDecl {
		return nil, nil
	}
	resolved, err := g.vertex(decl.scope, decl.usage.Value)
	if err != nil || resolved == nil {
		return nil, decl.refusal(false)
	}
	switch target := resolved.(type) {
	case *ast.StateNode:
		return target, nil
	case *ast.Usage:
		if target == g.machineDecl {
			return nil, nil
		}
	}
	return nil, decl.refusal(false)
}

// refusal builds the typed error for an invalid scope declaration.
func (decl runToCompletionDecl) refusal(notAncestor bool) error {
	return &RunToCompletionRedefinition{
		Feature:     runToCompletionScopeFeature,
		Decl:        decl.usage,
		Owner:       decl.owner,
		Written:     writtenValue(decl.usage.Value),
		NotAncestor: notAncestor,
	}
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
