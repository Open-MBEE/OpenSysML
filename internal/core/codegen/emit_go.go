package codegen

import (
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
)

const goAssign = "%s = %s"

// goPrelude is the support library of a generated Go program. It mirrors
// cPrelude: the same checks, the same errors, the same output format.
const goPrelude = `package main

import (
	"errors"
	"fmt"
	"math"
	"math/big"
	"os"
	"strconv"
	"strings"
)

type sysmlError struct{ msg string }

func (e sysmlError) Error() string { return e.msg }

func sysmlFail(msg string) { panic(sysmlError{msg}) }

var sysmlDepth int

func sysmlEnter() {
	if sysmlDepth >= sysmlMaxCalcDepth {
		sysmlFail("calc recursion limit exceeded")
	}
	sysmlDepth++
}

func sysmlLeave() { sysmlDepth-- }

func sysmlAdd(a, b int64) int64 {
	r := a + b
	if !((b <= 0 || r > a) && (b >= 0 || r < a)) {
		sysmlFail("arithmetic overflow: + exceeds the Integer range")
	}
	return r
}

func sysmlSub(a, b int64) int64 {
	r := a - b
	if !((b >= 0 || r > a) && (b <= 0 || r < a)) {
		sysmlFail("arithmetic overflow: - exceeds the Integer range")
	}
	return r
}

func sysmlMulOK(a, b int64) (int64, bool) {
	if a == 0 || b == 0 {
		return 0, true
	}
	if (a == math.MinInt64 && b == -1) || (b == math.MinInt64 && a == -1) {
		return 0, false
	}
	r := a * b
	return r, r/b == a
}

func sysmlMul(a, b int64) int64 {
	r, ok := sysmlMulOK(a, b)
	if !ok {
		sysmlFail("arithmetic overflow: * exceeds the Integer range")
	}
	return r
}

func sysmlNeg(a int64) int64 {
	if a == math.MinInt64 {
		sysmlFail("arithmetic overflow: negation exceeds the Integer range")
	}
	return -a
}

func sysmlMod(a, b int64) int64 {
	if b == 0 {
		sysmlFail("division by zero")
	}
	return a % b
}

func sysmlQuot(a, b int64) float64 {
	if b == 0 {
		sysmlFail("division by zero")
	}
	q, _ := new(big.Rat).SetFrac64(a, b).Float64()
	return q
}

func sysmlNonNegative(v int64, typ string) int64 {
	if v < 0 {
		sysmlFail(fmt.Sprintf("type mismatch: cannot write %d (an Integer) to a feature typed by %s", v, typ))
	}
	return v
}

func sysmlFinite(r float64) float64 {
	if math.IsNaN(r) || math.IsInf(r, 0) {
		sysmlFail("arithmetic overflow: result is not a finite Real")
	}
	return r
}

// Library functions: a NaN result is a domain error, an infinity an overflow.
func sysmlLibReal(r float64) float64 {
	if math.IsNaN(r) {
		sysmlFail("arithmetic domain error: argument outside the function's domain")
	}
	if math.IsInf(r, 0) {
		sysmlFail("arithmetic overflow: result is not a finite Real")
	}
	return r
}

func sysmlLibInt(x float64) int64 {
	if math.IsNaN(x) {
		sysmlFail("arithmetic domain error: argument outside the function's domain")
	}
	if !(x < 9223372036854775808.0 && x >= -9223372036854775808.0) {
		sysmlFail("arithmetic overflow: result exceeds the Integer range")
	}
	return int64(x)
}

func sysmlIAbs(a int64) int64 {
	if a == math.MinInt64 {
		sysmlFail("arithmetic overflow: abs exceeds the Integer range")
	}
	if a < 0 {
		return -a
	}
	return a
}

func sysmlIMax(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

func sysmlIMin(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}

func sysmlNaturalArg(v int64) int64 {
	if v < 0 {
		sysmlFail("type mismatch: requires Natural arguments")
	}
	return v
}

func sysmlTan(t float64) float64 { return sysmlLibReal(math.Sin(t) / math.Cos(t)) }
func sysmlCot(t float64) float64 { return sysmlLibReal(math.Cos(t) / math.Sin(t)) }

func sysmlLn(x float64) float64 {
	if x <= 0 {
		sysmlFail("arithmetic domain error: the logarithm of a non-positive argument is not a Real (requires x > 0.0)")
	}
	return sysmlLibReal(math.Log(x))
}

func sysmlLog(x, base float64) float64 {
	switch {
	case x <= 0:
		sysmlFail("arithmetic domain error: the logarithm of a non-positive argument is not a Real (requires x > 0.0)")
	case base <= 0:
		sysmlFail("arithmetic domain error: a non-positive base has no logarithm (requires base > 0.0)")
	case base == 1:
		sysmlFail("arithmetic domain error: base 1.0 has no logarithm")
	case base == 10:
		return sysmlLibReal(math.Log10(x))
	case base == 2:
		return sysmlLibReal(math.Log2(x))
	}
	return sysmlLibReal(math.Log(x) / math.Log(base))
}

func sysmlAtan2(y, x float64) float64 {
	if y == 0 && x == 0 {
		sysmlFail("arithmetic domain error: atan2(0.0, 0.0) has no angle")
	}
	return sysmlLibReal(math.Atan2(y, x))
}

func sysmlRDiv(a, b float64) float64 {
	if b == 0 {
		sysmlFail("division by zero")
	}
	return sysmlFinite(a / b)
}

func sysmlRMod(a, b float64) float64 {
	if b == 0 {
		sysmlFail("division by zero")
	}
	return sysmlFinite(math.Mod(a, b))
}

func sysmlIPow(a, n int64) int64 {
	res := int64(1)
	for n > 0 {
		var ok bool
		if n&1 == 1 {
			if res, ok = sysmlMulOK(res, a); !ok {
				sysmlFail("arithmetic overflow: ** exceeds the Integer range")
			}
		}
		if n >>= 1; n == 0 {
			break
		}
		if a, ok = sysmlMulOK(a, a); !ok {
			sysmlFail("arithmetic overflow: ** exceeds the Integer range")
		}
	}
	return res
}

func sysmlRPow(base, exp float64) float64 {
	switch {
	case base == 0 && exp < 0:
		sysmlFail("arithmetic domain: 0 ** negative exponent is undefined")
	case base < 0 && exp != math.Trunc(exp):
		sysmlFail("arithmetic domain: negative base with fractional exponent is not a Real")
	}
	return sysmlFinite(math.Pow(base, exp))
}

func sysmlParseInt(s, name string) int64 {
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		fmt.Fprintf(os.Stderr, "argument %s: %s is not an Integer\n", name, s)
		os.Exit(2)
	}
	return v
}

func sysmlParseReal(s, name string) float64 {
	if !sysmlRealNotation(s) {
		fmt.Fprintf(os.Stderr, "argument %s: %s is not a finite Real in decimal notation\n", name, s)
		os.Exit(2)
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil || v == 0 && sysmlNonzeroNotation(s) {
		fmt.Fprintf(os.Stderr, "argument %s: arithmetic overflow: %s is outside the Real range\n", name, s)
		os.Exit(1)
	}
	return v
}

// sysmlRealNotation reports whether s is decimal Real notation: an optional
// sign, digits with an optional fraction, and an optional decimal exponent.
func sysmlRealNotation(s string) bool {
	i := 0
	if i < len(s) && (s[i] == '+' || s[i] == '-') {
		i++
	}
	digits := 0
	for i < len(s) && '0' <= s[i] && s[i] <= '9' {
		i, digits = i+1, digits+1
	}
	if i < len(s) && s[i] == '.' {
		i++
		for i < len(s) && '0' <= s[i] && s[i] <= '9' {
			i, digits = i+1, digits+1
		}
	}
	if digits == 0 {
		return false
	}
	if i < len(s) && (s[i] == 'e' || s[i] == 'E') {
		i++
		if i < len(s) && (s[i] == '+' || s[i] == '-') {
			i++
		}
		exponent := 0
		for i < len(s) && '0' <= s[i] && s[i] <= '9' {
			i, exponent = i+1, exponent+1
		}
		if exponent == 0 {
			return false
		}
	}
	return i == len(s)
}

// sysmlNonzeroNotation reports whether the significand of decimal notation s
// has a nonzero digit, so a zero it parsed to is an underflow.
func sysmlNonzeroNotation(s string) bool {
	significand, _, _ := strings.Cut(strings.ToLower(s), "e")
	return strings.ContainsAny(significand, "123456789")
}

func sysmlParseBool(s, name string) bool {
	switch s {
	case "true":
		return true
	case "false":
		return false
	}
	fmt.Fprintf(os.Stderr, "argument %s: %s is not a Boolean\n", name, s)
	os.Exit(2)
	return false
}

func sysmlFormat(v any) string {
	if r, ok := v.(float64); ok {
		format := byte('f')
		if abs := math.Abs(r); r != 0 && (abs < 1e-4 || abs >= 1e21) {
			format = 'g'
		}
		text := strconv.FormatFloat(r, format, -1, 64)
		if !strings.ContainsAny(text, ".eEnN") {
			text += ".0"
		}
		return text
	}
	return fmt.Sprint(v)
}

// sysmlRun invokes fn, converting a failed check into an error.
func sysmlRun(fn func()) (err error) {
	sysmlDepth = 0
	defer func() {
		if r := recover(); r != nil {
			if e, ok := r.(sysmlError); ok {
				err = errors.New(e.msg)
				return
			}
			panic(r)
		}
	}()
	fn()
	return nil
}
`

