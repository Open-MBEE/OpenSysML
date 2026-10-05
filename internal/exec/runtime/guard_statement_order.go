package runtime

import (
	"fmt"
	"sort"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/ir/lower"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast/astcodec"
)

const guardOrderWherePrefix = "verdict of "

type guardOrderResult struct {
	holds   bool
	err     error
	choices []ChoiceTaken
}

func (ctx *Context) guardUnderStatementOrders(guard ast.Node, step int, scope *symbols.Scope, eval func() (bool, error)) (bool, error) {
	if !ctx.scheduling().ordersStatements() {
		return eval()
	}
	needsSweep, externalWrite := ctx.guardStatementOrderInfo(guard, scope)
	if externalWrite != nil {
		return false, fmt.Errorf("%w: verdict of %s: constraint body may write %s outside its performance",
			ErrOrderDependentGuardEffect, conditionText(guard), externalWrite.String())
	}
	if !needsSweep {
		return eval()
	}
	if ctx.statementOrderSweep != nil {
		return ctx.evalOrderAwareGuard(guard, eval)
	}

	probeDepth := ctx.probes
	results := make(map[string]guardOrderResult)
	bodyNames := make(map[string]bool)
	previousBodies := ctx.statementOrderGuardBodies
	previousGuard := ctx.statementOrderGuard
	previousLabel := ctx.statementOrderGuardLabel
	ctx.statementOrderGuardBodies = bodyNames
	ctx.statementOrderGuard = true
	ctx.statementOrderGuardLabel = conditionText(guard)
	err := ctx.sweepStatementOrderVariants(func(sweep *statementOrderSweep) error {
		holds, err := eval()
		key := guardOrderResultKey(holds, err)
		if _, seen := results[key]; !seen {
			results[key] = guardOrderResult{holds: holds, err: err, choices: append([]ChoiceTaken(nil), sweep.choices...)}
		}
		return nil
	})
	ctx.statementOrderGuardBodies = previousBodies
	ctx.statementOrderGuard = previousGuard
	ctx.statementOrderGuardLabel = previousLabel
	if err != nil {
		return false, err
	}
	ordered := guardOrderResults(results)
	if len(ordered) == 0 {
		return false, nil
	}
	if len(ordered) == 1 {
		return ctx.evalGuardAtOrder(guard, ordered[0].choices, eval)
	}
	body := guardOrderBodyName(bodyNames)
	label := conditionText(guard)
	if probeDepth > 0 {
		return false, fmt.Errorf("%w: verdict of %s depends on the statement orders of %s",
			ErrOrderDependentPreview, label, body)
	}
	alternatives := make([]string, len(ordered))
	for i, result := range ordered {
		switch {
		case result.err != nil:
			alternatives[i] = "error: " + result.err.Error()
		case result.holds:
			alternatives[i] = "holds"
		default:
			alternatives[i] = "does not hold"
		}
	}
	where := fmt.Sprintf("%s%s under the statement orders of %s", guardOrderWherePrefix, label, body)
	choice := ChoicePoint{
		Kind:         ChoiceGuardOrder,
		Step:         step,
		Where:        where,
		Alternatives: alternatives,
		Span:         guard.Span(),
	}
	if scope != nil {
		choice.File = scope.DocName()
	}
	choice.Taken = ctx.scheduling().choose(choice, nil)
	if err := ctx.scheduling().refusal(); err != nil {
		return false, err
	}
	ctx.noteChoice(choice)
	return ctx.evalGuardAtOrder(guard, ordered[choice.Taken].choices, eval)
}

func (ctx *Context) evalGuardAtOrder(guard ast.Node, choices []ChoiceTaken, eval func() (bool, error)) (bool, error) {
	prefix := make([]int, len(choices))
	for i, choice := range choices {
		prefix[i] = choice.Taken
	}
	sweep := &statementOrderSweep{prefix: prefix}
	previousSweep := ctx.statementOrderSweep
	previousGuard := ctx.statementOrderGuard
	previousLabel := ctx.statementOrderGuardLabel
	previousBodies := ctx.statementOrderGuardBodies
	ctx.statementOrderSweep = sweep
	ctx.statementOrderGuard = true
	ctx.statementOrderGuardLabel = conditionText(guard)
	ctx.statementOrderGuardBodies = make(map[string]bool)
	defer func() {
		ctx.statementOrderSweep = previousSweep
		ctx.statementOrderGuard = previousGuard
		ctx.statementOrderGuardLabel = previousLabel
		ctx.statementOrderGuardBodies = previousBodies
	}()
	return eval()
}

