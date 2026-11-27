package semantics

import (
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// ControlForm is the control action a declaration is written as (SysML v2 §7.16.9 to §7.16.12).
type ControlForm int

const (
	ControlNone ControlForm = iota
	ControlAssignment
	ControlIfThen
	ControlIfThenElse
	ControlWhileLoop
	ControlForLoop
)

// controlBaseFQNs are the base features of the control forms (SysML v2 Table 32,
// check*ActionUsageSpecialization).
var controlBaseFQNs = map[ControlForm]string{
	ControlAssignment: "Actions::assignmentActions",
	ControlIfThen:     "Actions::ifThenActions",
	ControlIfThenElse: "Actions::ifThenElseActions",
	ControlWhileLoop:  "Actions::whileLoopActions",
	ControlForLoop:    "Actions::forLoopActions",
}

// controlSubactionFQNs are the features a control form nested in an action subsets (SysML v2
// Table 32, check*ActionUsageSubactionSpecialization).
var controlSubactionFQNs = map[ControlForm]string{
	ControlAssignment: "Actions::Action::assignments",
	ControlIfThen:     "Actions::Action::ifSubactions",
	ControlIfThenElse: "Actions::Action::ifSubactions",
	ControlWhileLoop:  "Actions::Action::whileLoops",
	ControlForLoop:    "Actions::Action::forLoops",
}

// controlStatementOf is the assignment, conditional or loop decl is written as: the statement
// itself, or the one an action node declares; nil for any other declaration.
func controlStatementOf(decl ast.Node) ast.Node {
	if usage, ok := decl.(*ast.Usage); ok {
		decl = symbols.ActionNodeStatementOf(usage)
	}
	switch decl.(type) {
	case *ast.AssignmentActionNode, *ast.IfActionNode, *ast.WhileLoopActionNode:
		return decl
	}
	return nil
}

// controlFormOf is the control form decl is written as.
func controlFormOf(decl ast.Node) ControlForm {
	switch d := controlStatementOf(decl).(type) {
	case *ast.AssignmentActionNode:
		return ControlAssignment
	case *ast.IfActionNode:
		if d.Else != nil {
			return ControlIfThenElse
		}
		return ControlIfThen
	case *ast.WhileLoopActionNode:
		if d.Kind == ast.LoopFor {
			return ControlForLoop
		}
		return ControlWhileLoop
	}
	return ControlNone
}

// IsControlKindMember reports whether sym, a control action, takes member from the kind its
// notation implies: a loop written without `until` still inherits WhileLoopAction::untilTest.
func (m *Model) IsControlKindMember(sym, member *symbols.Symbol) bool {
	if m == nil || sym == nil || member == nil {
		return false
	}
	base := m.symbolByFQN(controlBaseFQNs[controlFormOf(sym.Decl)])
	owner := m.ownerOf(member)
	return base != nil && owner != nil && m.Conforms(base, owner)
}
