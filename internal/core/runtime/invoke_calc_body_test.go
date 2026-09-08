package runtime

import (
	"errors"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// inheritedBodyModel declares the same attribute name in two packages: the
// calculation stating the body reads the one its own package declares, and the
// calculation inheriting the body must read that same one.
const inheritedBodyModel = `
package lib {
	attribute factor = 10;
	calc def Scale {
		in n;
		attribute acc = 0;
		attribute i = 0;
		while i < n {
			assign acc := acc + factor;
			assign i := i + 1;
		}
		return : Integer = acc;
	}
}
package app {
	attribute factor = 100;
	calc def Scale :> lib::Scale {
		in n;
	}
}
`

// calcByName returns the calc named name declared by the package named pkg.
func calcByName(t *testing.T, root *symbols.Scope, pkg, name string) (*symbols.Symbol, *symbols.Scope) {
	t.Helper()
	for _, child := range root.Children() {
		if child.Owner() == nil || child.Owner().Name != pkg {
			continue
		}
		sym, ok := child.LookupLocal(name)
		if !ok || sym == nil {
			t.Fatalf("calc %s::%s not found", pkg, name)
		}
		return sym, child
	}
	t.Fatalf("package %s not found", pkg)
	return nil, nil
}

// TestInheritedCalcBodyRunsInDeclaringScope requires a body inherited through
// specialization to evaluate in the scope of the calculation declaring it, so a
// name the specializing package redeclares does not change the computation.
func TestInheritedCalcBodyRunsInDeclaringScope(t *testing.T) {
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, inheritedBodyModel))
	root := idx.DocumentRoot("<test>")

	stated, statedScope := calcByName(t, root, "lib", "Scale")
	inherited, inheritedScope := calcByName(t, root, "app", "Scale")

	statedValue, err := ctx.InvokeCalc(stated, []Value{constInt(3)}, statedScope)
	if err != nil {
		t.Fatalf("lib::Scale: %v", err)
	}
	if statedValue.Const.Int != 30 {
		t.Fatalf("lib::Scale(3) = %s, want 30", FormatTraceValue(statedValue))
	}

	inheritedValue, err := ctx.InvokeCalc(inherited, []Value{constInt(3)}, inheritedScope)
	if err != nil {
		t.Fatalf("app::Scale: %v", err)
	}
	if inheritedValue.Const.Int != 30 {
		t.Fatalf("app::Scale(3) = %s, want 30 (the factor lib declares)", FormatTraceValue(inheritedValue))
	}
}

// A bodiless calc referencing another (calc two ::> one) inherits the referenced
// body and parameters, as it does through specialization; a diamond over the
// same calc contributes it once.
func TestReferencedCalcBodyIsInherited(t *testing.T) {
	const src = `
package test {
	calc def Plus { in x : Integer; x + 1 }
	calc one : Plus;
	calc two ::> one;
	calc three :> two ::> one;
}
`
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))
	root := idx.DocumentRoot("<test>")
	for _, name := range []string{"one", "two", "three"} {
		sym, scope := calcByName(t, root, "test", name)
		got, err := ctx.InvokeCalc(sym, []Value{constInt(1)}, scope)
		if err != nil {
			t.Fatalf("test::%s: %v", name, err)
		}
		if got.Const.Int != 2 {
			t.Fatalf("test::%s(1) = %s, want 2", name, FormatTraceValue(got))
		}
		if shape, err := ctx.calcShapeOf(sym); err != nil || len(shape.Params) != 1 {
			t.Fatalf("test::%s has parameters %v (%v), want x only", name, shape.ParamNames, err)
		}
	}
}

// statementBodyModel is a calc with a statement body, invoked by every entry
// point the runtime offers.
const statementBodyModel = `
package test {
	calc def factorial {
		in n;
		attribute acc = 1;
		attribute i = 1;
		while i <= n {
			assign acc := acc * i;
			assign i := i + 1;
		}
		return : Integer = acc;
	}
	calc def twice {
		in n;
		factorial(n) + factorial(n)
	}
}
`

