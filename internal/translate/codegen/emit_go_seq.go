package codegen

import (
	"fmt"
	"strconv"
	"strings"
)

// goSeqPrelude is the collection runtime of a generated Go program: the same
// shapes, budget and operations as the C one, over a generic element type.
const goSeqPrelude = `
type sysmlShape uint8

const (
	sysmlNull sysmlShape = iota
	sysmlOne
	sysmlMany
)

type sysmlElem interface {
	sysmlInt | float64 | bool | sysmlNum
}

// sysmlSeq is a collection value: null, one bare value, or a sequence.
type sysmlSeq[T sysmlElem] struct {
	shape sysmlShape
	data  []T
}

var (
	sysmlElements    int64
	sysmlMaxElements int64 = sysmlDefaultMaxElements
)

// sysmlCharge counts n materialized elements against the budget a statement
// releases at its end.
func sysmlCharge(n int64) {
	sysmlElements += n
	if sysmlElements > sysmlMaxElements || sysmlElements < 0 {
		sysmlFailf("collection element limit exceeded (%d elements; raise OPENSYSML_MAX_ELEMENTS to allow more)", sysmlMaxElements)
	}
}

func sysmlMultFail(where string, n, lo, hi int64) {
	if n < lo {
		sysmlFailf("%s: multiplicity violation: %d value(s) bound to a feature with multiplicity lower bound %d", where, n, lo)
	}
	sysmlFailf("%s: multiplicity violation: %d value(s) bound to a feature with multiplicity upper bound %d", where, n, hi)
}

func sysmlNullSeq[T sysmlElem]() sysmlSeq[T] { return sysmlSeq[T]{} }

func sysmlOneSeq[T sysmlElem](v T) sysmlSeq[T] { return sysmlSeq[T]{sysmlOne, []T{v}} }

// sysmlManySeq is a sequence of n elements, charged to the budget.
func sysmlManySeq[T sysmlElem](n int64) sysmlSeq[T] {
	sysmlCharge(n)
	return sysmlSeq[T]{sysmlMany, make([]T, n)}
}

func sysmlConcat[T sysmlElem](parts ...sysmlSeq[T]) sysmlSeq[T] {
	var total int64
	for _, p := range parts {
		total += int64(len(p.data))
	}
	r := sysmlManySeq[T](total)
	k := 0
	for _, p := range parts {
		k += copy(r.data[k:], p.data)
	}
	return r
}

// sysmlOneOf is the one value bound to a [1] feature at where.
func sysmlOneOf[T sysmlElem](s sysmlSeq[T], where string) T {
	if len(s.data) != 1 {
		sysmlMultFail(where, int64(len(s.data)), 1, 1)
	}
	return s.data[0]
}

// sysmlDescribe is the interpreter's description of a collection by shape;
// one holding one element is described as one.
func sysmlDescribe(shape sysmlShape, bare bool, one string) string {
	switch {
	case shape == sysmlNull:
		return "null"
	case shape == sysmlOne:
		return one
	case bare:
		return "sequence"
	}
	return "a sequence"
}

// sysmlScalar is the one scalar an operator needs; format takes the shape
// found and, when other is non-empty, the other operand's description.
func sysmlScalar[T sysmlElem](s sysmlSeq[T], format string, bare bool, other string) T {
	if s.shape != sysmlOne {
		if other == "" {
			sysmlFailf(format, sysmlDescribe(s.shape, bare, ""))
		}
		sysmlFailf(format, sysmlDescribe(s.shape, bare, ""), other)
	}
	return s.data[0]
}

func sysmlCheck[T sysmlElem](s sysmlSeq[T], lo, hi int64, where string) sysmlSeq[T] {
	n := int64(len(s.data))
	if n < lo || (hi >= 0 && n > hi) {
		sysmlMultFail(where, n, lo, hi)
	}
	return s
}

// sysmlElemKind is the interpreter's description of an element's type.
func sysmlElemKind[T sysmlElem](v T) string {
	switch v := any(v).(type) {
	case sysmlInt:
		return "an Integer"
	case float64:
		return "a Real"
	case sysmlNum:
		if v.real {
			return "a Real"
		}
		return "an Integer"
	}
	return "a Boolean"
}

// sysmlUnique refuses the first element of s equal to an earlier one, as a
// write to a unique feature at where does.
func sysmlUnique[T sysmlElem](s sysmlSeq[T], where string) sysmlSeq[T] {
	seen := make(map[any]int, len(s.data))
	for i, v := range s.data {
		k := sysmlKey(v)
		if first, dup := seen[k]; dup {
			sysmlFailf("%s: uniqueness violation: %s (%s) is written at positions %d and %d of a unique feature", where, sysmlFormat(v), sysmlElemKind(v), first+1, i+1)
		}
		seen[k] = i
	}
	return s
}

// sysmlBigKey keys an Integer beyond int64 by its decimal digits.
type sysmlBigKey string

// sysmlKey is v as a map key: equal elements, and only they, share one.
func sysmlKey[T sysmlElem](v T) any {
	if n, ok := any(v).(sysmlNum); ok {
		// A whole Real keys as the Integer it equals.
		if n.real && (n.r != math.Trunc(n.r) || math.IsInf(n.r, 0)) {
			return n.r
		}
		if n.real {
			i, _ := new(big.Float).SetFloat64(n.r).Int(nil)
			return sysmlKey(sysmlWrap(i))
		}
		return sysmlKey(n.i)
	}
	if i, ok := any(v).(sysmlInt); ok {
		if i.big != nil {
			return sysmlBigKey(i.big.String())
		}
		return i.small
	}
	return v
}

// sysmlElemEq is the '==' of two elements.
func sysmlElemEq[T sysmlElem](a, b T) bool {
	switch x := any(a).(type) {
	case sysmlInt:
		return sysmlICmp(x, any(b).(sysmlInt)) == 0
	case sysmlNum:
		return sysmlNCmp(x, any(b).(sysmlNum)) == 0
	}
	return a == b
}

func sysmlAtLeastSeq(s sysmlSeq[sysmlInt], lo int64, typ string) sysmlSeq[sysmlInt] {
	for _, v := range s.data {
		sysmlAtLeast(v, lo, typ)
	}
	return s
}

// sysmlEq is the '==' of collections: same shape, same elements in order;
// every empty collection is null, whatever its shape.
func sysmlEq[T sysmlElem](a, b sysmlSeq[T]) bool {
	if len(a.data) == 0 || len(b.data) == 0 {
		return len(a.data) == 0 && len(b.data) == 0
	}
	return a.shape == b.shape && sysmlEquals(a, b)
}

// sysmlEquals is SequenceFunctions::equals and same: the elements in order,
// whatever the shape.
func sysmlEquals[T sysmlElem](a, b sysmlSeq[T]) bool {
	if len(a.data) != len(b.data) {
		return false
	}
	for i := range a.data {
		if !sysmlElemEq(a.data[i], b.data[i]) {
			return false
		}
	}
	return true
}

// sysmlPos is an index as a position: one beyond int64 addresses none.
func sysmlPos(i sysmlInt, op string) int64 {
	if i.big != nil {
		sysmlFailf("index out of range: %s: index %s addresses no position", op, i)
	}
	return i.small
}

func sysmlIndex[T sysmlElem](s sysmlSeq[T], at sysmlInt) T {
	i := sysmlPos(at, "sequence index")
	if i < 1 || i > int64(len(s.data)) {
		sysmlFailf("index out of range: sequence index %d is outside 1..%d", i, len(s.data))
	}
	return s.data[i-1]
}

func sysmlContains[T sysmlElem](s sysmlSeq[T], v T) bool {
	for _, e := range s.data {
		if sysmlElemEq(e, v) {
			return true
		}
	}
	return false
}

func sysmlIncludes[T sysmlElem](a, b sysmlSeq[T]) bool {
	for _, v := range b.data {
		if !sysmlContains(a, v) {
			return false
		}
	}
	return true
}

func sysmlIncludesOnly[T sysmlElem](a, b sysmlSeq[T]) bool {
	return sysmlIncludes(a, b) && sysmlIncludes(b, a)
}

func sysmlExcludes[T sysmlElem](a, b sysmlSeq[T]) bool {
	for _, v := range b.data {
		if sysmlContains(a, v) {
			return false
		}
	}
	return true
}

// sysmlSift is the elements of a that b holds (keep) or does not, in a's order.
func sysmlSift[T sysmlElem](a, b sysmlSeq[T], keep bool) sysmlSeq[T] {
	var n int64
	for _, v := range a.data {
		if sysmlContains(b, v) == keep {
			n++
		}
	}
	r := sysmlManySeq[T](n)
	k := 0
	for _, v := range a.data {
		if sysmlContains(b, v) == keep {
			r.data[k] = v
			k++
		}
	}
	return r
}

func sysmlIncludingAt[T sysmlElem](a, b sysmlSeq[T], at sysmlInt) sysmlSeq[T] {
	i := sysmlPos(at, "SequenceFunctions::includingAt")
	n := int64(len(a.data))
	if i < 1 || i > n+1 {
		sysmlFailf("index out of range: SequenceFunctions::includingAt insertion index %d is outside 1..%d", i, n+1)
	}
	return sysmlConcat(sysmlSeq[T]{sysmlMany, a.data[:i-1]}, b, sysmlSeq[T]{sysmlMany, a.data[i-1:]})
}

func sysmlSubsequence[T sysmlElem](s sysmlSeq[T], from, to sysmlInt, hasEnd bool) sysmlSeq[T] {
	start := sysmlPos(from, "SequenceFunctions::subsequence")
	n := int64(len(s.data))
	end := n
	if hasEnd {
		end = sysmlPos(to, "SequenceFunctions::subsequence")
	}
	if start < 1 {
		sysmlFailf("index out of range: SequenceFunctions::subsequence start index %d is outside 1..%d", start, n)
	}
	if start > end {
		return sysmlManySeq[T](0)
	}
	if end > n {
		sysmlFailf("index out of range: SequenceFunctions::subsequence end index %d is outside 1..%d", end, n)
	}
	return sysmlConcat(sysmlSeq[T]{sysmlMany, s.data[start-1 : end]})
}

func sysmlExcludingAt[T sysmlElem](s sysmlSeq[T], from, to sysmlInt, hasEnd bool) sysmlSeq[T] {
	start := sysmlPos(from, "SequenceFunctions::excludingAt")
	end := start
	if hasEnd {
		end = sysmlPos(to, "SequenceFunctions::excludingAt")
	}
	n := int64(len(s.data))
	if start < 1 || start > n {
		sysmlFailf("index out of range: SequenceFunctions::excludingAt start index %d is outside 1..%d", start, n)
	}
	if end < start || end > n {
		sysmlFailf("index out of range: SequenceFunctions::excludingAt end index %d is outside %d..%d", end, start, n)
	}
	return sysmlConcat(sysmlSeq[T]{sysmlMany, s.data[:start-1]}, sysmlSeq[T]{sysmlMany, s.data[end:]})
}

func sysmlHead[T sysmlElem](s sysmlSeq[T]) sysmlSeq[T] {
	if len(s.data) == 0 {
		return sysmlNullSeq[T]()
	}
	return sysmlSeq[T]{sysmlOne, s.data[:1]}
}

func sysmlLast[T sysmlElem](s sysmlSeq[T]) sysmlSeq[T] {
	if len(s.data) == 0 {
		return sysmlNullSeq[T]()
	}
	return sysmlSeq[T]{sysmlOne, s.data[len(s.data)-1:]}
}

func sysmlTail[T sysmlElem](s sysmlSeq[T]) sysmlSeq[T] {
	if len(s.data) == 0 {
		return sysmlManySeq[T](0)
	}
	return sysmlConcat(sysmlSeq[T]{sysmlMany, s.data[1:]})
}

// sysmlPush grows a sequence collected element by element, charged as it grows.
func sysmlPush[T sysmlElem](r *sysmlSeq[T], v T) {
	sysmlCharge(1)
	r.data = append(r.data, v)
}

func sysmlAppend[T sysmlElem](r *sysmlSeq[T], s sysmlSeq[T]) {
	for _, v := range s.data {
		sysmlPush(r, v)
	}
}

// sysmlRange is lo..hi, whose count the element budget refuses before
// anything is materialized.
func sysmlRange(lo, hi sysmlInt) sysmlSeq[sysmlInt] {
	if sysmlICmp(lo, hi) > 0 {
		return sysmlManySeq[sysmlInt](0)
	}
	count := sysmlAdd(sysmlSub(hi, lo), sysmlI(1))
	n := int64(math.MaxInt64)
	if count.big == nil {
		n = count.small
	}
	sysmlRangeCharge(n)
	r := sysmlSeq[sysmlInt]{sysmlMany, make([]sysmlInt, n)}
	v := lo
	for i := range r.data {
		r.data[i] = v
		v = sysmlAdd(v, sysmlI(1))
	}
	return r
}

// sysmlRangeCharge spends a step, then an element, per element of an
// n-element range, failing at the element where the interpreter's range does.
func sysmlRangeCharge(n int64) {
	if room := sysmlMaxSteps - sysmlSteps; n > room && room <= sysmlMaxElements-sysmlElements {
		sysmlStepFail()
	}
	sysmlCharge(n)
	sysmlSteps += n
}

// sysmlWiden is the Real copy of a collection of Integers or numbers, charged
// like any other materialized collection.
func sysmlWiden[T sysmlElem](s sysmlSeq[T]) sysmlSeq[float64] {
	sysmlCharge(int64(len(s.data)))
	r := sysmlSeq[float64]{s.shape, make([]float64, len(s.data))}
	for i, v := range s.data {
		switch v := any(v).(type) {
		case sysmlInt:
			r.data[i] = sysmlToReal(v)
		case sysmlNum:
			r.data[i] = v.toReal()
		}
	}
	return r
}

// sysmlNums is the number copy of a collection of Integers or Reals, each
// element keeping its kind.
func sysmlNums[T sysmlElem](s sysmlSeq[T]) sysmlSeq[sysmlNum] {
	sysmlCharge(int64(len(s.data)))
	r := sysmlSeq[sysmlNum]{s.shape, make([]sysmlNum, len(s.data))}
	for i, v := range s.data {
		switch v := any(v).(type) {
		case sysmlInt:
			r.data[i] = sysmlNI(v)
		case float64:
			r.data[i] = sysmlNR(v)
		}
	}
	return r
}

// sysmlNFold folds numbers from the Integer identity, by Integer arithmetic
// while both operands hold Integers and by Real arithmetic once one does not.
func sysmlNFold(s sysmlSeq[sysmlNum], identity int64, ints func(a, b sysmlInt) sysmlInt, reals func(a, b float64) float64, op string) sysmlNum {
	acc := sysmlNI(sysmlI(identity))
	for _, v := range s.data {
		if !acc.real && !v.real {
			acc = sysmlNI(ints(acc.i, v.i))
			continue
		}
		acc = sysmlNR(reals(acc.toReal(), v.toReal()))
		if math.IsInf(acc.r, 0) {
			sysmlFailf("arithmetic overflow: %s is not a finite Real", op)
		}
	}
	return acc
}

func sysmlISum(s sysmlSeq[sysmlInt], op string) sysmlInt {
	var acc sysmlInt
	for _, v := range s.data {
		acc = sysmlAdd(acc, v)
	}
	return acc
}

func sysmlIProduct(s sysmlSeq[sysmlInt], op string) sysmlInt {
	acc := sysmlI(1)
	for _, v := range s.data {
		acc = sysmlMul(acc, v)
	}
	return acc
}

func sysmlRSum(s sysmlSeq[float64], op string) float64 {
	var acc float64
	for _, v := range s.data {
		acc += v
		if math.IsInf(acc, 0) {
			sysmlFailf("arithmetic overflow: %s is not a finite Real", op)
		}
	}
	return acc
}

func sysmlRProduct(s sysmlSeq[float64], op string) float64 {
	acc := 1.0
	for _, v := range s.data {
		acc *= v
		if math.IsInf(acc, 0) {
			sysmlFailf("arithmetic overflow: %s is not a finite Real", op)
		}
	}
	return acc
}

func sysmlAllTrue(s sysmlSeq[bool]) bool {
	for _, v := range s.data {
		if !v {
			return false
		}
	}
	return true
}

func sysmlAnyTrue(s sysmlSeq[bool]) bool {
	for _, v := range s.data {
		if v {
			return true
		}
	}
	return false
}

func sysmlFormatSeq[T sysmlElem](s sysmlSeq[T]) string {
	switch s.shape {
	case sysmlNull:
		return "null"
	case sysmlOne:
		return sysmlFormat(s.data[0])
	}
	parts := make([]string, len(s.data))
	for i, v := range s.data {
		parts[i] = sysmlFormat(v)
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

// sysmlParseSeq parses a collection argument in the notation the interpreter
// reads and prints: null, a bare value, (a, b, ...) with (a) a bare value,
// [a, b, ...].
func sysmlParseSeq[T sysmlElem](s, name string, elem func(string, string) T) sysmlSeq[T] {
	if s == "null" {
		return sysmlNullSeq[T]()
	}
	if s == "" || (s[0] != '(' && s[0] != '[') {
		return sysmlOneSeq(elem(s, name))
	}
	closer := byte(')')
	if s[0] == '[' {
		closer = ']'
	}
	if len(s) < 2 || s[len(s)-1] != closer {
		fmt.Fprintf(os.Stderr, "argument %s: %s is not a sequence (a, b, ...)\n", name, s)
		os.Exit(2)
	}
	body := s[1 : len(s)-1]
	r := sysmlSeq[T]{sysmlMany, []T{}}
	if body == "" {
		return r
	}
	toks := strings.Split(body, ",")
	if s[0] == '(' && len(toks) == 1 {
		return sysmlOneSeq(elem(strings.TrimSpace(body), name))
	}
	for _, tok := range toks {
		r.data = append(r.data, elem(strings.TrimSpace(tok), name))
	}
	return r
}
`

