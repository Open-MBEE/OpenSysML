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

type sysmlElem interface{ int64 | float64 | bool }

// sysmlSeq is a collection value: null, one bare value, or a sequence.
type sysmlSeq[T sysmlElem] struct {
	shape sysmlShape
	data  []T
}

var (
	sysmlElements    int64
	sysmlMaxElements int64 = sysmlDefaultMaxElements
)

func sysmlFailf(format string, args ...any) { sysmlFail(fmt.Sprintf(format, args...)) }

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

func sysmlAtLeastSeq(s sysmlSeq[int64], lo int64, typ string) sysmlSeq[int64] {
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
		if a.data[i] != b.data[i] {
			return false
		}
	}
	return true
}

func sysmlIndex[T sysmlElem](s sysmlSeq[T], i int64) T {
	if i < 1 || i > int64(len(s.data)) {
		sysmlFailf("index out of range: sequence index %d is outside 1..%d", i, len(s.data))
	}
	return s.data[i-1]
}

func sysmlContains[T sysmlElem](s sysmlSeq[T], v T) bool {
	for _, e := range s.data {
		if e == v {
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

func sysmlIncludingAt[T sysmlElem](a, b sysmlSeq[T], i int64) sysmlSeq[T] {
	n := int64(len(a.data))
	if i < 1 || i > n+1 {
		sysmlFailf("index out of range: SequenceFunctions::includingAt insertion index %d is outside 1..%d", i, n+1)
	}
	return sysmlConcat(sysmlSeq[T]{sysmlMany, a.data[:i-1]}, b, sysmlSeq[T]{sysmlMany, a.data[i-1:]})
}

func sysmlSubsequence[T sysmlElem](s sysmlSeq[T], start, end int64, hasEnd bool) sysmlSeq[T] {
	n := int64(len(s.data))
	if !hasEnd {
		end = n
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

func sysmlExcludingAt[T sysmlElem](s sysmlSeq[T], start, end int64, hasEnd bool) sysmlSeq[T] {
	n := int64(len(s.data))
	if !hasEnd {
		end = start
	}
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

func sysmlRange(lo, hi int64) sysmlSeq[int64] {
	if lo > hi {
		return sysmlManySeq[int64](0)
	}
	n := hi - lo + 1
	if n <= 0 {
		n = math.MaxInt64
	}
	r := sysmlManySeq[int64](n)
	for i := range r.data {
		r.data[i] = lo + int64(i)
	}
	return r
}

// sysmlWiden is the Real copy of an Integer collection, charged like any
// other materialized collection.
func sysmlWiden(s sysmlSeq[int64]) sysmlSeq[float64] {
	sysmlCharge(int64(len(s.data)))
	r := sysmlSeq[float64]{s.shape, make([]float64, len(s.data))}
	for i, v := range s.data {
		r.data[i] = float64(v)
	}
	return r
}

func sysmlISum(s sysmlSeq[int64], op string) int64 {
	var acc int64
	for _, v := range s.data {
		r := acc + v
		if (r > acc) != (v > 0) {
			sysmlFailf("arithmetic overflow: %s exceeds the Integer range", op)
		}
		acc = r
	}
	return acc
}

func sysmlIProduct(s sysmlSeq[int64], op string) int64 {
	acc := int64(1)
	for _, v := range s.data {
		r, ok := sysmlMulOK(acc, v)
		if !ok {
			sysmlFailf("arithmetic overflow: %s exceeds the Integer range", op)
		}
		acc = r
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

func sysmlReadMaxElements() {
	raw, ok := os.LookupEnv("OPENSYSML_MAX_ELEMENTS")
	if !ok || strings.TrimSpace(raw) == "" {
		return
	}
	n, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	if err != nil {
		fmt.Fprintf(os.Stderr, "OPENSYSML_MAX_ELEMENTS=%q is not an integer: set it to a positive number of collection elements (default %d)\n", raw, sysmlDefaultMaxElements)
		os.Exit(2)
	}
	if n <= 0 {
		fmt.Fprintf(os.Stderr, "OPENSYSML_MAX_ELEMENTS=%q must be greater than zero: the budget is what stops a runaway run (default %d)\n", raw, sysmlDefaultMaxElements)
		os.Exit(2)
	}
	sysmlMaxElements = n
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
		return fmt.Sprintf("func() %s { %s := %s; return %s }()", goType(x.In.Type()), goLocal(x.Name), e.expr(x.Value), e.expr(x.In)), true
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
	}
	return "", false
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

// checked binds a collection: multiplicity first, then the elements' range.
func (e *goEmitter) checked(x Checked) string {
	v := e.expr(x.X)
	if x.M != MultAny {
		v = fmt.Sprintf("sysmlCheck(%s, %d, %d, %s)", v, x.M.Lower, x.M.Upper, strconv.Quote(x.Where))
	}
	if x.R != RangeAny {
		v = fmt.Sprintf("sysmlAtLeastSeq(%s, %d, %q)", v, x.R.Lower(), x.R.String())
	}
	return v
}

// seqCall applies a value operation to its evaluated operands.
func (e *goEmitter) seqCall(x SeqCall, v []string) string {
	switch x.Op {
	case SeqSize:
		return fmt.Sprintf("int64(len(%s.data))", v[0])
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
		fmt.Fprintf(&b, "for i, v := range s.data { %sk := %s; if i == 0 || k %s r { r = k } }; return r }()", bind("v"), body, less)
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
	e.linef("\tsysmlFail(%s)", strconv.Quote("type mismatch: 'for' iterates a collection, and "+article(elem)+" is not one"))
	e.linef("}")
	e.linef("for _, %s := range s.data {", goLocal(s.Var))
	e.indent++
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
