package symbols

import "github.com/Open-MBEE/OpenSysML/internal/syntax/ast"

// InStateMachine reports whether an edge written in scope belongs to a state
// machine, the body a vertex lookup applies to; an action body (a transition's
// included) or anything else is not one.
func InStateMachine(scope *Scope) bool {
	for s := scope; s != nil; s = s.Parent() {
		switch n := s.Node().(type) {
		case *ast.TransitionMember:
			return false
		case *ast.Definition:
			if n.Kind == ast.DefState {
				return true
			}
			if n.Kind == ast.DefAction {
				return false
			}
		case *ast.Usage:
			if n.Kind == ast.UsageState {
				return true
			}
			if n.Kind == ast.UsageAction {
				return false
			}
		}
	}
	return false
}

// FirstNamesSource reports whether the name after `first` in n refers to the
// source of a succession (`first a then b;`, parsed as an InitialNode only in
// an action body) rather than declaring a start marker (`first a;`).
func FirstNamesSource(n *ast.InitialNode) bool {
	return n.Successor != nil
}
