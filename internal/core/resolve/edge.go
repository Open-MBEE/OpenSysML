package resolve

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// resolveSuccessionEdge resolves the ends a succession names, the members
// lowering sequences the token flow over: a misspelled one is reported here
// rather than only when the model runs.
func (r *Resolver) resolveSuccessionEdge(scope *symbols.Scope, edge *ast.SuccessionEdge) {
	r.resolveEdgeEnd(scope, edge.Source, edge.SourceMember, edge.SourceImplied)
	r.resolveEdgeEnd(scope, edge.Target, edge.TargetMember, edge.TargetImplied)
}

// resolveControlFlowEdge resolves the ends of a guarded branch of a decision.
func (r *Resolver) resolveControlFlowEdge(scope *symbols.Scope, edge *ast.ControlFlowEdge) {
	r.resolveEdgeEnd(scope, edge.Source, edge.SourceMember, edge.SourceImplied)
	r.resolveEdgeEnd(scope, edge.Target, edge.TargetMember, edge.TargetImplied)
}

// resolveEdgeEnd resolves an edge or initial-node end the author named. An end bound to a member by
// position, or one the notation supplied from the member beside the keyword,
// names nothing an author could misspell: lowering reads that member itself.
func (r *Resolver) resolveEdgeEnd(scope *symbols.Scope, qn *ast.QualifiedName, member ast.Node, implied bool) {
	if qn == nil || len(qn.Parts) == 0 || member != nil {
		return
	}
	if implied {
		// The name is the body's own member's, the one a `first x` label also
		// starts at; record what it binds, undiagnosed.
		if len(qn.Parts) != 1 {
			return
		}
		sym, ok := memberPastLabels(scope, qn.Parts[0].Text)
		if !ok {
			sym, ok = scope.LookupLocal(qn.Parts[0].Text)
		}
		if ok {
			r.resolvedPart(qn, 0, sym)
		}
		return
	}
	if symbols.InStateMachine(scope) {
		// In a machine, judge the end as a transition endpoint, including its kind.
		r.ResolveEndpoint(scope, qn)
		return
	}
	r.ResolveQualified(scope, qn)
}
