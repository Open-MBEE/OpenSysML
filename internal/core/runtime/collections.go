package runtime

import (
	"fmt"
	"math"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

const subsequenceOp = "SequenceFunctions::subsequence"

// This file implements the collection operations of the Kernel Function
// Library — SequenceFunctions, CollectionFunctions and the collection part of
// ControlFunctions and NumericalFunctions — over runtime values, and the
// sequence index `seq#(i)` those operations are written in terms of.
//
// A KerML sequence is not a value of its own: every value is a sequence, of one
// element where it is a scalar and of none where it is null (`in seq: Anything
// [0..*]`, and SequenceFunctions::isEmpty is `seq == null`). elementsOf takes
// that view of a value, so `1->size()` is 1 and `null->isEmpty()` is true,
// exactly as the library's own definitions compute them.

// elementsOf views a value as the sequence of its elements: a sequence or a set
// as its own elements, null as the empty sequence, and any other value as the
// one-element sequence containing it. An array or a vector is one value — a
// Collection is a DataValue holding its `elements` — so it is not flattened
// here; its elements are read through that feature.
func elementsOf(val Value) []Value {
	switch val.Kind {
	case ValSequence:
		if val.Sequence() == nil {
			return nil
		}
		return val.Sequence().Elements()
	case ValSet:
		if val.Set() == nil {
			return nil
		}
		return val.Set().Elements()
	case ValNull, ValInvalid:
		return nil
	default:
		return []Value{val}
	}
}

// collectionElements views a value as the `elements` of the Collection it is:
// an array's or a vector's elements, and elementsOf for every other value. This
// is what CollectionFunctions operate on (`size(col.elements)`). Like elementsOf
// it is a view, not a collection the run keeps.
func collectionElements(val Value) []Value {
	switch val.Kind {
	case ValArray:
		return val.Array().Elements
	case ValVector:
		return constValues(val.Vector().Elements)
	case ValVectorQuantity:
		return constValues(val.VectorQuantity().Num)
	case ValTensorQuantity:
		return constValues(val.TensorQuantity().Num)
	}
	return elementsOf(val)
}

// overCollectionElements adapts a sequence operation to the CollectionFunctions
// form that the library defines over `col.elements`: an array or vector is
// passed as the charged sequence of its elements, a Collection object as what
// its `elements` hold (a set for a Set), any other value as itself.
func overCollectionElements(apply builtinFunc) builtinFunc {
	return func(ec *EvalContext, args []Value) (Value, error) {
		if len(args) > 0 {
			col, err := ec.collectionElementsValue(args[0])
			if err != nil {
				return Value{}, err
			}
			args = append([]Value{col}, args[1:]...)
		}
		return apply(ec, args)
	}
}

// collectionElementsValue is `col.elements` for a value passed as a Collection:
// the charged sequence of an array's or vector's elements, the collection a
// Collection object's `elements` feature holds, and any other value itself.
func (ec *EvalContext) collectionElementsValue(col Value) (Value, error) {
	switch col.Kind {
	case ValArray, ValVector, ValVectorQuantity, ValTensorQuantity:
		return ec.newSequence(collectionElements(col))
	case ValInstance:
		if elements, ok, err := ec.ctx.collectionObjectElements(col); ok || err != nil {
			return elements, err
		}
	}
	return col, nil
}

// collectionObjectElements is what a Collection object's `elements` feature
// reads as; ok is false for a value that is no Collection object.
func (ctx *Context) collectionObjectElements(val Value) (Value, bool, error) {
	if val.Kind != ValInstance {
		return Value{}, false, nil
	}
	inst, ok := ctx.Instance(val.Instance)
	if !ok || !ctx.specializes(inst.Type, ctx.librarySymbol(collectionType)) {
		return Value{}, false, nil
	}
	if _, ok := inst.FeatureValues[collectionElementsName]; !ok {
		return Value{}, false, nil
	}
	fv, err := inst.GetFeatureValue(ctx, collectionElementsName)
	if err != nil {
		return Value{}, true, err
	}
	elements, err := ctx.readFeatureValue(fv, collectionElementsName)
	return elements, true, err
}

// elementCount is len(elementsOf(val)) without materializing a scalar's
// one-element sequence.
func elementCount(val *Value) int64 {
	switch val.Kind {
	case ValSequence:
		if seq := val.Sequence(); seq != nil {
			return int64(seq.Size())
		}
		return 0
	case ValSet:
		if set := val.Set(); set != nil {
			return int64(set.Size())
		}
		return 0
	case ValNull, ValInvalid:
		return 0
	default:
		return 1
	}
}

// sequenceOf builds a sequence value from elements.
func sequenceOf(elements []Value) Value {
	seq := NewSequence()
	for _, elem := range elements {
		seq.Append(elem)
	}
	return NewSequenceValue(seq)
}

// newSequence builds a sequence value from elements, charging them against the
// run's element budget: elements are the memory a collection keeps, so every
// operation that materializes one goes through here.
func (ec *EvalContext) newSequence(elements []Value) (Value, error) {
	return ec.ctx.newSequence(elements)
}

// newSequence is EvalContext.newSequence for a materialization outside an evaluation.
func (ctx *Context) newSequence(elements []Value) (Value, error) {
	if err := ctx.chargeElements(int64(len(elements))); err != nil {
		return Value{}, err
	}
	return sequenceOf(elements), nil
}

// sequenceFrom is the sequence of kept drawn from sources: empty, it keeps the
// unit the sources' elements measure in, so an aggregate of it keeps their kind.
func (ec *EvalContext) sequenceFrom(kept []Value, sources ...Value) (Value, error) {
	if len(kept) == 0 {
		if unit, ok := elementUnitOf(sources...); ok {
			return NewEmptySequenceOf(unit), nil
		}
	}
	return ec.newSequence(kept)
}

// elementUnitOf is the unit the first source with one measures its elements in:
// the unit an empty sequence declares, or that of a quantity it holds.
func elementUnitOf(sources ...Value) (Unit, bool) {
	for _, source := range sources {
		if unit, ok := source.Sequence().ElementUnit(); ok {
			return unit, true
		}
		for _, elem := range elementsOf(source) {
			if q := elem.Quantity(); q != nil {
				return q.Unit, true
			}
		}
	}
	return Unit{}, false
}

// integerValue wraps a count as an Integer value, which is what the library's
// Natural-returning functions (size) and Positive parameters (index) carry.
func integerValue(n int64) Value {
	return Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: n}}
}

