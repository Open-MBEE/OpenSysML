package semantics

import (
	"slices"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// IsBehaviorType reports whether sym is a behavior type, directly or through a
// specialization, including interactions and behavior-like library types.
func (m *Model) IsBehaviorType(sym *symbols.Symbol) bool {
	if sym == nil {
		return false
	}
	if behaviorLike(sym) {
		return true
	}
	if m == nil {
		return false
	}
	for _, super := range m.AllSupertypes(sym) {
		if behaviorLike(super) {
			return true
		}
	}
	return false
}

// BehaviorSuccessionOwner reports whether a scope may own an executable behavior order.
func (m *Model) BehaviorSuccessionOwner(owner *symbols.Symbol) bool {
	if owner == nil || owner.Kind == symbols.SymbolPackage || owner.Kind == symbols.SymbolNamespace {
		return true
	}
	return !m.IsBehaviorType(owner)
}

// BehaviorSuccessionFeaturingType derives the featuring type of an outside-body succession.
func (m *Model) BehaviorSuccessionFeaturingType(owner *symbols.Symbol, paths [][]*symbols.Symbol, ends ...ast.Node) (*symbols.Symbol, bool) {
	if owner != nil && owner.Kind != symbols.SymbolPackage && owner.Kind != symbols.SymbolNamespace {
		if m.BehaviorSuccessionOwner(owner) {
			return owner, true
		}
		return nil, false
	}

	var endTypes []*symbols.Symbol
	for i, path := range paths {
		if len(path) == 0 {
			continue
		}
		target := ast.Node(nil)
		if i < len(ends) {
			target = ends[i]
		}
		typ := successionEndFeaturingType(path, target)
		if typ == nil || typ.Kind == symbols.SymbolPackage || typ.Kind == symbols.SymbolNamespace || IsAnything(typ) {
			continue
		}
		if !m.BehaviorSuccessionOwner(typ) {
			continue
		}
		endTypes = append(endTypes, typ)
	}
	return m.deriveConnectorDefaultFeaturingType(endTypes)
}

func (m *Model) deriveConnectorDefaultFeaturingType(endTypes []*symbols.Symbol) (*symbols.Symbol, bool) {
	var common []*symbols.Symbol
	for _, typ := range endTypes {
		types := []*symbols.Symbol{typ}
		if typ.Kind.IsDefinition() {
			for _, super := range m.AllSupertypes(typ) {
				if !IsAnything(super) {
					types = append(types, super)
				}
			}
		}
		if common == nil {
			common = types
			continue
		}
		common = intersectTypes(common, types)
		if len(common) == 0 {
			return nil, false
		}
	}
	if len(common) == 0 {
		return nil, true
	}

	var mostSpecific []*symbols.Symbol
	for _, candidate := range common {
		moreSpecific := false
		for _, other := range common {
			if other != candidate && m.Conforms(other, candidate) {
				moreSpecific = true
				break
			}
		}
		if !moreSpecific {
			mostSpecific = append(mostSpecific, candidate)
		}
	}
	if len(mostSpecific) != 1 {
		return nil, false
	}
	return mostSpecific[0], true
}

func successionEndFeaturingType(path []*symbols.Symbol, target ast.Node) *symbols.Symbol {
	for {
		switch n := target.(type) {
		case *ast.ConnectorEnd:
			target = n.AttachedTarget()
		case *ast.FeatureReference:
			target = n.Name
		default:
			goto resolved
		}
	}
resolved:
	switch n := target.(type) {
	case *ast.QualifiedName:
		chained := false
		for _, part := range n.Parts {
			chained = chained || part.Chained
		}
		if chained {
			return path[0].Owner()
		}
		return path[len(path)-1].Owner()
	case *ast.FeatureChainExpr:
		return path[0].Owner()
	default:
		return path[len(path)-1].Owner()
	}
}

// SuccessionEndPath resolves an outside-body succession end from its owning scope.
func (m *Model) SuccessionEndPath(scope *symbols.Scope, owner *symbols.Symbol, target ast.Node) []*symbols.Symbol {
	if m == nil || target == nil {
		return nil
	}
	if end, ok := target.(*ast.ConnectorEnd); ok {
		resolved := m.connectorEndOf(scope, owner, end)
		path := m.successionAttachmentPath(scope, end.AttachedTarget())
		if len(path) == 0 && resolved.Symbol != nil {
			return []*symbols.Symbol{resolved.Symbol}
		}
		return path
	}
	resolved := m.referenceEnd(scope, owner, target)
	path := m.successionAttachmentPath(scope, target)
	if len(path) == 0 && resolved.Symbol != nil {
		return []*symbols.Symbol{resolved.Symbol}
	}
	return path
}

func (m *Model) successionAttachmentPath(scope *symbols.Scope, target ast.Node) []*symbols.Symbol {
	if m == nil || m.resolver == nil || target == nil {
		return nil
	}
	switch n := target.(type) {
	case *ast.FeatureReference:
		return m.successionAttachmentPath(scope, n.Name)
	case *ast.QualifiedName:
		return m.qualifiedSuccessionPath(scope, n)
	case *ast.FeatureChainExpr:
		path := m.successionAttachmentPath(scope, n.Operand)
		if last, ok := m.resolver.ResolveTarget(scope, n); ok && last != nil {
			path = append(path, last)
		} else {
			return nil
		}
		return path
	default:
		if sym, ok := m.resolver.ResolveTarget(scope, target); ok && sym != nil {
			return []*symbols.Symbol{sym}
		}
		return nil
	}
}

func (m *Model) qualifiedSuccessionPath(scope *symbols.Scope, name *ast.QualifiedName) []*symbols.Symbol {
	headEnd := len(name.Parts)
	for i, part := range name.Parts {
		if part.Chained {
			headEnd = i
			break
		}
	}
	if headEnd == len(name.Parts) {
		if sym, ok := m.resolver.ResolveTarget(scope, name); ok && sym != nil {
			return []*symbols.Symbol{sym}
		}
		return nil
	}
	head := &ast.QualifiedName{NodeBase: name.NodeBase, Global: name.Global, Parts: slices.Clone(name.Parts[:headEnd])}
	first, ok := m.resolver.ResolveTarget(scope, head)
	if !ok || first == nil {
		return nil
	}
	path := []*symbols.Symbol{first}
	for i := headEnd; i < len(name.Parts); i++ {
		if !name.Parts[i].Chained {
			continue
		}
		prefix := &ast.QualifiedName{
			NodeBase: name.NodeBase,
			Global:   name.Global,
			Parts:    slices.Clone(name.Parts[:i+1]),
		}
		last, ok := m.resolver.ResolveTarget(scope, prefix)
		if !ok || last == nil {
			return nil
		}
		path = append(path, last)
	}
	last, ok := m.resolver.ResolveTarget(scope, name)
	if !ok || last == nil {
		return nil
	}
	if path[len(path)-1] != last {
		path = append(path, last)
	}
	return path
}

func intersectTypes(a, b []*symbols.Symbol) []*symbols.Symbol {
	set := make(map[*symbols.Symbol]bool, len(b))
	for _, typ := range b {
		set[typ] = true
	}
	var out []*symbols.Symbol
	for _, typ := range a {
		if set[typ] {
			out = append(out, typ)
		}
	}
	return out
}
