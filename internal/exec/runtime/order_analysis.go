package runtime

import (
	"github.com/Open-MBEE/OpenSysML/internal/ir/lower"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast/astcodec"
)

type bodyOrderAnalysis struct {
	ownReorders bool
	ownEffects  bool
	reorders    bool
	effects     bool
	calls       []*bodyOrderAnalysis
}

func (ctx *Context) reordersTransitively(sym *symbols.Symbol) bool {
	analysis := ctx.analyzeBodyOrder(sym)
	return analysis != nil && analysis.reorders
}

func (ctx *Context) pureTransitively(sym *symbols.Symbol) bool {
	analysis := ctx.analyzeBodyOrder(sym)
	return analysis != nil && !analysis.effects
}

func (ctx *Context) analyzeBodyOrder(sym *symbols.Symbol) *bodyOrderAnalysis {
	analysis := ctx.analyzeBodyOrderNode(sym)
	ctx.settleBodyOrderAnalysis()
	return analysis
}

func (ctx *Context) analyzeBodyOrderNode(sym *symbols.Symbol) *bodyOrderAnalysis {
	if sym == nil {
		return nil
	}
	if ctx.orderAnalysis == nil {
		ctx.orderAnalysis = make(map[*symbols.Symbol]*bodyOrderAnalysis)
	}
	if analysis := ctx.orderAnalysis[sym]; analysis != nil {
		return analysis
	}
	analysis := &bodyOrderAnalysis{}
	ctx.orderAnalysis[sym] = analysis
	switch {
	case isCalcSymbol(sym):
		shape, err := ctx.calcShapeOf(sym)
		if err == nil {
			shape.statementOrders.Range(func(_, value any) bool {
				if order, ok := value.(*lower.StatementOrder); ok && order.Reorders(false) {
					analysis.reorders = true
					return false
				}
				return true
			})
			ctx.analyzeOrderStatements(shape.Steps, analysis)
			if shape.performs() {
				analysis.effects = true
			}
			for _, param := range shape.Params {
				ctx.analyzeOrderExpression(param.Default, ctx.calcScope(param.Owner, shape.Sym, nil), analysis)
			}
			for _, binding := range shape.Bindings {
				for _, end := range binding.Ends {
					ctx.analyzeOrderExpression(end.Expr, binding.Scope, analysis)
				}
			}
			ctx.analyzeOrderExpression(shape.ResultExpr, shape.bodyScope(), analysis)
		}
	case isPredicateDecl(sym.Decl):
		conditions := ctx.ConditionsOf(sym, sym.Scope)
		ctx.analyzeOrderConditions(conditions, analysis)
		for _, feature := range ctx.conditionFeatures(sym) {
			ctx.analyzeOrderExpression(feature.expr, feature.scope, analysis)
		}
	}
	analysis.ownReorders, analysis.ownEffects = analysis.reorders, analysis.effects
	return analysis
}

func (ctx *Context) settleBodyOrderAnalysis() {
	for changed := true; changed; {
		changed = false
		for _, analysis := range ctx.orderAnalysis {
			reorders, effects := analysis.ownReorders, analysis.ownEffects
			for _, callee := range analysis.calls {
				reorders = reorders || callee.reorders
				effects = effects || callee.effects
			}
			if analysis.reorders != reorders || analysis.effects != effects {
				analysis.reorders, analysis.effects = reorders, effects
				changed = true
			}
		}
	}
}

func (ctx *Context) analyzeOrderConditions(conditions []Condition, analysis *bodyOrderAnalysis) {
	for _, condition := range conditions {
		if condition.Steps != nil {
			if condition.Steps.Order != nil && condition.Steps.Order.Reorders(false) {
				analysis.reorders = true
			}
			for _, write := range condition.Steps.Footprint.Writes {
				if !write.Local {
					analysis.effects = true
				}
			}
			ctx.analyzeOrderStatements(condition.Steps.Stmts, analysis)
		}
		ctx.analyzeOrderExpression(condition.Expr, condition.Scope, analysis)
		for _, constraint := range condition.Constraints {
			ctx.analyzeOrderCallee(constraint, analysis)
		}
		ctx.analyzeOrderConditions(condition.Group, analysis)
	}
}