// nullValue is the empty result of an operation declared `Anything[0..1]` —
// head, last and `#` of a sequence that has no such element.
func nullValue() Value { return Value{Kind: ValNull} }

// indexOf reads a sequence index (`in index: Positive[1]`): one whole number, counting
// from 1, so 4 / 2 indexes while a fractional Real is reported rather than truncated.
func indexOf(op string, val Value) (int64, error) {
	if val.Kind == ValConst {
		if index, ok := val.Const.WholeNumber(); ok {
			return index, nil
		}
	}
	return 0, fmt.Errorf("%w: %s requires an Integer index, got %s", ErrTypeMismatch, op, describeValue(val))
}

// describeValue names a value's kind for a diagnostic, distinguishing the
// numeric constants a single kind covers.
func describeValue(val Value) string {
	if val.Kind == ValComplex {
		return "a Complex"
	}
	if val.Kind == ValQuantity {
		return "a quantity in " + val.Quantity().Unit.String()
	}
	if val.Kind != ValConst {
		return val.Kind.String()
	}
	switch val.Const.Kind {
	case semantics.ValInt:
		return "an Integer"
	case semantics.ValReal:
		return "a Real"
	case semantics.ValBool:
		return "a Boolean"
	case semantics.ValInfinity:
		return "an infinity"
	default:
		return "a constant"
	}
}

// elementAt returns the element at a 1-based index, reporting an index that
// names no position rather than returning nothing for it. SequenceFunctions
// declares the result `Anything[0..1]`, but a missing element and an index
// outside the sequence are different facts: `(1,2,3)#(4)` is an error, while
// `head(())` — an index into an empty sequence the library itself takes — is
// empty. Only the operations that index a computed position accept emptiness,
// and they call elementAtOrEmpty.
func elementAt(op string, elements []Value, index int64) (Value, error) {
	if index < 1 || index > int64(len(elements)) {
		return Value{}, fmt.Errorf("%w: %s %d is outside 1..%d", ErrIndexOutOfRange, op, index, len(elements))
	}
	return elements[index-1], nil
}

// elementAtOrEmpty returns the element at a 1-based index, or the empty result
// the library declares (`Anything[0..1]`) where the sequence has no such
// position. This is how head and last answer for an empty sequence.
func elementAtOrEmpty(elements []Value, index int64) Value {
	if index < 1 || index > int64(len(elements)) {
		return nullValue()
	}
	return elements[index-1]
}

// evalSequenceIndex evaluates `seq#(i)`, the index operator (KerML
// BaseFunctions::'#'): the i-th element of the operand's sequence, counting
// from 1, or the element of an array at one index per dimension. It shares its
// AST node with the quantity expression `5 [m]`, which the bracket form marks
// and evalIndexExpr handles.
func (ec *EvalContext) evalSequenceIndex(n *ast.IndexExpr) (Value, error) {
	operand, err := ec.Eval(n.Operand)
	if err != nil {
		return Value{}, err
	}
	indexVal, err := ec.Eval(n.Index)
	if err != nil {
		return Value{}, err
	}
	if indexes := elementsOf(indexVal); len(indexes) != 1 || isStructuredValue(&operand) {
		return builtinBaseIndex(ec, []Value{operand, indexVal})
	}
	index, err := indexOf("sequence index", indexVal)
	if err != nil {
		return Value{}, err
	}
	return elementAt("sequence index", elementsOf(operand), index)
}

// bodyOf reads the body a collection operation takes as its function-valued
// parameter, checking it declares the parameters the operation calls it with.
// An argument that is not itself a body is evaluated to the body it denotes.
func (ec *EvalContext) bodyOf(op string, val Value, arity int) (Value, error) {
	val, err := ec.denotedBody(val)
	if err != nil {
		return Value{}, err
	}
	return checkBody(op, val, arity)
}

// bodyOver is bodyOf for an operation over elements whose body parameter is
// `[0..*]`: empty over no elements, there is nothing to call it with and
// applied reports false. A body literal is checked as bodyOf checks it; a
// deferred argument that is not one is evaluated only where an element needs it.
func (ec *EvalContext) bodyOver(op string, val Value, arity int, elements []Value) (body Value, applied bool, err error) {
	if len(elements) == 0 && isDeferredNonBody(val) {
		return Value{}, false, nil
	}
	val, err = ec.denotedBody(val)
	if err != nil {
		return Value{}, false, err
	}
	if len(elements) == 0 && isEmptyValue(val) {
		return Value{}, false, nil
	}
	body, err = checkBody(op, val, arity)
	return body, err == nil, err
}

// isDeferredNonBody reports a deferred argument that denotes a body rather
// than being one, so reading it means evaluating it.
func isDeferredNonBody(val Value) bool {
	if val.Kind != ValExpr {
		return false
	}
	_, isBody := val.Expr().(*ast.BodyExpr)
	return !isBody
}

// denotedBody evaluates a deferred argument that is not itself a body to the
// value it denotes; a body or an evaluated value is returned as is.
func (ec *EvalContext) denotedBody(val Value) (Value, error) {
	if val.Kind == ValExpr {
		if _, ok := val.Expr().(*ast.BodyExpr); !ok {
			return ec.evalClosure(val)
		}
	}
	return val, nil
}

// checkBody verifies a denoted value is a body declaring arity parameters.
func checkBody(op string, val Value, arity int) (Value, error) {
	if val.Kind != ValExpr {
		return Value{}, fmt.Errorf("%w: %s requires a body expression, got %s", ErrTypeMismatch, op, describeValue(val))
	}
	body, ok := val.Expr().(*ast.BodyExpr)
	if !ok {
		return Value{}, fmt.Errorf("%w: %s requires a body expression, got %T", ErrTypeMismatch, op, val.Expr())
	}
	if len(body.Params) != arity {
		return Value{}, fmt.Errorf("%w: %s calls its body with %d argument(s), but it declares %d parameter(s)",
			ErrBodyArity, op, arity, len(body.Params))
	}
	return val, nil
}

