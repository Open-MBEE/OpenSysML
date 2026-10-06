package lower

import (
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// Binding is a lowered binding connector with its two endpoint expressions and
// the scope in which those expressions were declared.
type Binding struct {
	Ends  [2]BindingEnd
	Scope *symbols.Scope
	Decl  *ast.Usage
	// Multiplicity is the connector's own as written (`binding [1] bind …`), nil when none.
	Multiplicity *ast.Multiplicity
}

// BindingEnd is one binding endpoint. Path is the runtime lvalue path, empty for
// an end that is read as an expression — an indexed end `xs#(2)` selects one
// element of a feature and is no feature to write through; Expr retains the
// lossless expression for diagnostics and calc evaluation; Multiplicity is the
// end multiplicity as written (`bind [0..1] a = b`), nil when none.
type BindingEnd struct {
	Path         string
	Expr         ast.Node
	Multiplicity *ast.Multiplicity
}

// ToBindings lowers binding connectors directly declared by a type or usage.
// Namespace-owned bindings are intentionally left to callers to exclude.
func ToBindings(decl ast.Node, scope *symbols.Scope) []Binding {
	var members []ast.Node
	switch n := decl.(type) {
	case *ast.Usage:
		members = n.Members
	case *ast.Definition:
		members = n.Members
	default:
		return nil
	}

	var out []Binding
	for _, member := range members {
		u, ok := unwrapMembership(member).(*ast.Usage)
		if !ok || u.Kind != ast.UsageBinding {
			continue
		}
		binding, ok := lowerBinding(u, scope)
		if ok {
			out = append(out, binding)
		}
	}
	return out
}

// NamespaceBindings lowers the binding connectors a namespace scope owns
// directly. ToBindings reads a declaration's members, which a package or
// namespace body is not, so bindings owned there are collected from the
// scope's own symbols.
func NamespaceBindings(scope *symbols.Scope) []Binding {
	if scope == nil {
		return nil
	}
	var out []Binding
	scope.ForEachMember(func(sym *symbols.Symbol) bool {
		u, ok := sym.Decl.(*ast.Usage)
		if !ok || u.Kind != ast.UsageBinding {
			return true
		}
		if binding, ok := lowerBinding(u, scope); ok {
			out = append(out, binding)
		}
		return true
	})
	return out
}

func lowerBinding(u *ast.Usage, scope *symbols.Scope) (Binding, bool) {
	if u == nil {
		return Binding{}, false
	}

	if len(u.ConnectorEnds) != 2 {
		return Binding{}, false
	}
	var ends [2]BindingEnd
	for i, end := range u.ConnectorEnds {
		if end == nil {
			return Binding{}, false
		}
		target := end.AttachedTarget()
		if target == nil {
			return Binding{}, false
		}
		if _, failed := target.(*ast.ErrorNode); failed {
			return Binding{}, false
		}
		ends[i] = BindingEnd{Path: FeaturePath(target), Expr: target, Multiplicity: end.Multiplicity}
		if selection := end.AttachedSelection(); selection != target {
			ends[i] = BindingEnd{Expr: selection, Multiplicity: end.Multiplicity}
		}
	}
	return Binding{Ends: ends, Scope: scope, Decl: u, Multiplicity: u.Multiplicity}, true
}