// EmitGo writes program as a self-contained Go main package whose command line
// and output match the C program's.
func EmitGo(w io.Writer, p *Program) error {
	e := &goEmitter{w: w, collections: p.Collections}
	e.raw(goPrelude)
	e.raw(fmt.Sprintf("const sysmlMaxCalcDepth = %d\n\n", runtime.DefaultMaxCalcDepth))
	if p.Collections {
		e.raw(fmt.Sprintf("const sysmlDefaultMaxElements = %d\n", runtime.DefaultMaxElements))
		e.raw(goSeqPrelude)
	}
	for _, fn := range p.Funcs {
		e.function(fn)
	}
	e.main(p.Entry)
	return e.err
}

type goEmitter struct {
	w      io.Writer
	err    error
	indent int
	// resultRange is checked on each return of the function being emitted.
	resultRange Range
	// collections brackets every statement with the element budget's release.
	collections bool
	temps       int
}

func (e *goEmitter) raw(s string) {
	if e.err != nil {
		return
	}
	_, e.err = io.WriteString(e.w, s)
}

func (e *goEmitter) linef(format string, args ...any) {
	e.raw(strings.Repeat("\t", e.indent) + fmt.Sprintf(format, args...) + "\n")
}

func goType(t Type) string {
	switch t {
	case TypeInt:
		return "int64"
	case TypeReal:
		return "float64"
	case TypeBool:
		return "bool"
	case TypeSeqInt, TypeSeqReal, TypeSeqBool:
		return goSeqType(t)
	}
	return "struct{}"
}