// TestCalcStatementBodyFromEveryEntryPoint requires the statement body to run
// the same way whether it is invoked positionally, by name, or from another
// calculation's expression.
func TestCalcStatementBodyFromEveryEntryPoint(t *testing.T) {
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, statementBodyModel))
	root := idx.DocumentRoot("<test>")
	factorial, scope := calcByName(t, root, "test", "factorial")
	twice, _ := calcByName(t, root, "test", "twice")

	positional, err := ctx.InvokeCalc(factorial, []Value{constInt(6)}, scope)
	if err != nil {
		t.Fatalf("InvokeCalc: %v", err)
	}
	if positional.Const.Int != 720 {
		t.Fatalf("InvokeCalc(6) = %s, want 720", FormatTraceValue(positional))
	}

	named, err := ctx.InvokeCalcNamed(factorial, map[string]Value{"n": constInt(6)}, scope)
	if err != nil {
		t.Fatalf("InvokeCalcNamed: %v", err)
	}
	if named.Const.Int != 720 {
		t.Fatalf("InvokeCalcNamed(n = 6) = %s, want 720", FormatTraceValue(named))
	}

	nested, err := ctx.InvokeCalc(twice, []Value{constInt(6)}, scope)
	if err != nil {
		t.Fatalf("nested invocation: %v", err)
	}
	if nested.Const.Int != 1440 {
		t.Fatalf("twice(6) = %s, want 1440", FormatTraceValue(nested))
	}
}

// TestCalcStatementBodySpendsTheStepBudget requires a calc invocation to begin a
// run of its own, so a loop spending steps is bounded by the budget rather than
// by whatever an earlier run left.
func TestCalcStatementBodySpendsTheStepBudget(t *testing.T) {
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, statementBodyModel))
	root := idx.DocumentRoot("<test>")
	factorial, scope := calcByName(t, root, "test", "factorial")

	if _, err := ctx.InvokeCalc(factorial, []Value{constInt(8)}, scope); err != nil {
		t.Fatalf("first run: %v", err)
	}
	first := ctx.steps
	if first == 0 {
		t.Fatalf("a looping calc spent no step of the budget")
	}
	for run := 1; run < 3; run++ {
		if _, err := ctx.InvokeCalc(factorial, []Value{constInt(8)}, scope); err != nil {
			t.Fatalf("run %d: %v", run, err)
		}
		if ctx.steps != first {
			t.Fatalf("run %d spent %d steps, want %d: each run begins its own budget", run, ctx.steps, first)
		}
	}
}

// constSequence returns a sequence value of the given integers, the argument a
// `for` loop over a parameter iterates.
func constSequence(values ...int64) Value {
	sequence := NewSequence()
	for _, v := range values {
		sequence.Append(Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: v}})
	}
	return NewSequenceValue(sequence)
}

// TestCalcForLoopOverParameterSequence requires a `for` loop to iterate the
// sequence a parameter declared to hold one carries, in the order the sequence
// states.
func TestCalcForLoopOverParameterSequence(t *testing.T) {
	const src = `
package test {
	calc def total {
		in xs[*];
		attribute sum = 0;
		for x in xs {
			assign sum := sum + x;
		}
		return : Integer = sum;
	}
}
`
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))
	root := idx.DocumentRoot("<test>")
	total, scope := calcByName(t, root, "test", "total")

	value, err := ctx.InvokeCalc(total, []Value{constSequence(1, 2, 3, 4)}, scope)
	if err != nil {
		t.Fatalf("InvokeCalc: %v", err)
	}
	if value.Const.Int != 10 {
		t.Fatalf("total((1, 2, 3, 4)) = %s, want 10", FormatTraceValue(value))
	}
}

// TestCalcStatementBodyIsLoweredOnce requires the shape cache to hold the
// lowered body, so an invocation does not lower the calculation again.
func TestCalcStatementBodyIsLoweredOnce(t *testing.T) {
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, statementBodyModel))
	root := idx.DocumentRoot("<test>")
	factorial, _ := calcByName(t, root, "test", "factorial")

	first, err := ctx.calcShapeOf(factorial)
	if err != nil {
		t.Fatalf("calcShapeOf: %v", err)
	}
	second, err := ctx.calcShapeOf(factorial)
	if err != nil {
		t.Fatalf("calcShapeOf again: %v", err)
	}
	if first != second {
		t.Fatalf("calcShapeOf returned a new shape, want the cached one")
	}
	if len(first.Body) == 0 {
		t.Fatalf("shape of %s carries no body", first.Name)
	}
	if first.BodyOwner == nil || first.BodyOwner.Decl == nil {
		t.Fatalf("shape of %s names no body owner", first.Name)
	}
	if _, ok := first.BodyOwner.Decl.(*ast.Definition); !ok {
		t.Fatalf("body owner of %s is %T, want the calc definition", first.Name, first.BodyOwner.Decl)
	}
}

