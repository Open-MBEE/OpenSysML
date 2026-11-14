package lower

import (
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// internExprs interns what the graph's features, attributes, successions and accept
// triggers state in its table, each in the scope it is evaluated in.
func (g *ActionGraph) internExprs() {
	if g.Exprs == nil {
		return
	}
	g.internAttributes(g.Attributes)
	g.internFeatures(g.Parameters)
	for _, node := range g.Nodes {
		g.internFeatures(g.Features[node])
		if accept, ok := g.Accepts[node]; ok {
			accept.TriggerExprID = g.Exprs.Intern(triggerExpr(accept.Trigger), g.Scope)
			g.Accepts[node] = accept
		}
		edges := g.Edges[node]
		for i := range edges {
			scope := g.DeclScope(edges[i].Decl)
			edges[i].GuardID = g.Exprs.Intern(edges[i].Guard, scope)
			edges[i].Probability.intern(g.Exprs, scope)
		}
	}
}

func (g *ActionGraph) internAttributes(attrs []Attribute) {
	for i := range attrs {
		attrs[i].ValueID = g.Exprs.Intern(attrs[i].Value, scopeOr(attrs[i].Scope, g.Scope))
	}
}

func (g *ActionGraph) internFeatures(features []Feature) {
	for i := range features {
		features[i].ValueID = g.Exprs.Intern(features[i].Value, scopeOr(features[i].Scope, g.Scope))
	}
}

// internExprs interns the expressions the machine's attributes, features, transitions
// and entry transitions state in its table, each in the scope it is evaluated in.
func (g *StateGraph) internExprs() {
	if g.Exprs == nil {
		return
	}
	attributes := func(attrs []Attribute) {
		for i := range attrs {
			attrs[i].ValueID = g.Exprs.Intern(attrs[i].Value, scopeOr(attrs[i].Scope, g.Scope))
		}
	}
	attributes(g.Attributes)
	for _, state := range g.States {
		attributes(g.StateAttributes[state])
		if config, ok := g.RunToCompletion[state]; ok {
			config.ValueID = g.Exprs.Intern(config.Value, config.ValueScope)
			g.RunToCompletion[state] = config
		}
	}
	for _, features := range g.Features {
		for i := range features {
			features[i].ValueID = g.Exprs.Intern(features[i].Value, scopeOr(features[i].Scope, g.Scope))
		}
	}
	for _, transitions := range g.Transitions {
		for _, trans := range transitions {
			trans.GuardID = g.Exprs.Intern(trans.Guard, trans.BodyScope)
			trans.Probability.intern(g.Exprs, trans.BodyScope)
			trans.TriggerExprID = g.Exprs.Intern(triggerExpr(trans.Trigger), trans.Scope)
		}
	}
	for _, entries := range g.EntryTransitions {
		for _, entry := range entries {
			entry.GuardID = g.Exprs.Intern(entry.Guard, entry.Scope)
		}
	}
}

// triggerExpr is the expression a trigger states: the condition of a change, the
// time of a timer; nil for a trigger stating none.
func triggerExpr(trigger ast.Node) ast.Node {
	switch event := trigger.(type) {
	case *ast.ChangeEvent:
		return event.Condition
	case *ast.TimeEvent:
		return event.Duration
	}
	return nil
}

// intern interns the weight in exprs, in the scope its succession or transition
// is evaluated in.
func (p *Probability) intern(exprs *ExprTable, scope *symbols.Scope) {
	if p != nil {
		p.ExprID = exprs.Intern(p.Expr, scope)
	}
}

func scopeOr(scope, otherwise *symbols.Scope) *symbols.Scope {
	if scope != nil {
		return scope
	}
	return otherwise
}