// goElem is the Go element type of a collection.
func goElem(t Type) string { return goType(t.Elem()) }

// goSeqType is the Go type of a collection of t's elements.
func goSeqType(t Type) string { return "sysmlSeq[" + goElem(t) + "]" }

// seqExpr emits a collection expression; ok is false for a scalar node.
func (e *goEmitter) seqExpr(x Expr) (string, bool) {
	switch x := x.(type) {
	case NullLit:
		return fmt.Sprintf("sysmlNullSeq[%s]()", goElem(x.T)), true
	case SeqLit:
		return e.seqLit(x), true
	case ToMany:
		return fmt.Sprintf("sysmlOneSeq[%s](%s)", goElem(x.X.Type()), e.expr(x.X)), true
	case ToOne:
		if x.Where != "" {
			return fmt.Sprintf("sysmlOneOf(%s, %s)", e.expr(x.X), strconv.Quote(x.Where)), true
		}
		other := `""`
		if x.Other != nil {
			other = fmt.Sprintf("sysmlDescribe(%s.shape, %t, %s)", e.expr(x.Other), x.Bare, strconv.Quote(x.OtherOne))
		}
		return fmt.Sprintf("sysmlScalar(%s, %s, %t, %s)", e.expr(x.X), strconv.Quote(x.Fail), x.Bare, other), true
	case Let:
		return fmt.Sprintf("func() %s { %s := %s; _ = %s; return %s }()", goType(x.In.Type()), goLocal(x.Name), e.expr(x.Value), goLocal(x.Name), e.expr(x.In)), true
	case Checked:
		return e.checked(x), true
	case Coalesce:
		return fmt.Sprintf("func() %s { l := %s; if len(l.data) != 0 { return l }; return %s }()", goSeqType(x.T), e.expr(x.L), e.expr(x.R)), true
	case SeqEq:
		eq := fmt.Sprintf("sysmlEq(%s, %s)", e.expr(x.L), e.expr(x.R))
		if x.Neq {
			return "(!" + eq + ")", true
		}
		return eq, true
	case Index:
		return fmt.Sprintf("sysmlIndex(%s, %s)", e.expr(x.Seq), e.expr(x.I)), true
	case RangeExpr:
		return fmt.Sprintf("sysmlRange(%s, %s)", e.expr(x.Lo), e.expr(x.Hi)), true
	case SeqCall:
		v := make([]string, len(x.Args))
		for i, a := range x.Args {
			v[i] = e.expr(a)
		}
		return e.seqCall(x, v), true
	case Fold:
		return e.fold(x), true
	case Framed:
		return fmt.Sprintf("func() %s { sysmlEnter(); r := %s; sysmlLeave(); return r }()", goType(x.Type()), e.expr(x.X)), true
	case Sampled:
		return fmt.Sprintf("func() %s { %s; return %s }()", goType(x.Type()), e.sample(x.S), e.expr(x.In)), true
	}
	return "", false
}