// resultPlacementModel declares the result of each calculation before the steps
// computing it, and states one result as a bare expression followed by a note.
const resultPlacementModel = `
package test {
	calc def factorial {
		in n;
		return : Integer = acc;
		attribute acc = 1;
		attribute i = 1;
		while i <= n {
			assign acc := acc * i;
			assign i := i + 1;
		}
	}
	calc def twice {
		in n;
		n * 2
		doc /* the result is the expression above */
	}
}
`

// TestCalcResultIsAnsweredAfterTheSteps requires the result a body declares to
// be evaluated once the steps have run, wherever among the members it is
// written: a result parameter names the answer, it does not stop the body.
func TestCalcResultIsAnsweredAfterTheSteps(t *testing.T) {
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, resultPlacementModel))
	root := idx.DocumentRoot("<test>")

	factorial, scope := calcByName(t, root, "test", "factorial")
	value, err := ctx.InvokeCalc(factorial, []Value{constInt(6)}, scope)
	if err != nil {
		t.Fatalf("InvokeCalc: %v", err)
	}
	if value.Const.Int != 720 {
		t.Fatalf("factorial(6) = %s, want 720: the loop must run before the result is read", FormatTraceValue(value))
	}

	twice, twiceScope := calcByName(t, root, "test", "twice")
	doubled, err := ctx.InvokeCalc(twice, []Value{constInt(4)}, twiceScope)
	if err != nil {
		t.Fatalf("InvokeCalc: %v", err)
	}
	if doubled.Const.Int != 8 {
		t.Fatalf("twice(4) = %s, want 8: an expression result is not dropped by what follows it", FormatTraceValue(doubled))
	}
}

// successionModel writes a succession among the members of a calculation body,
// the form `succession first a then b;` a member-attached `then` is written back as.
const successionModel = `
package test {
	calc def sequenced {
		in n;
		action first;
		action second;
		succession first first then second;
		return : Integer = n + 1;
	}
}
`

// TestCalcBodySuccessionIsNotAStep requires a succession among a calculation's
// members to leave the computation alone: the body states its order already.
func TestCalcBodySuccessionIsNotAStep(t *testing.T) {
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, successionModel))
	root := idx.DocumentRoot("<test>")

	sequenced, scope := calcByName(t, root, "test", "sequenced")
	value, err := ctx.InvokeCalc(sequenced, []Value{constInt(3)}, scope)
	if err != nil {
		t.Fatalf("InvokeCalc: %v", err)
	}
	if value.Const.Int != 4 {
		t.Fatalf("sequenced(3) = %s, want 4", FormatTraceValue(value))
	}
}

// unevaluableResultModel answers with an expression the evaluator has no value
// kind for, so the body states a result that cannot be computed.
const unevaluableResultModel = `
package test {
	item def Foo { attribute v; }
	calc def complement {
		in n;
		~n
	}
	calc def cast {
		in n;
		n as Foo
	}
}
`

const unboundResultModel = `
package test {
	private import ScalarValues::*;

	calc def Named {
		in a : Real;
		attribute h : Real = a * 2.0;
		return h;
	}

	calc def UntypedSibling {
		in a : Real;
		attribute h = a * 2.0;
		return h;
	}

	calc def Anonymous {
		in a : Real;
		return : Real;
	}

	calc def Fresh {
		in a : Real;
		return selected :> a;
	}

	calc def Plain {
		in a : Real;
	}

	attribute def 'Wide Real' :> Real;

	calc def Unrestricted {
		in a : Real;
		attribute 'my result' : 'Wide Real' = a;
		return 'my result';
	}

	calc def Keyword {
		in a : Real;
		attribute 'action' = a;
		return 'action' : $::ScalarValues::'Real';
	}

	calc def Uninitialized {
		in a : Real;
		attribute h : Real;
		return h;
	}
}
`

