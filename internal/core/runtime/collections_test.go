package runtime

import (
	"errors"
	"testing"
	"time"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
)

// evalCollectionExpr evaluates expr as the value of an attribute of a model
// declaring `xs`, `ys`, `factor` and `flags`, so a case can be written as the
// expression it is about.
func evalCollectionExpr(t *testing.T, expr string) (Value, error) {
	t.Helper()
	return evalCollectionExprBounded(t, expr, 10000)
}

// evalCollectionExprBounded evaluates expr under a step budget, on its own
// goroutine, so an evaluation that neither answers nor fails within the budget
// fails the test rather than hanging it.
func evalCollectionExprBounded(t *testing.T, expr string, maxSteps int64) (Value, error) {
	t.Helper()
	ec, value := collectionExprContext(t, expr, maxSteps)

	type outcome struct {
		value Value
		err   error
	}
	done := make(chan outcome, 1)
	go func() {
		got, err := ec.Eval(value)
		done <- outcome{got, err}
	}()
	select {
	case got := <-done:
		return got.value, got.err
	case <-time.After(10 * time.Second):
		t.Fatalf("%s did not terminate", expr)
		return Value{}, nil
	}
}

// collectionExprContext builds the model of expr and answers the context and
// the expression node to evaluate in it.
func collectionExprContext(t *testing.T, expr string, maxSteps int64) (*EvalContext, ast.Node) {
	t.Helper()
	src := `
package test {
	private import SequenceFunctions::*;
	private import CollectionFunctions::*;
	private import ControlFunctions::*;
	private import NumericalFunctions::*;
	attribute xs = (1, 2, 3);
	attribute ys = (2, 4);
	attribute factor = 10;
	attribute flags = (true, false);
	attribute result = ` + expr + `;
}`
	model, resolver, root := parseAndBuildLibraryModel(t, src)
	pkg, ok := root.LookupLocal("test")
	if !ok || pkg == nil {
		t.Fatal("package test not found")
	}
	scope := pkg.Scope
	if scope == nil {
		t.Fatal("package test has no scope")
	}
	sym, ok := scope.LookupLocal("result")
	if !ok || sym == nil {
		t.Fatal("attribute result not found")
	}
	decl, ok := sym.Decl.(*ast.Usage)
	if !ok {
		t.Fatalf("result declares %T, want a usage", sym.Decl)
	}
	ctx := NewContext(model, resolver, maxSteps)
	return NewEvalContext(ctx, scope), decl.Value
}

// intsOf renders a value's elements as integers, failing the test where any
// element is not one.
func intsOf(t *testing.T, val Value) []int64 {
	t.Helper()
	elements := elementsOf(val)
	out := make([]int64, 0, len(elements))
	for _, elem := range elements {
		if elem.Kind != ValConst || elem.Const.Kind != semantics.ValInt {
			t.Fatalf("element %v is not an Integer", elem)
		}
		out = append(out, elem.Const.Int)
	}
	return out
}

func equalInts(a, b []int64) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// TestSequenceIndexing pins the sequence index `seq#(i)`: 1-based per
// SequenceFunctions::'#' (`in index: Positive[1]`), reporting an index that
// names no position rather than answering with nothing.
func TestSequenceIndexing(t *testing.T) {
	tests := []struct {
		expr string
		want int64
	}{
		{"xs#(1)", 1},
		{"xs#(2)", 2},
		{"xs#(3)", 3},
		{"xs#(SequenceFunctions::size(xs))", 3},
		{"(10, 20, 30)#(2)", 20},
		{"xs#(1 + 1)", 2},
		{"factor#(1)", 10},
		{"SequenceFunctions::'#'(xs, 2)", 2},
		{"xs->SequenceFunctions::'#'(3)", 3},
	}
	for _, tt := range tests {
		got, err := evalCollectionExpr(t, tt.expr)
		if err != nil {
			t.Errorf("%s: %v", tt.expr, err)
			continue
		}
		if got.Kind != ValConst || got.Const.Kind != semantics.ValInt || got.Const.Int != tt.want {
			t.Errorf("%s = %v, want the Integer %d", tt.expr, got, tt.want)
		}
	}
}

