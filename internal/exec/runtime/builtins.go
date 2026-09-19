package runtime

import (
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// builtins maps the fully-qualified name of a Kernel Function Library function
// to its implementation. These are the functions whose parameters or results are
// collections, or whose arguments are bodies rather than values, which the
// scalar library registry (library_functions.go) cannot express: it applies to
// semantics.Values, while these need runtime values and an evaluation context to
// call a body with. Dispatch is by the declaration a call resolves to, never by
// its bare name: `seq->size()` is SequenceFunctions::size only where imported.
var builtins map[string]builtinFunc

func init() {
	builtins = map[string]builtinFunc{
		// SequenceFunctions: the operations on general sequences of values.
		"SequenceFunctions::#":            builtinSequenceIndex,
		"SequenceFunctions::size":         builtinSequenceSize,
		"SequenceFunctions::isEmpty":      builtinSequenceIsEmpty,
		"SequenceFunctions::notEmpty":     builtinSequenceNotEmpty,
		"SequenceFunctions::includes":     builtinSequenceIncludes,
		"SequenceFunctions::includesOnly": builtinSequenceIncludesOnly,
		"SequenceFunctions::excludes":     builtinSequenceExcludes,
		"SequenceFunctions::equals":       builtinSequenceEquals,
		"SequenceFunctions::same":         builtinSequenceSame,
		"SequenceFunctions::union":        builtinSequenceUnion,
		"SequenceFunctions::intersection": builtinSequenceIntersection,
		"SequenceFunctions::including":    builtinSequenceIncluding,
		"SequenceFunctions::includingAt":  builtinSequenceIncludingAt,
		"SequenceFunctions::excluding":    builtinSequenceExcluding,
		"SequenceFunctions::subsequence":  builtinSequenceSubsequence,
		"SequenceFunctions::excludingAt":  builtinSequenceExcludingAt,
		"SequenceFunctions::head":         builtinSequenceHead,
		"SequenceFunctions::tail":         builtinSequenceTail,
		"SequenceFunctions::last":         builtinSequenceLast,

		// CollectionFunctions: the same operations on a Collection, each defined
		// by the library as the sequence operation over the collection's
		// elements.
		"CollectionFunctions::#":           builtinCollectionIndex,
		"CollectionFunctions::array#":      builtinArrayIndex,
		"CollectionFunctions::size":        overCollectionElements(builtinSequenceSize),
		"CollectionFunctions::isEmpty":     overCollectionElements(builtinSequenceIsEmpty),
		"CollectionFunctions::notEmpty":    overCollectionElements(builtinSequenceNotEmpty),
		"CollectionFunctions::contains":    overCollectionElements(builtinCollectionContains),
		"CollectionFunctions::containsAll": overCollectionElements(builtinCollectionContainsAll),
		"CollectionFunctions::head":        overCollectionElements(builtinSequenceHead),
		"CollectionFunctions::tail":        overCollectionElements(builtinSequenceTail),
		"CollectionFunctions::last":        overCollectionElements(builtinSequenceLast),

		// ControlFunctions: the operations whose argument is a body the
		// operation itself decides the evaluation of.
		"ControlFunctions::select":    builtinControlSelect,
		"ControlFunctions::selectOne": builtinControlSelectOne,
		"ControlFunctions::reject":    builtinControlReject,
		"ControlFunctions::collect":   builtinControlCollect,
		"ControlFunctions::forAll":    builtinControlForAll,
		"ControlFunctions::exists":    builtinControlExists,
		"ControlFunctions::allTrue":   builtinControlAllTrue,
		"ControlFunctions::anyTrue":   builtinControlAnyTrue,
		"ControlFunctions::reduce":    builtinControlReduce,
		"ControlFunctions::minimize":  builtinControlMinimize,
		"ControlFunctions::maximize":  builtinControlMaximize,

		// NumericalFunctions::sum and ::product, and the specializations that
		// fix the identity element of an empty aggregation: sum of Integers is
		// `sum0(collection, 0)` and product is `product1(collection, 1)`. The
		// result keeps the elements' kind, so IntegerFunctions::sum of Integers
		// is an Integer, as its `return : Integer[1]` declares.
		// ComplexFunctions and VectorFunctions declare sum and product too, over
		// values this runtime has no representation of; they are left out rather
		// than answered wrongly (docs/project/spec-compliance.md).
		"NumericalFunctions::sum":     builtinNumericalSum,
		"NumericalFunctions::product": builtinNumericalProduct,
		// IntegerFunctions::'..', the range, whose result the library declares
		// `Integer[0..*]`: an ordered sequence, not a value kind of its own.
		"IntegerFunctions::..": rangeBuiltin("IntegerFunctions::'..'"),

		"IntegerFunctions::sum":      builtinNumericalSum,
		"IntegerFunctions::product":  builtinNumericalProduct,
		"RationalFunctions::sum":     builtinRealSum,
		"RationalFunctions::product": builtinRealProduct,
		"RealFunctions::sum":         builtinRealSum,
		"RealFunctions::product":     builtinRealProduct,
	}
	registerNamedOperatorBuiltins()
}

// builtinFor answers the implementation of a library-declared collection
// function; a calc the model declares under the same name is never answered here.
func (ctx *Context) builtinFor(sym *symbols.Symbol) (builtinFunc, bool) {
	if sym == nil || !ctx.libraryDeclared(sym) {
		return nil, false
	}
	fn, ok := builtins[ctx.qualifiedSymbolName(sym)]
	return fn, ok
}

// boolValue wraps a Boolean result.
func boolValue(b bool) Value {
	return Value{
		Kind: ValConst,
		Const: semantics.Value{
			Kind: semantics.ValBool,
			Bool: b,
		},
	}
}
