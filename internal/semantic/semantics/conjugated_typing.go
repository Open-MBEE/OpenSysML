package semantics

import (
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// ConjugatedPortDefinitionOf returns the ConjugatedPortDefinition a port definition
// implies (SysML v2 8.3.12.3), a member of its scope; nil for any other element.
func (m *Model) ConjugatedPortDefinitionOf(def *symbols.Symbol) *symbols.Symbol {
	if def == nil || def.Scope == nil {
		return nil
	}
	var conjugate *symbols.Symbol
	def.Scope.ForEachMember(func(member *symbols.Symbol) bool {
		if member.IsConjugatedPortDefinition() {
			conjugate = member
			return false
		}
		return true
	})
	return conjugate
}

// OriginalPortDefinitionOf returns the port definition a
// ConjugatedPortDefinition conjugates, which owns it; nil for any other element.
func OriginalPortDefinitionOf(conjugate *symbols.Symbol) *symbols.Symbol {
	if !conjugate.IsConjugatedPortDefinition() || conjugate.OwnerScope == nil {
		return nil
	}
	return conjugate.OwnerScope.Owner()
}

// IsPortDefinition reports whether sym is a port definition, written or the
// conjugate a written one implies.
func IsPortDefinition(sym *symbols.Symbol) bool {
	if sym == nil {
		return false
	}
	if kind, isDef := sym.DefinitionKind(); isDef && kind == ast.DefPort {
		return true
	}
	return sym.IsConjugatedPortDefinition()
}

// conjugatedTypingTarget reads a typing's `~` as the ConjugatedPortDefinition the target
// implies (SysML v2 8.3.12.3); a target implying none keeps its conjugating edge.
func (m *Model) conjugatedTypingTarget(target *symbols.Symbol, conjugated bool) (*symbols.Symbol, bool) {
	if !conjugated || target == nil {
		return target, conjugated
	}
	// A record names the conjugate itself as the typing's target.
	if target.IsConjugatedPortDefinition() {
		return target, false
	}
	if conjugate := m.ConjugatedPortDefinitionOf(target); conjugate != nil {
		return conjugate, false
	}
	return target, true
}

// TypingTarget resolves the type rel, a typing sym declares, gives sym: its
// target through any alias, read through the `~` of a conjugated typing.
func (m *Model) TypingTarget(sym *symbols.Symbol, rel *ast.Relationship) *symbols.Symbol {
	if rel == nil || rel.Kind != ast.RelTyping {
		return nil
	}
	return m.relationshipTarget(sym, rel)
}
