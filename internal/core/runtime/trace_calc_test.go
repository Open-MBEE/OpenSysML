package runtime

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// TestCalcTraceIsStableAcrossRuns: the recorded trace is a contract, so
// evaluating the same model twice in independent contexts must record the same
// entries in the same order.
func TestCalcTraceIsStableAcrossRuns(t *testing.T) {
	src := `
		package test {
			calc combine {
				in a : Integer;
				in b : Integer = 4;
				double(a) + double(b)
			}

			calc double {
				in x : Integer;
				x * 2
			}
		}
	`

	traceOnce := func() string {
		idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))
		trace := NewTraceRecorder()
		ctx.SetTrace(trace)

		rootScope := idx.DocumentRoot("<test>")
		sym := findSymbolByName(rootScope, "combine", ast.DefCalc)
		if sym == nil {
			t.Fatal("calc combine not found")
		}
		arg := Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: 3}}
		if _, err := ctx.InvokeCalc(sym, []Value{arg}, rootScope); err != nil {
			t.Fatalf("InvokeCalc: %v", err)
		}
		return trace.String()
	}

	first := traceOnce()
	for i := 0; i < 10; i++ {
		if got := traceOnce(); got != first {
			t.Fatalf("trace differs between runs\n=== FIRST ===\n%s\n=== RUN %d ===\n%s", first, i, got)
		}
	}

	// Binding precedes evaluation, and the nested invocations are recorded in
	// argument order.
	want := []string{
		"enter calc test::combine",
		"bind a = 3 [argument]",
		"bind b = 4 [default]",
		"enter calc test::double",
		"exit calc test::double -> 6",
		"enter calc test::double",
		"exit calc test::double -> 8",
		"exit calc test::combine -> 14",
	}
	rest := first
	for _, entry := range want {
		at := strings.Index(rest, entry)
		if at < 0 {
			t.Fatalf("trace is missing %q in order:\n%s", entry, first)
		}
		rest = rest[at+len(entry):]
	}
}

// TestFormatTraceValueCanonicalizesSets: a set has no order of its own and its
// backing map does not iterate in a stable one, so its rendering is sorted.
func TestFormatTraceValueCanonicalizesSets(t *testing.T) {
	set := NewSet()
	for _, n := range []int64{3, 1, 2} {
		set.Add(Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: n}})
	}

	value := NewSetValue(set)
	const want = "{1, 2, 3}"
	for i := 0; i < 50; i++ {
		if got := FormatTraceValue(value); got != want {
			t.Fatalf("set rendering = %q, want %q", got, want)
		}
	}
}

// TestFormatTraceValueDistinguishesKinds: the trace must not print an integer
// and a whole real the same way, or a golden could not tell them apart.
func TestFormatTraceValueDistinguishesKinds(t *testing.T) {
	sequence := NewSequence()
	sequence.Append(Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: 1}})
	sequence.Append(Value{Kind: ValNull})

	cases := []struct {
		name  string
		value Value
		want  string
	}{
		{"integer", Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: 1}}, "1"},
		{"whole real", Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValReal, Real: 1}}, "1.0"},
		{"fractional real", Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValReal, Real: 0.25}}, "0.25"},
		{"boolean", Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValBool, Bool: true}}, "true"},
		{"infinity", Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInfinity}}, "*"},
		{"string", NewStringValue("on"), `"on"`},
		{"null", Value{Kind: ValNull}, "null"},
		{"instance", Value{Kind: ValInstance, Instance: 7}, "instance#7"},
		{"sequence", NewSequenceValue(sequence), "(1, null)"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := FormatTraceValue(tc.value); got != tc.want {
				t.Errorf("rendering = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestTraceLabelsExpressionKinds: labels identify a sub-expression by kind and
// the token that names it, independent of source formatting.
func TestTraceLabelsExpressionKinds(t *testing.T) {
	cases := []struct {
		node ast.Node
		want string
	}{
		{&ast.LiteralInteger{Value: "42"}, "literal 42"},
		{&ast.LiteralBool{Value: true}, "literal true"},
		{&ast.NullExpr{}, "null"},
		{&ast.OperatorExpr{Operator: ast.OpAdd}, "operator +"},
		{&ast.SequenceExpr{Elements: []ast.Node{&ast.NullExpr{}}}, "sequence of 1"},
		{&ast.CollectExpr{}, "collect"},
		{&ast.SelectExpr{}, "select"},
		{nil, "nil"},
	}

	for _, tc := range cases {
		if got := TraceLabel(tc.node); got != tc.want {
			t.Errorf("TraceLabel(%T) = %q, want %q", tc.node, got, tc.want)
		}
	}
}

// TestCalcTraceRecordsFailure: a trace of a failed calc shows how far binding
// got before the error, so an ordering regression is still diagnosable.
func TestCalcTraceRecordsFailure(t *testing.T) {
	src := `
		package test {
			calc add {
				in x : Integer;
				in y : Integer;
				x + y
			}
		}
	`
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))
	trace := NewTraceRecorder()
	ctx.SetTrace(trace)

	rootScope := idx.DocumentRoot("<test>")
	sym := findSymbolByName(rootScope, "add", ast.DefCalc)
	if sym == nil {
		t.Fatal("calc add not found")
	}
	arg := Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: 3}}
	if _, err := ctx.InvokeCalc(sym, []Value{arg}, rootScope); err == nil {
		t.Fatal("expected an unbound parameter error")
	}

	got := trace.String()
	if !strings.Contains(got, "bind x = 3 [argument]") {
		t.Errorf("trace is missing the successful binding:\n%s", got)
	}
	if !strings.Contains(got, "exit calc test::add -> error: unbound parameter") {
		t.Errorf("trace is missing the failure entry:\n%s", got)
	}
}