// evalClosure evaluates a deferred expression in the environment it closes over.
func (ec *EvalContext) evalClosure(val Value) (Value, error) {
	return val.exprEnv(ec).Eval(val.Expr())
}

// applyBody evaluates a body with its parameters bound to args, as a frame of
// its own over the environment the body closes over.
func (ec *EvalContext) applyBody(val Value, args ...Value) (Value, error) {
	body, ok := val.Expr().(*ast.BodyExpr)
	if !ok {
		return Value{}, fmt.Errorf("%w: a body expression is required, got %s", ErrTypeMismatch, describeValue(val))
	}
	ec = val.exprEnv(ec)
	if body.Result == nil {
		return Value{}, fmt.Errorf("%w: body expression states no result", ErrNoResultExpression)
	}
	for _, member := range body.Members {
		decl := member
		if membership, ok := member.(*ast.Membership); ok {
			decl = membership.Member
		}
		if _, ok := decl.(*ast.Documentation); ok {
			continue
		}
		if _, ok := decl.(*ast.Usage); !ok {
			if decl == nil {
				return Value{}, fmt.Errorf("%w: nil body member", ErrUnsupportedBodyDeclaration)
			}
			return Value{}, fmt.Errorf("%w: %T", ErrUnsupportedBodyDeclaration, decl)
		}
	}
	bindings := make(map[string]Value, len(body.Params))
	for i := range body.Params {
		bindings[body.Params[i].Name] = args[i]
	}
	ec.Push(bindings)
	defer ec.Pop()
	// Each application is its own activation, so a calc usage read from the body
	// is evaluated once per element rather than once for the whole collection.
	outer, entered := ec.activation, ec.ctx.newActivation()
	ec.activation = entered
	defer func() {
		ec.ctx.endActivation(entered)
		ec.activation = outer
	}()
	// The parameters are declared in a scope of their own, so the result
	// resolves names there: a parameter is a declaration the body's expression
	// can name, not only a runtime binding.
	inner := ec
	if ec.scope != nil {
		inner = ec.evalIn(symbols.BodyExprScope(ec.scope, body))
	}
	return inner.Eval(body.Result)
}

// applyValueBody is applyBody for an operation needing a value: a body answering
// a feature holding none is a NoValueError, not a value of the wrong kind.
func (ec *EvalContext) applyValueBody(body Value, args ...Value) (Value, error) {
	val, err := ec.applyBody(body, args...)
	if err != nil {
		return Value{}, err
	}
	if expr, ok := body.Expr().(*ast.BodyExpr); ok && ec.ctx.HoldsNoValue(val) {
		return Value{}, ec.ctx.noValueError(val, expr.Result)
	}
	return val, nil
}

// applyPredicate evaluates a body expression whose result the library declares
// `Boolean[1]` — a selector, a rejector, a test — and reports a result that is
// not a Boolean rather than reading it as false, which would silently drop the
// element it was asked about.
func (ec *EvalContext) applyPredicate(op string, body Value, arg Value) (bool, error) {
	val, err := ec.applyBody(body, arg)
	if err != nil {
		return false, err
	}
	if val.Kind != ValConst || val.Const.Kind != semantics.ValBool {
		return false, fmt.Errorf("%w: %s: predicate must return boolean, got %s", ErrTypeMismatch, op, describeValue(val))
	}
	return val.Const.Bool, nil
}

// builtinSequenceIndex is SequenceFunctions::'#' called as a function.
func builtinSequenceIndex(ec *EvalContext, args []Value) (Value, error) {
	return sequenceIndex("SequenceFunctions::'#'", args)
}

// builtinCollectionIndex is CollectionFunctions::'#', the index over a
// collection's elements (`col.elements#(index)`).
func builtinCollectionIndex(ec *EvalContext, args []Value) (Value, error) {
	return overCollectionElements(func(_ *EvalContext, args []Value) (Value, error) {
		return sequenceIndex("CollectionFunctions::'#'", args)
	})(ec, args)
}

// sequenceIndex is a scalar-index `'#'` form: the element at one Positive index.
func sequenceIndex(op string, args []Value) (Value, error) {
	if err := checkArity(op, args, 2); err != nil {
		return Value{}, err
	}
	index, err := indexOf(op, args[1])
	if err != nil {
		return Value{}, err
	}
	return elementAt(op+" index", elementsOf(args[0]), index)
}

// builtinBaseIndex is BaseFunctions::'#': over an Array (a vector included) it is
// its specialization CollectionFunctions::'array#', one index per dimension;
// over a sequence its one `Positive[1..*]` index selects the element.
func builtinBaseIndex(ec *EvalContext, args []Value) (Value, error) {
	const op = "BaseFunctions::'#'"
	if err := checkArity(op, args, 2); err != nil {
		return Value{}, err
	}
	if isStructuredValue(&args[0]) {
		return arrayIndex(op, args[0], args[1])
	}
	indexes := elementsOf(args[1])
	switch len(indexes) {
	case 0:
		return Value{}, fmt.Errorf("%w: %s requires at least one index, got none", ErrMultiplicityViolation, op)
	case 1:
		return sequenceIndex(op, []Value{args[0], indexes[0]})
	}
	return Value{}, fmt.Errorf("%w: %s: %d indexes address an Array, got %s",
		ErrTypeMismatch, op, len(indexes), describeValue(args[0]))
}

// builtinArrayIndex is CollectionFunctions::'array#': the element of an Array
// at one Positive index per dimension.
func builtinArrayIndex(ec *EvalContext, args []Value) (Value, error) {
	const op = "CollectionFunctions::'array#'"
	if err := checkArity(op, args, 2); err != nil {
		return Value{}, err
	}
	return arrayIndex(op, args[0], args[1])
}