// sample declares a Sample's two variables and fills them one frame deeper; the domain is an
// argument, evaluated before the frame. Each pair is the three elements a collected SamplePair is.
func (e *goEmitter) sample(s Sample) string {
	dom, rng := goLocal(s.Dom), goLocal(s.Rng)
	x := goLocal(s.Body.Params[0].Name)
	var b strings.Builder
	fmt.Fprintf(&b, "var %s = %s{sysmlMany, nil}; var %s = %s{sysmlMany, nil}; ", dom, goSeqType(s.DomType()), rng, goSeqType(s.RngType()))
	st := s.Steps
	fmt.Fprintf(&b, "{ s := %s; sysmlEnter(); sysmlStep(%d); for _, %s := range s.data { sysmlStep(%d); y := %s; sysmlStep(%d); sysmlPush(&%s, %s); sysmlPush(&%s, y); sysmlCharge(1) }; sysmlStep(%d); sysmlLeave() }",
		e.expr(s.Seq), st.Enter, x, st.Before, e.expr(s.Body.Body), st.After, dom, x, rng, st.Done)
	return b.String()
}

// seqLit concatenates the operands' elements, evaluated left to right.
func (e *goEmitter) seqLit(x SeqLit) string {
	elem := goElem(x.T)
	if len(x.Elems) == 0 {
		return fmt.Sprintf("sysmlManySeq[%s](0)", elem)
	}
	parts := make([]string, len(x.Elems))
	for i, el := range x.Elems {
		if el.Type().Scalar() {
			parts[i] = e.expr(ToMany{X: el})
		} else {
			parts[i] = e.expr(el)
		}
	}
	return fmt.Sprintf("sysmlConcat[%s](%s)", elem, strings.Join(parts, ", "))
}