func goLocal(name string) string { return cLocal(name) }

// goNarrowed checks v against the range of the feature it is written to.
func goNarrowed(v string, r Range) string {
	if r == RangeAny {
		return v
	}
	return fmt.Sprintf("sysmlNonNegative(%s, %q)", v, r.String())
}

func goParams(fn *Func) string {
	parts := make([]string, len(fn.Params))
	for i, p := range fn.Params {
		parts[i] = goLocal(p.Name) + " " + goType(p.Type)
	}
	return strings.Join(parts, ", ")
}

func (e *goEmitter) function(fn *Func) {
	e.raw("\n")
	e.linef("func %s(%s) %s {", fn.Ident, goParams(fn), goType(fn.Result))
	e.indent++
	e.linef("sysmlEnter()")
	e.linef("defer sysmlLeave()")
	for _, p := range fn.Params {
		switch {
		case p.Type.Many():
			if p.Mult != MultAny || p.Range != RangeAny || p.Unique {
				v := e.checked(Checked{X: Var{Name: p.Name, T: p.Type}, M: p.Mult, R: p.Range, Unique: p.Unique, Where: paramWhere(p.Name)})
				e.linef(goAssign, goLocal(p.Name), v)
			}
		case p.Range != RangeAny:
			e.linef(goAssign, goLocal(p.Name), goNarrowed(goLocal(p.Name), p.Range))
		}
	}
	e.resultRange = fn.ResultRange
	e.block(fn.Body)
	e.linef("sysmlFail(%s)", strconv.Quote("calc "+fn.Name+" completed without returning a value"))
	e.linef("panic(\"unreachable\")")
	e.indent--
	e.linef("}")
}

// block emits statements; with collections, the elements a statement
// materializes are released when it ends, as the interpreter's step does.
func (e *goEmitter) block(stmts []Stmt) {
	releases := false
	for _, s := range stmts {
		if _, ok := s.(Return); !ok {
			releases = true
		}
	}
	if !e.collections || !releases {
		for _, s := range stmts {
			e.stmt(s)
		}
		return
	}
	e.temps++
	held := fmt.Sprintf("sysmlH%d", e.temps)
	e.linef("%s := sysmlElements", held)
	for _, s := range stmts {
		e.stmt(s)
		if _, ok := s.(Return); !ok {
			e.linef("sysmlElements = %s", held)
		}
	}
}

