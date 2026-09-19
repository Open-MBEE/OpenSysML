package runtime

import (
	"strings"
	"testing"
)

// tracedBuiltinModel reaches built-ins through invocation expressions, one of
// which fails when its selected branch is evaluated.
const tracedBuiltinModel = `package test {
	private import ScalarValues::*;
	calc def IfTrue { return : Integer = ControlFunctions::'if'(true, 1, 2); }
	calc def IfBoom { return : Integer = ControlFunctions::'if'(true, 1 / 0, 2); }
	calc def Size { return : Integer = (7, 8)->SequenceFunctions::size(); }
}`

// inOrder fails unless every entry appears in got in the order listed.
func inOrder(t *testing.T, got string, entries ...string) {
	t.Helper()
	rest := got
	for _, entry := range entries {
		at := strings.Index(rest, entry)
		if at < 0 {
			t.Fatalf("trace is missing %q in order:\n%s", entry, got)
		}
		rest = rest[at+len(entry):]
	}
}

// balanced fails when the trace leaves a nesting level open: the last entry
// must sit at the top level.
func balanced(t *testing.T, trace *TraceRecorder) {
	t.Helper()
	entries := trace.Entries()
	if last := entries[len(entries)-1]; strings.HasPrefix(last, traceIndent) {
		t.Fatalf("trace ends nested:\n%s", trace.String())
	}
}

// A built-in reached by an invocation expression is traced like any other
// function: entered, its parameters bound (a deferred one as the expression it
// holds), and exited with its result. The branch not selected is never evaluated.
func TestBuiltinInvocationExpressionIsTraced(t *testing.T) {
	ctx, idx := libraryModelContext(t, tracedBuiltinModel)
	ctx.SetCalcCompile(false)
	trace := NewTraceRecorder()
	ctx.SetTrace(trace)

	if _, err := ctx.InvokeCalc(lookupOne(t, idx, "test::IfTrue"), nil, nil); err != nil {
		t.Fatalf("IfTrue: %v", err)
	}
	got := trace.String()
	inOrder(t, got,
		"enter calc test::IfTrue",
		"enter calc ControlFunctions::if",
		"bind test = true [argument]",
		"bind thenValue = expr(literal 1) [argument]",
		"bind elseValue = expr(literal 2) [argument]",
		"exit calc ControlFunctions::if -> 1",
		"exit calc test::IfTrue -> 1",
	)
	if strings.Contains(got, "eval literal 2 ->") {
		t.Errorf("the branch not selected was evaluated:\n%s", got)
	}
	balanced(t, trace)

	trace.Clear()
	if _, err := ctx.InvokeCalc(lookupOne(t, idx, "test::Size"), nil, nil); err != nil {
		t.Fatalf("Size: %v", err)
	}
	inOrder(t, trace.String(),
		"enter calc SequenceFunctions::size",
		"bind seq = (7, 8) [argument]",
		"exit calc SequenceFunctions::size -> 2",
	)
	balanced(t, trace)
}

// A built-in that fails while applied records how far it got before failing.
func TestBuiltinInvocationExpressionTracesFailure(t *testing.T) {
	ctx, idx := libraryModelContext(t, tracedBuiltinModel)
	ctx.SetCalcCompile(false)
	trace := NewTraceRecorder()
	ctx.SetTrace(trace)

	if _, err := ctx.InvokeCalc(lookupOne(t, idx, "test::IfBoom"), nil, nil); err == nil {
		t.Fatal("IfBoom: expected a division error")
	}
	inOrder(t, trace.String(),
		"enter calc ControlFunctions::if",
		"bind test = true [argument]",
		"exit calc ControlFunctions::if -> error: ",
		"exit calc test::IfBoom -> error: ",
	)
	balanced(t, trace)
}

// A direct invocation of a built-in records the same events as the expression
// form, and a binding that fails closes the level the invocation opened.
func TestInvokeCalcBuiltinIsTraced(t *testing.T) {
	ctx, idx := libraryModelContext(t, tracedBuiltinModel)
	trace := NewTraceRecorder()
	ctx.SetTrace(trace)
	cond := lookupOne(t, idx, "ControlFunctions::if")

	if _, err := ctx.InvokeCalc(cond, []Value{constBool(false), constInt(1), constInt(2)}, nil); err != nil {
		t.Fatalf("InvokeCalc('if'): %v", err)
	}
	inOrder(t, trace.String(),
		"enter calc ControlFunctions::if",
		"bind test = false [argument]",
		"bind thenValue = 1 [argument]",
		"bind elseValue = 2 [argument]",
		"exit calc ControlFunctions::if -> 2",
	)
	balanced(t, trace)

	trace.Clear()
	if _, err := ctx.InvokeCalcNamed(cond, map[string]Value{"test": constBool(true), "other": constInt(1)}, nil); err == nil {
		t.Fatal("InvokeCalcNamed('if', other): expected an unknown parameter error")
	}
	got := trace.String()
	inOrder(t, got,
		"enter calc ControlFunctions::if",
		"exit calc ControlFunctions::if -> error: ",
	)
	if strings.Contains(got, "bind ") {
		t.Errorf("a binding that failed recorded a parameter:\n%s", got)
	}
	balanced(t, trace)
}
