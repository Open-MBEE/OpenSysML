package lower

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// CalcBody lowers the computation a calculation or case body states, in
// declaration order and in the scope it was written in. A member stating no
// computation — an input parameter, documentation, a nested definition — is
// skipped. A result the body declares names the value answered with, not a
// step, so it runs after the steps; a `return` inside a branch or loop stops
// the body there. owner is the declaration whose body members are: a case's
// body whose members include action nodes performs them as its steps, lowered
// as one Block over the flow they state (caseSteps); a calc's does not.
func CalcBody(owner ast.Node, members []ast.Node, scope *symbols.Scope) []Statement {
	body := make([]ast.Node, 0, len(members))
	for _, member := range members {
		if actual := unwrapMembership(member); actual != nil {
			body = append(body, actual)
		}
	}
	if PerformsSteps(owner) && len(flowNodesAmong(body)) > 0 {
		return caseSteps(owner, body, scope)
	}

	var stmts, results []Statement
	for _, member := range body {
		stmt, ok := calcStep(member, scope)
		if !ok {
			continue
		}
		if _, isResult := stmt.(Return); isResult {
			results = append(results, stmt)
			continue
		}
		stmts = append(stmts, stmt)
	}
	return append(stmts, results...)
}

// calcStep lowers one member of a calculation body and reports whether it
// states a step or a result; a result is a Return.
func calcStep(member ast.Node, scope *symbols.Scope) (Statement, bool) {
	switch m := member.(type) {
	case *ast.Usage:
		if m.Direction == ast.DirIn || m.Direction == ast.DirInOut {
			// An input parameter is bound by the invocation, not by the body.
			return nil, false
		}
		return usageStatement(m, scope)
	case *ast.Definition, *ast.Documentation, *ast.Comment, *ast.Import, *ast.Alias:
		// Declares a member of the calculation, not a step of it.
		return nil, false
	case *ast.SuccessionEdge:
		// A calculation body runs its steps in declaration order, so a
		// succession states nothing the order does not already state.
		return nil, false
	default:
		if ast.IsExpression(member) {
			return Return{Value: member, Node: member, Scope: scope}, true
		}
		return lowerStatement(member, scope), true
	}
}

// usageStatement lowers a usage written in a statement position: a bound result
// parameter returns the value it binds, an accept parameter states an effect,
// and an attribute declares a value the statements around it read and write.
func usageStatement(u *ast.Usage, scope *symbols.Scope) (Statement, bool) {
	if u.IsAccept {
		return Effect{Kind: EffectAccept, Node: u, Scope: scope}, true
	}
	name, _ := ast.EffectiveName(u)
	// A result parameter binding no value only names the result, so it states no
	// step of the computation.
	if u.IsResult || u.Direction == ast.DirOut {
		if u.Value == nil {
			return nil, false
		}
		// An initial value (`:=`) is what the output holds when the body starts,
		// for its assignments to replace; a binding (`=`) is the value it returns.
		if u.ValueIsInitial && name != "" {
			return Declare{Name: name, Value: u.Value, Node: u, Scope: scope}, true
		}
		return Return{Value: u.Value, Node: u, Scope: scope}, true
	}
	if u.Kind == ast.UsageAttribute && name != "" {
		return Declare{Name: name, Value: u.Value, Node: u, Scope: scope}, true
	}
	if (u.Kind == ast.UsageCalc || u.Kind == ast.UsageAnalysisCase || u.Kind == ast.UsageVerificationCase) && name != "" {
		return DeclareUsage{Name: name, Node: u, Scope: scope}, true
	}
	return nil, false
}

// Returns reports whether the statements return a value on some path, so a
// calculation whose body only computes can be told from one that has no result.
func Returns(stmts []Statement) bool {
	for _, stmt := range stmts {
		switch s := stmt.(type) {
		case Return:
			return true
		case If:
			if blockReturns(s.Then) {
				return true
			}
			if s.Else != nil && blockReturns(*s.Else) {
				return true
			}
		case Loop:
			if blockReturns(s.Body) {
				return true
			}
		case Block:
			if blockReturns(s) {
				return true
			}
		}
	}
	return false
}

// blockReturns reports whether a block returns a value on some path, wherever
// its statements live.
func blockReturns(block Block) bool {
	return Returns(block.Steps())
}