// TestSequenceIndexingErrors pins the diagnostics of an index that names no
// position: an index outside the sequence, one counting from 0, and one that is
// not a whole number are each reported rather than answered.
func TestSequenceIndexingErrors(t *testing.T) {
	tests := []struct {
		expr string
		want error
	}{
		{"xs#(0)", ErrIndexOutOfRange},
		{"xs#(4)", ErrIndexOutOfRange},
		{"xs#(0 - 1)", ErrIndexOutOfRange},
		{"()#(1)", ErrIndexOutOfRange},
		{"factor#(2)", ErrIndexOutOfRange},
		{"xs#(1.5)", ErrTypeMismatch},
		{"xs#(true)", ErrTypeMismatch},
		{`xs#("2")`, ErrTypeMismatch},
		{"xs#(ys)", ErrTypeMismatch},
	}
	for _, tt := range tests {
		got, err := evalCollectionExpr(t, tt.expr)
		if !errors.Is(err, tt.want) {
			t.Errorf("%s = (%v, %v), want %v", tt.expr, got, err, tt.want)
		}
	}
}

// TestCollectionResults pins the results of the collection operations, in each
// of the three ways a model can write them: the notation (`xs.{in x; …}`), the
// qualified call, and the receiver form.
func TestCollectionResults(t *testing.T) {
	tests := []struct {
		expr string
		want []int64
	}{
		{"xs.{in x; x * 2}", []int64{2, 4, 6}},
		{"ControlFunctions::collect(xs, {in x; x * 2})", []int64{2, 4, 6}},
		{"xs->collect {in x; x * 2}", []int64{2, 4, 6}},
		{"xs->ControlFunctions::collect {in x; x * 2}", []int64{2, 4, 6}},
		{"xs.?{in x; x > 1}", []int64{2, 3}},
		{"ControlFunctions::select(xs, {in x; x > 1})", []int64{2, 3}},
		{"xs->select {in x; x > 1}", []int64{2, 3}},
		{"xs->reject {in x; x > 1}", []int64{1}},
		// A body names the scope it was written in as well as its parameter.
		{"xs.{in x; x * factor}", []int64{10, 20, 30}},
		// A parameter is named by the body that declares it, whatever it is
		// called: nothing is bound to a fixed name.
		{"xs.{in element; element + 1}", []int64{2, 3, 4}},
		// An empty collection maps and filters to nothing.
		{"().{in x; x * 2}", nil},
		{"().?{in x; x > 0}", nil},
		// A collection operation is an expression, so it nests.
		{"xs.?{in x; x > 1}.{in x; x * 2}", []int64{4, 6}},
		{"xs.{in x; ys.{in y; x * y}#(1)}", []int64{2, 4, 6}},
		{"xs.?{in x; ys->includes(x)}", []int64{2}},
		// A nested body's parameter shadows the enclosing one for its own
		// result, and the enclosing one is visible again after it.
		{"xs.{in x; (ys.{in x; x * 100}#(1)) + x}", []int64{201, 202, 203}},
		{"SequenceFunctions::union(xs, ys)", []int64{1, 2, 3, 2, 4}},
		{"xs->union(ys)", []int64{1, 2, 3, 2, 4}},
		{"SequenceFunctions::intersection(xs, ys)", []int64{2}},
		{"xs->including(4)", []int64{1, 2, 3, 4}},
		{"xs->excluding(2)", []int64{1, 3}},
		{"xs->tail()", []int64{2, 3}},
		{"()->tail()", nil},
		{"xs->subsequence(2)", []int64{2, 3}},
		{"xs->subsequence(1, 2)", []int64{1, 2}},
		{"xs->excludingAt(2)", []int64{1, 3}},
		{"xs->excludingAt(1, 2)", []int64{3}},
		{"xs->excludingAt(1, 3)", nil},
		// includingAt inserts before the given position, shifting the tail right,
		// so the result is longer by the values inserted and size+1 appends.
		{"xs->includingAt(9, 1)", []int64{9, 1, 2, 3}},
		{"xs->includingAt(9, 2)", []int64{1, 9, 2, 3}},
		{"xs->includingAt(9, 4)", []int64{1, 2, 3, 9}},
		{"SequenceFunctions::includingAt(xs, 9, 3)", []int64{1, 2, 9, 3}},
		{"xs->includingAt(ys, 2)", []int64{1, 2, 4, 2, 3}},
		{"xs->includingAt((), 2)", []int64{1, 2, 3}},
		{"()->includingAt(9, 1)", []int64{9}},
	}
	for _, tt := range tests {
		got, err := evalCollectionExpr(t, tt.expr)
		if err != nil {
			t.Errorf("%s: %v", tt.expr, err)
			continue
		}
		if ints := intsOf(t, got); !equalInts(ints, tt.want) {
			t.Errorf("%s = %v, want %v", tt.expr, ints, tt.want)
		}
	}
}