// arrayIndex addresses an array — or a vector, which is an Array of one
// dimension — by its indexes in row-major order. The library body answers null
// for an array of rank 0, which no index addresses.
func arrayIndex(op string, arr Value, indexesVal Value) (Value, error) {
	indexes, err := indexList(op, elementsOf(indexesVal))
	if err != nil {
		return Value{}, err
	}
	switch arr.Kind {
	case ValArray:
		a := arr.Array()
		if a.Rank() == 0 && len(indexes) == 0 {
			return nullValue(), nil
		}
		return a.at(op, indexes)
	case ValVector:
		v := arr.Vector()
		return (&Array{Dimensions: []int64{int64(v.Dimension())}, Elements: constValues(v.Elements)}).at(op, indexes)
	case ValVectorQuantity:
		vq := arr.VectorQuantity()
		elements := make([]Value, vq.Dimension())
		for i := range elements {
			elements[i] = NewQuantityValue(vq.component(i))
		}
		return (&Array{Dimensions: []int64{int64(vq.Dimension())}, Elements: elements}).at(op, indexes)
	case ValTensorQuantity:
		return arr.TensorQuantity().componentArray().at(op, indexes)
	}
	return Value{}, fmt.Errorf("%w: %s requires an Array (arr: Array[1]), got %s", ErrTypeMismatch, op, describeValue(arr))
}

// builtinSequenceSize is SequenceFunctions::size.
func builtinSequenceSize(ec *EvalContext, args []Value) (Value, error) {
	if err := checkArity("SequenceFunctions::size", args, 1); err != nil {
		return Value{}, err
	}
	return integerValue(int64(len(elementsOf(args[0])))), nil
}

// builtinSequenceIsEmpty is SequenceFunctions::isEmpty.
func builtinSequenceIsEmpty(ec *EvalContext, args []Value) (Value, error) {
	if err := checkArity("SequenceFunctions::isEmpty", args, 1); err != nil {
		return Value{}, err
	}
	return boolValue(len(elementsOf(args[0])) == 0), nil
}

// builtinSequenceNotEmpty is SequenceFunctions::notEmpty.
func builtinSequenceNotEmpty(ec *EvalContext, args []Value) (Value, error) {
	if err := checkArity("SequenceFunctions::notEmpty", args, 1); err != nil {
		return Value{}, err
	}
	return boolValue(len(elementsOf(args[0])) > 0), nil
}

// builtinSequenceIncludes is SequenceFunctions::includes, which asks whether
// every element of the second sequence is an element of the first
// (`seq2->forAll {in x; seq1->exists {in y; x == y}}`). A single value is a
// sequence of one, so `includes(seq, x)` asks about that one element.
func builtinSequenceIncludes(ec *EvalContext, args []Value) (Value, error) {
	if err := checkArity("SequenceFunctions::includes", args, 2); err != nil {
		return Value{}, err
	}
	return boolValue(includesAll(elementsOf(args[0]), elementsOf(args[1]))), nil
}

// builtinSequenceExcludes is SequenceFunctions::excludes: no element of the
// second sequence is an element of the first.
func builtinSequenceExcludes(ec *EvalContext, args []Value) (Value, error) {
	if err := checkArity("SequenceFunctions::excludes", args, 2); err != nil {
		return Value{}, err
	}
	seq1, seq2 := elementsOf(args[0]), elementsOf(args[1])
	for _, elem := range seq2 {
		if containsValue(seq1, elem) {
			return boolValue(false), nil
		}
	}
	return boolValue(true), nil
}

// builtinSequenceIncludesOnly is SequenceFunctions::includesOnly: each sequence
// includes the other, so they have the same elements whatever their order or
// their repetitions.
func builtinSequenceIncludesOnly(ec *EvalContext, args []Value) (Value, error) {
	if err := checkArity("SequenceFunctions::includesOnly", args, 2); err != nil {
		return Value{}, err
	}
	seq1, seq2 := elementsOf(args[0]), elementsOf(args[1])
	return boolValue(includesAll(seq1, seq2) && includesAll(seq2, seq1)), nil
}

// builtinSequenceEquals is SequenceFunctions::equals: the sequences have the
// same size and equal elements at every position.
func builtinSequenceEquals(ec *EvalContext, args []Value) (Value, error) {
	if err := checkArity("SequenceFunctions::equals", args, 2); err != nil {
		return Value{}, err
	}
	if args[0].Kind == ValSet && args[1].Kind == ValSet {
		return boolValue(args[0].Set().Equal(args[1].Set())), nil
	}
	x, y := elementsOf(args[0]), elementsOf(args[1])
	if len(x) != len(y) {
		return boolValue(false), nil
	}
	for i := range x {
		if !valueEqual(x[i], y[i]) {
			return boolValue(false), nil
		}
	}
	return boolValue(true), nil
}

// builtinCollectionEquals is CollectionFunctions::'==', equals over the two
// collections' elements (`col1.elements->equals(col2.elements)`).
func builtinCollectionEquals(ec *EvalContext, args []Value) (Value, error) {
	if err := checkArity("CollectionFunctions::'=='", args, 2); err != nil {
		return Value{}, err
	}
	col1, err := ec.collectionElementsValue(args[0])
	if err != nil {
		return Value{}, err
	}
	col2, err := ec.collectionElementsValue(args[1])
	if err != nil {
		return Value{}, err
	}
	return builtinSequenceEquals(ec, []Value{col1, col2})
}

// builtinSequenceSame is SequenceFunctions::same: the sequences have the same
// size and identical (`===`, not `==`) elements at every position, so a
// sequence of Integers is not the same as a sequence of equal Reals.
func builtinSequenceSame(ec *EvalContext, args []Value) (Value, error) {
	if err := checkArity("SequenceFunctions::same", args, 2); err != nil {
		return Value{}, err
	}
	x, y := elementsOf(args[0]), elementsOf(args[1])
	if len(x) != len(y) {
		return boolValue(false), nil
	}
	for i := range x {
		if !valueIdentical(x[i], y[i]) {
			return boolValue(false), nil
		}
	}
	return boolValue(true), nil
}

// builtinSequenceUnion is SequenceFunctions::union, the two sequences one after
// the other (`(seq1, seq2)`). It keeps repetitions: the library declares the
// result `ordered nonunique`.
func builtinSequenceUnion(ec *EvalContext, args []Value) (Value, error) {
	if err := checkArity("SequenceFunctions::union", args, 2); err != nil {
		return Value{}, err
	}
	return ec.concatSequences(args[0], args[1])
}

// concatSequences is `(seq1, seq2)`: the elements of the first followed by the
// elements of the second.
func (ec *EvalContext) concatSequences(first, second Value) (Value, error) {
	seq1, seq2 := elementsOf(first), elementsOf(second)
	joined := make([]Value, 0, len(seq1)+len(seq2))
	joined = append(joined, seq1...)
	joined = append(joined, seq2...)
	return ec.sequenceFrom(joined, first, second)
}

