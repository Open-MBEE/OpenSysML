package runtime

import (
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/ir/lower"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

type stepMultiplicityKey struct {
	graph *lower.ActionGraph
	node  ast.Node
}

type stepMultiplicityResult struct {
	count int64
	err   error
}

type repetitionGroupID int64

type stepRepetition struct {
	node      ast.Node
	remaining int64
	live      []*actionFrame
}

func (e *ActionExecutor) stepMultiplicity(graph *lower.ActionGraph, node ast.Node) (int64, error) {
	if graph == nil {
		return 1, nil
	}
	if _, declared := graph.Multiplicities[node]; !declared {
		return 1, nil
	}
	if e.stepCounts == nil {
		e.stepCounts = make(map[stepMultiplicityKey]stepMultiplicityResult)
	}
	key := stepMultiplicityKey{graph: graph, node: node}
	if result, ok := e.stepCounts[key]; ok {
		return result.count, result.err
	}
	count, err := graph.StepCount(node, e.ctx.Semantics())
	if err == nil {
		err = graph.CheckStep(node, e.ctx.Semantics())
	}
	if err != nil {
		err = fmt.Errorf("%w: %w", ErrActionStepMultiplicity, err)
	}
	e.stepCounts[key] = stepMultiplicityResult{count: count, err: err}
	return count, err
}

func (e *ActionExecutor) splitRepeatedStep(tokenIdx int, count int64, node ast.Node) error {
	if count > e.ctx.maxActionSteps || count > int64(int(^uint(0)>>1)) {
		return budgetExceeded(ErrActionStepLimitExceeded,
			fmt.Sprintf("execution exceeded max steps (%d steps; raise %s to allow more), possible infinite loop",
				e.ctx.maxActionSteps, MaxActionStepsEnvVar))
	}
	token := e.tokens[tokenIdx]
	frame := token.frame
	if e.nextRepetitionID == 0 {
		e.nextRepetitionID = 1
	}
	group := e.nextRepetitionID
	e.nextRepetitionID++
	if frame.repeats == nil {
		frame.repeats = make(map[repetitionGroupID]*stepRepetition)
	}
	frame.repeats[group] = &stepRepetition{node: node, remaining: count}
	for repetition := int64(1); repetition <= count; repetition++ {
		if repetition == 1 {
			e.tokens[tokenIdx].repetition = repetition
			e.tokens[tokenIdx].repetitionGroup = group
			continue
		}
		next := token
		next.ID = e.nextTokenID
		next.repetition = repetition
		next.repetitionGroup = group
		next.body = nil
		next.Wait = nil
		e.nextTokenID++
		e.tokens = append(e.tokens, next)
	}
	frame.live += int(count) - 1
	return nil
}

func (e *ActionExecutor) passZeroStep(tokenIdx int, node ast.Node) error {
	frame := e.tokens[tokenIdx].frame
	successors, err := e.enabledSuccessions(frame, node)
	if err != nil {
		return err
	}
	return e.follow(tokenIdx, successors)
}

func (e *ActionExecutor) trackRepeated(tokenID int64, perf *actionFrame) {
	index := e.tokenIndex(tokenID)
	if index < 0 {
		return
	}
	token := &e.tokens[index]
	if token.repetition == 0 {
		return
	}
	perf.repetition = token.repetition
	perf.repetitionGroup = token.repetitionGroup
	if state := token.frame.repeats[token.repetitionGroup]; state != nil {
		state.live = append(state.live, perf)
	}
}
