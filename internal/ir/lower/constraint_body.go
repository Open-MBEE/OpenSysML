package lower

import (
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// ConstraintStep lowers one member of a constraint body to the step it states,
// as calcStep does for a calculation body: statements run in declaration order
// and body-local declarations declare names the steps around them read. false
// for what a constraint body states other ways: the parameters a check binds
// (`in x`), the conditions the body exists to hold (expressions and nested
// constraint, require, assume and assert members), the unnamed bindings a
// parameter redefinition writes, and members declaring nothing a step can run.
// The successions and control nodes a calculation body resolves into a flow of
// steps are lowered as unexecutable steps — a verdict orders its steps by
// declaration, so the host reports them rather than skipping them.
func ConstraintStep(member ast.Node, scope *symbols.Scope) (Statement, bool) {
	if actual := unwrapMembership(member); actual != nil {
		member = actual
	}
	switch m := member.(type) {
	case *ast.ConstraintMember, *ast.RequireMember, *ast.AssumeMember, *ast.SubjectMember:
		// Conditions and the subject declaration of the body, evaluated in the
		// state the steps leave — not steps of it.
		return nil, false
	case *ast.SuccessionEdge, *ast.ControlFlowEdge, *ast.InitialNode, *ast.ForkNode,
		*ast.JoinNode, *ast.MergeNode, *ast.DecisionNode, *ast.FinalNode:
		return Unsupported{Description: statedFlowKeyword(member), Node: member, Scope: scope}, true
	case *ast.Usage:
		if m.Direction == ast.DirIn || m.Direction == ast.DirInOut {
			// A parameter is bound by the check, not by the body.
			return nil, false
		}
		if m.IsSuccessionFlow() || m.Kind == ast.UsageSuccession {
			return Unsupported{Description: statedFlowKeyword(member), Node: member, Scope: scope}, true
		}
		if stmt, ok := usageStatement(m, scope); ok {
			return stmt, true
		}
		if m.IsTerminate {
			return Effect{Kind: EffectTerminate, Node: m, Scope: scope, Terminates: TerminateEnclosing}, true
		}
		if m.Kind == ast.UsageAction {
			// An action usage states a performance of the action — refused by
			// the host as an effect outside the body's own performance.
			return performEffect(m, scope), true
		}
		// An unnamed binding (`:>> limit = 5.0`), an actor or a feature of
		// another kind states something the check's environment holds rather
		// than a step performing.
		return nil, false
	case *ast.Definition, *ast.Documentation, *ast.Comment, *ast.Import, *ast.Alias:
		return nil, false
	default:
		if ast.IsExpression(member) {
			return nil, false
		}
		return lowerStatement(member, scope), true
	}
}

// statedFlowKeyword names the statement a flow member was written with, as the
// refusal to run it in a verdict must name it — a verdict orders steps by
// declaration, not by the successions stated.
func statedFlowKeyword(node ast.Node) string {
	switch n := node.(type) {
	case *ast.InitialNode:
		return "`first` statement"
	case *ast.ForkNode:
		return "`fork` statement"
	case *ast.JoinNode:
		return "`join` statement"
	case *ast.MergeNode:
		return "`merge` statement"
	case *ast.DecisionNode:
		return "`decide` statement"
	case *ast.FinalNode:
		return "`done` statement"
	case *ast.Usage:
		if n.IsSuccessionFlow() {
			return "`succession flow` statement"
		}
		return "`succession` statement"
	default:
		return "`then` statement"
	}
}
