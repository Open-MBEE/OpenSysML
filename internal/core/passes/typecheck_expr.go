package passes

import (
	"fmt"
	"slices"
	"strconv"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// exprChecker types expressions against the stdlib scalar lattice and reports
// operand, binding, and invocation mismatches. Every rule is one-sided: a
// diagnostic is only produced when both the expected and the actual type are
// known, so partial type information never yields a false positive.
type exprChecker struct {
	resolver *resolve.Resolver
	model    *semantics.Model
	// lang is the document's language: KerML gives `[` no function to invoke.
	lang  source.Kind
	diags []Diagnostic
	// chaining guards the type of a feature read through a chain against a
	// feature whose value names itself, directly or through another feature.
	chaining map[*symbols.Symbol]bool
	// walkMembers checks the declarations an expression body owns, which are
	// members of the body's own scope (F64); bodiesChecked keeps one body from
	// being checked twice when its type is inferred more than once.
	walkMembers   func(*symbols.Scope, []ast.Node)
	bodiesChecked map[*ast.BodyExpr]bool
	// performed are the calls that are the values of action usages, which run
	// an action rather than evaluate a behavior (see performs).
	performed map[*ast.InvocationExpr]bool
}

// codeTypeExpr is the code of an expression typing diagnostic no rule of its
// own claims.
const codeTypeExpr = "type.expr"

func (ec *exprChecker) errorf(span source.Span, format string, args ...any) {
	ec.errorCode(codeTypeExpr, span, format, args...)
}

// errorCode is errorf under the code of a rule with its own.
func (ec *exprChecker) errorCode(code string, span source.Span, format string, args ...any) {
	ec.diags = append(ec.diags, Diagnostic{
		Severity: SeverityError,
		Span:     span,
		Message:  fmt.Sprintf(format, args...),
		Code:     code,
		Source:   "type",
	})
}

func (ec *exprChecker) warnf(span source.Span, format string, args ...any) {
	ec.warnCode(codeTypeExpr, span, format, args...)
}

// warnCode is warnf under the code of a rule with its own.
func (ec *exprChecker) warnCode(code string, span source.Span, format string, args ...any) {
	ec.diags = append(ec.diags, Diagnostic{
		Severity: SeverityWarning,
		Span:     span,
		Message:  fmt.Sprintf(format, args...),
		Code:     code,
		Source:   "type",
	})
}

// CodeUnboundParameter advises of an invocation leaving a default-less input parameter
// unbound: well formed (KerML 1.0 §8.3.4.8.8), but the runtime refuses to evaluate it.
const CodeUnboundParameter = "unbound-parameter"

// checkDeclValue checks a feature's bound value (`attribute x : T = expr`)
// against the type and multiplicity the feature declares; an accept's trigger
// is checked by the body walk instead.
func (ec *exprChecker) checkDeclValue(scope *symbols.Scope, d featureDecl) {
	if d.value == nil || isTriggerValue(d.value) {
		return
	}
	if u, ok := d.node.(*ast.Usage); ok {
		ec.markPerformed(u.PerformedInvocation())
	}
	// The node's own body folds over the parameters of an invocation written as
	// its value (`action n = B(a = 1) { in b = 2; }`).
	var node *symbols.Symbol
	if body := scope.ChildFor(d.node); body != nil {
		node = body.Owner()
	}
	ec.checkBoundValue(scope, scope, d, d.value, node)
}

// checkPerform types the action a `perform` statement runs.
func (ec *exprChecker) checkPerform(scope *symbols.Scope, n *ast.PerformActionNode) {
	ec.markPerformed(n.PerformedInvocation())
	ec.infer(scope, n.ActionRef)
}

// markPerformed records inv, when not nil, as a call that runs an action.
func (ec *exprChecker) markPerformed(inv *ast.InvocationExpr) {
	if inv == nil {
		return
	}
	if ec.performed == nil {
		ec.performed = map[*ast.InvocationExpr]bool{}
	}
	ec.performed[inv] = true
}

// checkBoundValue checks a value against the type and multiplicity of the
// feature it is bound to. The value's names resolve in valueScope and the
// feature's declaration in declScope, which differ when the value is written by
// an assignment rather than declared on the feature. node is the feature's own
// symbol when the value is declared on it, or nil.
func (ec *exprChecker) checkBoundValue(valueScope, declScope *symbols.Scope, d featureDecl, value ast.Node, node *symbols.Symbol) {
	want := ec.declaredPrimType(declScope, d.relationships)
	// A collection literal binds elementwise, so each element is checked
	// against the feature's type rather than the sequence as a whole.
	for _, element := range valueElements(value) {
		// A collection value binds the elements its body or collection produces. Inferring
		// the value checks and reports on them; their types are then read silently.
		if elements, collection := ec.model.CollectionElements(valueScope, element); collection {
			ec.infer(valueScope, element)
			silent := ec.silent()
			for _, produced := range elements {
				if produced.Node != nil {
					ec.checkScalarBinding(produced.Node, silent.infer(produced.Scope, produced.Node), want)
				}
			}
			continue
		}
		var got semantics.PrimType
		if inv, ok := element.(*ast.InvocationExpr); ok && node != nil {
			got = ec.inferNodeInvocation(valueScope, inv, node)
		} else {
			got = ec.infer(valueScope, element)
		}
		ec.checkScalarBinding(element, got, want)
	}
	ec.checkValueConformance(valueScope, declScope, d, value)
	ec.checkValueDimension(valueScope, declScope, d, value)
	ec.checkValueCount(valueScope, declScope, d, value)
}

// checkScalarBinding reports a got-typed value that may not bind to a want-typed feature,
// where both are known.
func (ec *exprChecker) checkScalarBinding(value ast.Node, got, want semantics.PrimType) {
	if want == semantics.PrimUnknown || got == semantics.PrimUnknown {
		return
	}
	if !bindable(value, got, want) {
		ec.errorf(value.Span(), "cannot bind %s value to a feature typed by %s", got, want)
	}
}

// bindable reports whether a got-typed value may bind to a want-typed feature: a literal's
// type is exact, a quotient's is Rational whatever it divides (a Real-typed feature may
// still hold an Integer), any other expression's only bounds its values.
func bindable(value ast.Node, got, want semantics.PrimType) bool {
	if semantics.PrimConforms(got, want) {
		return true
	}
	if spellsOneValue(value) || isQuotient(value) && semantics.PrimConforms(want, semantics.PrimInteger) {
		return false
	}
	return semantics.PrimConforms(want, got)
}

// isQuotient reports a division, whose result is a Rational however whole (IntegerFunctions::'/').
func isQuotient(n ast.Node) bool {
	op, ok := n.(*ast.OperatorExpr)
	return ok && op.Operator == ast.OpDiv && len(op.Operands) == 2
}

// spellsOneValue reports whether an expression writes its value out: a literal, or a signed one.
func spellsOneValue(n ast.Node) bool {
	if isLiteral(n) {
		return true
	}
	op, ok := n.(*ast.OperatorExpr)
	return ok && (op.Operator == ast.OpNeg || op.Operator == ast.OpPos) &&
		len(op.Operands) == 1 && isLiteral(op.Operands[0])
}

// declaredPrimType returns the scalar type a usage is typed by, or PrimUnknown.
func (ec *exprChecker) declaredPrimType(scope *symbols.Scope, rels []*ast.Relationship) semantics.PrimType {
	for _, rel := range rels {
		if rel == nil || rel.Kind != ast.RelTyping || rel.Target == nil {
			continue
		}
		if sym := ec.resolveTarget(scope, rel.Target); sym != nil {
			if prim := ec.model.PrimTypeOf(sym); prim != semantics.PrimUnknown {
				return prim
			}
		}
	}
	return semantics.PrimUnknown
}

// resolveTarget resolves a relationship target node to its symbol, following
// an alias to the type it names.
func (ec *exprChecker) resolveTarget(scope *symbols.Scope, target ast.Node) *symbols.Symbol {
	if fr, ok := target.(*ast.FeatureReference); ok {
		target = fr.Name
	}
	qn, ok := target.(*ast.QualifiedName)
	if !ok {
		return nil
	}
	sym, ok := ec.resolver.ResolveQualified(scope, qn)
	if !ok || sym == nil {
		return nil
	}
	if sym.Kind == symbols.SymbolAlias {
		if resolved, ok := ec.resolver.ResolveAliasTarget(sym); ok && resolved != nil {
			return resolved
		}
	}
	return sym
}

// checkBoolean checks an expression used where a condition is required.
func (ec *exprChecker) checkBoolean(scope *symbols.Scope, n ast.Node, context string) {
	ec.checkCondition(scope, n, codeTypeExpr, context+" must be Boolean, found %s", false)
}

// checkCondition is checkBoolean reporting under a rule's own code and message,
// whose one verb takes the type found. A condition naming an untyped feature is
// left to evaluation unless the rule requires a Boolean statically (mustType).
func (ec *exprChecker) checkCondition(scope *symbols.Scope, n ast.Node, code, format string, mustType bool) {
	if n == nil {
		return
	}
	got := ec.infer(scope, n)
	if got == semantics.PrimBoolean {
		if mustType {
			// A conditional with two Boolean branches is still typed Anything.
			if c := ec.model.ExprConformsToLibrary(scope, n, semantics.FQNBoolean); c.Known && !c.Holds {
				ec.errorCode(code, n.Span(), format, c.Found)
			}
		}
		return
	}
	if got == semantics.PrimUnknown {
		ec.checkNonScalarCondition(scope, n, code, format, mustType)
		return
	}
	found := got.String()
	if got == semantics.PrimExpression {
		// A body is described by what it is, when the library says so.
		if c := ec.model.ExprConformsToLibrary(scope, n, semantics.FQNBoolean); c.Known && !c.Holds {
			found = c.Found
		}
	}
	ec.errorCode(code, n.Span(), format, found)
}

// checkNonScalarCondition reports a condition naming a feature typed by
// something no Boolean can come from: a part, an item, an enumeration — or
// one whose type is inherited, chained to or computed and is not Boolean: a
// redefined duration, a quantity.
func (ec *exprChecker) checkNonScalarCondition(scope *symbols.Scope, n ast.Node, code, format string, mustType bool) {
	typeSym := ec.valueTypeSymbol(scope, n)
	// A collection value is judged element by element by the model below.
	if _, collection := ec.model.CollectionResultTypes(scope, n); typeSym == nil && !collection {
		typeSym = ec.invocationResultTypeSymbol(scope, n)
	}
	if typeSym == nil {
		c := ec.model.ExprConformsToLibrary(scope, n, semantics.FQNBoolean)
		if c.Known && !c.Holds && (mustType || !c.Untyped) {
			ec.errorCode(code, n.Span(), format, c.Found)
		}
		return
	}
	if !ec.isDefinitelyNonBehavior(typeSym) ||
		ec.model.CouldHold(typeSym, semantics.PrimBoolean) {
		return
	}
	ec.errorCode(code, n.Span(), format, typeSym.Name)
}

// infer returns the scalar type of an expression, checking its operands on the
// way down. PrimUnknown means "not a scalar the checker models".
func (ec *exprChecker) infer(scope *symbols.Scope, n ast.Node) semantics.PrimType {
	switch e := n.(type) {
	case *ast.LiteralBool:
		return semantics.PrimBoolean
	case *ast.LiteralString:
		return semantics.PrimString
	case *ast.LiteralInteger:
		// The grammar has no negative literals (negation is a unary operator),
		// so an integer literal is always a Natural and conforms upward.
		return semantics.PrimNatural
	case *ast.LiteralReal:
		// A decimal literal denotes an exact ratio, so it conforms to Rational
		// as well as Real.
		return semantics.PrimRational
	case *ast.FeatureReference:
		return ec.inferQualified(scope, e.Name)
	case *ast.QualifiedName:
		return ec.inferQualified(scope, e)
	case *ast.FeatureChainExpr:
		return ec.inferFeatureChain(scope, e)
	case *ast.OperatorExpr:
		return ec.inferOperator(scope, e)
	case *ast.InvocationExpr:
		return ec.inferInvocation(scope, e)
	case *ast.ConstructorExpr:
		// An instance has no scalar type; the arguments are checked in place.
		return ec.inferConstructor(scope, e)
	case *ast.SequenceExpr:
		// A sequence has no scalar type of its own; walking the elements keeps
		// errors inside them reported.
		for _, el := range e.Elements {
			ec.infer(scope, el)
		}
		return semantics.PrimUnknown
	case *ast.IndexExpr:
		return ec.inferIndex(scope, e)
	case *ast.CollectExpr:
		return ec.inferCollect(scope, e)
	case *ast.SelectExpr:
		return ec.inferSelect(scope, e)
	case *ast.BodyExpr:
		return ec.inferBody(scope, e)
	case *ast.CastExpr:
		ec.checkBoundOperators(scope, e.Multiplicity)
	}
	return semantics.PrimUnknown
}

// inferIndex types `seq#(i)`, the sequence index, and `n [unit]`, the quantity
// expression the notation shares its node with — the bracket the expression was
// written with says which of the two it is.
func (ec *exprChecker) inferIndex(scope *symbols.Scope, e *ast.IndexExpr) semantics.PrimType {
	if e.Bracket {
		// The unit is a measurement reference, not a scalar; the magnitude and the
		// operators inside the unit are expressions of their own and are checked.
		ec.infer(scope, e.Operand)
		if e.Index != nil {
			ec.infer(scope, e.Index)
		}
		ec.checkBracket(scope, e)
		return semantics.PrimUnknown
	}

	// `SequenceFunctions::'#'` declares `in index: Positive[1]`, so a Real, a
	// Boolean or a String names no position. Whether a whole number names one is
	// known from the value, so an Integer index is checked at evaluation.
	index := ec.infer(scope, e.Index)
	if !semantics.PrimConforms(index, semantics.PrimInteger) && e.Index != nil {
		ec.errorf(e.Index.Span(), "sequence index must be an Integer, found %s", index)
	}

	// The index of a sequence of one scalar type is a value of that type, which
	// is what makes `(1, 2, 3)#(2) + 1` checkable. Typing the elements here is
	// what walks them, so they are not inferred a second time below.
	var elem semantics.PrimType
	if seq, isSeq := e.Operand.(*ast.SequenceExpr); isSeq {
		elem = ec.commonElementType(scope, seq.Elements)
	} else {
		elem = ec.infer(scope, e.Operand)
	}

	// A literal index names a position at check time, so an index the operand
	// cannot have is an error before the model is ever run: index 0 for a
	// notation counting from 1, and any index beyond a sequence written out.
	if lit, ok := e.Index.(*ast.LiteralInteger); ok {
		if written, err := strconv.ParseInt(lit.Value, 10, 64); err == nil {
			length, known := writtenLength(e.Operand)
			switch {
			case written == 0:
				ec.errorf(lit.Span(), "sequence index counts from 1, found 0")
			case known && written > length:
				ec.errorf(lit.Span(), "sequence index %d is outside 1..%d", written, length)
			}
		}
	}

	return elem
}

// writtenLength answers how many elements an operand written out holds, and
// whether that is knowable at all: a literal is one, `null` is none, and a KerML
// sequence is flat, so a written element contributes its own count while a name
// or a call may be a collection whose length only the values know.
func writtenLength(operand ast.Node) (int64, bool) {
	if isLiteral(operand) {
		return 1, true
	}
	switch n := operand.(type) {
	case *ast.NullExpr:
		return 0, true
	case *ast.SequenceExpr:
		total := int64(0)
		for _, el := range n.Elements {
			length, known := writtenLength(el)
			if !known {
				return 0, false
			}
			total += length
		}
		return total, true
	}
	return 0, false
}

// isLiteral reports whether an expression is a scalar literal, whose exact type is written out.
func isLiteral(n ast.Node) bool {
	switch n.(type) {
	case *ast.LiteralInteger, *ast.LiteralReal, *ast.LiteralString, *ast.LiteralBool:
		return true
	default:
		return false
	}
}

// commonElementType returns the scalar type every element of a sequence
// expression conforms to, or PrimUnknown where they have none in common or any
// one of them has no known type: PrimConforms holds of PrimUnknown either way
// round, so it decides conformance but cannot merge types.
func (ec *exprChecker) commonElementType(scope *symbols.Scope, elements []ast.Node) semantics.PrimType {
	common := semantics.PrimUnknown
	unknown := false
	for i, el := range elements {
		elem := ec.infer(scope, el)
		if elem == semantics.PrimUnknown {
			// Every element is still typed, so an error inside one is reported.
			unknown = true
			continue
		}
		if i == 0 || common == semantics.PrimUnknown {
			common = elem
			continue
		}
		switch {
		case elem == common:
		case semantics.PrimConforms(elem, common):
		case semantics.PrimConforms(common, elem):
			common = elem
		default:
			unknown = true
		}
	}
	if unknown {
		return semantics.PrimUnknown
	}
	return common
}

// inferCollect types `operand.{in x; ...}`, the collect notation: a sequence of
// the body's results, which is no scalar of its own. Its purpose here is to
// check the body, in the scope its parameters are declared in.
func (ec *exprChecker) inferCollect(scope *symbols.Scope, e *ast.CollectExpr) semantics.PrimType {
	ec.infer(scope, e.Operand)
	ec.infer(scope, e.Body)
	return semantics.PrimUnknown
}

// inferSelect types `operand.?{in x; ...}`, the select notation. The library
// declares the selector's result `Boolean[1]`, so a body that answers something
// else is reported here rather than only where the model is run.
func (ec *exprChecker) inferSelect(scope *symbols.Scope, e *ast.SelectExpr) semantics.PrimType {
	ec.infer(scope, e.Operand)
	if body, ok := e.Body.(*ast.BodyExpr); ok && body.Result != nil {
		ec.checkBodyMembers(scope, body)
		ec.checkBoolean(ec.bodyScope(scope, body), body.Result, "select predicate")
		return semantics.PrimUnknown
	}
	ec.infer(scope, e.Body)
	return semantics.PrimUnknown
}

// inferBody checks a body's members and result (in its parameters' scope); the
// body's value is the expression itself, not that result.
func (ec *exprChecker) inferBody(scope *symbols.Scope, e *ast.BodyExpr) semantics.PrimType {
	inner := ec.bodyScope(scope, e)
	ec.checkBodyMembers(scope, e)
	for i := range e.Params {
		ec.infer(scope, e.Params[i].Value)
	}
	if e.Result != nil {
		ec.infer(inner, e.Result)
	}
	return semantics.PrimExpression
}

// bodyScope returns the scope a body expression's result is written in: its
// parameters are declarations of that scope, the same one the resolver and the
// runtime read the body's names in.
func (ec *exprChecker) bodyScope(scope *symbols.Scope, body *ast.BodyExpr) *symbols.Scope {
	if scope == nil {
		return nil
	}
	return symbols.BodyExprScope(scope, body)
}

// checkBodyMembers types the declarations an expression body owns — its
// parameters' bounds and its members — once per body, in the scope they are members of.
func (ec *exprChecker) checkBodyMembers(scope *symbols.Scope, body *ast.BodyExpr) {
	if body == nil || (len(body.Params) == 0 && len(body.Members) == 0) {
		return
	}
	if ec.bodiesChecked == nil {
		ec.bodiesChecked = make(map[*ast.BodyExpr]bool)
	} else if ec.bodiesChecked[body] {
		return
	}
	ec.bodiesChecked[body] = true
	inner := ec.bodyScope(scope, body)
	for i := range body.Params {
		ec.checkBoundOperators(inner, body.Params[i].Multiplicity)
	}
	if ec.walkMembers != nil {
		ec.walkMembers(inner, body.Members)
	}
}

func (ec *exprChecker) inferQualified(scope *symbols.Scope, qn *ast.QualifiedName) semantics.PrimType {
	if qn == nil {
		return semantics.PrimUnknown
	}
	sym, ok := ec.resolver.ResolveQualified(scope, qn)
	if !ok || sym == nil {
		return semantics.PrimUnknown
	}
	return ec.model.PrimTypeOf(sym)
}

// inferFeatureChain types a feature chain (`c.a`) as the feature its last
// segment names. The segments of a chain are members of the preceding segment
// rather than of the enclosing scope (SysML 7.6.6), which is how a calc usage's
// `out` feature — inherited from the calc it is typed by — is reached.
func (ec *exprChecker) inferFeatureChain(scope *symbols.Scope, e *ast.FeatureChainExpr) semantics.PrimType {
	// A constructed or invoked head's arguments are checked where they are written.
	switch head := chainHead(e).(type) {
	case *ast.ConstructorExpr:
		ec.inferConstructor(scope, head)
	case *ast.InvocationExpr:
		ec.inferNodeInvocation(scope, head, nil)
	}
	sym, ok := ec.resolver.ResolveTarget(scope, e)
	if !ok || sym == nil {
		// An unresolved chain is reported by the name-resolution tier; typing it
		// again here would double-report it.
		return semantics.PrimUnknown
	}
	return ec.featurePrimType(sym)
}

// chainHead returns the operand below every nested `.member` of a chain.
func chainHead(e *ast.FeatureChainExpr) ast.Node {
	head := e.Operand
	for {
		inner, ok := head.(*ast.FeatureChainExpr)
		if !ok {
			return head
		}
		head = inner.Operand
	}
}

// featurePrimType returns the scalar type of the feature sym declares, falling
// back to the type of the value it is bound to when it declares none: an `out a
// = n + 1` of a calc carries its type in its default.
func (ec *exprChecker) featurePrimType(sym *symbols.Symbol) semantics.PrimType {
	if prim := ec.model.PrimTypeOf(sym); prim != semantics.PrimUnknown {
		return prim
	}
	usage, ok := sym.Decl.(*ast.Usage)
	if !ok || usage.Value == nil || sym.OwnerScope == nil {
		return semantics.PrimUnknown
	}
	if ec.chaining[sym] {
		return semantics.PrimUnknown
	}
	if ec.chaining == nil {
		ec.chaining = make(map[*symbols.Symbol]bool)
	}
	ec.chaining[sym] = true
	defer delete(ec.chaining, sym)
	// The value belongs to the declaring scope, and is checked there in its own
	// right, so this only reads its type: diagnostics raised here would be
	// reported once per reader.
	silent := exprChecker{resolver: ec.resolver, model: ec.model, lang: ec.lang, chaining: ec.chaining}
	return silent.infer(sym.OwnerScope, usage.Value)
}

func (ec *exprChecker) inferOperator(scope *symbols.Scope, e *ast.OperatorExpr) semantics.PrimType {
	ec.checkDimensions(scope, e)
	switch e.Operator {
	case ast.OpNot:
		return ec.checkUnaryBoolean(scope, e)
	case ast.OpNeg, ast.OpPos:
		return ec.checkUnaryNumeric(scope, e)
	case ast.OpAnd, ast.OpConditionalAnd, ast.OpOr, ast.OpConditionalOr, ast.OpXor, ast.OpImplies:
		return ec.checkBinaryBoolean(scope, e)
	case ast.OpAdd:
		return ec.checkAddition(scope, e)
	case ast.OpSub, ast.OpMul, ast.OpMod, ast.OpPow:
		return ec.checkArithmetic(scope, e, semantics.PrimWiden)
	case ast.OpDiv:
		return ec.checkArithmetic(scope, e, divisionResult)
	case ast.OpLt, ast.OpGt, ast.OpLe, ast.OpGe:
		return ec.checkComparison(scope, e)
	case ast.OpEq, ast.OpNeq:
		return ec.checkEquality(scope, e)
	case ast.OpConditional:
		return ec.checkConditional(scope, e)
	case ast.OpAs:
		ec.checkCast(scope, e)
	}
	// Operators outside the scalar lattice (casts, classification, ranges,
	// indexing): still walk operands so nested errors surface.
	for _, operand := range e.Operands {
		ec.infer(scope, operand)
	}
	return semantics.PrimUnknown
}

// checkCast warns of `x as T` when no type of x and T specialize one another
// (KerML validateOperatorExpressionCastConformance).
func (ec *exprChecker) checkCast(scope *symbols.Scope, e *ast.OperatorExpr) {
	if ec.model == nil || e.TypeRef == nil || len(e.TypeRef.Parts) == 0 {
		return
	}
	if c := ec.model.CastConformance(scope, e); c.Known && !c.Holds {
		ec.warnCode(codeCastConformance, e.Span(), msgCastConformance, c.Found, e.TypeRef.Parts[len(e.TypeRef.Parts)-1].Text)
	}
}

// checkBracket judges `x [u]`: a misspelt index in KerML (validateOperatorExpressionBracketOperator),
// a quantity whose unit u is a measurement reference in SysML (validateOperatorExpressionQuantity).
func (ec *exprChecker) checkBracket(scope *symbols.Scope, e *ast.IndexExpr) {
	if ec.lang == source.KindKerML {
		ec.warnCode(codeBracketOperator, e.Span(), msgBracketOperator)
		return
	}
	if ec.model == nil || e.Index == nil {
		return
	}
	if c := ec.model.UnitOperandConformance(scope, e.Index); c.Known && !c.Holds {
		ec.warnCode(codeQuantityUnit, e.Index.Span(), msgQuantityUnit, c.Found)
	}
}

func (ec *exprChecker) checkUnaryBoolean(scope *symbols.Scope, e *ast.OperatorExpr) semantics.PrimType {
	if len(e.Operands) != 1 {
		return semantics.PrimUnknown
	}
	got := ec.infer(scope, e.Operands[0])
	if got != semantics.PrimUnknown && got != semantics.PrimBoolean {
		ec.errorf(e.Span(), "operator 'not' requires a Boolean operand, found %s", got)
	} else if found, mismatch := ec.nonBooleanOperand(scope, e.Operands[0], got); mismatch {
		ec.errorf(e.Span(), "operator 'not' requires a Boolean operand, found %s", found)
	}
	return semantics.PrimBoolean
}

// nonBooleanOperand judges an operand outside the scalar lattice by the
// conformance query: a body, a collection, a feature typed by no Boolean.
func (ec *exprChecker) nonBooleanOperand(scope *symbols.Scope, operand ast.Node, got semantics.PrimType) (string, bool) {
	if got != semantics.PrimUnknown || ec.model == nil {
		return "", false
	}
	c := ec.model.ExprConformsToLibrary(scope, operand, semantics.FQNBoolean)
	if !c.Known || c.Holds || c.Untyped {
		return "", false
	}
	return c.Found, true
}

func (ec *exprChecker) checkUnaryNumeric(scope *symbols.Scope, e *ast.OperatorExpr) semantics.PrimType {
	if len(e.Operands) != 1 {
		return semantics.PrimUnknown
	}
	got := ec.infer(scope, e.Operands[0])
	if got == semantics.PrimUnknown {
		return semantics.PrimUnknown
	}
	if !got.IsNumeric() {
		ec.errorf(e.Span(), "operator '%s' requires a numeric operand, found %s", e.Operator, got)
		return semantics.PrimUnknown
	}
	if e.Operator == ast.OpNeg {
		// Negation leaves the naturals.
		return semantics.PrimWiden(got, semantics.PrimInteger)
	}
	return got
}

func (ec *exprChecker) checkBinaryBoolean(scope *symbols.Scope, e *ast.OperatorExpr) semantics.PrimType {
	lhs, rhs, ok := ec.operands(scope, e)
	if !ok {
		return semantics.PrimBoolean
	}
	for _, got := range []semantics.PrimType{lhs, rhs} {
		if got != semantics.PrimUnknown && got != semantics.PrimBoolean {
			ec.errorf(e.Span(), "operator '%s' requires Boolean operands, found %s and %s", e.Operator, lhs, rhs)
			return semantics.PrimBoolean
		}
	}
	for i, got := range []semantics.PrimType{lhs, rhs} {
		if found, mismatch := ec.nonBooleanOperand(scope, e.Operands[i], got); mismatch {
			ec.errorf(e.Span(), "operator '%s' requires Boolean operands, found %s", e.Operator, found)
			break
		}
	}
	return semantics.PrimBoolean
}

// checkAddition allows the numeric tower plus String concatenation, which the
// stdlib defines as StringFunctions::'+'.
func (ec *exprChecker) checkAddition(scope *symbols.Scope, e *ast.OperatorExpr) semantics.PrimType {
	lhs, rhs, ok := ec.operands(scope, e)
	if !ok {
		return semantics.PrimUnknown
	}
	if lhs == semantics.PrimUnknown || rhs == semantics.PrimUnknown {
		return semantics.PrimUnknown
	}
	if lhs == semantics.PrimString && rhs == semantics.PrimString {
		return semantics.PrimString
	}
	if lhs.IsNumeric() && rhs.IsNumeric() {
		return semantics.PrimWiden(lhs, rhs)
	}
	ec.errorf(e.Span(), "operator '+' is not defined for %s and %s", lhs, rhs)
	return semantics.PrimUnknown
}

// divisionResult types a whole-number quotient as Rational, as the reference
// evaluates it (see docs/project/omg-issues.md); wider types divide within themselves.
func divisionResult(lhs, rhs semantics.PrimType) semantics.PrimType {
	switch widened := semantics.PrimWiden(lhs, rhs); widened {
	case semantics.PrimNatural, semantics.PrimInteger:
		return semantics.PrimRational
	default:
		return widened
	}
}

// checkArithmetic requires numeric operands and types the result with the
// operator's own result rule.
func (ec *exprChecker) checkArithmetic(
	scope *symbols.Scope,
	e *ast.OperatorExpr,
	result func(lhs, rhs semantics.PrimType) semantics.PrimType,
) semantics.PrimType {
	lhs, rhs, ok := ec.operands(scope, e)
	if !ok {
		return semantics.PrimUnknown
	}
	if lhs == semantics.PrimUnknown || rhs == semantics.PrimUnknown {
		return semantics.PrimUnknown
	}
	if !lhs.IsNumeric() || !rhs.IsNumeric() {
		ec.errorf(e.Span(), "operator '%s' requires numeric operands, found %s and %s", e.Operator, lhs, rhs)
		return semantics.PrimUnknown
	}
	return result(lhs, rhs)
}

func (ec *exprChecker) checkComparison(scope *symbols.Scope, e *ast.OperatorExpr) semantics.PrimType {
	lhs, rhs, ok := ec.operands(scope, e)
	if !ok {
		return semantics.PrimBoolean
	}
	if lhs == semantics.PrimUnknown || rhs == semantics.PrimUnknown {
		return semantics.PrimBoolean
	}
	bothNumeric := lhs.IsNumeric() && rhs.IsNumeric()
	bothString := lhs == semantics.PrimString && rhs == semantics.PrimString
	if !bothNumeric && !bothString {
		ec.errorf(e.Span(), "operator '%s' is not defined for %s and %s", e.Operator, lhs, rhs)
	}
	return semantics.PrimBoolean
}

// checkEquality warns rather than errors: the stdlib declares '==' over
// Anything, so comparing disjoint scalars is legal but almost always a mistake.
func (ec *exprChecker) checkEquality(scope *symbols.Scope, e *ast.OperatorExpr) semantics.PrimType {
	lhs, rhs, ok := ec.operands(scope, e)
	if !ok {
		return semantics.PrimBoolean
	}
	if lhs == semantics.PrimUnknown || rhs == semantics.PrimUnknown {
		return semantics.PrimBoolean
	}
	if !semantics.PrimConforms(lhs, rhs) && !semantics.PrimConforms(rhs, lhs) {
		result := "false"
		if e.Operator == ast.OpNeq {
			result = "true"
		}
		ec.warnf(e.Span(), "comparing %s with %s is always %s", lhs, rhs, result)
	}
	return semantics.PrimBoolean
}

func (ec *exprChecker) checkConditional(scope *symbols.Scope, e *ast.OperatorExpr) semantics.PrimType {
	if len(e.Operands) != 3 {
		return semantics.PrimUnknown
	}
	ec.checkBoolean(scope, e.Operands[0], "condition of 'if'")
	thenType := ec.infer(scope, e.Operands[1])
	elseType := ec.infer(scope, e.Operands[2])
	if thenType == elseType {
		return thenType
	}
	return semantics.PrimWiden(thenType, elseType)
}

func (ec *exprChecker) operands(scope *symbols.Scope, e *ast.OperatorExpr) (semantics.PrimType, semantics.PrimType, bool) {
	if len(e.Operands) != 2 {
		return semantics.PrimUnknown, semantics.PrimUnknown, false
	}
	return ec.infer(scope, e.Operands[0]), ec.infer(scope, e.Operands[1]), true
}

// inferInvocation checks a call's arguments against the `in` parameters of the declaration
// SelectInvocation chose and types it by its result; a receiver `x->f(a)` is the first argument.
func (ec *exprChecker) inferInvocation(scope *symbols.Scope, e *ast.InvocationExpr) semantics.PrimType {
	return ec.inferNodeInvocation(scope, e, nil)
}

// inferNodeInvocation is inferInvocation for an invocation performed by node (nil for a bare
// call).
func (ec *exprChecker) inferNodeInvocation(scope *symbols.Scope, e *ast.InvocationExpr, node *symbols.Symbol) semantics.PrimType {
	args := InvocationArgs(e)
	// Typed once, for selecting the overload, so nested errors report once.
	argTypes := ec.argumentTypes(scope, e)
	if chain := ChainCallee(e); chain != nil {
		return ec.inferChainInvocation(scope, e, chain, args, node)
	}
	if e.Type == nil {
		for _, arg := range e.NamedArgs {
			ec.infer(scope, arg.Value)
		}
		return semantics.PrimUnknown
	}
	performs := ec.performs(e)
	sel := ec.selectInvocation(scope, e, argTypes, performs)
	if !sel.Resolved() {
		return semantics.PrimUnknown
	}
	if sel.Ambiguous {
		ec.diags = append(ec.diags, Diagnostic{
			Severity: SeverityError,
			Span:     e.Type.Span(),
			Message: fmt.Sprintf("call of %s is ambiguous between %s",
				e.Type.Parts[len(e.Type.Parts)-1].Text, candidateNames(sel.Tied)),
			Code:   "invocation-ambiguous",
			Source: "type",
		})
		return semantics.PrimUnknown
	}
	// With no candidate the arguments fit, the first is checked as before and
	// the diagnostic names the rest.
	var considered []*symbols.Symbol
	sym := sel.Called()
	if sel.Selected == nil {
		considered = sel.Candidates
	}
	if !ec.isInvocationBehavior(sym, map[*symbols.Symbol]bool{}) {
		if ec.isDefinitelyNonBehavior(sym) {
			ec.diags = append(ec.diags, Diagnostic{
				Severity: SeverityError,
				Span:     e.Type.Span(),
				Message:  "Must invoke a behavior or a behavioral feature",
				Code:     "invocation-not-behavior",
				Source:   "type",
			})
		}
		return semantics.PrimUnknown
	}
	// A performed call runs its behavior, so it must name an action (SysML v2
	// §8.3.16.7 validatePerformActionUsage), as the runtime requires.
	if performs == semantics.PerformsAction && !ec.model.Performable(performs, sym) {
		ec.diags = append(ec.diags, Diagnostic{
			Severity: SeverityError,
			Span:     e.Type.Span(),
			Message:  msgReferenceAction,
			Code:     "usage-reference-kind",
			Source:   "type",
		})
		return semantics.PrimUnknown
	}
	if isInvocationBehaviorKind(sym.Kind) && !isBehaviorKind(sym.Kind) {
		return semantics.PrimUnknown
	}
	params, ok := ec.effectiveInParameters(sym, node)
	if !ok {
		return semantics.PrimUnknown
	}
	ec.checkArguments(scope, invocation{e, sym, args, params}, considered)
	if considered != nil {
		return semantics.PrimUnknown
	}
	return ec.model.PrimTypeOf(ec.model.ResultParameterOf(sym))
}

// inferChainInvocation is inferNodeInvocation for `x.f(a)`: the chain names the
// calc feature applied, whose effective inputs the arguments bind.
func (ec *exprChecker) inferChainInvocation(scope *symbols.Scope, e *ast.InvocationExpr, chain *ast.FeatureChainExpr, args []ast.Node, node *symbols.Symbol) semantics.PrimType {
	ec.infer(scope, chain)
	sym, ok := ec.resolver.ResolveTarget(scope, chain)
	if !ok || sym == nil {
		return semantics.PrimUnknown
	}
	if !ec.isInvocationBehavior(sym, map[*symbols.Symbol]bool{}) {
		if ec.isDefinitelyNonBehavior(sym) {
			ec.diags = append(ec.diags, Diagnostic{
				Severity: SeverityError,
				Span:     chain.Span(),
				Message:  "Must invoke a behavior or a behavioral feature",
				Code:     "invocation-not-behavior",
				Source:   "type",
			})
		}
		return semantics.PrimUnknown
	}
	if isInvocationBehaviorKind(sym.Kind) && !isBehaviorKind(sym.Kind) {
		return semantics.PrimUnknown
	}
	params, ok := ec.effectiveInParameters(sym, node)
	if !ok {
		return semantics.PrimUnknown
	}
	ec.checkArguments(scope, invocation{e, sym, args, params}, nil)
	return ec.model.PrimTypeOf(ec.model.ResultParameterOf(sym))
}

// reporter reports a finding about an invocation's arguments.
type reporter func(span source.Span, format string, args ...any)

// listing is report with considered (the other declarations a call could name) on each finding.
func listing(report reporter, considered []*symbols.Symbol) reporter {
	if len(considered) < 2 {
		return report
	}
	suffix := " (candidates: " + candidateNames(considered) + ")"
	return func(span source.Span, format string, args ...any) {
		report(span, "%s"+suffix, fmt.Sprintf(format, args...))
	}
}

// msgInvocationParameterRedefinition opens the report of an argument that binds no `in`
// parameter of the invoked type (KerML §8.3.4.8 validateInvocationExpressionParameterRedefinition).
const msgInvocationParameterRedefinition = "Must correspond to one input parameter of the invoked type"

// invocation is a call under argument checking: the expression, the behavior it names,
// its positional arguments, and the `in` parameters.
type invocation struct {
	e      *ast.InvocationExpr
	sym    *symbols.Symbol
	args   []ast.Node
	params []parameter
}

// checkArguments reports the arguments of the call that do not bind to its `in` parameters,
// listing considered (the other declarations the call could name) on each report.
func (ec *exprChecker) checkArguments(scope *symbols.Scope, call invocation, considered []*symbols.Symbol) {
	e, sym, args, params := call.e, call.sym, call.args, call.params
	report := listing(ec.errorf, considered)
	advise := listing(func(span source.Span, format string, args ...any) {
		ec.warnCode(CodeUnboundParameter, span, format, args...)
	}, considered)
	if len(e.NamedArgs) > 0 {
		ec.checkNamedArguments(scope, call, report, advise)
		return
	}
	if len(args) > len(params) {
		report(args[len(params)].Span(), "%s: %s takes %d argument(s), found %d", msgInvocationParameterRedefinition, sym.Name, len(params), len(args))
		return
	}
	for i, arg := range args {
		for _, m := range ec.argumentMismatches(scope, arg, params[i]) {
			report(m.span, "argument %d of %s %s", i+1, sym.Name, m.why)
		}
	}
	// Arguments bind in order, so the parameters past the last one are unbound.
	for _, p := range params[len(args):] {
		if p.required(ec.model) {
			advise(e.Span(), msgUnboundParameter, sym.Name, p.name())
		}
	}
}

// msgUnboundParameter is the CodeUnboundParameter advisory: the call's name, then the parameter's.
const msgUnboundParameter = "%s leaves parameter %s unbound, so the call cannot be evaluated"

// inferConstructor checks the arguments of `new T(…)` against T's features: a
// positional argument binds the feature at its position, a label the feature it
// names, and a feature is bound at most once.
func (ec *exprChecker) inferConstructor(scope *symbols.Scope, e *ast.ConstructorExpr) semantics.PrimType {
	for _, a := range e.Args {
		ec.infer(scope, a)
	}
	for _, na := range e.NamedArgs {
		ec.infer(scope, na.Value)
	}
	if e.Type == nil {
		return semantics.PrimUnknown
	}
	typ := ec.resolveTarget(scope, e.Type)
	if typ == nil {
		return semantics.PrimUnknown
	}
	// Only a Type is instantiated (KerML §8.3.4.8 validateInstantiationExpressionInstantiatedType).
	if !isTypeKind(typ.Kind) {
		ec.diags = append(ec.diags, Diagnostic{
			Severity: SeverityError,
			Span:     e.Type.Span(),
			Message:  fmt.Sprintf("Must have an invoked/instantiated type: %s is a %s, not a type", typ.Name, typ.Kind),
			Code:     "instantiation-not-type",
			Source:   "type",
		})
		return semantics.PrimUnknown
	}
	features := ec.model.ConstructibleFeatures(typ)
	bound := make(map[*symbols.Symbol]bool, len(features))
	for i, arg := range e.Args {
		if i >= len(features) {
			ec.errorf(arg.Span(), "new %s takes %d argument(s), found %d", typ.Name, len(features), len(e.Args))
			break
		}
		bound[features[i]] = true
		ec.checkFeatureBinding(scope, arg, features[i], typ)
	}
	for _, na := range e.NamedArgs {
		if na.Name == nil {
			continue
		}
		// An unresolved label is the resolver's to report.
		feature, ok := ec.resolver.ResolveReference(resolve.Reference{Scope: scope, QN: na.Name, Constructed: e.Type})
		if !ok || feature == nil {
			continue
		}
		if !ec.memberOf(typ, feature) {
			ec.errorf(na.Name.Span(), "%s is not a feature of %s", ast.QualifiedText(na.Name), typ.Name)
			continue
		}
		// A redefinition and its target are one feature: they share one binding.
		slot := ec.model.ConstructibleFeatureFor(typ, feature)
		if slot == nil {
			ec.errorf(na.Name.Span(), "%s", notConstructible(feature, typ))
			continue
		}
		if bound[slot] {
			ec.errorf(na.Name.Span(), "%s of %s is already bound by an earlier argument", feature.Name, typ.Name)
			continue
		}
		bound[slot] = true
		ec.checkFeatureBinding(scope, na.Value, feature, typ)
	}
	return semantics.PrimUnknown
}

// checkFeatureBinding reports a constructor argument its feature cannot take: the values written
// together by scalar type or each by conformance, each held element on its own, and the count.
func (ec *exprChecker) checkFeatureBinding(scope *symbols.Scope, arg ast.Node, feature, typ *symbols.Symbol) {
	u, ok := feature.Decl.(*ast.Usage)
	if !ok {
		return
	}
	want := ec.model.PrimTypeOf(feature)
	wants := w8cMostSpecific(ec.model, ec.model.DeclaredFeatureTypes(feature))
	written, held := ec.argumentElements(scope, arg)
	// A value with no scalar type (an object, a constructor) is checked by
	// conformance against the feature's types whether or not those are scalar.
	if got := ec.silent().commonElementType(scope, written); want != semantics.PrimUnknown && got != semantics.PrimUnknown {
		if !bindable(arg, got, want) {
			ec.errorf(arg.Span(), "%s of %s expects %s, found %s", feature.Name, typ.Name, want, got)
		}
	} else {
		for _, value := range written {
			ec.checkObjectBinding(value, value.Span(), ec.argumentTypeSymbols(scope, value), wants, feature, typ)
		}
	}
	for _, el := range held {
		if got := ec.heldPrim(el); want != semantics.PrimUnknown && got != semantics.PrimUnknown {
			if !bindable(el.Node, got, want) {
				ec.errorf(heldSpan(el, arg), "%s of %s expects %s, found %s", feature.Name, typ.Name, want, got)
			}
			continue
		}
		ec.checkObjectBinding(el.Node, heldSpan(el, arg), el.Types, wants, feature, typ)
	}
	if held, known := ec.heldCount(scope, arg); known {
		if r, ok := ec.effectiveRange(feature.OwnerScope, usageDecl(u), 0); ok {
			if msg := r.HeldViolation(held); msg != "" {
				ec.errorf(arg.Span(), "%s of %s: %s", feature.Name, typ.Name, msg)
			}
		}
	}
}

// checkObjectBinding checks one value a constructor argument binds, typed gots, against wants,
// the types its feature declares or inherits; a `new T(…)` is exactly a T, another may be a subtype.
func (ec *exprChecker) checkObjectBinding(value ast.Node, at source.Span, gots, wants []*symbols.Symbol, feature, typ *symbols.Symbol) {
	if len(gots) == 0 {
		return
	}
	_, exact := value.(*ast.ConstructorExpr)
	for _, want := range wants {
		if slices.ContainsFunc(gots, func(got *symbols.Symbol) bool {
			return ec.model.Conforms(got, want) || (!exact && ec.model.Conforms(want, got))
		}) {
			continue
		}
		ec.errorf(at, "%s of %s is typed by %s; cannot bind a value of type %s", feature.Name, typ.Name, want.Name, typeNames(gots))
	}
}

// argumentTypeSymbols is argumentTypeSymbol as a list, empty for none.
func (ec *exprChecker) argumentTypeSymbols(scope *symbols.Scope, value ast.Node) []*symbols.Symbol {
	if got := ec.argumentTypeSymbol(scope, value); got != nil {
		return []*symbols.Symbol{got}
	}
	return nil
}

// argumentTypeSymbol resolves a value's type in scope (feature reference or
// chain, invocation result, body, constructed type, literal scalar), or nil.
func (ec *exprChecker) argumentTypeSymbol(scope *symbols.Scope, value ast.Node) *symbols.Symbol {
	if got := ec.declaredValueType(scope, value); got != nil {
		return got
	}
	if got := ec.constructedTypeSymbol(scope, value); got != nil {
		return got
	}
	return ec.model.ScalarSymbol(literalPrimType(value))
}

// notConstructible words the rejection of a label for a member no constructor of
// typ binds: a feature every object of its kind has from a more general library.
func notConstructible(feature, typ *symbols.Symbol) string {
	msg := fmt.Sprintf("%s is not a feature a constructor of %s binds", feature.Name, typ.Name)
	if owner := ownerOf(feature); owner != nil && owner != typ {
		msg += fmt.Sprintf(": %s declares it for every %s; redefine it in %s to bind it", owner.Name, owner.Name, typ.Name)
	}
	return msg
}

// ownerOf returns the type declaring feature, or nil.
func ownerOf(feature *symbols.Symbol) *symbols.Symbol {
	if feature.OwnerScope == nil {
		return nil
	}
	return feature.OwnerScope.Owner()
}

// memberOf reports whether feature is a member typ declares or inherits.
func (ec *exprChecker) memberOf(typ, feature *symbols.Symbol) bool {
	for _, m := range ec.model.MembersOf(typ) {
		if m == feature {
			return true
		}
	}
	return false
}

// checkNamedArguments reports named arguments that name no `in` parameter of sym or do
// not bind to it, a parameter bound twice (by whichever name or position), and advises
// of default-less parameters no argument names.
func (ec *exprChecker) checkNamedArguments(scope *symbols.Scope, call invocation, report, advise reporter) {
	e, sym, args, params := call.e, call.sym, call.args, call.params
	// A receiver binds by position, which named arguments leave unstated; runtime/eval.go
	// reports the same call.
	if e.Operand != nil && ChainCallee(e) == nil {
		report(e.Span(), "%s cannot be called with a receiver and named arguments", sym.Name)
		return
	}
	if len(args) > len(params) {
		report(args[len(params)].Span(), "%s: %s takes %d argument(s), found %d", msgInvocationParameterRedefinition, sym.Name, len(params), len(args)+len(e.NamedArgs))
		return
	}
	bound := make([]bool, len(params))
	for i, arg := range args {
		bound[i] = true
		for _, m := range ec.argumentMismatches(scope, arg, params[i]) {
			report(m.span, "argument %d of %s %s", i+1, sym.Name, m.why)
		}
	}
	unknown := false
	for _, arg := range e.NamedArgs {
		if arg.Name == nil || len(arg.Name.Parts) == 0 {
			continue
		}
		at := ec.namedParameter(scope, sym, params, arg.Name)
		if at < 0 {
			report(arg.Name.Span(), "%s: %s has no parameter named %q", msgInvocationParameterRedefinition, sym.Name, semantics.QualifiedNameText(arg.Name))
			unknown = true
			continue
		}
		p := params[at]
		if bound[at] {
			report(arg.Value.Span(), "%s binds parameter %q twice", sym.Name, p.name())
			continue
		}
		bound[at] = true
		for _, m := range ec.argumentMismatches(scope, arg.Value, p) {
			report(m.span, "argument %s of %s %s", p.name(), sym.Name, m.why)
		}
	}
	// A misspelt name is the likelier cause of a parameter left unbound.
	if unknown {
		return
	}
	for i, p := range params {
		if !bound[i] && p.required(ec.model) {
			advise(e.Span(), msgUnboundParameter, sym.Name, p.name())
		}
	}
}

// namedParameter is the index in params of the parameter a named argument binds, or -1:
// the one named as written, else the one the name resolves to within sym.
func (ec *exprChecker) namedParameter(
	scope *symbols.Scope,
	sym *symbols.Symbol,
	params []parameter,
	name *ast.QualifiedName,
) int {
	if len(name.Parts) == 1 {
		if i := indexOfName(params, name.Parts[0].Text); i >= 0 {
			return i
		}
	}
	target, ok := ec.model.LookupBinding(scope, sym, name)
	if !ok {
		return -1
	}
	for i, p := range params {
		if p.usage == target.Decl {
			return i
		}
	}
	return -1
}

// parameterPrimType is the scalar type p is declared with, PrimUnknown when none is known.
func (ec *exprChecker) parameterPrimType(p parameter) semantics.PrimType {
	return ec.declaredPrimType(p.scope(), p.usage.Relationships)
}

// misbinding is a value an argument binds that does not bind to its parameter, and why.
type misbinding struct {
	span source.Span
	why  string
}

// argumentMismatches says why value does not bind to p (none when it does or a type is unknown):
// the values written are judged together, as a collection literal is; each held element on its own.
func (ec *exprChecker) argumentMismatches(scope *symbols.Scope, value ast.Node, p parameter) []misbinding {
	var out []misbinding
	written, held := ec.argumentElements(scope, value)
	if len(written) > 0 {
		got := ec.silent().argumentOf(scope, written)
		if why := ec.argumentMismatch(value, got.Prim, symbolList(got.Type), p); why != "" {
			out = append(out, misbinding{value.Span(), why})
		}
	}
	for _, el := range held {
		if why := ec.argumentMismatch(el.Node, ec.heldPrim(el), el.Types, p); why != "" {
			out = append(out, misbinding{heldSpan(el, value), why})
		}
	}
	return out
}

// symbolList is sym as a list, empty for nil.
func symbolList(sym *symbols.Symbol) []*symbols.Symbol {
	if sym == nil {
		return nil
	}
	return []*symbols.Symbol{sym}
}

// argumentMismatch says why value, of scalar type prim and declared types, does not bind to p (""
// when it does or a type is unknown); a Collection parameter takes any sequence, Element any element.
func (ec *exprChecker) argumentMismatch(value ast.Node, prim semantics.PrimType, types []*symbols.Symbol, p parameter) string {
	if want := ec.parameterPrimType(p); want != semantics.PrimUnknown {
		if prim == semantics.PrimUnknown || bindable(value, prim, want) {
			return ""
		}
		return fmt.Sprintf("expects %s, found %s", want, prim)
	}
	want := ec.declaredTypeSymbol(p.scope(), p.usage.Relationships)
	if want == nil || len(types) == 0 || semantics.IsCollection(want) || semantics.IsElementType(want) ||
		ec.boundTypesConform(nil, types, []*symbols.Symbol{want}) {
		return ""
	}
	return fmt.Sprintf("expects %s, found %s", want.Name, typeNames(types))
}

// isBehaviorKind reports the behavior kinds whose parameter lists are checked.
func isBehaviorKind(k symbols.SymbolKind) bool {
	switch k {
	case symbols.SymbolCalcDef, symbols.SymbolCalcUsage,
		symbols.SymbolActionDef, symbols.SymbolActionUsage:
		return true
	}
	return false
}

func isInvocationBehaviorKind(k symbols.SymbolKind) bool {
	switch k {
	case symbols.SymbolCalcDef, symbols.SymbolCalcUsage,
		symbols.SymbolActionDef, symbols.SymbolActionUsage,
		symbols.SymbolConstraintDef, symbols.SymbolConstraintUsage,
		symbols.SymbolStateDef, symbols.SymbolStateUsage:
		return true
	}
	return false
}

func (ec *exprChecker) isInvocationBehavior(sym *symbols.Symbol, visiting map[*symbols.Symbol]bool) bool {
	if sym == nil || visiting[sym] {
		return false
	}
	if isInvocationBehaviorKind(sym.Kind) || isBehaviorDeclaration(sym.Decl) {
		return true
	}
	if sym.Decl == nil {
		return false
	}
	visiting[sym] = true
	defer delete(visiting, sym)

	typed := make([]*symbols.Symbol, 0, 1)
	for _, rel := range semantics.RelationshipsOf(sym) {
		if rel == nil || rel.Kind != ast.RelTyping || rel.Target == nil {
			continue
		}
		target, ok := ec.resolver.ResolveTarget(sym.OwnerScope, rel.Target)
		if !ok || target == nil {
			return false
		}
		typed = append(typed, target)
	}
	if len(typed) != 1 {
		return false
	}
	return ec.isInvocationBehavior(typed[0], visiting)
}

func isBehaviorDeclaration(decl ast.Node) bool {
	switch d := decl.(type) {
	case *ast.Definition:
		return d.Kind == ast.DefBehavior || d.Kind == ast.DefState ||
			d.Kind == ast.DefPredicate || d.Kind == ast.DefConstraint
	case *ast.Usage:
		switch d.Kind {
		case ast.UsageBehavior, ast.UsageState, ast.UsagePredicate,
			ast.UsageExpr, ast.UsageConstraint, ast.UsageInteraction:
			return true
		}
	}
	return false
}

func (ec *exprChecker) isDefinitelyNonBehavior(sym *symbols.Symbol) bool {
	if sym == nil {
		return false
	}
	if isInvocationBehaviorKind(sym.Kind) || isBehaviorDeclaration(sym.Decl) {
		return false
	}
	if isRequirementDeclaration(sym.Decl) {
		return false
	}
	if sym.Decl != nil {
		switch sym.Decl.(type) {
		case *ast.Definition, *ast.Usage:
			return true
		}
	}
	switch sym.Kind {
	case symbols.SymbolUnknown, symbols.SymbolKerMLType,
		symbols.SymbolRequirementDef, symbols.SymbolRequirementUsage:
		return false
	default:
		return true
	}
}

func isRequirementDeclaration(decl ast.Node) bool {
	switch d := decl.(type) {
	case *ast.Definition:
		return d.Kind == ast.DefRequirement
	case *ast.Usage:
		return d.Kind == ast.UsageRequirement
	default:
		return false
	}
}

// parameter is one `in` parameter of an invoked behavior together with the
// symbol declaring it, whose scope its type names resolve in.
type parameter struct {
	usage *ast.Usage
	owner *symbols.Symbol
	// redefined is the inherited parameter this declaration redefines, whose
	// default and multiplicity it keeps where it states none (KerML 1.0 §7.3.4.5).
	redefined *parameter
}

// required reports whether an invocation must supply the parameter: it has no
// default, its own or inherited, and its multiplicity admits no omission.
func (p parameter) required(m *semantics.Model) bool {
	for q := &p; q != nil; q = q.redefined {
		if q.usage.Value != nil {
			return false
		}
	}
	for q := &p; q != nil; q = q.redefined {
		if q.usage.Multiplicity != nil {
			return !m.IsOptionalParameter(q.usage)
		}
	}
	return true
}

// name returns the name the parameter answers to, which a declaration written
// as a redefinition takes from what it redefines (`in redefines ifTest;`).
func (p parameter) name() string {
	name, _ := ast.EffectiveName(p.usage)
	return name
}

// scope returns the scope the parameter's type names resolve in, which is the
// declaring behavior's, not the invoking one's.
func (p parameter) scope() *symbols.Scope {
	if p.owner.Scope != nil {
		return p.owner.Scope
	}
	return p.owner.OwnerScope
}

// effectiveInParameters returns the `in` parameters a calc/action is invoked
// with, in inherited declaration order. A behavior inherits the signature of the
// types it specializes or is typed by; a parameter it declares itself replaces
// the inherited one it redefines and is appended otherwise, so a specialization
// refining only a subset of them keeps the full signature. The parameters a
// performing node declares in its body fold in the same way, so one it binds a
// value to is not required of the call. ok is false when no signature can be
// determined, in which case the invocation is left unchecked.
func (ec *exprChecker) effectiveInParameters(sym, node *symbols.Symbol) ([]parameter, bool) {
	// Merging runs over every parameter, because redefinition is positional
	// over the whole parameter list (KerML 7.4.7.2), as
	// semantics.Model.parametersOf computes it; only the invocation signature
	// is restricted to the inputs.
	all := ec.mergedParameters(sym, map[*symbols.Symbol]bool{})
	if node != nil {
		all = mergeParameters(all, node)
	}
	var params []parameter
	for _, p := range all {
		if p.usage.Direction == ast.DirIn || p.usage.Direction == ast.DirInOut {
			params = append(params, p)
		}
	}
	if len(params) > 0 {
		return params, true
	}
	// A parameterless declaration with no supertypes really takes no
	// arguments; with supertypes the signature may live somewhere the checker
	// cannot see, so stay silent.
	return nil, len(ec.model.DirectSupertypes(sym)) == 0 && sym.Decl != nil
}

// mergedParameters returns sym's parameter list: the parameter lists of the
// types it specializes or is typed by, in declaration order, with sym's own
// parameters folded in. Recursing keeps each type's parameters positioned
// against the ones it actually inherits, which is what implicit redefinition is
// relative to; visiting is the set of symbols on the current path, which breaks
// cycles.
func (ec *exprChecker) mergedParameters(sym *symbols.Symbol, visiting map[*symbols.Symbol]bool) []parameter {
	if visiting[sym] {
		return nil
	}
	visiting[sym] = true
	defer delete(visiting, sym)

	var inherited []parameter
	for _, super := range ec.model.DirectSupertypes(sym) {
		for _, p := range ec.mergedParameters(super, visiting) {
			// A parameter reached through more than one supertype (a
			// diamond) contributes one signature entry.
			if i := indexOfName(inherited, p.name()); i >= 0 {
				inherited[i] = p
				continue
			}
			inherited = append(inherited, p)
		}
	}
	return mergeParameters(inherited, sym)
}

// mergeParameters returns a symbol's parameter list: the ones it declares, in
// declaration order, followed by the inherited ones none of them redefines. A
// declaration redefines the inherited parameter its `:>>` names, or, failing
// that, the one at its own position with the same direction; only once the inherited parameters are used
// up does a declaration purely add to the list. This is the order and the
// matching semantics.Model.parametersOf derives (KerML 7.4.7.2), so both tiers
// see one parameter list.
func mergeParameters(inherited []parameter, sym *symbols.Symbol) []parameter {
	declared := declaredParameters(sym)
	if len(declared) == 0 {
		return inherited
	}
	merged := make([]parameter, 0, len(declared)+len(inherited))
	claimed := make([]bool, len(inherited))
	for position, u := range declared {
		p := parameter{usage: u, owner: sym}
		i := indexOfRedefined(inherited, u)
		if i < 0 {
			i = position
		}
		// A position whose directions disagree is not a redefinition, so the
		// inherited parameter there stays in the list.
		if i < len(inherited) && inherited[i].usage.Direction == u.Direction {
			claimed[i] = true
			p.redefined = &inherited[i]
		}
		merged = append(merged, p)
	}
	for i, p := range inherited {
		if !claimed[i] {
			merged = append(merged, p)
		}
	}
	return merged
}

// indexOfRedefined finds the inherited parameter a declaration redefines, named
// by its `:>>` target. A declaration with no explicit target redefines by
// position (KerML 7.4.7.2), which the caller applies, so its own name does not
// select the inherited parameter.
func indexOfRedefined(params []parameter, u *ast.Usage) int {
	for _, name := range redefinedNames(u) {
		if i := indexOfName(params, name); i >= 0 {
			return i
		}
	}
	return -1
}

// indexOfName finds the parameter with the given name, or -1.
func indexOfName(params []parameter, name string) int {
	if name == "" {
		return -1
	}
	for i, p := range params {
		if p.name() == name {
			return i
		}
	}
	return -1
}

// redefinedNames returns the unqualified names a usage redefines (`:>>`).
func redefinedNames(u *ast.Usage) []string {
	var names []string
	for _, rel := range u.Relationships {
		if rel == nil || rel.Kind != ast.RelRedefines {
			continue
		}
		qn, ok := rel.Target.(*ast.QualifiedName)
		if !ok || len(qn.Parts) == 0 {
			continue
		}
		names = append(names, qn.Parts[len(qn.Parts)-1].Text)
	}
	return names
}

// declaredParameters returns the parameters declared directly by a symbol's
// def/usage declaration: its directed features, minus the result parameter,
// which is redefined as the result rather than by position.
func declaredParameters(sym *symbols.Symbol) []*ast.Usage {
	var members []ast.Node
	switch d := sym.Decl.(type) {
	case *ast.Definition:
		members = d.Members
	case *ast.Usage:
		members = d.Members
	default:
		return nil
	}
	var params []*ast.Usage
	for _, m := range members {
		u, ok := unwrapType(m).(*ast.Usage)
		if !ok {
			continue
		}
		if u.Direction != ast.DirNone && !u.IsResult {
			params = append(params, u)
		}
	}
	return params
}