func (e *goEmitter) stmt(s Stmt) {
	switch s := s.(type) {
	case Declare:
		e.linef("var %s %s = %s", goLocal(s.Name), goType(s.T), e.declInit(s))
		e.linef("_ = %s", goLocal(s.Name))
	case Assign:
		e.linef(goAssign, goLocal(s.Name), goNarrowed(e.expr(s.Value), s.Range))
	case If:
		e.linef("if %s {", e.expr(s.Cond))
		e.indent++
		e.block(s.Then)
		e.indent--
		if len(s.Else) > 0 {
			e.linef("} else {")
			e.indent++
			e.block(s.Else)
			e.indent--
		}
		e.linef("}")
	case While:
		e.linef("for %s {", e.expr(s.Cond))
		e.indent++
		e.block(s.Body)
		if s.Until != nil {
			e.linef("if %s {", e.expr(s.Until))
			e.linef("\tbreak")
			e.linef("}")
		}
		e.indent--
		e.linef("}")
	case ForEach:
		e.forEach(s)
	case Return:
		e.linef("return %s", goNarrowed(e.expr(s.Value), e.resultRange))
	default:
		e.err = fmt.Errorf("codegen: Go emitter has no case for %T", s)
	}
}

func (e *goEmitter) expr(x Expr) string {
	switch x := x.(type) {
	case IntLit:
		if x.Value == math.MinInt64 {
			return "int64(math.MinInt64)"
		}
		return fmt.Sprintf("int64(%d)", x.Value)
	case RealLit:
		return "float64(" + cReal(x.Value) + ")"
	case BoolLit:
		return strconv.FormatBool(x.Value)
	case Var:
		return goLocal(x.Name)
	case ToReal:
		if x.X.Type().Many() {
			return "sysmlWiden(" + e.expr(x.X) + ")"
		}
		return "float64(" + e.expr(x.X) + ")"
	case Unary:
		operand := e.expr(x.X)
		switch x.Op {
		case ast.OpNot:
			return "(!" + operand + ")"
		case ast.OpPos:
			return operand
		case ast.OpNeg:
			if x.T == TypeInt {
				return "sysmlNeg(" + operand + ")"
			}
			return "(-" + operand + ")"
		}
	case Binary:
		return e.binary(x)
	case Cond:
		return fmt.Sprintf("func() %s { if %s { return %s }; return %s }()", goType(x.T), e.expr(x.C), e.expr(x.Then), e.expr(x.Else))
	case Call:
		return e.call(x.Args, len(x.Fn.Params), goType(x.Fn.Result), func(operands []string) string {
			return fmt.Sprintf("%s(%s)", x.Fn.Ident, strings.Join(operands, ", "))
		})
	case LibCall:
		return e.call(x.Args, len(x.Op.Operands()), goType(x.Op.Result()), x.Op.goExpr)
	}
	if s, ok := e.seqExpr(x); ok {
		return s
	}
	e.err = fmt.Errorf("codegen: Go emitter has no case for %T", x)
	return "0"
}

// call emits a call; Go evaluates operands left to right, so only arguments
// written out of parameter order need temporaries to keep source order.
func (e *goEmitter) call(args []Arg, nParams int, result string, apply func(operands []string) string) string {
	names := make([]string, len(args))
	for i, a := range args {
		names[i] = e.expr(a.Value)
	}
	if inParamOrder(args, nParams) {
		return apply(names)
	}
	var b strings.Builder
	fmt.Fprintf(&b, "func() %s { ", result)
	for i := range args {
		fmt.Fprintf(&b, "t%d := %s; _ = t%d; ", i, names[i], i)
		names[i] = fmt.Sprintf("t%d", i)
	}
	fmt.Fprintf(&b, "return %s }()", apply(callOperands(args, nParams, names)))
	return b.String()
}

