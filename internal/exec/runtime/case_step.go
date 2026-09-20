package runtime

import (
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/ir/lower"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
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

// caseStepFrame is a case step whose body paused for a wait: the evaluation under
// way, to finish once the wait is over, and the step it runs as.
type caseStepFrame struct {
	start   *calcUsageStart
	run     *calcRun // the finished evaluation, nil while the body runs
	endStep func()
}

// abandon ends the step with the body that was performing the case.
func (f *caseStepFrame) abandon(*Context) { f.endStep() }

func (f *caseStepFrame) clone() bodyFrame { c := *f; return &c }

func (f *caseStepFrame) spell(s *stateSpeller) string {
	return "case " + f.start.shape.Label + " " + s.nested(f.start.host.flow)
}

// performCase runs a nested case as a step of the body (SysML v2 §7.21.2): its own
// body evaluates, taking the enclosing subject where it binds none, and the
// performance holds its outputs for later steps to read as `step.pin`.
func (e *performances) performCase(perf *actionFrame) error {
	f, resumed, err := popFrame[*caseStepFrame](e.ctx)
	if err != nil {
		return err
	}
	if !resumed {
		sym, declaring, err := caseStepSymbol(perf.flow, perf.node)
		if err != nil {
			return err
		}
		activation, endStep := e.ctx.beginStep()
		reader := e.evalContextAround(perf, declaring)
		reader.inBehaviorBody = true
		reader.activation = activation
		f = &caseStepFrame{endStep: endStep}
		if f.start, f.run, err = e.ctx.beginCalcUsage(reader, sym); err != nil {
			endStep()
			return fmt.Errorf("%s: %w", nodeDescription(perf.node), err)
		}
	}
	if f.run == nil {
		// The case's own flow may wait on the clock, pausing the body; the step
		// is finished once the body resumes past the wait.
		run, err := e.ctx.finishCalcUsage(f.start)
		if paused(err) {
			return e.ctx.pausing(f, err)
		}
		if err != nil {
			f.endStep()
			return fmt.Errorf("%s: %w", nodeDescription(perf.node), err)
		}
		f.run = run
	}
	defer f.endStep()
	run := f.run
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