// TestCollectionScalarResults pins the operations answering one value, and that
// the unqualified, qualified and receiver forms of each agree.
func TestCollectionScalarResults(t *testing.T) {
	tests := []struct {
		expr string
		want Value
	}{
		{"SequenceFunctions::size(xs)", integerValue(3)},
		{"size(xs)", integerValue(3)},
		{"xs->size()", integerValue(3)},
		{"size(())", integerValue(0)},
		{"size(factor)", integerValue(1)},
		{"isEmpty(())", boolValue(true)},
		{"isEmpty(xs)", boolValue(false)},
		{"notEmpty(xs)", boolValue(true)},
		{"xs->head()", integerValue(1)},
		{"xs->last()", integerValue(3)},
		{"()->head()", nullValue()},
		{"()->last()", nullValue()},
		{"xs->includes(2)", boolValue(true)},
		{"xs->includes(4)", boolValue(false)},
		{"xs->includes(ys)", boolValue(false)},
		{"xs->includes((1, 3))", boolValue(true)},
		{"xs->excludes(4)", boolValue(true)},
		{"xs->includesOnly((3, 2, 1))", boolValue(true)},
		{"xs->equals((1, 2, 3))", boolValue(true)},
		{"xs->equals((3, 2, 1))", boolValue(false)},
		{"xs->contains(2)", boolValue(true)},
		{"xs->containsAll(ys)", boolValue(false)},
		{"sum(xs)", integerValue(6)},
		{"NumericalFunctions::sum(xs)", integerValue(6)},
		{"xs->sum()", integerValue(6)},
		{"IntegerFunctions::sum(xs)", integerValue(6)},
		{"product(xs)", integerValue(6)},
		{"xs->product()", integerValue(6)},
		// The identity of each aggregation is its answer for an empty
		// collection, as the library's own sum0/product1 compute it.
		{"sum(())", integerValue(0)},
		{"product(())", integerValue(1)},
		{"RealFunctions::sum(())", Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValReal, Real: 0}}},
		{"RealFunctions::product(xs)", Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValReal, Real: 6}}},
		{"sum((1, 2.5))", Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValReal, Real: 3.5}}},
		{"xs.{in x; x * 2}->sum()", integerValue(12)},
		{"xs->forAll {in x; x > 0}", boolValue(true)},
		{"xs->forAll {in x; x > 1}", boolValue(false)},
		{"()->forAll {in x; x > 1}", boolValue(true)},
		{"xs->exists {in x; x == 3}", boolValue(true)},
		{"()->exists {in x; x == 3}", boolValue(false)},
		{"flags->allTrue()", boolValue(false)},
		{"flags->anyTrue()", boolValue(true)},
		{"xs->selectOne {in x; x > 1}", integerValue(2)},
		{"xs->selectOne {in x; x > 5}", nullValue()},
		{"xs->size() == SequenceFunctions::size(xs)", boolValue(true)},
		// same asks identity, so an Integer sequence is not the same as a Real
		// one of equal magnitude, though equals holds for both.
		{"xs->same((1, 2, 3))", boolValue(true)},
		{"xs->same((1.0, 2.0, 3.0))", boolValue(false)},
		{"xs->equals((1.0, 2.0, 3.0))", boolValue(true)},
		{"xs->reduce {in a; in b; a + b}", integerValue(6)},
		{"xs->reduce {in a; in b; a * b}", integerValue(6)},
		{"()->reduce {in a; in b; a + b}", nullValue()},
		{"factor->reduce {in a; in b; a + b}", integerValue(10)},
		{"xs->minimize {in x; 0 - x}", integerValue(-3)},
		{"xs->maximize {in x; x * x}", integerValue(9)},
	}
	for _, tt := range tests {
		got, err := evalCollectionExpr(t, tt.expr)
		if err != nil {
			t.Errorf("%s: %v", tt.expr, err)
			continue
		}
		if !valueEqual(got, tt.want) {
			t.Errorf("%s = %v, want %v", tt.expr, got, tt.want)
		}
	}
}