func (e *goEmitter) binary(x Binary) string {
	l, r := e.expr(x.L), e.expr(x.R)
	ints := x.L.Type() == TypeInt
	switch x.Op {
	case ast.OpAdd, ast.OpSub, ast.OpMul:
		if ints {
			return fmt.Sprintf("sysml%s(%s, %s)", map[ast.OperatorKind]string{ast.OpAdd: "Add", ast.OpSub: "Sub", ast.OpMul: "Mul"}[x.Op], l, r)
		}
		return fmt.Sprintf("sysmlFinite(%s %s %s)", l, cOperator(x.Op), r)
	case ast.OpDiv:
		if ints {
			return fmt.Sprintf("sysmlQuot(%s, %s)", l, r)
		}
		return fmt.Sprintf("sysmlRDiv(%s, %s)", l, r)
	case ast.OpMod:
		if ints {
			return fmt.Sprintf("sysmlMod(%s, %s)", l, r)
		}
		return fmt.Sprintf("sysmlRMod(%s, %s)", l, r)
	case ast.OpPow:
		if x.T == TypeInt {
			return fmt.Sprintf("sysmlIPow(%s, %s)", l, r)
		}
		return fmt.Sprintf("sysmlRPow(%s, %s)", l, r)
	case ast.OpLt, ast.OpLe, ast.OpGt, ast.OpGe, ast.OpEq, ast.OpNeq:
		return fmt.Sprintf("(%s %s %s)", l, cOperator(x.Op), r)
	case ast.OpAnd, ast.OpConditionalAnd:
		return fmt.Sprintf("(%s && %s)", l, r)
	case ast.OpOr, ast.OpConditionalOr:
		return fmt.Sprintf("(%s || %s)", l, r)
	case ast.OpXor:
		return fmt.Sprintf("(%s != %s)", l, r)
	case ast.OpImplies:
		return fmt.Sprintf("(!%s || %s)", l, r)
	}
	e.err = fmt.Errorf("codegen: Go emitter has no binary case for %s", x.Op)
	return "0"
}

func (e *goEmitter) main(fn *Func) {
	e.raw("\n")
	e.linef("func main() {")
	e.indent++
	e.linef("args := os.Args[1:]")
	e.linef("repeat, badRepeat := 1, false")
	e.linef("if len(args) >= 2 && args[0] == \"--repeat\" {")
	e.linef("\tn, err := strconv.Atoi(args[1])")
	e.linef("\trepeat, badRepeat = n, err != nil || n < 1")
	e.linef("\targs = args[2:]")
	e.linef("}")
	e.linef("if badRepeat || len(args) != %d {", len(fn.Params))
	e.linef("\tfmt.Fprintf(os.Stderr, \"usage: %%s [--repeat N]%s\\n\", os.Args[0])", cUsage(fn))
	e.linef("\tos.Exit(2)")
	e.linef("}")
	if e.collections {
		e.linef("sysmlReadMaxElements()")
	}
	args := make([]string, len(fn.Params))
	for i, p := range fn.Params {
		parser := map[Type]string{TypeInt: "sysmlParseInt", TypeReal: "sysmlParseReal", TypeBool: "sysmlParseBool"}[p.Type.Elem()]
		if p.Type.Many() {
			parser = fmt.Sprintf("sysmlParseSeq[%s](args[%d], %q, %s)", goElem(p.Type), i, p.Name, parser)
		} else {
			parser = fmt.Sprintf("%s(args[%d], %q)", parser, i, p.Name)
		}
		e.linef("%s := %s", goLocal(p.Name), parser)
		args[i] = goLocal(p.Name)
	}
	e.linef("var result %s", goType(fn.Result))
	reset := ""
	if e.collections {
		// A run holds its collection arguments throughout, as the interpreter
		// holds the literals it evaluated them from.
		reset = "sysmlElements = 0; "
		for i, p := range fn.Params {
			if p.Type.Many() {
				reset += fmt.Sprintf("sysmlCharge(int64(len(%s.data))); ", args[i])
			}
		}
	}
	e.linef("for i := 0; i < repeat; i++ {")
	e.linef("\tif err := sysmlRun(func() { %sresult = %s(%s) }); err != nil {", reset, fn.Ident, strings.Join(args, ", "))
	e.linef("\t\tfmt.Fprintf(os.Stderr, \"%s: %%s\\n\", err)", fn.Name)
	e.linef("\t\tos.Exit(1)")
	e.linef("\t}")
	e.linef("}")
	if fn.Result.Many() {
		e.linef("fmt.Println(sysmlFormatSeq(result))")
	} else {
		e.linef("fmt.Println(sysmlFormat(result))")
	}
	e.indent--
	e.linef("}")
}
