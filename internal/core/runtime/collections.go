package runtime

import (
	"fmt"
	"math"
	"slices"

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

// soleElement is what a value denotes where one value is taken: a one-element collection is its
// element, a feature's values being a sequence its multiplicity constrains (KerML 1.0 §7.3.4.1, §7.4.12).
func soleElement(val Value) Value {
	if val.Kind != ValSequence && val.Kind != ValSet {
		return val
	}
	if elements := elementsOf(val); len(elements) == 1 {
		return elements[0]
	}
	return val
}

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
	val = soleElement(val)
	if val.Kind == ValConst {
		if index, ok := val.Const.WholeNumber(); ok {
			return index, nil
		}
	}
	return 0, fmt.Errorf("%w: %s requires an Integer index, got %s", ErrTypeMismatch, op, describeValue(val))
}

// fixedIndex is indexOf for an index argument the model determines; an open one is unread.
func fixedIndex(op string, val Value) (index int64, fixed bool, err error) {
	return fixedArg(val, func(v Value) (int64, error) { return indexOf(op, v) })
}

// indexWithin rejects an index that names no position in a sequence of count values: below
// first, or past the most positions the count admits, slack beyond its last element.
func indexWithin(op, what string, index, first int64, count semantics.Range, slack int64) error {
	last := count.Plus(semantics.CountRange(slack)).Upper
	if index < first || (last.Known && !last.Infinite && index > last.Value) {
		return fmt.Errorf("%w: %s %s %d is outside %d..%s", ErrIndexOutOfRange, op, what, index, first, last.Text())
	}
	return nil
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
	if val, open, err := ec.ctx.undeterminedIndex("sequence index", operand, indexVal); open {
		return val, err
	}
	index, err := indexOf("sequence index", indexVal)
	if err != nil {
		return Value{}, err
	}
	return elementAt("sequence index", elementsOf(operand), index)
}

// undeterminedIndex is `seq#(index)` where the model leaves seq or index open: the
// value at a position seq fixes, else undetermined; an index past the count seq may
// reach is out of range.
func (ctx *Context) undeterminedIndex(op string, seq, indexVal Value) (Value, bool, error) {
	if indexVal.Kind == ValUndetermined {
		if err := ctx.openNumericIndex(op, indexVal); err != nil {
			return Value{}, true, err
		}
		if certainlyEmpty(seq) {
			return Value{}, true, fmt.Errorf("%w: %s into an empty sequence", ErrIndexOutOfRange, op)
		}
		return undeterminedOne(seq, indexVal), true, nil
	}
	u := seq.Undetermined()
	if u == nil {
		return Value{}, false, nil
	}
	index, err := indexOf(op, indexVal)
	if err != nil {
		return Value{}, true, err
	}
	upper := u.Count().Upper
	if index < 1 || (upper.Known && !upper.Infinite && index > upper.Value) {
		return Value{}, true, fmt.Errorf("%w: %s %d is outside 1..%s", ErrIndexOutOfRange, op, index, upper.Text())
	}
	if val, fixed := positionAt(u.Positions(), index); fixed {
		return val, true, nil
	}
	return undeterminedOne(seq), true, nil
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
	val, err := ec.applyTest(op, body, arg)
	if err != nil {
		return false, err
	}
	if val.Kind == ValUndetermined {
		return false, fmt.Errorf("%w: %s: predicate must return boolean, got %s", ErrTypeMismatch, op, describeValue(val))
	}
	return val.Const.Bool, nil
}

// applyTest is applyPredicate admitting a test the model leaves undetermined,
// which it returns as such for the operation to weigh.
func (ec *EvalContext) applyTest(op string, body Value, arg Value) (Value, error) {
	val, err := ec.applyBody(body, arg)
	if err != nil {
		return Value{}, err
	}
	if val.Kind == ValUndetermined {
		return val, nil
	}
	if val.Kind != ValConst || val.Const.Kind != semantics.ValBool {
		return Value{}, fmt.Errorf("%w: %s: predicate must return boolean, got %s", ErrTypeMismatch, op, describeValue(val))
	}
	return val, nil
}