func (ctx *Context) analyzeOrderStatements(stmts []lower.Statement, analysis *bodyOrderAnalysis) {
	for _, stmt := range stmts {
		switch statement := stmt.(type) {
		case lower.Send:
			analysis.effects = true
			ctx.analyzeOrderExpression(statement.Message, statement.Scope, analysis)
			ctx.analyzeOrderExpression(statement.TargetExpr, statement.Scope, analysis)
			ctx.analyzeOrderExpression(statement.ReceiverExpr, statement.Scope, analysis)
		case lower.Assign:
			ctx.analyzeOrderExpression(statement.Value, statement.Scope, analysis)
			if statement.Chain != nil {
				analysis.effects = true
				ctx.analyzeOrderExpression(statement.Chain.Base, statement.Scope, analysis)
			}
		case lower.Declare:
			ctx.analyzeOrderExpression(statement.Value, statement.Scope, analysis)
		case lower.DeclareUsage:
		case lower.Block:
			if statement.Order != nil && statement.Order.Reorders(false) {
				analysis.reorders = true
			}
			ctx.analyzeOrderStatements(statement.Statements, analysis)
			ctx.analyzeOrderGraph(statement.Graph, analysis)
		case lower.Loop:
			ctx.analyzeOrderExpression(statement.Condition, statement.Scope, analysis)
			ctx.analyzeOrderExpression(statement.Until, statement.Scope, analysis)
			ctx.analyzeOrderExpression(statement.Collection, statement.Scope, analysis)
			ctx.analyzeOrderStatements(statement.Body.Statements, analysis)
			ctx.analyzeOrderGraph(statement.Body.Graph, analysis)
			if statement.Body.Order != nil && statement.Body.Order.Reorders(false) {
				analysis.reorders = true
			}
		case lower.If:
			ctx.analyzeOrderExpression(statement.Condition, statement.Scope, analysis)
			ctx.analyzeOrderStatements(statement.Then.Statements, analysis)
			ctx.analyzeOrderGraph(statement.Then.Graph, analysis)
			if statement.Then.Order != nil && statement.Then.Order.Reorders(false) {
				analysis.reorders = true
			}
			if statement.Else != nil {
				ctx.analyzeOrderStatements(statement.Else.Statements, analysis)
				ctx.analyzeOrderGraph(statement.Else.Graph, analysis)
				if statement.Else.Order != nil && statement.Else.Order.Reorders(false) {
					analysis.reorders = true
				}
			}
		case lower.Return:
			ctx.analyzeOrderExpression(statement.Value, statement.Scope, analysis)
		case lower.Effect:
			analysis.effects = true
			ctx.analyzeOrderExpression(statement.TargetExpr, statement.Scope, analysis)
		case lower.Assert:
			ctx.analyzeOrderCallee(statement.Sym, analysis)
		case lower.Unsupported:
		}
	}
}

func (ctx *Context) analyzeOrderGraph(graph *lower.ActionGraph, analysis *bodyOrderAnalysis) {
	if graph == nil {
		return
	}
	if len(graph.Connections) > 0 || len(graph.DataFlows) > 0 || len(graph.Accepts) > 0 {
		analysis.effects = true
	}
	for _, node := range graph.Nodes {
		if usage, ok := node.(*ast.Usage); ok {
			if _, performs := nestedInvocation(usage); performs || connectsPins(graph, usage) {
				analysis.effects = true
			}
		}
	}
	for _, order := range graph.StatementOrders {
		if order != nil && order.Reorders(false) {
			analysis.reorders = true
		}
	}
	for _, stmts := range graph.Bodies {
		ctx.analyzeOrderStatements(stmts, analysis)
	}
	for _, attribute := range graph.Attributes {
		ctx.analyzeOrderExpression(attribute.Value, attribute.Scope, analysis)
	}
	for _, features := range graph.Features {
		for _, feature := range features {
			ctx.analyzeOrderExpression(feature.Value, feature.Scope, analysis)
		}
	}
	for _, binding := range append(append([]lower.PinBinding(nil), graph.Bindings...), graph.ValueBindings...) {
		ctx.analyzeOrderExpression(binding.Other, binding.Scope, analysis)
		if binding.OtherChain != nil {
			ctx.analyzeOrderExpression(binding.OtherChain.Base, binding.Scope, analysis)
		}
	}
	for _, accept := range graph.Accepts {
		ctx.analyzeOrderExpression(accept.Trigger, accept.Scope, analysis)
	}
	for _, subflow := range graph.Subflows {
		if subflow != nil {
			ctx.analyzeOrderGraph(subflow.Graph, analysis)
		}
	}
}

func (ctx *Context) analyzeOrderExpression(expression ast.Node, scope *symbols.Scope, analysis *bodyOrderAnalysis) {
	if expression == nil || ctx.model == nil || ctx.model.resolver == nil {
		return
	}
	for node := range astcodec.Reachable(expression) {
		var candidates []*symbols.Symbol
		switch expression := node.(type) {
		case *ast.InvocationExpr:
			if expression.Type != nil {
				candidates = ctx.model.resolver.InvocationCandidates(scope, expression.Type)
			}
		case *ast.FeatureReference:
			if expression.Name != nil {
				if sym, ok := ctx.model.resolver.ResolveQualified(scope, expression.Name); ok {
					candidates = []*symbols.Symbol{sym}
				}
			}
		}
		for _, candidate := range candidates {
			ctx.analyzeOrderCallee(candidate, analysis)
		}
	}
}

func (ctx *Context) analyzeOrderCallee(sym *symbols.Symbol, analysis *bodyOrderAnalysis) {
	if sym == nil {
		return
	}
	if sym.Kind == symbols.SymbolAlias && ctx.model != nil && ctx.model.resolver != nil {
		if target, ok := ctx.model.resolver.ResolveAliasTarget(sym); ok {
			ctx.analyzeOrderCallee(target, analysis)
		}
		return
	}
	ctx.mergeOrderAnalysis(ctx.analyzeBodyOrderNode(sym), analysis)
}

func (ctx *Context) mergeOrderAnalysis(from, into *bodyOrderAnalysis) {
	if from == nil || into == nil {
		return
	}
	for _, callee := range into.calls {
		if callee == from {
			return
		}
	}
	into.calls = append(into.calls, from)
}
