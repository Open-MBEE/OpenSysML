package semantics

import (
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// SubactionKind returns the StateSubactionKind of a state's `entry`, `do` or
// `exit` member (SysML.xtext EntryActionMember, DoActionMember, ExitActionMember).
func SubactionKind(node ast.Node) (string, bool) {
	switch node.(type) {
	case *ast.EntryMember:
		return "entry", true
	case *ast.DoMember:
		return "do", true
	case *ast.ExitMember:
		return "exit", true
	}
	return "", false
}

// subactionActions returns the actions a state's subaction member performs:
// none for a bare `entry;`.
func subactionActions(node ast.Node) []ast.Node {
	switch n := node.(type) {
	case *ast.EntryMember:
		return n.Actions
	case *ast.DoMember:
		return n.Actions
	case *ast.ExitMember:
		return n.Actions
	}
	return nil
}

// subactionMembers returns the `entry`, `do` and `exit` members written in a
// state's body, in source order.
func subactionMembers(owner ast.Node) []ast.Node {
	var out []ast.Node
	for _, member := range declaredBodyOf(owner) {
		if _, ok := SubactionKind(member); ok {
			out = append(out, member)
		}
	}
	return out
}

// subactionOf returns the subaction member of owner's body that performs the
// action decl, and its kind; ok is false for a member performed by no subaction.
func subactionOf(owner, decl ast.Node) (ast.Node, string, bool) {
	if decl == nil {
		return nil, "", false
	}
	for _, member := range subactionMembers(owner) {
		for _, action := range subactionActions(member) {
			if action == decl {
				kind, _ := SubactionKind(member)
				return member, kind, true
			}
		}
	}
	return nil, "", false
}

// declaredBodyOf returns the members a state declaration's body is written
// with, as parsed: the subaction members among them stand for their actions.
func declaredBodyOf(decl ast.Node) []ast.Node {
	switch d := decl.(type) {
	case *ast.Usage:
		return d.Members
	case *ast.Definition:
		return d.Members
	}
	return nil
}

// withSubactionActions returns members with each subaction member's actions
// standing in its place, so a statement `entry` performs is a statement of the body.
func withSubactionActions(members []ast.Node) []ast.Node {
	var out []ast.Node
	for _, member := range members {
		if _, ok := SubactionKind(member); ok {
			out = append(out, subactionActions(member)...)
			continue
		}
		out = append(out, member)
	}
	return out
}

// emptySubactionMemberships returns the StateSubactionMemberships of sym's subaction members that
// perform no action (`entry;`), each a membership owning nothing, as the conversion writes it.
func (m *Model) emptySubactionMemberships(sym *symbols.Symbol) []*symbols.Symbol {
	if m == nil || sym == nil || sym.Implicit != nil || sym.Expr != nil || sym.Recorded() {
		return nil
	}
	var out []*symbols.Symbol
	for _, member := range subactionMembers(sym.Decl) {
		if len(subactionActions(member)) != 0 {
			continue
		}
		out = append(out, &symbols.Symbol{
			Kind:       symbols.SymbolRelationship,
			DocName:    sym.DocName,
			DeclSpan:   member.Span(),
			Visibility: sym.Visibility,
			Implicit:   &symbols.ImplicitRelationship{Owner: sym, Form: symbols.ImplicitMembership, Decl: member},
		})
	}
	return out
}

// SubactionMemberOf returns the `entry`, `do` or `exit` member of owner's body
// that performs member, through which owner owns it.
func SubactionMemberOf(owner, member *symbols.Symbol) (ast.Node, bool) {
	if owner == nil || member == nil {
		return nil, false
	}
	node, _, ok := subactionOf(owner.Decl, member.Decl)
	return node, ok
}
