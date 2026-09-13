package runtime

import (
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/lower"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// isCaseStep reports whether node is a case nested in a body, performed as a step.
func isCaseStep(node ast.Node) bool {
	usage, ok := node.(*ast.Usage)
	return ok && lower.IsCaseNode(usage)
}

// stepSymbol resolves the symbol a nested usage node declares and the body declaring
// it; nil when the node declares none.
func stepSymbol(flow *lower.ActionGraph, node ast.Node) (*symbols.Symbol, *symbols.Scope) {
	scope := nodeScope(flow, node)
	declaring := scope.Parent()
	if declaring == nil {
		declaring = flow.Scope
	}
	sym := memberSymbol(declaring, node)
	if sym == nil {
		sym = memberSymbol(flow.Scope, node)
		declaring = flow.Scope
	}
	return sym, declaring
}

// caseStepSymbol resolves the case a nested usage node declares, in the body that declares it.
func caseStepSymbol(flow *lower.ActionGraph, node ast.Node) (*symbols.Symbol, *symbols.Scope, error) {
	sym, declaring := stepSymbol(flow, node)
	if sym == nil || !isCalcUsageSymbol(sym) {
		return nil, nil, fmt.Errorf("%w: %s is not resolved to a case", ErrNotACalcUsage, nodeDescription(node))
	}
	return sym, declaring, nil
}

// performCase runs a nested case as a step of the body (SysML v2 §7.21.2): its own
// body evaluates, taking the enclosing subject where it binds none, and the
// performance holds its outputs for later steps to read as `step.pin`.
func (e *performances) performCase(perf *actionFrame) error {
	sym, declaring, err := caseStepSymbol(perf.flow, perf.node)
	if err != nil {
		return err
	}
	activation, endStep := e.ctx.beginStep()
	defer endStep()
	reader := e.evalContextAround(perf, declaring)
	reader.inBehaviorBody = true
	reader.activation = activation
	run, err := e.ctx.calcUsageRun(reader, sym)
	if err != nil {
		return fmt.Errorf("%s: %w", nodeDescription(perf.node), err)
	}
	outputs, err := run.outputValues(e.ctx)
	if err != nil {
		return fmt.Errorf("%s: %w", nodeDescription(perf.node), err)
	}
	for _, out := range outputs {
		perf.features[out.Name] = ast.DirOut
		perf.data[perf.key(out.Name)] = out.Value
	}
	if out := run.shape.resultOutput(); out != nil {
		perf.result = out.Name
		if perf.result == "" {
			perf.result = resultOutputName
		}
	}
	return nil
}
