package runtime

import (
	"errors"
	"fmt"
	"slices"
)

const maxStatementOrderEvaluations = 1024

var ErrStatementOrderSweepLimit = errors.New("statement-order evaluation limit exceeded")

type statementOrderSweep struct {
	prefix       []int
	alternatives []int
	taken        []int
	choices      []ChoiceTaken
}

func (s *statementOrderSweep) choose(choice *ChoicePoint) int {
	position := len(s.alternatives)
	s.alternatives = append(s.alternatives, len(choice.Alternatives))
	taken := 0
	if position < len(s.prefix) {
		taken = s.prefix[position]
	}
	if taken < 0 || taken >= len(choice.Alternatives) {
		taken = 0
	}
	s.taken = append(s.taken, taken)
	choice.Taken = taken
	s.choices = append(s.choices, choice.Choice())
	return taken
}

func (ctx *Context) sweepStatementOrders(eval func() error) error {
	return ctx.sweepStatementOrderVariants(func(*statementOrderSweep) error { return eval() })
}

func (ctx *Context) sweepStatementOrderVariants(eval func(*statementOrderSweep) error) error {
	if ctx.statementOrderSweep != nil {
		return eval(ctx.statementOrderSweep)
	}
	return ctx.sweepStatementOrderVariantsNested(eval)
}

func (ctx *Context) sweepStatementOrderVariantsNested(eval func(*statementOrderSweep) error) error {
	previous := ctx.statementOrderSweep
	pending := [][]int{{}}
	seen := map[string]bool{"[]": true}
	evaluations := 0
	for len(pending) > 0 {
		if evaluations == maxStatementOrderEvaluations {
			return fmt.Errorf("%w: at most %d statement orders", ErrStatementOrderSweepLimit, maxStatementOrderEvaluations)
		}
		last := len(pending) - 1
		prefix := pending[last]
		pending = pending[:last]
		sweep := &statementOrderSweep{prefix: prefix}
		ctx.statementOrderSweep = sweep
		restore := ctx.beginProbe()
		err := eval(sweep)
		restore()
		ctx.statementOrderSweep = previous
		evaluations++
		if err != nil {
			return err
		}
		for position := len(sweep.alternatives) - 1; position >= 0; position-- {
			for alternative := sweep.alternatives[position] - 1; alternative > 0; alternative-- {
				next := append(slices.Clone(sweep.taken[:position]), alternative)
				key := fmt.Sprint(next)
				if !seen[key] {
					seen[key] = true
					pending = append(pending, next)
				}
			}
		}
	}
	return nil
}

func (ctx *Context) sweepStatementOrdersAt(choices []ChoiceTaken, eval func() error) error {
	if ctx.statementOrderSweep != nil {
		return eval()
	}
	prefix := make([]int, len(choices))
	for i, choice := range choices {
		prefix[i] = choice.Taken
	}
	sweep := &statementOrderSweep{prefix: prefix}
	previous := ctx.statementOrderSweep
	ctx.statementOrderSweep = sweep
	restore := ctx.beginProbe()
	err := eval()
	restore()
	ctx.statementOrderSweep = previous
	if err != nil {
		return err
	}
	if len(sweep.choices) != len(choices) {
		return fmt.Errorf("the final statement orders do not match the replay witness")
	}
	for i, choice := range choices {
		got := sweep.choices[i]
		if got.Kind != choice.Kind || got.Step != choice.Step ||
			got.Where != choice.Where || got.Alternatives != choice.Alternatives ||
			got.Took != choice.Took || !slices.Equal(got.Among, choice.Among) {
			return fmt.Errorf("final statement order %d does not match the replay witness", i+1)
		}
	}
	return nil
}

func (ctx *Context) everyStatementOrder(eval func() (bool, error)) (bool, error) {
	if ctx.statementOrderSweep != nil {
		return eval()
	}
	holds := true
	err := ctx.sweepStatementOrders(func() error {
		result, err := eval()
		if err != nil {
			holds = false
			return err
		}
		if !result {
			holds = false
		}
		return nil
	})
	return holds, err
}