// TestReturnedBodyIsTracedAsApplied: a body a calc returns closes over that
// calc's bindings, not over its tracing state. Made before tracing starts, its
// application under a trace is recorded; made under a trace, its application
// after tracing stops is not.
func TestReturnedBodyIsTracedAsApplied(t *testing.T) {
	src := `
		package test {
			private import ScalarValues::*;
			private import ControlFunctions::*;
			calc def AboveThreshold { in threshold : Integer; return : expr = { in x; x > threshold }; }
			calc def Keep { in xs : Integer[*]; in expr pred; return : Integer[*] = xs->select pred; }
		}
	`
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
	ctx.SetCalcCompile(false)
	rootScope := idx.DocumentRoot("<test>")
	above := findSymbolByName(rootScope, "AboveThreshold", ast.DefCalc)
	keep := findSymbolByName(rootScope, "Keep", ast.DefCalc)
	if above == nil || keep == nil {
		t.Fatal("calcs not found")
	}
	intValue := func(n int64) Value {
		return Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: n}}
	}
	makeBody := func(threshold int64) Value {
		t.Helper()
		body, err := ctx.InvokeCalc(above, []Value{intValue(threshold)}, rootScope)
		if err != nil {
			t.Fatalf("AboveThreshold(%d): %v", threshold, err)
		}
		if body.Kind != ValExpr {
			t.Fatalf("AboveThreshold(%d) = %s, want a body", threshold, FormatTraceValue(body))
		}
		return body
	}
	applyBody := func(body Value, want string) {
		t.Helper()
		xs := NewSequence()
		for _, n := range []int64{1, 2, 3, 4} {
			xs.Append(intValue(n))
		}
		kept, err := ctx.InvokeCalc(keep, []Value{NewSequenceValue(xs), body}, rootScope)
		if err != nil {
			t.Fatalf("Keep: %v", err)
		}
		if got := FormatTraceValue(kept); got != want {
			t.Fatalf("Keep = %s, want %s", got, want)
		}
	}
	const applied = "eval operator > ->"

	// Made untraced, applied traced: the body's evaluation is recorded.
	body := makeBody(2)
	trace := NewTraceRecorder()
	ctx.SetTrace(trace)
	applyBody(body, "(3, 4)")
	if got := trace.String(); !strings.Contains(got, applied) {
		t.Errorf("trace does not record the body applied after tracing started:\n%s", got)
	}

	// Made traced, applied untraced: nothing more is recorded.
	body = makeBody(3)
	ctx.SetTrace(nil)
	before := len(trace.Entries())
	applyBody(body, "(4)")
	if got := trace.Entries()[before:]; len(got) != 0 {
		t.Errorf("trace records %d entries after tracing stopped:\n%s", len(got), strings.Join(got, "\n"))
	}

	// A body made under one recorder and applied under another is recorded by
	// the latter alone.
	ctx.SetTrace(trace)
	body = makeBody(0)
	later := NewTraceRecorder()
	ctx.SetTrace(later)
	before = len(trace.Entries())
	applyBody(body, "(1, 2, 3, 4)")
	if got := trace.Entries()[before:]; len(got) != 0 {
		t.Errorf("the recorder the body was made under records %d entries of its application:\n%s", len(got), strings.Join(got, "\n"))
	}
	if got := later.String(); !strings.Contains(got, applied) {
		t.Errorf("the applying recorder does not record the body:\n%s", got)
	}
}