func (ctx *Context) evalOrderAwareGuard(guard ast.Node, eval func() (bool, error)) (bool, error) {
	previousGuard := ctx.statementOrderGuard
	previousLabel := ctx.statementOrderGuardLabel
	ctx.statementOrderGuard = true
	ctx.statementOrderGuardLabel = conditionText(guard)
	defer func() {
		ctx.statementOrderGuard = previousGuard
		ctx.statementOrderGuardLabel = previousLabel
	}()
	return eval()
}

func (ctx *Context) guardStatementOrderInfo(guard ast.Node, scope *symbols.Scope) (bool, *lower.Place) {
	if guard == nil || ctx.model == nil || ctx.model.resolver == nil {
		return false, nil
	}
	needsSweep := false
	var externalWrite *lower.Place
	seen := make(map[*symbols.Symbol]bool)
	var inspect func(*symbols.Symbol)
	inspect = func(sym *symbols.Symbol) {
		if sym == nil || seen[sym] {
			return
		}
		seen[sym] = true
		switch sym.Kind {
		case symbols.SymbolConstraintDef, symbols.SymbolConstraintUsage,
			symbols.SymbolCalcDef, symbols.SymbolCalcUsage:
			needsSweep = needsSweep || ctx.reordersTransitively(sym)
			if sym.Kind == symbols.SymbolCalcDef || sym.Kind == symbols.SymbolCalcUsage {
				return
			}
			for _, condition := range ctx.ConditionsOf(sym, sym.Scope) {
				write := conditionExternalWrite(condition)
				if externalWrite == nil && write != nil {
					externalWrite = write
				}
				ctx.inspectConditionCalls(condition, sym.Scope, inspect)
				for _, constraint := range condition.Constraints {
					inspect(constraint)
				}
			}
		case symbols.SymbolAlias:
			if target, ok := ctx.model.resolver.ResolveAliasTarget(sym); ok {
				inspect(target)
			}
		}
	}
	for node := range astcodec.Reachable(guard) {
		switch expression := node.(type) {
		case *ast.InvocationExpr:
			if expression.Type != nil {
				for _, sym := range ctx.model.resolver.InvocationCandidates(scope, expression.Type) {
					inspect(sym)
				}
			}
		case *ast.FeatureReference:
			if expression.Name != nil {
				if sym, ok := ctx.model.resolver.ResolveQualified(scope, expression.Name); ok {
					inspect(sym)
				}
			}
		}
	}
	return needsSweep || externalWrite != nil, externalWrite
}

func (ctx *Context) inspectConditionCalls(condition Condition, fallback *symbols.Scope, inspect func(*symbols.Symbol)) {
	scope := condition.Scope
	if scope == nil {
		scope = fallback
	}
	for node := range astcodec.Reachable(condition.Expr) {
		switch expression := node.(type) {
		case *ast.InvocationExpr:
			if expression.Type != nil {
				for _, sym := range ctx.model.resolver.InvocationCandidates(scope, expression.Type) {
					inspect(sym)
				}
			}
		case *ast.FeatureReference:
			if expression.Name != nil {
				if sym, ok := ctx.model.resolver.ResolveQualified(scope, expression.Name); ok {
					inspect(sym)
				}
			}
		}
	}
	for _, nested := range condition.Group {
		ctx.inspectConditionCalls(nested, scope, inspect)
	}
}

func conditionExternalWrite(condition Condition) *lower.Place {
	var externalWrite *lower.Place
	if condition.Steps != nil {
		for _, write := range condition.Steps.Footprint.Writes {
			if !write.Local {
				copy := write
				externalWrite = &copy
				break
			}
		}
	}
	for _, nested := range condition.Group {
		write := conditionExternalWrite(nested)
		if externalWrite == nil {
			externalWrite = write
		}
	}
	return externalWrite
}

func guardOrderResultKey(holds bool, err error) string {
	if err != nil {
		return "error:" + err.Error()
	}
	if holds {
		return "holds"
	}
	return "does not hold"
}

func guardOrderResults(results map[string]guardOrderResult) []guardOrderResult {
	keys := make([]string, 0, len(results))
	for key := range results {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		return guardOrderResultRank(keys[i]) < guardOrderResultRank(keys[j]) ||
			guardOrderResultRank(keys[i]) == guardOrderResultRank(keys[j]) && keys[i] < keys[j]
	})
	ordered := make([]guardOrderResult, len(keys))
	for i, key := range keys {
		ordered[i] = results[key]
	}
	return ordered
}

func guardOrderResultRank(key string) int {
	switch key {
	case "holds":
		return 0
	case "does not hold":
		return 1
	default:
		return 2
	}
}

func guardOrderBodyName(names map[string]bool) string {
	ordered := make([]string, 0, len(names))
	for name := range names {
		ordered = append(ordered, name)
	}
	sort.Strings(ordered)
	if len(ordered) == 0 {
		return "constraint or calculation body"
	}
	if len(ordered) == 1 {
		return ordered[0]
	}
	return strings.Join(ordered, ", ")
}
