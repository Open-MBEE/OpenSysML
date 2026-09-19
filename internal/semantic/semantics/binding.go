package semantics

import (
	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// LookupBinding resolves the name a named argument binds within typ: a single segment
// is looked up as a member of typ, a qualified name from scope; aliases resolve through.
func (m *Model) LookupBinding(scope *symbols.Scope, typ *symbols.Symbol, name *ast.QualifiedName) (*symbols.Symbol, bool) {
	if m == nil || name == nil || len(name.Parts) == 0 {
		return nil, false
	}
	var (
		found *symbols.Symbol
		ok    bool
	)
	if len(name.Parts) == 1 {
		found, ok = m.LookupMember(typ, name.Parts[0].Text)
	} else {
		found, ok = m.resolver.ProbeReference(resolve.Reference{Scope: scope, QN: name})
	}
	if !ok || found == nil {
		return nil, false
	}
	if target, isAlias := m.resolver.ResolveAliasTarget(found); isAlias {
		return target, true
	}
	return found, true
}

// BoundParameter is the input parameter of callee a named argument binds, by the name
// callee's signature gives it — what a runtime keys its bindings by; false when none.
func (m *Model) BoundParameter(scope *symbols.Scope, callee *symbols.Symbol, name *ast.QualifiedName) (string, bool) {
	if m == nil || callee == nil || name == nil || len(name.Parts) == 0 {
		return "", false
	}
	sig := m.signatureOf(callee)
	i := m.parameterIndex(scope, sig, name)
	if i < 0 {
		return "", false
	}
	return sig.params[i].name, true
}
