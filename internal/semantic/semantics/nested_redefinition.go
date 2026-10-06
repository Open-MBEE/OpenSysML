package semantics

import (
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// NestedRedefinition is a member of a type redefining a feature below one or
// more of its composite features.
type NestedRedefinition struct {
	Feature *symbols.Symbol // the redefining member
	Path    []string        // chain segment names, e.g. ["mid","leaf","value"]
	Target  *symbols.Symbol // resolved last feature of the chain
}

// NestedRedefinitionsOf lists the nested redefinitions declared directly by sym's
// body (own members only; inherited ones are found through the type's generals by callers).
func (m *Model) NestedRedefinitionsOf(sym *symbols.Symbol) []NestedRedefinition {
	if m == nil || sym == nil {
		return nil
	}
	defer m.own(sym).LeaveDoc()
	if cached, ok := m.nestedRedefs[sym]; ok {
		return cached
	}
	var out []NestedRedefinition
	for _, member := range declMembers(sym) {
		usage, ok := unwrapUsage(member)
		if !ok {
			continue
		}
		for _, rel := range usage.Relationships {
			if rel == nil || rel.Kind != ast.RelRedefines {
				continue
			}
			chain, ok := rel.Target.(*ast.FeatureChainExpr)
			if !ok {
				continue
			}
			path := chainSegments(chain)
			if len(path) < 2 {
				continue
			}
			memberSym := memberSymbol(sym.Scope, usage)
			if memberSym == nil {
				continue
			}
			target := m.relationshipTarget(memberSym, rel)
			if target == nil {
				continue
			}
			out = append(out, NestedRedefinition{Feature: memberSym, Path: path, Target: target})
		}
	}
	journal(m, m.nestedRedefs, sym, sym.Decl)
	m.nestedRedefs[sym] = out
	return out
}

// chainSegments flattens a feature chain (`mid.leaf.value`) into its segment
// names. A `::`-qualified segment names its simple member name.
func chainSegments(node ast.Node) []string {
	switch n := node.(type) {
	case *ast.FeatureReference:
		return chainSegments(n.Name)
	case *ast.FeatureChainExpr:
		return append(chainSegments(n.Operand), chainSegments(n.Member)...)
	case *ast.QualifiedName:
		var out []string
		for _, part := range n.Parts {
			if part.Chained || len(out) == 0 {
				out = append(out, part.Text)
			} else {
				out[len(out)-1] = part.Text
			}
		}
		return out
	}
	return nil
}

// IsReferenceUsage reports a usage declared `ref` or with a `references` relationship
// (SysML v2 §7.6.2: Usage::isReference), which owns none of the objects it holds.
func IsReferenceUsage(sym *symbols.Symbol) bool {
	if sym == nil {
		return false
	}
	usage, ok := sym.Decl.(*ast.Usage)
	if !ok {
		return false
	}
	if usage.IsReference {
		return true
	}
	for _, rel := range usage.Relationships {
		if rel != nil && rel.Kind == ast.RelReferences {
			return true
		}
	}
	return false
}

// IsSubjectUsage reports whether sym is the subject parameter of a case.
func IsSubjectUsage(sym *symbols.Symbol) bool {
	if sym == nil {
		return false
	}
	switch decl := sym.Decl.(type) {
	case *ast.SubjectMember:
		return true
	case *ast.Usage:
		return decl.Kind == ast.UsageSubject
	}
	return false
}

// IsActorUsage reports whether sym is an actor or stakeholder membership.
func IsActorUsage(sym *symbols.Symbol) bool {
	return roleOf(sym) == actorRole
}

// IsObjectiveUsage reports whether sym is an objective membership.
func IsObjectiveUsage(sym *symbols.Symbol) bool {
	return roleOf(sym) == objectiveRole
}