// TestUnboundResultParameterHint requires a body whose 'return' declares a
// result parameter without binding it to say so, with the two forms that state
// a computed result, and a body with no result parameter at all to say nothing more.
func TestUnboundResultParameterHint(t *testing.T) {
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, unboundResultModel))
	root := idx.DocumentRoot("<test>")

	cases := []struct{ calc, want string }{
		{"Named", "no result expression: calc test::Named has no return expression: result parameter h binds no value; " +
			"write the result as the trailing expression `h`, or bind it with `return : Real = h;`"},
		{"UntypedSibling", "no result expression: calc test::UntypedSibling has no return expression: result parameter h binds no value; " +
			"write the result as the trailing expression `h`, or bind it with `return : <type> = h;`"},
		{"Anonymous", "no result expression: calc test::Anonymous has no return expression: the result parameter binds no value; " +
			"write the result as the trailing expression of the body, or bind it with `return : Real = <expr>;`"},
		{"Fresh", "no result expression: calc test::Fresh has no return expression: result parameter selected binds no value; " +
			"write the result as the trailing expression of the body, or bind it with `return : <type> = <expr>;`"},
		{"Plain", "no result expression: calc test::Plain has no return expression"},
		{"Unrestricted", "no result expression: calc test::Unrestricted has no return expression: result parameter 'my result' binds no value; " +
			"write the result as the trailing expression `'my result'`, or bind it with `return : 'Wide Real' = 'my result';`"},
		{"Keyword", "no result expression: calc test::Keyword has no return expression: result parameter 'action' binds no value; " +
			"write the result as the trailing expression `'action'`, or bind it with `return : $::ScalarValues::Real = 'action';`"},
		{"Uninitialized", "no result expression: calc test::Uninitialized has no return expression: result parameter h binds no value; " +
			"write the result as the trailing expression of the body, or bind it with `return : <type> = <expr>;`"},
	}
	for _, tc := range cases {
		calc, scope := calcByName(t, root, "test", tc.calc)
		_, err := ctx.InvokeCalc(calc, []Value{constReal(1)}, scope)
		if err == nil {
			t.Fatalf("InvokeCalc(%s) succeeded, want %q", tc.calc, tc.want)
		}
		if !errors.Is(err, ErrNoResultExpression) {
			t.Errorf("InvokeCalc(%s) = %v, want ErrNoResultExpression", tc.calc, err)
		}
		if err.Error() != tc.want {
			t.Errorf("InvokeCalc(%s) = %q, want %q", tc.calc, err, tc.want)
		}
		for _, form := range suggestedForms(err.Error()) {
			if diags := parseDiagnostics("calc def C { in a : Real; " + form + " }"); len(diags) > 0 {
				t.Errorf("InvokeCalc(%s) suggests %q, which does not parse: %v", tc.calc, form, diags)
			}
		}
	}
}

// suggestedForms returns the notation a diagnostic proposes in backticks,
// skipping a form that leaves a `<placeholder>` to fill.
func suggestedForms(msg string) []string {
	parts := strings.Split(msg, "`")
	var forms []string
	for i := 1; i < len(parts); i += 2 {
		if !strings.Contains(parts[i], "<") {
			forms = append(forms, parts[i])
		}
	}
	return forms
}

// parseDiagnostics parses src and returns the parser's error diagnostics.
func parseDiagnostics(src string) []parser.Diagnostic {
	p := parser.New(source.New("<suggested>", []byte(src)))
	p.ParseFile()
	return p.Diagnostics
}

// TestUnevaluableResultIsNotReportedAsMissing requires a body stating a result
// the evaluator cannot compute to report that, not that there is no result.
func TestUnevaluableResultIsNotReportedAsMissing(t *testing.T) {
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, unevaluableResultModel))
	root := idx.DocumentRoot("<test>")

	for _, name := range []string{"complement", "cast"} {
		calc, scope := calcByName(t, root, "test", name)
		_, err := ctx.InvokeCalc(calc, []Value{constInt(1)}, scope)
		if err == nil {
			t.Fatalf("InvokeCalc(%s) succeeded, want the evaluator to report the expression", name)
		}
		if errors.Is(err, ErrNoResultExpression) {
			t.Fatalf("InvokeCalc(%s) = %v, want the unevaluable expression reported, not a missing result", name, err)
		}
	}
}
