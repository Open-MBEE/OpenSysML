package runtime

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

type invocationOrderKey struct {
	shape     *calcShape
	self      int64
	arguments string
}

type invocationOrderResult struct {
	value    Value
	err      error
	spelling string
	key      string
	prefix   []int
	first    bool
}

type invocationOrderMemo struct {
	results   map[invocationOrderKey][]invocationOrderResult
	resolving map[invocationOrderKey]bool
}

const resultOrderWherePrefix = "result of "

func (ctx *Context) invokeWithStatementOrderResults(
	shape *calcShape,
	args calcArgs,
	self *Instance,
	memoize bool,
	invoke func() (Value, error),
) (Value, error) {
	outermost := ctx.invocationOrderMemo == nil
	if outermost {
		ctx.invocationOrderMemo = &invocationOrderMemo{
			results:   make(map[invocationOrderKey][]invocationOrderResult),
			resolving: make(map[invocationOrderKey]bool),
		}
	}
	if outermost {
		defer func() { ctx.invocationOrderMemo = nil }()
	}

	key := invocationOrderKey{shape: shape, arguments: canonicalInvocationArguments(shape, args)}
	if self != nil {
		key.self = self.ID
	}
	memo := ctx.invocationOrderMemo
	if memoize && memo.resolving[key] {
		if ctx.statementOrderSweep == nil {
			return invoke()
		}
		previous := ctx.statementOrderSweep
		ctx.statementOrderSweep = &statementOrderSweep{}
		defer func() { ctx.statementOrderSweep = previous }()
		return invoke()
	}
	results, ok := []invocationOrderResult(nil), false
	if memoize {
		results, ok = memo.results[key]
	}
	if !ok {
		if memoize {
			memo.resolving[key] = true
		}
		err := ctx.sweepStatementOrderVariantsNested(func(sweep *statementOrderSweep) error {
			value, invokeErr := invoke()
			if errors.Is(invokeErr, ErrStatementOrderSweepLimit) {
				return invokeErr
			}
			result := invocationOrderResult{
				value:  value,
				err:    invokeErr,
				key:    invocationOrderResultKey(value, invokeErr),
				prefix: append([]int(nil), sweep.taken...),
			}
			if invokeErr != nil {
				result.spelling = "error: " + invokeErr.Error()
			} else {
				result.spelling = FormatValue(value)
			}
			if _, exists := memoizedInvocationResult(results, result.key); !exists {
				result.first = len(results) == 0
				results = append(results, result)
			}
			return nil
		})
		if memoize {
			delete(memo.resolving, key)
		}
		if err != nil {
			return Value{}, err
		}
		results = orderedInvocationResults(results)
		if memoize {
			memo.results[key] = results
		}
	}
	if len(results) == 0 {
		return Value{}, fmt.Errorf("%w: result of %s under its statement orders was not evaluated", ErrOrderDependentPreview, shape.Label)
	}
	if len(results) > 1 && ctx.probes > 0 && ctx.statementOrderSweep == nil {
		return Value{}, fmt.Errorf("%w: result of %s depends on the statement orders of its body", ErrOrderDependentPreview, shape.Label)
	}
	selected := 0
	if len(results) > 1 {
		alternatives := make([]string, len(results))
		for i := range results {
			alternatives[i] = results[i].spelling
		}
		choice := ChoicePoint{
			Kind:         ChoiceStatementOrder,
			Step:         ctx.enclosingExecutorStep(),
			Where:        "result of " + shape.Label + " under the statement orders of its body",
			Alternatives: alternatives,
		}
		if ctx.statementOrderSweep != nil {
			selected = ctx.statementOrderSweep.choose(&choice)
		} else {
			choice.Taken = ctx.scheduling().choose(choice, nil)
			if err := ctx.scheduling().refusal(); err != nil {
				return Value{}, err
			}
			ctx.noteChoice(choice)
			selected = choice.Taken
		}
	}
	result := results[selected]
	previous := ctx.statementOrderSweep
	ctx.statementOrderSweep = &statementOrderSweep{prefix: append([]int(nil), result.prefix...)}
	if memoize {
		memo.resolving[key] = true
	}
	var value Value
	var err error
	func() {
		defer func() {
			if memoize {
				delete(memo.resolving, key)
			}
		}()
		defer func() { ctx.statementOrderSweep = previous }()
		value, err = invoke()
	}()
	if invocationOrderResultKey(value, err) != result.key {
		return Value{}, fmt.Errorf("%w: result of %s did not follow its statement-order witness", ErrOrderDependentPreview, shape.Label)
	}
	return value, err
}

func canonicalInvocationArguments(shape *calcShape, args calcArgs) string {
	var b strings.Builder
	for i, name := range shape.ParamNames {
		var value Value
		var supplied bool
		switch {
		case i < len(args.positional):
			value, supplied = args.positional[i], true
		case args.named != nil:
			value, supplied = args.named[name]
		}
		fmt.Fprintf(&b, "%s=", strconv.Quote(name))
		if !supplied {
			b.WriteString("<default>")
		} else {
			fmt.Fprintf(&b, "%d:%s", value.Kind, strconv.Quote(FormatValue(value)))
		}
		b.WriteByte(';')
	}
	if len(args.positional) > len(shape.ParamNames) {
		b.WriteString("extra-pos:")
		for _, value := range args.positional[len(shape.ParamNames):] {
			fmt.Fprintf(&b, "%d:%s;", value.Kind, strconv.Quote(FormatValue(value)))
		}
	}
	names := make([]string, 0, len(args.named))
	for name := range args.named {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if !shape.hasParameter(name) {
			value := args.named[name]
			fmt.Fprintf(&b, "unknown:%s=%d:%s;", strconv.Quote(name), value.Kind, strconv.Quote(FormatValue(value)))
			continue
		}
		for i, param := range shape.ParamNames {
			if param == name && i < len(args.positional) {
				value := args.named[name]
				fmt.Fprintf(&b, "duplicate:%s=%d:%s;", strconv.Quote(name), value.Kind, strconv.Quote(FormatValue(value)))
				break
			}
		}
	}
	return b.String()
}

func invocationOrderResultKey(value Value, err error) string {
	if err != nil {
		return "error:" + err.Error()
	}
	return "value:" + FormatValue(value)
}

func memoizedInvocationResult(results []invocationOrderResult, key string) (invocationOrderResult, bool) {
	for _, result := range results {
		if result.key == key {
			return result, true
		}
	}
	return invocationOrderResult{}, false
}

func orderedInvocationResults(results []invocationOrderResult) []invocationOrderResult {
	ordered := append([]invocationOrderResult(nil), results...)
	sort.SliceStable(ordered, func(i, j int) bool {
		a, b := ordered[i], ordered[j]
		if (a.err != nil) != (b.err != nil) {
			return a.err == nil
		}
		if a.err == nil && a.first != b.first {
			return a.first
		}
		if a.err != nil {
			return a.err.Error() < b.err.Error()
		}
		return a.spelling < b.spelling
	})
	return ordered
}

func (ctx *Context) invokeOrderedPredicate(
	shape *calcShape,
	args calcArgs,
	self *Instance,
	invoke func() (Value, error),
) (Value, error) {
	return ctx.invokeWithStatementOrderResults(shape, args, self, true, invoke)
}

func predicateOrderAware(ctx *Context, sym *symbols.Symbol) bool {
	return ctx.scheduling().ordersStatements() &&
		ctx.reordersTransitively(sym) && ctx.pureTransitively(sym)
}