// builtinSequenceIntersection is SequenceFunctions::intersection, the elements
// of the first sequence that the second includes
// (`seq1->select {in x; seq2->includes(x)}`), in the first sequence's order.
func builtinSequenceIntersection(ec *EvalContext, args []Value) (Value, error) {
	if err := checkArity("SequenceFunctions::intersection", args, 2); err != nil {
		return Value{}, err
	}
	seq1, seq2 := elementsOf(args[0]), elementsOf(args[1])
	var common []Value
	for _, elem := range seq1 {
		if containsValue(seq2, elem) {
			common = append(common, elem)
		}
	}
	return ec.sequenceFrom(common, args[0])
}

// builtinSequenceIncluding is SequenceFunctions::including, the sequence with
// the given values appended (`union(seq, values)`).
func builtinSequenceIncluding(ec *EvalContext, args []Value) (Value, error) {
	if err := checkArity("SequenceFunctions::including", args, 2); err != nil {
		return Value{}, err
	}
	return builtinSequenceUnion(ec, args)
}

// builtinSequenceExcluding is SequenceFunctions::excluding, the sequence
// without the elements the second argument includes
// (`seq->reject {in x; values->includes(x)}`).
func builtinSequenceExcluding(ec *EvalContext, args []Value) (Value, error) {
	if err := checkArity("SequenceFunctions::excluding", args, 2); err != nil {
		return Value{}, err
	}
	seq, values := elementsOf(args[0]), elementsOf(args[1])
	var kept []Value
	for _, elem := range seq {
		if !containsValue(values, elem) {
			kept = append(kept, elem)
		}
	}
	return ec.sequenceFrom(kept, args[0])
}

// builtinSequenceIncludingAt inserts values before the 1-based index, shifting
// the tail right; index size+1 appends. The vendored body drops the element at
// index instead, recorded as an OMG source bug (docs/project/omg-issues.md).
func builtinSequenceIncludingAt(ec *EvalContext, args []Value) (Value, error) {
	const op = "SequenceFunctions::includingAt"
	if err := checkArity(op, args, 3); err != nil {
		return Value{}, err
	}
	return ec.insertAt(op, args[0], args[1], args[2])
}

// insertAt is seq with values inserted before its index-th element, or after
// its last when index is one past it; any other index is out of range.
func (ec *EvalContext) insertAt(op string, seq, values, at Value) (Value, error) {
	elements, inserted := elementsOf(seq), elementsOf(values)
	index, err := indexOf(op, at)
	if err != nil {
		return Value{}, err
	}
	if index < 1 || index > int64(len(elements))+1 {
		return Value{}, fmt.Errorf("%w: %s insertion index %d is outside 1..%d",
			ErrIndexOutOfRange, op, index, len(elements)+1)
	}
	result := make([]Value, 0, len(elements)+len(inserted))
	result = append(result, elements[:index-1]...)
	result = append(result, inserted...)
	result = append(result, elements[index-1:]...)
	return ec.newSequence(result)
}

// builtinSequenceSubsequence is SequenceFunctions::subsequence, the elements
// from startIndex to endIndex inclusive (`(startIndex..endIndex)->collect {in
// i; seq#(i)}`). endIndex defaults to the sequence's size, as the library
// declares. A start past the end selects nothing — that is how the library's
// own `tail` is `subsequence(seq, 2)` for a one-element sequence — but an index
// beyond the sequence is reported rather than silently clamped.
func builtinSequenceSubsequence(ec *EvalContext, args []Value) (Value, error) {
	if err := checkArity(subsequenceOp, args, 3); err != nil {
		return Value{}, err
	}
	elements := elementsOf(args[0])
	start, err := indexOf(subsequenceOp, args[1])
	if err != nil {
		return Value{}, err
	}
	end := int64(len(elements))
	if args[2].Kind != ValNull {
		if end, err = indexOf(subsequenceOp, args[2]); err != nil {
			return Value{}, err
		}
	}
	if start < 1 {
		return Value{}, fmt.Errorf("%w: SequenceFunctions::subsequence start index %d is outside 1..%d",
			ErrIndexOutOfRange, start, len(elements))
	}
	if start > end {
		return ec.sequenceFrom(nil, args[0])
	}
	if end > int64(len(elements)) {
		return Value{}, fmt.Errorf("%w: SequenceFunctions::subsequence end index %d is outside 1..%d",
			ErrIndexOutOfRange, end, len(elements))
	}
	return ec.sequenceFrom(elements[start-1:end], args[0])
}

// builtinSequenceExcludingAt is SequenceFunctions::excludingAt, the sequence
// without the elements from startIndex to endIndex
// (`(seq->subsequence(1, startIndex - 1), seq->subsequence(endIndex + 1))`).
// endIndex defaults to startIndex, as the library declares, so one argument
// removes one element.
func builtinSequenceExcludingAt(ec *EvalContext, args []Value) (Value, error) {
	const op = "SequenceFunctions::excludingAt"
	if err := checkArity(op, args, 3); err != nil {
		return Value{}, err
	}
	elements := elementsOf(args[0])
	start, err := indexOf(op, args[1])
	if err != nil {
		return Value{}, err
	}
	end := start
	if args[2].Kind != ValNull {
		if end, err = indexOf(op, args[2]); err != nil {
			return Value{}, err
		}
	}
	if start < 1 || start > int64(len(elements)) {
		return Value{}, fmt.Errorf("%w: %s start index %d is outside 1..%d",
			ErrIndexOutOfRange, op, start, len(elements))
	}
	if end < start || end > int64(len(elements)) {
		return Value{}, fmt.Errorf("%w: %s end index %d is outside %d..%d",
			ErrIndexOutOfRange, op, end, start, len(elements))
	}
	kept := make([]Value, 0, len(elements)-int(end-start+1))
	kept = append(kept, elements[:start-1]...)
	kept = append(kept, elements[end:]...)
	return ec.sequenceFrom(kept, args[0])
}

// builtinSequenceHead is SequenceFunctions::head, `seq#(1)`: the first element,
// or nothing where the sequence is empty.
func builtinSequenceHead(ec *EvalContext, args []Value) (Value, error) {
	if err := checkArity("SequenceFunctions::head", args, 1); err != nil {
		return Value{}, err
	}
	return elementAtOrEmpty(elementsOf(args[0]), 1), nil
}

