package codegen

// SeqOp is a collection operation of the Kernel Function Library, computed by
// the generated runtime as internal/exec/runtime/collections.go computes it.
type SeqOp int

const (
	SeqInvalid SeqOp = iota
	SeqSize
	SeqIsEmpty
	SeqNotEmpty
	SeqIncludes
	SeqIncludesOnly
	SeqExcludes
	SeqEquals
	SeqSame
	SeqUnion
	SeqIntersection
	SeqIncluding
	SeqIncludingAt
	SeqExcluding
	SeqSubsequence
	SeqExcludingAt
	SeqHead
	SeqTail
	SeqLast
	SeqAllTrue
	SeqAnyTrue
	SeqSum
	SeqProduct
	// Body operations: the last parameter is a body applied per element.
	SeqSelect
	SeqSelectOne
	SeqReject
	SeqCollect
	SeqForAll
	SeqExists
	SeqReduce
	SeqMinimize
	SeqMaximize
)

// seqOpSpec describes one operation: its library name, the kinds of its
// value parameters (seq or Integer) and how many are optional.
type seqOpSpec struct {
	name     string
	params   []seqParam
	optional int
	body     int // parameters of the body, 0 for a value operation
}

type seqParam int

const (
	paramSeq seqParam = iota // a collection of the operation's element type
	paramInt                 // an Integer index
)

var seqOpSpecs = [...]seqOpSpec{
	SeqSize:         {name: "SequenceFunctions::size", params: []seqParam{paramSeq}},
	SeqIsEmpty:      {name: "SequenceFunctions::isEmpty", params: []seqParam{paramSeq}},
	SeqNotEmpty:     {name: "SequenceFunctions::notEmpty", params: []seqParam{paramSeq}},
	SeqIncludes:     {name: "SequenceFunctions::includes", params: []seqParam{paramSeq, paramSeq}},
	SeqIncludesOnly: {name: "SequenceFunctions::includesOnly", params: []seqParam{paramSeq, paramSeq}},
	SeqExcludes:     {name: "SequenceFunctions::excludes", params: []seqParam{paramSeq, paramSeq}},
	SeqEquals:       {name: "SequenceFunctions::equals", params: []seqParam{paramSeq, paramSeq}},
	SeqSame:         {name: "SequenceFunctions::same", params: []seqParam{paramSeq, paramSeq}},
	SeqUnion:        {name: "SequenceFunctions::union", params: []seqParam{paramSeq, paramSeq}},
	SeqIntersection: {name: "SequenceFunctions::intersection", params: []seqParam{paramSeq, paramSeq}},
	SeqIncluding:    {name: "SequenceFunctions::including", params: []seqParam{paramSeq, paramSeq}},
	SeqIncludingAt:  {name: "SequenceFunctions::includingAt", params: []seqParam{paramSeq, paramSeq, paramInt}},
	SeqExcluding:    {name: "SequenceFunctions::excluding", params: []seqParam{paramSeq, paramSeq}},
	SeqSubsequence:  {name: "SequenceFunctions::subsequence", params: []seqParam{paramSeq, paramInt, paramInt}, optional: 1},
	SeqExcludingAt:  {name: "SequenceFunctions::excludingAt", params: []seqParam{paramSeq, paramInt, paramInt}, optional: 1},
	SeqHead:         {name: "SequenceFunctions::head", params: []seqParam{paramSeq}},
	SeqTail:         {name: "SequenceFunctions::tail", params: []seqParam{paramSeq}},
	SeqLast:         {name: "SequenceFunctions::last", params: []seqParam{paramSeq}},
	SeqAllTrue:      {name: "ControlFunctions::allTrue", params: []seqParam{paramSeq}},
	SeqAnyTrue:      {name: "ControlFunctions::anyTrue", params: []seqParam{paramSeq}},
	SeqSum:          {name: "NumericalFunctions::sum", params: []seqParam{paramSeq}},
	SeqProduct:      {name: "NumericalFunctions::product", params: []seqParam{paramSeq}},
	SeqSelect:       {name: "ControlFunctions::select", params: []seqParam{paramSeq}, body: 1},
	SeqSelectOne:    {name: "ControlFunctions::selectOne", params: []seqParam{paramSeq}, body: 1},
	SeqReject:       {name: "ControlFunctions::reject", params: []seqParam{paramSeq}, body: 1},
	SeqCollect:      {name: "ControlFunctions::collect", params: []seqParam{paramSeq}, body: 1},
	SeqForAll:       {name: "ControlFunctions::forAll", params: []seqParam{paramSeq}, body: 1},
	SeqExists:       {name: "ControlFunctions::exists", params: []seqParam{paramSeq}, body: 1},
	SeqReduce:       {name: "ControlFunctions::reduce", params: []seqParam{paramSeq}, body: 2},
	SeqMinimize:     {name: "ControlFunctions::minimize", params: []seqParam{paramSeq}, body: 1},
	SeqMaximize:     {name: "ControlFunctions::maximize", params: []seqParam{paramSeq}, body: 1},
}

func (op SeqOp) spec() seqOpSpec { return seqOpSpecs[op] }

// Name is the qualified library function the operation computes.
func (op SeqOp) Name() string { return op.spec().name }

// String is Name without its library, the way the runtime's diagnostics name it.
func (op SeqOp) String() string {
	n := op.Name()
	for i := len(n) - 1; i >= 0; i-- {
		if n[i] == ':' {
			return n[i+1:]
		}
	}
	return n
}

// Body is the number of parameters of the body the operation applies, 0 for a
// value operation.
func (op SeqOp) Body() int { return op.spec().body }

// seqOpByName maps every library declaration the compiler computes to its
// operation: the SequenceFunctions, their CollectionFunctions counterparts and
// the ControlFunctions, plus the typed sum/product specializations. realAgg
// marks an aggregation whose identity element is Real.
func seqOpByName(fqn string) (op SeqOp, realAgg bool, ok bool) {
	for i := range seqOpSpecs {
		if SeqOp(i) != SeqInvalid && seqOpSpecs[i].name == fqn {
			return SeqOp(i), false, true
		}
	}
	switch fqn {
	case "CollectionFunctions::size":
		return SeqSize, false, true
	case "CollectionFunctions::isEmpty":
		return SeqIsEmpty, false, true
	case "CollectionFunctions::notEmpty":
		return SeqNotEmpty, false, true
	case "CollectionFunctions::contains", "CollectionFunctions::containsAll":
		return SeqIncludes, false, true
	case "CollectionFunctions::head":
		return SeqHead, false, true
	case "CollectionFunctions::tail":
		return SeqTail, false, true
	case "CollectionFunctions::last":
		return SeqLast, false, true
	case "IntegerFunctions::sum":
		return SeqSum, false, true
	case "IntegerFunctions::product":
		return SeqProduct, false, true
	case "RealFunctions::sum", "RationalFunctions::sum":
		return SeqSum, true, true
	case "RealFunctions::product", "RationalFunctions::product":
		return SeqProduct, true, true
	}
	return SeqInvalid, false, false
}