// checked binds a collection: multiplicity first, then the elements' range,
// then their uniqueness.
func (e *goEmitter) checked(x Checked) string {
	v := e.expr(x.X)
	if x.M != MultAny {
		v = fmt.Sprintf("sysmlCheck(%s, %d, %d, %s)", v, x.M.Lower, x.M.Upper, strconv.Quote(x.Where))
	}
	if x.R != RangeAny {
		v = fmt.Sprintf("sysmlAtLeastSeq(%s, %d, %q)", v, x.R.Lower(), x.R.String())
	}
	if x.Unique {
		v = fmt.Sprintf("sysmlUnique(%s, %s)", v, strconv.Quote(x.Where))
	}
	return v
}

// seqCall applies a value operation to its evaluated operands.
func (e *goEmitter) seqCall(x SeqCall, v []string) string {
	switch x.Op {
	case SeqSize:
		return fmt.Sprintf("sysmlI(int64(len(%s.data)))", v[0])
	case SeqIsEmpty:
		return fmt.Sprintf("(len(%s.data) == 0)", v[0])
	case SeqNotEmpty:
		return fmt.Sprintf("(len(%s.data) != 0)", v[0])
	case SeqIncludes:
		return fmt.Sprintf("sysmlIncludes(%s, %s)", v[0], v[1])
	case SeqIncludesOnly:
		return fmt.Sprintf("sysmlIncludesOnly(%s, %s)", v[0], v[1])
	case SeqExcludes:
		return fmt.Sprintf("sysmlExcludes(%s, %s)", v[0], v[1])
	case SeqEquals, SeqSame:
		return fmt.Sprintf("sysmlEquals(%s, %s)", v[0], v[1])
	case SeqUnion, SeqIncluding:
		return fmt.Sprintf("sysmlConcat(%s, %s)", v[0], v[1])
	case SeqIntersection:
		return fmt.Sprintf("sysmlSift(%s, %s, true)", v[0], v[1])
	case SeqExcluding:
		return fmt.Sprintf("sysmlSift(%s, %s, false)", v[0], v[1])
	case SeqIncludingAt:
		return fmt.Sprintf("sysmlIncludingAt(%s, %s, %s)", v[0], v[1], v[2])
	case SeqSubsequence, SeqExcludingAt:
		name := "sysmlSubsequence"
		if x.Op == SeqExcludingAt {
			name = "sysmlExcludingAt"
		}
		end, has := "0", "false"
		if len(v) == 3 {
			end, has = v[2], "true"
		}
		return fmt.Sprintf("%s(%s, %s, %s, %s)", name, v[0], v[1], end, has)
	case SeqHead:
		return fmt.Sprintf("sysmlHead(%s)", v[0])
	case SeqTail:
		return fmt.Sprintf("sysmlTail(%s)", v[0])
	case SeqLast:
		return fmt.Sprintf("sysmlLast(%s)", v[0])
	case SeqAllTrue:
		return fmt.Sprintf("sysmlAllTrue(%s)", v[0])
	case SeqAnyTrue:
		return fmt.Sprintf("sysmlAnyTrue(%s)", v[0])
	case SeqSum, SeqProduct:
		if x.T == TypeNum {
			if x.Op == SeqSum {
				return fmt.Sprintf("sysmlNFold(%s, 0, sysmlAdd, func(a, b float64) float64 { return a + b }, %q)", v[0], x.Op.Name())
			}
			return fmt.Sprintf("sysmlNFold(%s, 1, sysmlMul, func(a, b float64) float64 { return a * b }, %q)", v[0], x.Op.Name())
		}
		fn := map[SeqOp]string{SeqSum: "Sum", SeqProduct: "Product"}[x.Op]
		prefix := "I"
		if x.T == TypeReal {
			prefix = "R"
		}
		return fmt.Sprintf("sysml%s%s(%s, %q)", prefix, fn, v[0], x.Op.Name())
	}
	e.err = fmt.Errorf("codegen: Go emitter has no case for collection operation %s", x.Op)
	return "0"
}