// builtinSequenceTail is SequenceFunctions::tail, `subsequence(seq, 2)`: every
// element but the first.
func builtinSequenceTail(ec *EvalContext, args []Value) (Value, error) {
	if err := checkArity("SequenceFunctions::tail", args, 1); err != nil {
		return Value{}, err
	}
	elements := elementsOf(args[0])
	if len(elements) == 0 {
		return ec.sequenceFrom(nil, args[0])
	}
	return ec.sequenceFrom(elements[1:], args[0])
}

// builtinSequenceLast is SequenceFunctions::last, `seq#(size(seq))`.
func builtinSequenceLast(ec *EvalContext, args []Value) (Value, error) {
	if err := checkArity("SequenceFunctions::last", args, 1); err != nil {
		return Value{}, err
	}
	elements := elementsOf(args[0])
	return elementAtOrEmpty(elements, int64(len(elements))), nil
}

// builtinCollectionContains is CollectionFunctions::contains, which asks
// whether the collection's elements include the given values
// (`col.elements->includes(values)`).
func builtinCollectionContains(ec *EvalContext, args []Value) (Value, error) {
	if err := checkArity("CollectionFunctions::contains", args, 2); err != nil {
		return Value{}, err
	}
	return boolValue(includesAll(elementsOf(args[0]), elementsOf(args[1]))), nil
}

// builtinCollectionContainsAll is CollectionFunctions::containsAll, contains of
// the second collection's elements (`contains(col1, col2.elements)`).
func builtinCollectionContainsAll(ec *EvalContext, args []Value) (Value, error) {
	if err := checkArity("CollectionFunctions::containsAll", args, 2); err != nil {
		return Value{}, err
	}
	col2, err := ec.collectionElementsValue(args[1])
	if err != nil {
		return Value{}, err
	}
	return boolValue(includesAll(elementsOf(args[0]), collectionElements(col2))), nil
}

// builtinControlSelect is ControlFunctions::select, the elements the selector
// holds for, in the collection's order. A selector that answers something other
// than a Boolean is reported: dropping the element instead would answer a
// filter the model never wrote.
func builtinControlSelect(ec *EvalContext, args []Value) (Value, error) {
	return ec.filter("ControlFunctions::select", args, true)
}

// builtinControlReject is ControlFunctions::reject, the elements the rejector
// does not hold for — select's complement.
func builtinControlReject(ec *EvalContext, args []Value) (Value, error) {
	return ec.filter("ControlFunctions::reject", args, false)
}

// filter is select (keep=true) and reject (keep=false).
func (ec *EvalContext) filter(op string, args []Value, keep bool) (Value, error) {
	if err := checkArity(op, args, 2); err != nil {
		return Value{}, err
	}
	elements := elementsOf(args[0])
	body, applied, err := ec.bodyOver(op, args[1], 1, elements)
	if err != nil {
		return Value{}, err
	}
	// A filter keeps the elements' type (KerML checkSelectExpressionResultSpecialization).
	if !applied {
		return ec.sequenceFrom(nil, args[0])
	}
	var kept []Value
	for _, elem := range elements {
		holds, err := ec.applyPredicate(op, body, elem)
		if err != nil {
			return Value{}, err
		}
		if holds == keep {
			kept = append(kept, elem)
		}
	}
	return ec.sequenceFrom(kept, args[0])
}

// emptyMapping is the result of mapping no element through body: empty, and
// typed by the dimension the body's result declares where its parameters fix one.
func (ec *EvalContext) emptyMapping(val Value) Value {
	body, ok := val.Expr().(*ast.BodyExpr)
	if !ok || body.Result == nil {
		return sequenceOf(nil)
	}
	env := val.exprEnv(ec)
	if env.scope == nil {
		return sequenceOf(nil)
	}
	if typed, ok := ec.ctx.emptyOfDeclared(symbols.BodyExprScope(env.scope, body), body.Result); ok {
		return typed
	}
	return sequenceOf(nil)
}

// builtinControlSelectOne is ControlFunctions::selectOne, the first element the
// selector holds for (`collection->select {...}#(1)`), or nothing where it
// holds for none.
func builtinControlSelectOne(ec *EvalContext, args []Value) (Value, error) {
	selected, err := ec.filter("ControlFunctions::selectOne", args, true)
	if err != nil {
		return Value{}, err
	}
	pick := elementAtOrEmpty(elementsOf(selected), 1)
	ec.ctx.evaluations.pick(pick)
	return pick, nil
}

// builtinControlCollect is ControlFunctions::collect, the mapper's result for
// each element of the collection, in the collection's order.
func builtinControlCollect(ec *EvalContext, args []Value) (Value, error) {
	const op = "ControlFunctions::collect"
	if err := checkArity(op, args, 2); err != nil {
		return Value{}, err
	}
	elements := elementsOf(args[0])
	body, applied, err := ec.bodyOver(op, args[1], 1, elements)
	if err != nil {
		return Value{}, err
	}
	// The mapper returns `Anything[0..*]`, so a mapper answering several values
	// contributes them all: the collected sequence is flat, as every KerML
	// sequence is.
	if !applied || len(elements) == 0 {
		return ec.emptyMapping(args[1]), nil
	}
	var mapped, answers []Value
	for _, elem := range elements {
		val, err := ec.applyBody(body, elem)
		if err != nil {
			return Value{}, err
		}
		// Charged as the result grows, so a run over the ceiling is reported
		// before the whole mapping is held.
		contributed := elementsOf(val)
		if err := ec.ctx.chargeElements(int64(len(contributed))); err != nil {
			return Value{}, err
		}
		mapped = append(mapped, contributed...)
		answers = append(answers, val)
	}
	if len(mapped) == 0 {
		if unit, ok := elementUnitOf(answers...); ok {
			return NewEmptySequenceOf(unit), nil
		}
		return ec.emptyMapping(args[1]), nil
	}
	return sequenceOf(mapped), nil
}

