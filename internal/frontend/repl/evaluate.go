package repl

import (
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/exec/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// evaluated is what an expression evaluated to: the value, the context it is
// read through, how the prompt spells it, and the lines %eval prints.
type evaluated struct {
	ctx   *runtime.Context
	val   runtime.Value
	text  string
	lines []string
	// unset marks a feature holding no value, which val then does not describe.
	unset bool
}

// answered is the evaluation of label to val, spelled text, as %eval reports it.
func answered(label string, ctx *runtime.Context, val runtime.Value, text string) evaluated {
	return evaluated{
		ctx:   ctx,
		val:   val,
		text:  text,
		lines: []string{fmt.Sprintf("✓ %s", label), fmt.Sprintf("  = %s", text)},
	}
}

// EvalResult is what an expression evaluated to, for a caller reporting the
// value as data beside the lines `%eval` prints: the value, the context it is
// read through, and the index naming the declarations it refers to.
type EvalResult struct {
	Expression string
	// Text is the value as the prompt prints it.
	Text string
	// Unset is a feature holding no value, printed `<unset>`; Value is then invalid.
	Unset   bool
	Value   runtime.Value
	Context *runtime.Context
	Index   *symbols.Index
	Lines   []string
}

// Evaluate evaluates an expression as EvalExpr does and returns the value with
// the lines, with an error for one that could not be evaluated.
func (s *Session) Evaluate(expr string) (EvalResult, error) {
	defer s.enter()()
	ev, err := s.evalValue(expr)
	if err != nil {
		return EvalResult{}, err
	}
	return EvalResult{
		Expression: expr,
		Text:       ev.text,
		Unset:      ev.unset,
		Value:      ev.val,
		Context:    ev.ctx,
		Index:      s.symbolIndex(),
		Lines:      append(s.drainTrace(), ev.lines...),
	}, nil
}
