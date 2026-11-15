package runtime

import (
	"slices"

	"github.com/Open-MBEE/OpenSysML/internal/ir/lower"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// caseAssertions holds what a case's flow decided of the assertions its
// successions order, each judged where the token reached it (SysML v2 §7.20.3).
type caseAssertions struct {
	shape *loadedCalc
	// judged are the verdicts by asserting member, one per required condition.
	judged map[ast.Node][]AnalysisVerdict
}

// unreachedAssertion is why an ordered assertion the flow never reached is undecided.
const unreachedAssertion = "the flow did not reach the assertion"

// judge checks the assertion against the values in reach where the flow stands;
// a later reach keeps the graver verdict of each condition, so a loop reports its first failure.
func (a *caseAssertions) judge(ctx *Context, ec *EvalContext, s lower.Assert) {
	check := conditionCheck{
		sym: a.shape.Sym, kind: a.shape.Kind, what: "assertion",
		element: a.shape.Name, self: ec.self, frames: slices.Clone(ec.frames),
	}
	var verdicts []AnalysisVerdict
	for _, cond := range ctx.caseMemberConditions(scopedMember{node: s.Node, scope: s.Scope}) {
		if cond.Required {
			verdicts = append(verdicts, ctx.assertionVerdict(check, cond))
		}
	}
	earlier, reached := a.judged[s.Node]
	if !reached {
		a.judged[s.Node] = verdicts
		return
	}
	for i := range verdicts {
		if i < len(earlier) && verdictGravity(verdicts[i].Status) > verdictGravity(earlier[i].Status) {
			earlier[i] = verdicts[i]
		}
	}
}

// verdictGravity orders the statuses by what a repeated check reports: a
// violation over a check that could not be evaluated, either over one that held.
func verdictGravity(status VerdictStatus) int {
	switch status {
	case VerdictNotSatisfied:
		return 2
	case VerdictUndecided:
		return 1
	default:
		return 0
	}
}

// assertionVerdict is the verdict of one asserted condition of a case.
func (ctx *Context) assertionVerdict(check conditionCheck, cond Condition) AnalysisVerdict {
	name, named := assertionName(cond)
	verdict := ctx.analysisVerdict("assertion", name, check, []Condition{cond})
	verdict.Symbol = named
	return verdict
}

// positionalAssertions are the verdicts the run's flow reached for the asserting
// member, and whether the flow orders it at all, so is judged there alone.
func (run *calcRun) positionalAssertions(node ast.Node) (verdicts []AnalysisVerdict, reached, ordered bool) {
	if run.perf != nil && run.perf.assertions != nil {
		verdicts, reached = run.perf.assertions.judged[node]
	}
	return verdicts, reached, reached || run.shape.ordersAssertion(node)
}

// ordersAssertion reports whether the flow the case's body states performs the
// asserting member as a step.
func (shape *loadedCalc) ordersAssertion(node ast.Node) bool {
	for _, step := range shape.Steps {
		block, ok := step.(lower.Block)
		if !ok || block.Graph == nil {
			continue
		}
		for _, stmt := range block.Graph.Bodies[node] {
			if _, asserts := stmt.(lower.Assert); asserts {
				return true
			}
		}
	}
	return false
}