// TestCollectionOperationErrors pins the diagnostics of the collection
// operations. Every one of these would otherwise be a wrong answer: a selector
// that is not a predicate read as false silently drops elements, and a body
// called with an argument it declares no parameter for silently ignores it.
func TestCollectionOperationErrors(t *testing.T) {
	tests := []struct {
		expr string
		want error
	}{
		{"xs.?{in x; x + 1}", ErrTypeMismatch},
		{"xs->select {in x; x * 2}", ErrTypeMismatch},
		{"xs->reject {in x; 1}", ErrTypeMismatch},
		{"xs->forAll {in x; x}", ErrTypeMismatch},
		{"xs->select {in x; in y; x > 0}", ErrBodyArity},
		{"xs->collect {in x; in y; x}", ErrBodyArity},
		{"xs->collect {}", ErrBodyArity},
		{"xs->select(2)", ErrTypeMismatch},
		{"xs->collect(xs)", ErrTypeMismatch},
		{"sum(flags)", ErrTypeMismatch},
		{"product((1, \"a\"))", ErrTypeMismatch},
		{"flags->allTrue(xs)", ErrCalcArity},
		{"xs->size(2)", ErrCalcArity},
		{"xs->subsequence(1, 4)", ErrIndexOutOfRange},
		{"xs->subsequence(0)", ErrIndexOutOfRange},
		{"xs->collect {in x; x + \"a\"}", ErrTypeMismatch},
		// An index outside the sequence names no element to remove and is
		// reported, where the vendored body would answer the sequence unchanged
		// (docs/project/spec-compliance.md records the divergence).
		{"xs->includingAt(9, 5)", ErrIndexOutOfRange},
		{"xs->includingAt(9, 0)", ErrIndexOutOfRange},
		{"()->includingAt(9, 2)", ErrIndexOutOfRange},
		{"xs->includingAt(9, 1.5)", ErrTypeMismatch},
		{"xs->excludingAt(4)", ErrIndexOutOfRange},
		{"xs->excludingAt(2, 1)", ErrIndexOutOfRange},
		{"xs->excludingAt(2, 9)", ErrIndexOutOfRange},
		{"()->excludingAt(1)", ErrIndexOutOfRange},
		{"xs->reduce {in a; a}", ErrBodyArity},
		// minimize declares `ScalarValue[1..*]`, so it has no answer for an
		// empty collection and says so rather than answering nothing.
		{"()->minimize {in x; x}", ErrMultiplicityViolation},
		{"()->maximize {in x; x}", ErrMultiplicityViolation},
		{"flags->maximize {in x; x}", ErrTypeMismatch},
		// A receiver binds by position, so it names no parameter of a call whose
		// arguments are named: reported rather than dropped.
		{"xs->size(seq = xs)", ErrReceiverWithNamedArgs},
		{"xs->collect(source = xs)", ErrReceiverWithNamedArgs},
	}
	for _, tt := range tests {
		got, err := evalCollectionExpr(t, tt.expr)
		if !errors.Is(err, tt.want) {
			t.Errorf("%s = (%v, %v), want %v", tt.expr, got, err, tt.want)
		}
	}
}

// A KerML sequence is flat: writing collections side by side is the same
// sequence as concatenating them, which is what makes SequenceFunctions::union
// the sequence expression `(seq1, seq2)`. A mapper answering several values
// contributes them all, for the same reason.
func TestSequenceExpressionsAreFlat(t *testing.T) {
	tests := []struct {
		expr string
		want []int64
	}{
		{"(xs, ys)", []int64{1, 2, 3, 2, 4}},
		{"xs->union(ys)", []int64{1, 2, 3, 2, 4}},
		{"(xs, ys, factor)", []int64{1, 2, 3, 2, 4, 10}},
		{"((xs, ys), 5)", []int64{1, 2, 3, 2, 4, 5}},
		// null is the empty sequence, so it contributes no element.
		{"(xs, null)", []int64{1, 2, 3}},
		{"xs.{in x; (x, x)}", []int64{1, 1, 2, 2, 3, 3}},
	}
	for _, tt := range tests {
		got, err := evalCollectionExpr(t, tt.expr)
		if err != nil {
			t.Errorf("%s: %v", tt.expr, err)
			continue
		}
		if !equalInts(intsOf(t, got), tt.want) {
			t.Errorf("%s = %v, want %v", tt.expr, intsOf(t, got), tt.want)
		}
	}

	// The operations count the flattened elements, so the two spellings of a
	// union answer alike and an index reaches an element rather than a sequence.
	scalars := []struct {
		expr string
		want int64
	}{
		{"(xs, ys)->size()", 5},
		{"xs->union(ys)->size()", 5},
		{"(xs, ys)#(4)", 2},
		{"(xs, ys)->sum()", 12},
	}
	for _, tt := range scalars {
		got, err := evalCollectionExpr(t, tt.expr)
		if err != nil {
			t.Errorf("%s: %v", tt.expr, err)
			continue
		}
		if !valueEqual(got, integerValue(tt.want)) {
			t.Errorf("%s = %v, want %d", tt.expr, got, tt.want)
		}
	}
}