// builtinControlReduce is ControlFunctions::reduce, the collection folded with a
// two-parameter body from its first element onwards: `(1,2,3)->reduce {in a; in
// b; a + b}` is 6. An empty collection reduces to nothing, since there is no
// element to start from and the reducer states no identity element.
func builtinControlReduce(ec *EvalContext, args []Value) (Value, error) {
	const op = "ControlFunctions::reduce"
	if err := checkArity(op, args, 2); err != nil {
		return Value{}, err
	}
	elements := elementsOf(args[0])
	body, applied, err := ec.bodyOver(op, args[1], 2, elements)
	if err != nil {
		return Value{}, err
	}
	if !applied || len(elements) == 0 {
		return nullValue(), nil
	}
	acc := elements[0]
	for _, elem := range elements[1:] {
		if acc, err = ec.applyBody(body, acc, elem); err != nil {
			return Value{}, err
		}
	}
	return acc, nil
}

// builtinControlMinimize is ControlFunctions::minimize, the least of the values
// the given body answers for the collection's elements
// (`collection->collect {in x; fn(x)}->reduce min`). The library declares the
// collection `ScalarValue[1..*]`, so an empty collection has no least value and
// is reported rather than answered with nothing.
func builtinControlMinimize(ec *EvalContext, args []Value) (Value, error) {
	return ec.extremum("ControlFunctions::minimize", args, true)
}

// builtinControlMaximize is ControlFunctions::maximize, minimize's counterpart.
func builtinControlMaximize(ec *EvalContext, args []Value) (Value, error) {
	return ec.extremum("ControlFunctions::maximize", args, false)
}

// extremum is minimize (least=true) and maximize over the body's results.
func (ec *EvalContext) extremum(op string, args []Value, least bool) (Value, error) {
	if err := checkArity(op, args, 2); err != nil {
		return Value{}, err
	}
	elements := elementsOf(args[0])
	if len(elements) == 0 {
		return Value{}, fmt.Errorf("%w: %s requires a collection of at least one element",
			ErrMultiplicityViolation, op)
	}
	body, err := ec.bodyOf(op, args[1], 1)
	if err != nil {
		return Value{}, err
	}
	var best Value
	for i, elem := range elements {
		val, err := ec.applyValueBody(body, elem)
		if err != nil {
			return Value{}, err
		}
		if val.Kind != ValConst || !val.Const.IsNumeric() {
			return Value{}, fmt.Errorf("%w: %s requires numeric values, got %s",
				ErrTypeMismatch, op, describeValue(val))
		}
		if i == 0 {
			best = val
			continue
		}
		if (toReal(val.Const) < toReal(best.Const)) == least &&
			toReal(val.Const) != toReal(best.Const) {
			best = val
		}
	}
	return best, nil
}

// builtinControlForAll is ControlFunctions::forAll: the test holds for every
// element, and vacuously for none.
func builtinControlForAll(ec *EvalContext, args []Value) (Value, error) {
	return ec.quantify("ControlFunctions::forAll", args, true)
}

// builtinControlExists is ControlFunctions::exists: the test holds for at least
// one element.
func builtinControlExists(ec *EvalContext, args []Value) (Value, error) {
	return ec.quantify("ControlFunctions::exists", args, false)
}

// quantify is forAll (universal=true) and exists. Both stop at the element that
// decides the answer, so a test with an error past that element is not reached,
// as the library's short-circuiting `and`/`or` do not reach it either.
func (ec *EvalContext) quantify(op string, args []Value, universal bool) (Value, error) {
	if err := checkArity(op, args, 2); err != nil {
		return Value{}, err
	}
	elements := elementsOf(args[0])
	body, applied, err := ec.bodyOver(op, args[1], 1, elements)
	if err != nil {
		return Value{}, err
	}
	if !applied {
		return boolValue(universal), nil
	}
	for _, elem := range elements {
		holds, err := ec.applyPredicate(op, body, elem)
		if err != nil {
			return Value{}, err
		}
		if holds != universal {
			return boolValue(!universal), nil
		}
	}
	return boolValue(universal), nil
}

// builtinControlAllTrue is ControlFunctions::allTrue, forAll over a collection
// of Booleans (`collection->forAll {in x; x}`).
func builtinControlAllTrue(ec *EvalContext, args []Value) (Value, error) {
	return truthOf("ControlFunctions::allTrue", args, true)
}

// builtinControlAnyTrue is ControlFunctions::anyTrue.
func builtinControlAnyTrue(ec *EvalContext, args []Value) (Value, error) {
	return truthOf("ControlFunctions::anyTrue", args, false)
}

// truthOf is allTrue (universal=true) and anyTrue over Boolean elements.
func truthOf(op string, args []Value, universal bool) (Value, error) {
	if err := checkArity(op, args, 1); err != nil {
		return Value{}, err
	}
	for _, elem := range elementsOf(args[0]) {
		if elem.Kind != ValConst || elem.Const.Kind != semantics.ValBool {
			return Value{}, fmt.Errorf("%w: %s requires Boolean elements, got %s", ErrTypeMismatch, op, describeValue(elem))
		}
		if elem.Const.Bool != universal {
			return boolValue(!universal), nil
		}
	}
	return boolValue(universal), nil
}

// builtinNumericalSum is NumericalFunctions::sum: the sum of the collection's
// elements, and the additive identity for an empty collection, which is what
// the library's `sum0(collection, 0)` computes. The result keeps the elements'
// kind: Integer elements sum to an Integer (IntegerFunctions::sum returns
// `Integer[1]`), a Real anywhere makes the sum a Real.
func builtinNumericalSum(ec *EvalContext, args []Value) (Value, error) {
	return aggregate("NumericalFunctions::sum", args, ast.OpAdd, false)
}

// builtinNumericalProduct is NumericalFunctions::product, with the
// multiplicative identity for an empty collection (`product1(collection, 1)`).
func builtinNumericalProduct(ec *EvalContext, args []Value) (Value, error) {
	return aggregate("NumericalFunctions::product", args, ast.OpMul, false)
}

// builtinRealSum is RealFunctions::sum and RationalFunctions::sum, whose
// identity the library declares Real (`sum0(collection, 0.0)`): an empty
// collection sums to 0.0, not 0.
func builtinRealSum(ec *EvalContext, args []Value) (Value, error) {
	return aggregate("RealFunctions::sum", args, ast.OpAdd, true)
}

