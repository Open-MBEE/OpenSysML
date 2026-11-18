package lower

import (
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/ir/program"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// NodeForm is how a node of an action graph was written, which decides what a
// consumer checks of it: finer than its NodeKind, which says what it does.
type NodeForm uint8

const (
	// FormOther is a node of no form below.
	FormOther NodeForm = iota
	FormInitial
	FormFinal
	FormFork
	FormJoin
	FormMerge
	FormDecision
	// FormAction is an action node written with the `action` keyword in a flow.
	FormAction
	// FormUsage is a usage standing as a node: a nested or performed action, an accept.
	FormUsage
	FormLoop
	FormIf
	FormAssign
	FormSend
	FormTerminate
)

// FormOf is how the node behind id was written.
func (g *ActionGraph) FormOf(id program.NodeID) NodeForm {
	switch g.Provenance.Node(id).(type) {
	case *ast.InitialNode:
		return FormInitial
	case *ast.FinalNode:
		return FormFinal
	case *ast.ForkNode:
		return FormFork
	case *ast.JoinNode:
		return FormJoin
	case *ast.MergeNode:
		return FormMerge
	case *ast.DecisionNode:
		return FormDecision
	case *ast.ActionExecutionNode:
		return FormAction
	case *ast.Usage:
		return FormUsage
	case *ast.WhileLoopActionNode:
		return FormLoop
	case *ast.IfActionNode:
		return FormIf
	case *ast.AssignmentActionNode:
		return FormAssign
	case *ast.SendStatement:
		return FormSend
	case *ast.TerminateStatement:
		return FormTerminate
	}
	return FormOther
}

// FormName names the form of the node behind id for a message refusing it.
func (g *ActionGraph) FormName(id program.NodeID) string {
	return fmt.Sprintf("%T", g.Provenance.Node(id))
}

// Invokes reports whether the node behind id performs another action rather
// than stating a body of its own, as the interpreter decides it.
func (g *ActionGraph) Invokes(id program.NodeID) bool {
	switch n := g.Provenance.Node(id).(type) {
	case *ast.ActionExecutionNode:
		return n.ActionRef != nil
	case *ast.Usage:
		if n.PerformedInvocation() != nil {
			return true
		}
		for _, rel := range n.Relationships {
			if rel.Kind != ast.RelTyping && rel.Kind != ast.RelReferences {
				continue
			}
			if _, ok := rel.Target.(*ast.QualifiedName); ok {
				return true
			}
		}
	}
	return false
}

// CaseNode reports whether the node behind id is a nested analysis or verification case.
func (g *ActionGraph) CaseNode(id program.NodeID) bool {
	usage, ok := g.Provenance.Node(id).(*ast.Usage)
	return ok && IsCaseNode(usage)
}

// ExpressionOf is the expression an action node written `action a = e` computes, 0 for none.
func (g *ActionGraph) ExpressionOf(id program.NodeID) program.ExprID {
	if action, ok := g.Provenance.Node(id).(*ast.ActionExecutionNode); ok {
		return g.Exprs.Intern(action.Expression, g.Scope)
	}
	return 0
}

// TraceLabel names the node behind id as a trace names it: by its name, else by what it does.
func (g *ActionGraph) TraceLabel(id program.NodeID) string {
	return traceLabel(g.Provenance.Node(id))
}

func traceLabel(node ast.Node) string {
	switch n := node.(type) {
	case nil:
		return "nil"
	case *ast.InitialNode:
		return labelOr(n.Name(), "initial")
	case *ast.FinalNode:
		return "done"
	case *ast.ForkNode:
		return labelOr(n.Name, "fork")
	case *ast.JoinNode:
		return labelOr(n.Name, "join")
	case *ast.MergeNode:
		return labelOr(n.Name, "merge")
	case *ast.DecisionNode:
		return labelOr(n.Name, "decision")
	case *ast.ActionExecutionNode:
		return labelOr(n.Name, "action")
	case *ast.Usage:
		if name, _ := ast.EffectiveName(n); name != "" {
			return name
		}
		return fmt.Sprintf("usage_%s", n.Kind)
	default:
		return fmt.Sprintf("%T", n)
	}
}

func labelOr(name, kind string) string {
	if name != "" {
		return name
	}
	return kind
}

// HasStepMultiplicityOf is HasStepMultiplicity of the node behind id.
func (g *ActionGraph) HasStepMultiplicityOf(id program.NodeID, model *semantics.Model) bool {
	return g.HasStepMultiplicity(g.Provenance.Node(id), model)
}

// StepCountOf is StepCount of the node behind id.
func (g *ActionGraph) StepCountOf(id program.NodeID, model *semantics.Model) (int64, error) {
	return g.StepCount(g.Provenance.Node(id), model)
}

// CheckStepOf is CheckStep of the node behind id.
func (g *ActionGraph) CheckStepOf(id program.NodeID, model *semantics.Model) error {
	return g.CheckStep(g.Provenance.Node(id), model)
}

// MultiplicityTextOf is MultiplicityText of the node behind id.
func (g *ActionGraph) MultiplicityTextOf(id program.NodeID, model *semantics.Model) string {
	return g.MultiplicityText(g.Provenance.Node(id), model)
}

// CrossesPerPerformanceOf is CrossesPerPerformance of the node behind id.
func (g *ActionGraph) CrossesPerPerformanceOf(id program.NodeID, model *semantics.Model) bool {
	return g.CrossesPerPerformance(g.Provenance.Node(id), model)
}

// BodyDividesOf is BodyDivides of the node behind id.
func (g *ActionGraph) BodyDividesOf(id program.NodeID) bool {
	return BodyDivides(g, g.Provenance.Node(id))
}

// BodyOrderOf is BodyStatementOrder of stmts, a body of the node behind id.
func (g *ActionGraph) BodyOrderOf(id program.NodeID, stmts []Statement) *StatementOrder {
	return BodyStatementOrder(g, g.Provenance.Node(id), stmts)
}

// Timed reports whether the accept waits on the clock rather than for a message.
func (a Accept) Timed() bool {
	_, timed := a.Trigger.(*ast.TimeEvent)
	return timed
}

// For reports a `for` loop, which iterates over a collection.
func (l Loop) For() bool { return l.Kind == ast.LoopFor }

// PostTested reports a loop whose condition is tested after each run of its body.
func (l Loop) PostTested() bool { return l.Kind == ast.LoopUntil }

// Word is the keyword the loop was written with.
func (l Loop) Word() string { return l.Kind.String() }

// Same reports whether l and other are one loop statement.
func (l Loop) Same(other Loop) bool { return l.Node == other.Node }

// Chained reports an assignment whose target walks a feature chain.
func (a Assign) Chained() bool { return a.Chain != nil }

// Signal names the signal type the send posts: the type its message names, else
// the form its message was written in where it is a value.
func (s Send) Signal() string {
	switch m := s.Message.(type) {
	case *ast.QualifiedName:
		return ast.QualifiedText(m)
	case *ast.InvocationExpr:
		return ast.QualifiedText(m.Type)
	}
	return traceLabel(s.Message)
}

// Written is where the flow was declared and whether it was declared at all.
func (f ObjectFlow) Written() (source.Span, bool) {
	if f.Decl == nil {
		return source.Span{}, false
	}
	return f.Decl.Span(), true
}

// Carrying reports whether the succession is the one flow's declaration states.
func (e ActionEdge) Carrying(flow ObjectFlow) bool { return e.Carries && e.Decl == flow.Decl }

// CountsTarget reports whether the succession states a multiplicity at its target end.
func (e ActionEdge) CountsTarget() bool { return e.TargetMultiplicity != nil }

// ScopeOf is the namespace the node behind id owns, nil for none.
func (g *ActionGraph) ScopeOf(id program.NodeID) *symbols.Scope {
	return g.Scopes[g.Provenance.Node(id)]
}

// Footprint is what evaluating the expression in scope reads and writes.
func (e *ExprEntry) Footprint(resolver *resolve.Resolver, sem *semantics.Model, scope *symbols.Scope) Footprint {
	if e == nil {
		return ExpressionFootprint(resolver, sem, scope, nil)
	}
	return ExpressionFootprint(resolver, sem, scope, e.Tree)
}

// Gated reports a succession flow a succession leads to, which may guard it.
func (f ObjectFlow) Gated() bool { return f.Gate != nil }
