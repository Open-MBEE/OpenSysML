package lower

import (
	"strings"

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
	Path string
	Expr ast.Node
	// Index selects one element of the feature Path reaches (`xs#(2)`), nil for the whole feature.
	Index        ast.Node
	Multiplicity *ast.Multiplicity
	// Via leads from the binding's holder to the connector object an expression or a path end is
	// read on, empty for the holder itself; Default marks the expression of a `default =` value.
	Via     []Hop
	Default bool
}

// Hop is one step from a holder to a connector object it holds: by feature name, or by the
// declaration of an anonymous connector member.
type Hop struct {
	Feature   string
	Anonymous *ast.Usage
}

// Reaches reports a hop that leads somewhere: a named feature or an anonymous declaration.
func (h Hop) Reaches() bool {
	return h.Feature != "" || h.Anonymous != nil
}

// HolderPath is the feature path the end reaches from the binding's holder: empty for an
// expression, or a path read on a connector object Via leads to.
func (e BindingEnd) HolderPath() string {
	if len(e.Via) != 0 {
		return ""
	}
	return e.Path
}

// ToBindings lowers binding connectors directly declared by a type or usage.
// Namespace-owned bindings are intentionally left to callers to exclude.
func ToBindings(decl ast.Node, scope *symbols.Scope) []Binding {
	var out []Binding
	for _, member := range declMembers(decl) {
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
		if index := end.AttachedIndex(); index != nil {
			ends[i].Expr, ends[i].Index = end.AttachedSelection(), index
		}
	}
	return Binding{Ends: ends, Scope: scope, Decl: u, Multiplicity: u.Multiplicity}, true
}

// EndValueBindings lowers each `=` or `default =` in a connector end's body as a binding of the end's
// feature to the value expression (SysML v2 §7.13.4, an implicit binding; a default yields to own values).
func EndValueBindings(decl ast.Node, scope *symbols.Scope) []Binding {
	var out []Binding
	for _, member := range declMembers(decl) {
		end, ok := unwrapMembership(member).(*ast.Usage)
		if !ok || !end.IsEnd {
			continue
		}
		endName := symbols.LastSegment(usageName(scope, end))
		if endName == "" {
			continue
		}
		endScope := scope
		if sym := scope.MemberDeclaring(end); sym != nil && sym.Scope != nil {
			endScope = sym.Scope
		}
		for _, node := range end.Members {
			u, ok := unwrapMembership(node).(*ast.Usage)
			if !ok || u.Value == nil || u.ValueIsInitial {
				continue
			}
			name := symbols.LastSegment(usageName(endScope, u))
			if name == "" {
				continue
			}
			out = append(out, Binding{
				Ends: [2]BindingEnd{
					{Path: endName + "." + name, Expr: u.Value},
					{Expr: u.Value, Default: u.ValueIsDefault},
				},
				Scope: endScope, Decl: u,
			})
		}
	}
	return out
}

func declMembers(decl ast.Node) []ast.Node {
	switch n := decl.(type) {
	case *ast.Usage:
		return n.Members
	case *ast.Definition:
		return n.Members
	}
	return nil
}

// usageName is the name u is known by in scope: declared, recorded by its symbol, or the redefined feature's.
func usageName(scope *symbols.Scope, u *ast.Usage) string {
	if u.Ident.Name != "" {
		return u.Ident.Name
	}
	if sym := scope.MemberDeclaring(u); sym != nil && sym.Name != "" {
		return sym.Name
	}
	for _, rel := range u.Relationships {
		if rel != nil && rel.Kind == ast.RelRedefines {
			return FeaturePath(rel.Target)
		}
	}
	return ""
}

// ThroughEnds re-roots a path at what roots maps its head to and leads an expression end, a path through a
// feature owned names, or one already on a connector object, to the connector at self; false if it cannot.
func (b Binding) ThroughEnds(roots map[string]string, owned map[string]bool, self Hop) (Binding, bool) {
	out := b
	for i, end := range b.Ends {
		head, rest, _ := strings.Cut(end.Path, ".")
		if end.Path == "" && end.Expr == nil {
			return Binding{}, false
		}
		if end.Path == "" || len(end.Via) != 0 || owned[head] {
			if !self.Reaches() {
				return Binding{}, false
			}
			out.Ends[i].Via = append([]Hop{self}, end.Via...)
			continue
		}
		target, ok := roots[head]
		if !ok || target == "" {
			return Binding{}, false
		}
		out.Ends[i].Path = joinPath(target, rest)
	}
	return out, true
}

func joinPath(base, rest string) string {
	if rest == "" {
		return base
	}
	return base + "." + rest
}