// builtinRealProduct is RealFunctions::product and RationalFunctions::product
// (`product1(collection, 1.0)`).
func builtinRealProduct(ec *EvalContext, args []Value) (Value, error) {
	return aggregate("RealFunctions::product", args, ast.OpMul, true)
}

// aggregate folds the collection's numeric elements with op, starting from its
// identity element: 0 for a sum, 1 for a product, a Real where real says so.
// A non-numeric element is reported rather than skipped or coerced. The sum of
// no elements read from a quantity-typed declaration is that quantity's zero.
func aggregate(op string, args []Value, operator ast.OperatorKind, real bool) (Value, error) {
	if err := checkArity(op, args, 1); err != nil {
		return Value{}, err
	}
	elements := elementsOf(args[0])
	if operator == ast.OpAdd && len(elements) == 0 {
		if unit, ok := args[0].Sequence().ElementUnit(); ok {
			return typedZero(unit, real), nil
		}
	}
	// A quantity carries its unit through an aggregation as through the folded
	// operator, so a collection of measured values aggregates to one.
	for _, elem := range elements {
		if elem.Kind == ValQuantity {
			return aggregateQuantities(op, elements, operator)
		}
	}
	// A Complex is a Number, so a collection holding one folds as ComplexFunctions'.
	if holdsComplex(elements) {
		return aggregateComplex(op, args[0], operator)
	}
	identity := int64(0)
	if operator == ast.OpMul {
		identity = 1
	}
	acc := semantics.Value{Kind: semantics.ValInt, Int: identity}
	if real {
		acc = semantics.Value{Kind: semantics.ValReal, Real: float64(identity)}
	}
	for _, elem := range elements {
		if elem.Kind != ValConst || !elem.Const.IsNumeric() {
			return Value{}, fmt.Errorf("%w: %s requires numeric elements, got %s", ErrTypeMismatch, op, describeValue(elem))
		}
		next, err := foldNumeric(op, operator, acc, elem.Const)
		if err != nil {
			return Value{}, err
		}
		acc = next
	}
	return Value{Kind: ValConst, Const: acc}, nil
}

// typedZero is the additive identity of the quantities measured in unit: an
// Integer 0 in that unit, or a Real one where real says so.
func typedZero(unit Unit, real bool) Value {
	num := semantics.Value{Kind: semantics.ValInt, Int: 0}
	if real {
		num = semantics.Value{Kind: semantics.ValReal, Real: 0}
	}
	return NewQuantityValue(&Quantity{Num: num, Unit: unit})
}

// aggregateQuantities folds a collection holding a quantity in the unit of its
// first element, as the binary operator does. A bare number is a magnitude of
// dimension one, so mixing one in reports incommensurable units.
func aggregateQuantities(op string, elements []Value, operator ast.OperatorKind) (Value, error) {
	var acc Value
	for i, elem := range elements {
		q, ok := asQuantity(elem)
		if !ok {
			return Value{}, fmt.Errorf("%w: %s requires numeric elements, got %s", ErrTypeMismatch, op, describeValue(elem))
		}
		if i == 0 {
			acc = NewQuantityValue(q)
			continue
		}
		accQ, _ := asQuantity(acc)
		var (
			next Value
			err  error
		)
		if operator == ast.OpAdd {
			next, err = addQuantities(operator, accQ, q)
		} else {
			next, err = scaleQuantities(operator, accQ, q)
		}
		if err != nil {
			return Value{}, fmt.Errorf("%s: %w", op, err)
		}
		acc = next
	}
	return acc, nil
}

// foldNumeric applies one step of an aggregation, keeping Integer arithmetic
// exact where both operands are Integers and reporting a result outside the
// Integer range rather than wrapping it.
func foldNumeric(op string, operator ast.OperatorKind, acc, elem semantics.Value) (semantics.Value, error) {
	if acc.Kind == semantics.ValInt && elem.Kind == semantics.ValInt {
		var result int64
		switch operator {
		case ast.OpAdd:
			result = acc.Int + elem.Int
			if (elem.Int > 0 && result < acc.Int) || (elem.Int < 0 && result > acc.Int) {
				return semantics.Value{}, fmt.Errorf("%w: %s exceeds the Integer range", semantics.ErrArithmeticOverflow, op)
			}
		case ast.OpMul:
			result = acc.Int * elem.Int
			if acc.Int != 0 && (result/acc.Int != elem.Int || (acc.Int == -1 && elem.Int == math.MinInt64)) {
				return semantics.Value{}, fmt.Errorf("%w: %s exceeds the Integer range", semantics.ErrArithmeticOverflow, op)
			}
		}
		return semantics.Value{Kind: semantics.ValInt, Int: result}, nil
	}
	// An infinity has no place in a sum or a product of measured values: the
	// aggregation would answer an infinity for every element that follows.
	if acc.Kind == semantics.ValInfinity || elem.Kind == semantics.ValInfinity {
		return semantics.Value{}, fmt.Errorf("%w: %s requires finite elements", ErrTypeMismatch, op)
	}
	var result float64
	switch operator {
	case ast.OpAdd:
		result = toReal(acc) + toReal(elem)
	case ast.OpMul:
		result = toReal(acc) * toReal(elem)
	}
	if math.IsInf(result, 0) {
		return semantics.Value{}, fmt.Errorf("%w: %s is not a finite Real", semantics.ErrArithmeticOverflow, op)
	}
	return semantics.Value{Kind: semantics.ValReal, Real: result}, nil
}

// includesAll reports whether every element of want is an element of have,
// which is what SequenceFunctions::includes computes.
func includesAll(have, want []Value) bool {
	for _, elem := range want {
		if !containsValue(have, elem) {
			return false
		}
	}
	return true
}

// containsValue reports whether elements holds a value equal to val.
func containsValue(elements []Value, val Value) bool {
	for _, elem := range elements {
		if valueEqual(elem, val) {
			return true
		}
	}
	return false
}

// checkArity reports an argument count a built-in function cannot be called
// with. Every collection parameter has multiplicity [1] or [0..*] with no
// default, so the count is exact.
func checkArity(op string, args []Value, want int) error {
	if len(args) != want {
		return fmt.Errorf("%w: %s takes %d argument(s), got %d", ErrCalcArity, op, want, len(args))
	}
	return nil
}