// builtinSequenceIndex is SequenceFunctions::'#' called as a function.
func builtinSequenceIndex(ec *EvalContext, args []Value) (Value, error) {
	return ec.ctx.sequenceIndex("SequenceFunctions::'#'", args)
}

// builtinCollectionIndex is CollectionFunctions::'#', the index over a
// collection's elements (`col.elements#(index)`).
func builtinCollectionIndex(ec *EvalContext, args []Value) (Value, error) {
	return overCollectionElements(func(_ *EvalContext, args []Value) (Value, error) {
		return ec.ctx.sequenceIndex("CollectionFunctions::'#'", args)
	})(ec, args)
}

// sequenceIndex is a scalar-index `'#'` form: the element at one Positive index.
func (ctx *Context) sequenceIndex(op string, args []Value) (Value, error) {
	if err := checkArity(op, args, 2); err != nil {
		return Value{}, err
	}
	if val, open, err := ctx.undeterminedIndex(op+" index", args[0], args[1]); open {
		return val, err
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
	if val, open, err := ec.ctx.undeterminedIndex(op+" index", args[0], args[1]); open {
		return val, err
	}
	indexes := elementsOf(args[1])
	switch len(indexes) {
	case 0:
		return Value{}, fmt.Errorf("%w: %s requires at least one index, got none", ErrMultiplicityViolation, op)
	case 1:
		return ec.ctx.sequenceIndex(op, []Value{args[0], indexes[0]})
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
	if n, exact := countOf(args[0]).Exactly(); exact {
		return integerValue(n), nil
	}
	return undeterminedOne(args[0]), nil
}

// builtinSequenceIsEmpty is SequenceFunctions::isEmpty.
func builtinSequenceIsEmpty(ec *EvalContext, args []Value) (Value, error) {
	if err := checkArity("SequenceFunctions::isEmpty", args, 1); err != nil {
		return Value{}, err
	}
	return emptiness(args[0], true)
}

// builtinSequenceNotEmpty is SequenceFunctions::notEmpty.
func builtinSequenceNotEmpty(ec *EvalContext, args []Value) (Value, error) {
	if err := checkArity("SequenceFunctions::notEmpty", args, 1); err != nil {
		return Value{}, err
	}
	return emptiness(args[0], false)
}

// emptiness answers whether val holds no value (empty) or some (!empty), which
// the count the model fixes decides even where the values stay undetermined.
func emptiness(val Value, empty bool) (Value, error) {
	switch {
	case certainlyEmpty(val):
		return boolValue(empty), nil
	case certainlyNonEmpty(val):
		return boolValue(!empty), nil
	}
	return undeterminedOne(val), nil
}

// builtinSequenceIncludes is SequenceFunctions::includes, which asks whether
// every element of the second sequence is an element of the first
// (`seq2->forAll {in x; seq1->exists {in y; x == y}}`). A single value is a
// sequence of one, so `includes(seq, x)` asks about that one element.
func builtinSequenceIncludes(ec *EvalContext, args []Value) (Value, error) {
	if err := checkArity("SequenceFunctions::includes", args, 2); err != nil {
		return Value{}, err
	}
	if _, open := undeterminedIn(args...); open {
		return ec.includesUndetermined(args[0], args[1])
	}
	return boolValue(ec.ctx.includesAll(elementsOf(args[0]), elementsOf(args[1]))), nil
}

// includesUndetermined decides `includes` over an undetermined operand from the
// elements each certainly holds and the counts the model fixes, else stays open.
func (ec *EvalContext) includesUndetermined(seq1, seq2 Value) (Value, error) {
	if certainlyEmpty(seq2) {
		return boolValue(true), nil
	}
	if certainlyEmpty(seq1) && certainlyNonEmpty(seq2) {
		return boolValue(false), nil
	}
	known1, known2 := knownElementsOf(seq1), knownElementsOf(seq2)
	if seq2.Kind != ValUndetermined && ec.ctx.includesAll(known1, known2) {
		return boolValue(true), nil
	}
	if seq1.Kind != ValUndetermined && !ec.ctx.includesAll(known1, known2) {
		return boolValue(false), nil
	}
	return undeterminedOne(seq1, seq2), nil
}

// builtinSequenceExcludes is SequenceFunctions::excludes: no element of the
// second sequence is an element of the first.
func builtinSequenceExcludes(ec *EvalContext, args []Value) (Value, error) {
	if err := checkArity("SequenceFunctions::excludes", args, 2); err != nil {
		return Value{}, err
	}
	if _, open := undeterminedIn(args...); open {
		return ec.excludesUndetermined(args[0], args[1])
	}
	seq1, seq2 := elementsOf(args[0]), elementsOf(args[1])
	for _, elem := range seq2 {
		if ec.ctx.containsValue(seq1, elem) {
			return boolValue(false), nil
		}
	}
	return boolValue(true), nil
}

// excludesUndetermined decides `excludes` over an undetermined operand: false once
// both certainly hold a common element, true when either is certainly empty, else open.
func (ec *EvalContext) excludesUndetermined(seq1, seq2 Value) (Value, error) {
	if certainlyEmpty(seq1) || certainlyEmpty(seq2) {
		return boolValue(true), nil
	}
	known := knownElementsOf(seq1)
	for _, elem := range knownElementsOf(seq2) {
		if ec.ctx.containsValue(known, elem) {
			return boolValue(false), nil
		}
	}
	return undeterminedOne(seq1, seq2), nil
}

// builtinSequenceIncludesOnly is SequenceFunctions::includesOnly: each sequence
// includes the other, so they have the same elements whatever their order or
// their repetitions.
func builtinSequenceIncludesOnly(ec *EvalContext, args []Value) (Value, error) {
	if err := checkArity("SequenceFunctions::includesOnly", args, 2); err != nil {
		return Value{}, err
	}
	seq1, seq2 := elementsOf(args[0]), elementsOf(args[1])
	return boolValue(ec.ctx.includesAll(seq1, seq2) && ec.ctx.includesAll(seq2, seq1)), nil
}

// builtinSequenceEquals is SequenceFunctions::equals: the sequences have the
// same size and equal elements at every position.
func builtinSequenceEquals(ec *EvalContext, args []Value) (Value, error) {
	if err := checkArity("SequenceFunctions::equals", args, 2); err != nil {
		return Value{}, err
	}
	if args[0].Kind == ValSet && args[1].Kind == ValSet {
		return boolValue(ec.ctx.setsEqual(args[0].Set(), args[1].Set())), nil
	}
	x, y := elementsOf(args[0]), elementsOf(args[1])
	if len(x) != len(y) {
		return boolValue(false), nil
	}
	for i := range x {
		if !ec.ctx.valueEqual(x[i], y[i]) {
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
	if _, open := undeterminedIn(first, second); open {
		return undeterminedElements([]Value{first, second}), nil
	}
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
		if ec.ctx.containsValue(seq2, elem) {
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
		if !ec.ctx.containsValue(values, elem) {
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
// its last when index is one past it; any other index is out of range. An open
// operand leaves the result open once the determined ones check out.
func (ec *EvalContext) insertAt(op string, seq, values, at Value) (Value, error) {
	index, fixed, err := fixedIndex(op, at)
	if err != nil {
		return Value{}, err
	}
	if fixed {
		if err := indexWithin(op, "insertion index", index, 1, countOf(seq), 1); err != nil {
			return Value{}, err
		}
	}
	elements, whole := fixedPositionsOf(seq)
	if !fixed || !whole {
		if _, open := undeterminedIn(seq, values, at); open {
			return undeterminedOf(countOf(seq).Plus(countOf(values)), seq, values, at), nil
		}
	}
	inserted, total := elementsOf(values), spanOf(elements...)
	result := make([]Value, 0, len(elements)+len(inserted))
	result = append(result, positionsBetween(elements, 1, index-1)...)
	result = append(result, inserted...)
	result = append(result, positionsBetween(elements, index, total)...)
	return ec.sequenceOfPositions(result)
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
	count := countOf(args[0])
	start, startFixed, err := fixedIndex(subsequenceOp, args[1])
	if err != nil {
		return Value{}, err
	}
	end, endFixed := count.Exactly()
	if args[2].Kind != ValNull {
		if end, endFixed, err = fixedIndex(subsequenceOp, args[2]); err != nil {
			return Value{}, err
		}
	}
	if startFixed && start < 1 {
		return Value{}, fmt.Errorf("%w: SequenceFunctions::subsequence start index %d is outside 1..%s",
			ErrIndexOutOfRange, start, count.Upper.Text())
	}
	if startFixed && endFixed {
		if start > end {
			return ec.sequenceFrom(nil, args[0])
		}
		if err := indexWithin(subsequenceOp, "end index", end, 1, count, 0); err != nil {
			return Value{}, err
		}
		if elements, whole := fixedPositionsOf(args[0]); whole || end <= spanOf(elements...) {
			return ec.sequenceOfPositions(positionsBetween(elements, start, end), args[0])
		}
	}
	if val, open := ec.ctx.openInvocation(subsequenceOp, args...); open {
		return val, nil
	}
	return ec.sequenceFrom(elementsOf(args[0])[start-1:end], args[0])
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
	count := countOf(args[0])
	start, startFixed, err := fixedIndex(op, args[1])
	if err != nil {
		return Value{}, err
	}
	end, endFixed := start, startFixed
	if args[2].Kind != ValNull {
		if end, endFixed, err = fixedIndex(op, args[2]); err != nil {
			return Value{}, err
		}
	}
	if startFixed {
		if err := indexWithin(op, "start index", start, 1, count, 0); err != nil {
			return Value{}, err
		}
	}
	if endFixed {
		first := int64(1)
		if startFixed {
			first = start
		}
		if err := indexWithin(op, "end index", end, first, count, 0); err != nil {
			return Value{}, err
		}
	}
	elements, whole := fixedPositionsOf(args[0])
	if !startFixed || !endFixed || !whole {
		if val, open := ec.ctx.openInvocation(op, args...); open {
			return val, nil
		}
	}
	kept := make([]Value, 0, len(elements))
	kept = append(kept, positionsBetween(elements, 1, start-1)...)
	kept = append(kept, positionsBetween(elements, end+1, spanOf(elements...))...)
	return ec.sequenceOfPositions(kept, args[0])
}

// builtinSequenceHead is SequenceFunctions::head, `seq#(1)`: the first element,
// or nothing where the sequence is empty.
func builtinSequenceHead(ec *EvalContext, args []Value) (Value, error) {
	const op = "SequenceFunctions::head"
	if err := checkArity(op, args, 1); err != nil {
		return Value{}, err
	}
	elements, whole := fixedPositionsOf(args[0])
	if first, ok := positionAt(elements, 1); ok {
		return first, nil
	}
	if !whole {
		return ec.openEndOf(op, args[0])
	}
	return nullValue(), nil
}

// openEndOf is head or last of a sequence the model leaves open: one value, or
// none where the sequence may be empty.
func (ec *EvalContext) openEndOf(op string, seq Value) (Value, error) {
	count := ec.ctx.libraryResultCount(op)
	if certainlyNonEmpty(seq) {
		count = nonEmptyCount(count)
	}
	return undeterminedOf(count, seq), nil
}

// builtinSequenceTail is SequenceFunctions::tail, `subsequence(seq, 2)`: every
// element but the first.
func builtinSequenceTail(ec *EvalContext, args []Value) (Value, error) {
	const op = "SequenceFunctions::tail"
	if err := checkArity(op, args, 1); err != nil {
		return Value{}, err
	}
	elements, whole := fixedPositionsOf(args[0])
	if !whole {
		count := countOf(args[0])
		count.Lower, count.Upper = lessBound(count.Lower, 1), lessBound(count.Upper, 1)
		return undeterminedOf(count, args[0]), nil
	}
	return ec.sequenceOfPositions(positionsBetween(elements, 2, spanOf(elements...)), args[0])
}

// builtinSequenceLast is SequenceFunctions::last, `seq#(size(seq))`.
func builtinSequenceLast(ec *EvalContext, args []Value) (Value, error) {
	const op = "SequenceFunctions::last"
	if err := checkArity(op, args, 1); err != nil {
		return Value{}, err
	}
	elements, whole := fixedPositionsOf(args[0])
	if !whole {
		return ec.openEndOf(op, args[0])
	}
	if last, ok := positionAt(elements, spanOf(elements...)); ok {
		return last, nil
	}
	return nullValue(), nil
}

// builtinCollectionContains is CollectionFunctions::contains, which asks
// whether the collection's elements include the given values
// (`col.elements->includes(values)`).
func builtinCollectionContains(ec *EvalContext, args []Value) (Value, error) {
	if err := checkArity("CollectionFunctions::contains", args, 2); err != nil {
		return Value{}, err
	}
	return boolValue(ec.ctx.includesAll(elementsOf(args[0]), elementsOf(args[1]))), nil
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
	return boolValue(ec.ctx.includesAll(elementsOf(args[0]), collectionElements(col2))), nil
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

// filter is select (keep=true) and reject (keep=false); over an open collection
// one element stands for those it may hold beyond the known, decided as a whole.
func (ec *EvalContext) filter(op string, args []Value, keep bool) (Value, error) {
	if err := checkArity(op, args, 2); err != nil {
		return Value{}, err
	}
	source := args[0]
	elements := knownElementsOf(source)
	over := elements
	if mayHoldUnknown(source) {
		over = append(slices.Clone(elements), unknownElementOf(source))
	}
	body, applied, err := ec.bodyOver(op, args[1], 1, over)
	if err != nil {
		return Value{}, err
	}
	// A filter keeps the elements' type (KerML checkSelectExpressionResultSpecialization).
	if !applied {
		return ec.sequenceFrom(nil, source)
	}
	fromUnknown := semantics.CountRange(0)
	if len(over) > len(elements) {
		holds, err := ec.applyTest(op, body, over[len(elements)])
		if err != nil {
			return Value{}, err
		}
		switch {
		case holds.Kind == ValUndetermined:
			fromUnknown = semantics.Range{Lower: semantics.Bound{Known: true}, Upper: unknownCountOf(source).Upper}
		case holds.Const.Bool == keep:
			fromUnknown = unknownCountOf(source)
		}
	}
	var kept, open []Value
	for _, elem := range elements {
		holds, err := ec.applyTest(op, body, elem)
		if err != nil {
			return Value{}, err
		}
		switch {
		case holds.Kind == ValUndetermined:
			open = append(open, holds)
		case holds.Const.Bool == keep:
			kept = append(kept, elem)
		}
	}
	if len(open) > 0 || fromUnknown.MayAdmitMore(0) {
		return undeterminedFiltered(source, kept, open, fromUnknown), nil
	}
	return ec.sequenceFrom(kept, source)
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
	if selected.Kind == ValUndetermined {
		count := optionalRange()
		if certainlyNonEmpty(selected) {
			count = semantics.AssumedRange()
		}
		return undeterminedOf(count, selected), nil
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
	source := args[0]
	elements := knownElementsOf(source)
	// An element the source may hold beyond those known is mapped too, standing
	// for each of them: how many values the mapper yields per unknown element.
	over := elements
	if mayHoldUnknown(source) {
		over = append(slices.Clone(elements), unknownElementOf(source))
	}
	body, applied, err := ec.bodyOver(op, args[1], 1, over)
	if err != nil {
		return Value{}, err
	}
	// The mapper returns `Anything[0..*]`, so a mapper answering several values
	// contributes them all: the collected sequence is flat, as every KerML
	// sequence is.
	if !applied || len(over) == 0 {
		if source.Kind == ValUndetermined {
			return undeterminedCollected(source, nil, semantics.CountRange(0)), nil
		}
		return ec.emptyMapping(args[1]), nil
	}
	fromUnknown := semantics.CountRange(0)
	if len(over) > len(elements) {
		perUnknown, err := ec.applyBody(body, over[len(elements)])
		if err != nil {
			return Value{}, err
		}
		fromUnknown = unknownCountOf(source).Times(countOf(perUnknown))
	}
	var mapped, answers []Value
	var open bool
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
		open = open || val.Kind == ValUndetermined
		mapped = append(mapped, contributed...)
		answers = append(answers, val)
	}
	if open || source.Kind == ValUndetermined {
		return undeterminedCollected(source, answers, fromUnknown), nil
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
// as the library's short-circuiting `and`/`or` do not reach it either. Over an open
// collection, the elements it certainly holds decide first; an element it may hold
// beyond them is tested once, standing for each, and decides where the test's
// answer does not depend on the element.
func (ec *EvalContext) quantify(op string, args []Value, universal bool) (Value, error) {
	if err := checkArity(op, args, 2); err != nil {
		return Value{}, err
	}
	source := args[0]
	if certainlyEmpty(source) {
		return boolValue(universal), nil
	}
	elements := knownElementsOf(source)
	over := elements
	if mayHoldUnknown(source) {
		over = append(slices.Clone(elements), unknownElementOf(source))
	}
	body, applied, err := ec.bodyOver(op, args[1], 1, over)
	if err != nil {
		return Value{}, err
	}
	if !applied {
		if source.Kind == ValUndetermined {
			return undeterminedOne(source), nil
		}
		return boolValue(universal), nil
	}
	var open []Value
	for _, elem := range elements {
		holds, err := ec.applyTest(op, body, elem)
		if err != nil {
			return Value{}, err
		}
		if holds.Kind == ValUndetermined {
			open = append(open, holds)
			continue
		}
		if holds.Const.Bool != universal {
			return boolValue(!universal), nil
		}
	}
	if len(over) > len(elements) {
		holds, err := ec.applyTest(op, body, over[len(elements)])
		if err != nil {
			return Value{}, err
		}
		switch {
		case holds.Kind == ValUndetermined:
			open = append(open, holds)
		case holds.Const.Bool != universal:
			// Decides only where the source certainly holds such an element.
			if guaranteesOne(unknownCountOf(source)) {
				return boolValue(!universal), nil
			}
			open = append(open, source)
		}
	}
	if len(open) > 0 {
		return undeterminedOne(open...), nil
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

// truthOf is allTrue (universal=true) and anyTrue over Boolean elements; over an open
// collection, only the elements it certainly holds can decide.
func truthOf(op string, args []Value, universal bool) (Value, error) {
	if err := checkArity(op, args, 1); err != nil {
		return Value{}, err
	}
	if certainlyEmpty(args[0]) {
		return boolValue(universal), nil
	}
	var open []Value
	if args[0].Kind == ValUndetermined {
		open = append(open, args[0])
	}
	for _, elem := range knownElementsOf(args[0]) {
		if elem.Kind == ValUndetermined {
			open = append(open, elem)
			continue
		}
		if elem.Kind != ValConst || elem.Const.Kind != semantics.ValBool {
			return Value{}, fmt.Errorf("%w: %s requires Boolean elements, got %s", ErrTypeMismatch, op, describeValue(elem))
		}
		if elem.Const.Bool != universal {
			return boolValue(!universal), nil
		}
	}
	if len(open) > 0 {
		return undeterminedOne(open...), nil
	}
	return boolValue(universal), nil
}

// builtinNumericalSum is NumericalFunctions::sum: the sum of the collection's
// elements, and the additive identity for an empty collection, which is what
// the library's `sum0(collection, 0)` computes. The result keeps the elements'
// kind: Integer elements sum to an Integer (IntegerFunctions::sum returns
// `Integer[1]`), a Real anywhere makes the sum a Real.
func builtinNumericalSum(ec *EvalContext, args []Value) (Value, error) {
	return ec.ctx.aggregate("NumericalFunctions::sum", args, ast.OpAdd, false)
}

// builtinNumericalProduct is NumericalFunctions::product, with the
// multiplicative identity for an empty collection (`product1(collection, 1)`).
func builtinNumericalProduct(ec *EvalContext, args []Value) (Value, error) {
	return ec.ctx.aggregate("NumericalFunctions::product", args, ast.OpMul, false)
}

// builtinRealSum is RealFunctions::sum and RationalFunctions::sum, whose
// identity the library declares Real (`sum0(collection, 0.0)`): an empty
// collection sums to 0.0, not 0.
func builtinRealSum(ec *EvalContext, args []Value) (Value, error) {
	return ec.ctx.aggregate("RealFunctions::sum", args, ast.OpAdd, true)
}

// builtinRealProduct is RealFunctions::product and RationalFunctions::product
// (`product1(collection, 1.0)`).
func builtinRealProduct(ec *EvalContext, args []Value) (Value, error) {
	return ec.ctx.aggregate("RealFunctions::product", args, ast.OpMul, true)
}

// aggregate folds the collection's numeric elements with op, starting from its
// identity element: 0 for a sum, 1 for a product, a Real where asReal says so.
// A non-numeric element is reported rather than skipped or coerced. The sum of
// no elements read from a quantity-typed declaration is that quantity's zero.
func (ctx *Context) aggregate(op string, args []Value, operator ast.OperatorKind, asReal bool) (Value, error) {
	if err := checkArity(op, args, 1); err != nil {
		return Value{}, err
	}
	elements := elementsOf(args[0])
	if operator == ast.OpAdd && len(elements) == 0 {
		if unit, ok := args[0].Sequence().ElementUnit(); ok {
			return typedZero(unit, asReal), nil
		}
	}
	// A quantity carries its unit through an aggregation as through the folded
	// operator, so a collection of measured values aggregates to one.
	for _, elem := range elements {
		if elem.Kind == ValQuantity {
			return ctx.aggregateQuantities(op, elements, operator)
		}
	}
	// A Complex is a Number, so a collection holding one folds as ComplexFunctions'.
	if holdsComplex(elements) {
		return ctx.aggregateComplex(op, args[0], operator)
	}
	identity := int64(0)
	if operator == ast.OpMul {
		identity = 1
	}
	acc := semantics.Value{Kind: semantics.ValInt, Int: identity}
	if asReal {
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
// Integer 0 in that unit, or a Real one where asReal says so.
func typedZero(unit Unit, asReal bool) Value {
	num := semantics.Value{Kind: semantics.ValInt, Int: 0}
	if asReal {
		num = semantics.Value{Kind: semantics.ValReal, Real: 0}
	}
	return NewQuantityValue(&Quantity{Num: num, Unit: unit})
}

// aggregateQuantities folds a collection holding a quantity in the unit of its
// first element, as the binary operator does. A bare number is a magnitude of
// dimension one, so mixing one in reports incommensurable units. Points on a
// measurement scale have no sum or product, so a fold over them is refused.
func (ctx *Context) aggregateQuantities(op string, elements []Value, operator ast.OperatorKind) (Value, error) {
	var acc Value
	for i, elem := range elements {
		q, ok := asQuantity(elem)
		if !ok {
			return Value{}, fmt.Errorf("%w: %s requires numeric elements, got %s", ErrTypeMismatch, op, describeValue(elem))
		}
		if err := ctx.refusePoints(op, "points have no sum or product to fold; fold their differences from one point instead", q); err != nil {
			return Value{}, fmt.Errorf("%s: %w", op, err)
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
			next, err = ctx.addQuantities(operator, accQ, q)
		} else {
			next, err = ctx.scaleQuantities(operator, accQ, q)
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
func (ctx *Context) includesAll(have, want []Value) bool {
	for _, elem := range want {
		if !ctx.containsValue(have, elem) {
			return false
		}
	}
	return true
}

// containsValue reports whether elements holds a value equal to val.
func (ctx *Context) containsValue(elements []Value, val Value) bool {
	for _, elem := range elements {
		if ctx.valueEqual(elem, val) {
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