// fold emits a body operation as a loop in a closure; the body's parameters
// and locals are closure-local variables.
func (e *goEmitter) fold(x Fold) string {
	elem := goElem(x.Seq.Type())
	var b strings.Builder
	fmt.Fprintf(&b, "func() %s { s := %s; ", goType(x.T), e.expr(x.Seq))
	if x.Steps > 0 {
		fmt.Fprintf(&b, "sysmlStep(%d); ", x.Steps)
	}
	// bind opens the loop body with the parameters bound to args.
	bind := func(args ...string) string {
		var s strings.Builder
		for _, a := range args {
			fmt.Fprintf(&s, "_ = %s; ", a)
		}
		for i, p := range x.Body.Params {
			fmt.Fprintf(&s, "%s := %s; _ = %s; ", goLocal(p.Name), args[i], goLocal(p.Name))
		}
		return s.String()
	}
	body := e.expr(x.Body.Body)
	switch x.Op {
	case SeqSelect, SeqReject:
		fmt.Fprintf(&b, "r := sysmlSeq[%s]{sysmlMany, make([]%s, 0, len(s.data))}; ", elem, elem)
		fmt.Fprintf(&b, "for _, v := range s.data { %sif %s == %t { r.data = append(r.data, v) } }; ", bind("v"), body, x.Op == SeqSelect)
		fmt.Fprintf(&b, "sysmlCharge(int64(len(r.data))); return r }()")
	case SeqSelectOne:
		fmt.Fprintf(&b, "for i, v := range s.data { %sif %s { return sysmlSeq[%s]{sysmlOne, s.data[i : i+1]} } }; ", bind("v"), body, elem)
		fmt.Fprintf(&b, "return sysmlNullSeq[%s]() }()", elem)
	case SeqCollect:
		add := fmt.Sprintf("sysmlPush(&r, %s)", body)
		if x.Body.Body.Type().Many() {
			add = fmt.Sprintf("sysmlAppend(&r, %s)", body)
		}
		fmt.Fprintf(&b, "r := sysmlSeq[%s]{sysmlMany, nil}; for _, v := range s.data { %s%s }; return r }()", goElem(x.T), bind("v"), add)
	case SeqForAll, SeqExists:
		universal := x.Op == SeqForAll
		fmt.Fprintf(&b, "for _, v := range s.data { %sif %s != %t { return %t } }; return %t }()", bind("v"), body, universal, !universal, universal)
	case SeqReduce:
		fmt.Fprintf(&b, "if len(s.data) == 0 { return sysmlNullSeq[%s]() }; a := s.data[0]; ", elem)
		fmt.Fprintf(&b, "for _, v := range s.data[1:] { %sa = %s }; return sysmlOneSeq(a) }()", bind("a", "v"), body)
	case SeqMinimize, SeqMaximize:
		less := "<"
		if x.Op == SeqMaximize {
			less = ">"
		}
		fmt.Fprintf(&b, "if len(s.data) == 0 { sysmlFail(%s) }; var r %s; ", strconv.Quote("multiplicity violation: "+x.Op.Name()+" requires a collection of at least one element"), goType(x.T))
		better := fmt.Sprintf("k %s r", less)
		switch x.T {
		case TypeInt:
			better = fmt.Sprintf("sysmlICmp(k, r) %s 0", less)
		case TypeNum:
			better = fmt.Sprintf("sysmlNCmp(k, r) %s 0", less)
		}
		fmt.Fprintf(&b, "for i, v := range s.data { %sk := %s; if i == 0 || %s { r = k } }; return r }()", bind("v"), body, better)
	default:
		e.err = fmt.Errorf("codegen: Go emitter has no case for body operation %s", x.Op)
	}
	return b.String()
}

// forEach iterates a collection; a bare scalar is not iterable.
func (e *goEmitter) forEach(s ForEach) {
	elem := s.Seq.Type().Elem()
	e.linef("{")
	e.indent++
	e.linef("s := %s", e.expr(s.Seq))
	e.linef("if s.shape == sysmlOne {")
	if elem == TypeNum {
		e.linef("\tsysmlFail(\"type mismatch: 'for' iterates a collection, and \" + sysmlElemKind(s.data[0]) + \" is not one\")")
	} else {
		e.linef("\tsysmlFail(%s)", strconv.Quote("type mismatch: 'for' iterates a collection, and "+article(elem)+" is not one"))
	}
	e.linef("}")
	e.linef("for _, %s := range s.data {", goLocal(s.Var))
	e.indent++
	e.linef("sysmlStep(1)")
	e.linef("_ = %s", goLocal(s.Var))
	e.block(s.Body)
	e.indent--
	e.linef("}")
	e.indent--
	e.linef("}")
}

// declInit is a declaration's initial value, null where it states none.
func (e *goEmitter) declInit(d Declare) string {
	if d.Init == nil {
		return fmt.Sprintf("sysmlNullSeq[%s]()", goElem(d.T))
	}
	return e.expr(d.Init)
}