// A receiver written before a call whose arguments are named binds to no
// parameter: the receiver binds by position and the arguments by name, so the
// call is reported rather than computed as if the receiver had never been
// written. This holds of a calc the model declares as much as of a library one.
func TestReceiverWithNamedArgumentsIsReported(t *testing.T) {
	src := `
package test {
	attribute factor = 10;
	calc scale { in n; return : Integer = n * 2; }
	attribute result = factor->scale(n = 1);
}`
	model, resolver, root := parseAndBuildModel(t, src)
	pkg, _ := root.LookupLocal("test")
	sym, ok := pkg.Scope.LookupLocal("result")
	if !ok || sym == nil {
		t.Fatal("attribute result not found")
	}
	decl, ok := sym.Decl.(*ast.Usage)
	if !ok {
		t.Fatalf("result declares %T, want a usage", sym.Decl)
	}
	ec := NewEvalContext(NewContext(model, resolver, 10000), pkg.Scope)
	got, err := ec.Eval(decl.Value)
	if !errors.Is(err, ErrReceiverWithNamedArgs) {
		t.Fatalf("factor->scale(n = 1) = (%v, %v), want %v", got, err, ErrReceiverWithNamedArgs)
	}
}

// TestCollectionOperationsOverSets pins that the operations read a set as the
// sequence of its elements in canonical order, whatever order the set was built
// in, so a model iterating or filtering a set gets the same answer for equal sets.
func TestCollectionOperationsOverSets(t *testing.T) {
	set := NewSet()
	for _, n := range []int64{3, 1, 2, 3} {
		set.Add(integerValue(n))
	}
	setVal := NewSetValue(set)

	if got := intsOf(t, sequenceOf(elementsOf(setVal))); !equalInts(got, []int64{1, 2, 3}) {
		t.Fatalf("set elements = %v, want the distinct elements in canonical order", got)
	}

	size, err := builtinSequenceSize(nil, []Value{setVal})
	if err != nil {
		t.Fatalf("size of a set: %v", err)
	}
	if !valueEqual(size, integerValue(3)) {
		t.Errorf("size of a set = %v, want 3", size)
	}

	// select and collect over a set need a body, so they are driven through a
	// model: the body is the same node kind the notation parses to.
	got, err := evalCollectionExpr(t, "xs.?{in x; x > 1}")
	if err != nil {
		t.Fatalf("select: %v", err)
	}
	if ints := intsOf(t, got); !equalInts(ints, []int64{2, 3}) {
		t.Errorf("select = %v, want [2 3]", ints)
	}
}

// TestSequenceIndexKeepsQuantityForm pins the disambiguation the two meanings of
// IndexExpr rest on: `n [unit]` is a quantity and `seq#(i)` is an index, and
// neither reads as the other.
func TestSequenceIndexKeepsQuantityForm(t *testing.T) {
	if _, err := evalCollectionExpr(t, "xs#(1) [m]"); !errors.Is(err, ErrNotAQuantity) {
		// The model of this test declares no unit, so the quantity form fails
		// as a quantity — not as an index.
		t.Errorf("xs#(1) [m] error = %v, want ErrNotAQuantity", err)
	}
	got, err := evalCollectionExpr(t, "xs#(2)")
	if err != nil || !valueEqual(got, integerValue(2)) {
		t.Errorf("xs#(2) = (%v, %v), want the Integer 2", got, err)
	}
}

// TestAggregateQuantities: a roll-up over measured values answers one in the first
// element's unit, of the kind the bare magnitudes give, and refuses a bare number mixed in.
func TestAggregateQuantities(t *testing.T) {
	ctx, scope := quantityContext(t)

	got, err := evalIn(t, ctx, scope, "sum((1 [m], 2 [m], 3 [m]))")
	if err != nil {
		t.Fatalf("sum of metres: %v", err)
	}
	if got.Kind != ValQuantity || got.Quantity().String() != "6 [m]" {
		t.Errorf("sum of metres = %v (%s), want 6 [m]", got, got.Kind)
	}

	// A commensurable element converts into the first element's unit.
	got, err = evalIn(t, ctx, scope, "sum((1 [m], 50 [cm]))")
	if err != nil {
		t.Fatalf("sum of mixed length units: %v", err)
	}
	if got.Kind != ValQuantity || got.Quantity().String() != "1.5 [m]" {
		t.Errorf("sum of mixed length units = %v, want 1.5 [m]", got)
	}

	if _, err := evalIn(t, ctx, scope, "sum((1 [m], 2))"); err == nil {
		t.Error("a bare number mixed with a length aggregated without an error")
	}
}
