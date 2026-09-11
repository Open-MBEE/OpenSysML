package semantics

import (
	"errors"
	"fmt"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/lexer"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// Element filters (KerML 8.2.4 ElementFilterMembership, SysML v2 7.4.4) select
// which of a namespace's imported memberships are memberships of it, and which
// elements an import brings in. A filter condition is a predicate over one
// candidate *element* — not over a value — evaluated with the candidate as the
// implicit `self`, so it is answered here, against symbols, and not by the
// runtime value evaluator, which has no instance to evaluate against while names
// are still being resolved.
//
// Evaluation is in two steps, both memoized: a condition is compiled once to a
// symbols.FilterPredicate whose element references are resolved, and the
// predicate is then run against each candidate. The compiled form is also what a
// library carries in its index cache, so a filtered import selects the same
// elements whether its library was parsed or restored.
//
// Reading a feature bound to nothing yields the empty sequence, which orderings
// (declared over `DataValue[1]`) propagate and `==`/`!=` (over `[0..1]`) decide
// against any value. Nothing is not true, so the candidate is not selected: a
// verdict, not the unevaluable case, and so not reported.

var (
	// ErrFilterUnevaluable reports a filter condition outside the subset of
	// predicates the evaluator implements, or one naming something that does
	// not resolve. Such a condition selects nothing and rejects nothing: it is
	// reported, and the candidate is kept, so that an unevaluable filter never
	// silently hides model content.
	ErrFilterUnevaluable = errors.New("filter condition cannot be evaluated")

	// ErrFilterNotBoolean reports a filter condition that evaluates to
	// something other than a boolean, which cannot select elements.
	ErrFilterNotBoolean = errors.New("filter condition is not boolean-valued")
)

// FilterError is why a filter condition could not decide a candidate. Reason
// describes the part at fault in the terms a diagnostic message uses, and Span
// locates it.
type FilterError struct {
	Reason string
	Span   source.Span
	Err    error
}

func (e *FilterError) Error() string {
	if e.Reason == "" {
		return e.Err.Error()
	}
	return e.Err.Error() + ": " + e.Reason
}

func (e *FilterError) Unwrap() error { return e.Err }

// filterKey memoizes one candidate's verdict for one compiled condition.
type filterKey struct {
	pred *symbols.FilterPredicate
	cand *symbols.Symbol
}

// filterVerdict is a decided candidate: the truth value, or why there is none.
type filterVerdict struct {
	value bool
	err   error
}

// SatisfiesElementFilter reports whether cand is selected by the filter
// condition. It is the form the symbol layer's enumeration installs (see
// symbols.Index.SetElementFilter): a condition that cannot be evaluated, or that
// is not boolean-valued, keeps the candidate rather than dropping it — the
// diagnostic is what reports it (see passes.checkElementFilters), because losing
// model content silently is worse than surfacing an element a filter meant to
// hide.
func (m *Model) SatisfiesElementFilter(f symbols.ElementFilter, cand *symbols.Symbol) bool {
	ok, err := m.EvalElementFilter(f, cand)
	if err != nil {
		return true
	}
	return ok
}

// EvalElementFilter evaluates a filter condition for one candidate element,
// returning whether the candidate is selected, or a *FilterError saying why no
// verdict is possible.
func (m *Model) EvalElementFilter(f symbols.ElementFilter, cand *symbols.Symbol) (bool, error) {
	if cand == nil {
		return true, &FilterError{Err: ErrFilterUnevaluable, Reason: "there is no element to evaluate it for", Span: f.Span}
	}
	pred := m.CompileElementFilter(f)
	if pred == nil {
		return true, &FilterError{Err: ErrFilterUnevaluable, Reason: "the condition is empty", Span: f.Span}
	}
	key := filterKey{pred: pred, cand: cand}
	if v, ok := m.filterVerdicts[key]; ok {
		return v.value, v.err
	}
	val, err := m.evalPredicate(pred, cand)
	if err == nil && val.Kind == symbols.FilterValueEmpty {
		val = boolValue(false) // nothing is not true, so it selects nothing
	}
	if err == nil && val.Kind != symbols.FilterValueBool {
		err = &FilterError{Err: ErrFilterNotBoolean, Reason: describeValueKind(val.Kind), Span: pred.Span}
	}
	verdict := filterVerdict{value: err == nil && val.Bool, err: err}
	// Only a decided verdict is remembered. A condition can be unevaluable
	// because the element it names is not indexed yet — an import is expanded
	// while the index is still filling — and re-deciding it later is what keeps
	// the answer from depending on when it was first asked.
	if err == nil {
		m.filterVerdicts[key] = verdict
	}
	return verdict.value, verdict.err
}

// CompileElementFilter returns the condition compiled to a predicate over a
// candidate element, by resolving the elements it names against the scope it was
// written in. Returns nil for an empty condition.
func (m *Model) CompileElementFilter(f symbols.ElementFilter) *symbols.FilterPredicate {
	if f.Expr == nil {
		return nil
	}
	if pred, ok := m.filterPreds[f.Expr]; ok {
		return pred
	}
	// The names a condition uses are not subject to the condition itself, so
	// they resolve through the namespace's imports unfiltered.
	var pred *symbols.FilterPredicate
	m.resolver.InCondition(func() { pred = m.compileCondition(f.Scope, f.Expr) })
	m.filterPreds[f.Expr] = pred
	return pred
}

// compileCondition compiles one filter expression, resolving every element it
// names. A part of the condition it does not implement compiles to a
// FilterUnsupported node carrying the reason, so that the whole condition is
// still a predicate — one that reports instead of deciding.
func (m *Model) compileCondition(scope *symbols.Scope, n ast.Node) *symbols.FilterPredicate {
	switch e := n.(type) {
	case *ast.OperatorExpr:
		if v, ok := evalConst(n); ok {
			return &symbols.FilterPredicate{Op: symbols.FilterConst, Value: constValue(v), Span: spanOf(n)}
		}
		return m.compileOperator(scope, e)
	case *ast.FeatureChainExpr:
		return m.compileFeatureChain(scope, e)
	case *ast.FeatureReference:
		return m.compileReference(scope, e.Name, spanOf(e))
	case *ast.LiteralInteger, *ast.LiteralReal, *ast.LiteralBool, *ast.LiteralInfinity:
		if v, ok := evalConst(n); ok {
			return &symbols.FilterPredicate{Op: symbols.FilterConst, Value: constValue(v), Span: spanOf(n)}
		}
	case *ast.LiteralString:
		return &symbols.FilterPredicate{
			Op:    symbols.FilterConst,
			Value: symbols.FilterValue{Kind: symbols.FilterValueString, Str: unquote(e.Value)},
			Span:  spanOf(e),
		}
	case *ast.NullExpr:
		// `null` is the empty sequence, and model-level evaluable (KerML 7.4.9).
		return &symbols.FilterPredicate{Op: symbols.FilterConst, Value: emptyValue(), Span: spanOf(e)}
	case *ast.ConstructorExpr:
		return m.compileConstructor(scope, e)
	}
	return unsupported(spanOf(n), fmt.Sprintf("%s is not a supported filter condition", describeNode(n)))
}

// compileConstructor compiles `new T(…)`: model-level evaluable when its
// arguments are (KerML 7.4.9), and yielding an instance, never a truth value.
func (m *Model) compileConstructor(scope *symbols.Scope, e *ast.ConstructorExpr) *symbols.FilterPredicate {
	span := spanOf(e)
	if e.Type == nil {
		return unsupported(span, "the constructor names no type")
	}
	typeFQN, ok := m.resolveFQN(scope, e.Type)
	if !ok {
		return unsupported(span, fmt.Sprintf("the constructed type %s does not resolve", qnText(e.Type)))
	}
	for _, arg := range constructorArgs(e) {
		if p := m.compileCondition(scope, arg); p.Op == symbols.FilterUnsupported {
			return p
		}
	}
	for _, arg := range e.NamedArgs {
		if p := m.compileCondition(scope, arg.Value); p.Op == symbols.FilterUnsupported {
			return p
		}
	}
	return &symbols.FilterPredicate{
		Op:    symbols.FilterConst,
		Value: symbols.FilterValue{Kind: symbols.FilterValueInstance, RefFQN: typeFQN},
		Span:  span,
	}
}

// compileOperator compiles the operators a filter condition is written with:
// classification against a metadata type, boolean composition, and comparison.
func (m *Model) compileOperator(scope *symbols.Scope, e *ast.OperatorExpr) *symbols.FilterPredicate {
	span := spanOf(e)
	switch e.Operator {
	case ast.OpAt, ast.OpMetaAt:
		return m.compileClassification(scope, e)

	case ast.OpNot:
		if len(e.Operands) != 1 {
			return unsupported(span, "`not` needs one operand")
		}
		return &symbols.FilterPredicate{
			Op:       symbols.FilterNot,
			Operands: []*symbols.FilterPredicate{m.compileCondition(scope, e.Operands[0])},
			Span:     span,
		}

	case ast.OpAnd, ast.OpConditionalAnd, ast.OpOr, ast.OpConditionalOr,
		ast.OpXor, ast.OpImplies, ast.OpEq, ast.OpNeq, ast.OpLt, ast.OpLe,
		ast.OpGt, ast.OpGe:
		if len(e.Operands) != 2 {
			return unsupported(span, fmt.Sprintf("`%s` needs two operands", e.Operator))
		}
		return &symbols.FilterPredicate{
			Op: binaryFilterOp(e.Operator),
			Operands: []*symbols.FilterPredicate{
				m.compileCondition(scope, e.Operands[0]),
				m.compileCondition(scope, e.Operands[1]),
			},
			Span: span,
		}
	}
	return unsupported(span, fmt.Sprintf("the `%s` operator is not supported in a filter condition", e.Operator))
}

// compileClassification compiles `@T` and `@@T`. The operand is the element the
// condition is evaluated for, which the syntax leaves out; an explicit `self`
// says the same thing, and anything else names another element, which a filter
// condition cannot reach.
func (m *Model) compileClassification(scope *symbols.Scope, e *ast.OperatorExpr) *symbols.FilterPredicate {
	if len(e.Operands) > 1 || (len(e.Operands) == 1 && !isSelfReference(e.Operands[0])) {
		return unsupported(spanOf(e), fmt.Sprintf("`%s` in a filter condition classifies the element being filtered, so it takes no left operand", e.Operator))
	}
	return m.classificationPredicate(scope, e)
}

// classificationPredicate compiles the test `@T`/`@@T` states, leaving the
// element it is evaluated for to the caller: a filter condition supplies the
// candidate, and the value evaluator the element its subject denotes.
func (m *Model) classificationPredicate(scope *symbols.Scope, e *ast.OperatorExpr) *symbols.FilterPredicate {
	span := spanOf(e)
	op := symbols.FilterClassify
	if e.Operator == ast.OpMetaAt {
		op = symbols.FilterMetaClassify
	}
	if e.TypeRef == nil {
		return unsupported(span, fmt.Sprintf("`%s` names no type", e.Operator))
	}
	typeFQN, ok := m.resolveFQN(scope, e.TypeRef)
	if !ok {
		return unsupported(span, fmt.Sprintf("the metadata type %s does not resolve", qnText(e.TypeRef)))
	}
	return &symbols.FilterPredicate{Op: op, TypeFQN: typeFQN, Span: span}
}

// EvalClassification evaluates the classification `@T`/`@@T` that e writes
// against one element, for a caller that has settled which element the subject
// denotes — the runtime value evaluator, whose `@` has a subject expression a
// filter condition has no equivalent of. e's operand is not looked at here.
//
// The type is resolved and the verdict decided by the same code a filter
// condition is compiled and run with, so a classification cannot answer one way
// in a filter and another in an expression. A condition outside the evaluable
// subset — a type that does not resolve — is a *FilterError wrapping
// ErrFilterUnevaluable, as it is at the model level.
func (m *Model) EvalClassification(scope *symbols.Scope, e *ast.OperatorExpr, elem *symbols.Symbol) (bool, error) {
	if e == nil {
		return false, &FilterError{Err: ErrFilterUnevaluable, Reason: "there is no classification to evaluate"}
	}
	if elem == nil {
		return false, UnevaluableClassification("there is no element to classify", spanOf(e))
	}
	pred := m.classificationPredicate(scope, e)
	val, err := m.evalPredicate(pred, m.indexedElement(elem))
	if err != nil {
		return false, err
	}
	if val.Kind != symbols.FilterValueBool {
		return false, &FilterError{Err: ErrFilterNotBoolean, Reason: describeValueKind(val.Kind), Span: pred.Span}
	}
	return val.Bool, nil
}

// UnevaluableClassification reports a classification whose subject the caller
// could not settle — an expression denoting no element — as the same
// ErrFilterUnevaluable a filter condition outside the evaluable subset reports.
func UnevaluableClassification(reason string, span source.Span) error {
	return &FilterError{Err: ErrFilterUnevaluable, Reason: reason, Span: span}
}

// compileFeatureChain compiles the value of a feature of an annotation of the
// candidate: `(as Safety).isMandatory` reads isMandatory from the candidate's
// Safety annotation.
func (m *Model) compileFeatureChain(scope *symbols.Scope, e *ast.FeatureChainExpr) *symbols.FilterPredicate {
	span := spanOf(e)
	if e.Member == nil || len(e.Member.Parts) != 1 {
		if e.Member != nil {
			resultType, ok := m.resolver.ResolveTarget(scope, e)
			if !ok || resultType == nil {
				return unresolvedReference(span, "a feature chain does not resolve")
			}
			return evaluatorUnsupported(span, chainLimitation, resultType)
		}
		return unsupported(span, "a filter condition reads a single feature of an annotation")
	}
	feature := e.Member.Parts[0].Text
	switch operand := e.Operand.(type) {
	case *ast.CastExpr:
		if operand.TargetType == nil {
			return unsupported(span, "the cast names no type")
		}
		typeFQN, ok := m.resolveFQN(scope, operand.TargetType)
		if !ok {
			return unsupported(span, fmt.Sprintf("the metadata type %s does not resolve", qnText(operand.TargetType)))
		}
		return &symbols.FilterPredicate{
			Op:         symbols.FilterFeature,
			TypeFQN:    typeFQN,
			Feature:    feature,
			ResultType: m.filterMemberType(typeFQN, feature),
			Span:       span,
		}
	case *ast.OperatorExpr:
		// `self.f` and `(self as T).f` are written this way too; only the
		// annotation form carries a metadata type to read the feature from.
		if operand.Operator == ast.OpAs && operand.TypeRef != nil {
			typeFQN, ok := m.resolveFQN(scope, operand.TypeRef)
			if !ok {
				return unsupported(span, fmt.Sprintf("the metadata type %s does not resolve", qnText(operand.TypeRef)))
			}
			return &symbols.FilterPredicate{
				Op:         symbols.FilterFeature,
				TypeFQN:    typeFQN,
				Feature:    feature,
				ResultType: m.filterMemberType(typeFQN, feature),
				Span:       span,
			}
		}
	case *ast.FeatureReference:
		return m.compileFeatureChainRead(scope, operand, feature, span)
	}
	if root := chainRoot(e); root != nil {
		if rootSym, ok := m.resolver.ResolveTarget(scope, root); ok {
			if reason, notEvaluable := m.referenceNotEvaluable(rootSym); notEvaluable {
				return unsupported(span, reason)
			}
		}
	}
	resultType, ok := m.resolver.ResolveTarget(scope, e)
	if !ok || resultType == nil {
		return unresolvedReference(span, "a feature chain does not resolve")
	}
	return evaluatorUnsupported(span, chainLimitation, resultType)
}

// chainLimitation is the reason a chain outside the evaluable subset reports.
const chainLimitation = "a filter condition reads a feature through a chain of features, which OpenSysML does not evaluate (known limitation: the reference accepts chains rooted in a feature with no featuring type)"

// compileFeatureChainRead compiles `p.n`: a chain rooted in a feature with no
// featuring type is model-level evaluable when the feature it reads has a
// constant value ([KerML, 7.4.9] isModelLevelEvaluable).
func (m *Model) compileFeatureChainRead(scope *symbols.Scope, operand *ast.FeatureReference, feature string, span source.Span) *symbols.FilterPredicate {
	if operand.Name == nil {
		return unresolvedReference(span, "the chain operand names nothing")
	}
	root, ok := m.resolver.ResolveQualified(scope, operand.Name)
	if !ok || root == nil {
		return unresolvedReference(span, fmt.Sprintf("%s does not resolve", qnText(operand.Name)))
	}
	if reason, notEvaluable := m.referenceNotEvaluable(root); notEvaluable {
		return unsupported(span, reason)
	}
	read, ok := m.LookupMember(root, feature)
	if !ok || read == nil {
		return unresolvedReference(span, fmt.Sprintf("%s has no feature %s to read", qnText(operand.Name), feature))
	}
	usage, isUsage := read.Decl.(*ast.Usage)
	if !isUsage || usage.Value == nil {
		return evaluatorUnsupported(span, chainLimitation, read)
	}
	v, ok := evalConst(usage.Value)
	if !ok {
		return evaluatorUnsupported(span, chainLimitation, read)
	}
	return &symbols.FilterPredicate{
		Op:         symbols.FilterConst,
		Value:      constValue(v),
		ResultType: read,
		Span:       span,
	}
}

// compileReference compiles a name a filter condition uses as a value: a feature
// of a metadata type, read from the candidate's annotation of that type
// (`Safety::level`), or an element compared by identity, such as an enumeration
// literal.
func (m *Model) compileReference(scope *symbols.Scope, qn *ast.QualifiedName, span source.Span) *symbols.FilterPredicate {
	if qn == nil || len(qn.Parts) == 0 {
		return unsupported(span, "the reference names nothing")
	}
	sym, ok := m.resolver.ResolveQualified(scope, qn)
	if !ok || sym == nil {
		return unresolvedReference(span, fmt.Sprintf("%s does not resolve", qnText(qn)))
	}
	if owner := m.ownerOf(sym); owner != nil && IsMetadataType(owner) {
		ownerFQN := m.fqnOf(owner)
		if ownerFQN == "" {
			return unsupported(span, fmt.Sprintf("the metadata type of %s has no qualified name", qnText(qn)))
		}
		return &symbols.FilterPredicate{
			Op:         symbols.FilterFeature,
			TypeFQN:    ownerFQN,
			Feature:    simpleSymbolName(sym),
			ResultType: sym,
			Span:       span,
		}
	}
	if reason, notEvaluable := m.referenceNotEvaluable(sym); notEvaluable {
		return unsupported(span, reason)
	}
	fqn := m.fqnOf(sym)
	if fqn == "" {
		return unsupported(span, fmt.Sprintf("%s has no qualified name to compare", qnText(qn)))
	}
	return &symbols.FilterPredicate{
		Op:         symbols.FilterConst,
		Value:      symbols.FilterValue{Kind: symbols.FilterValueRef, RefFQN: fqn},
		ResultType: sym,
		Span:       span,
	}
}

func (m *Model) filterMemberType(typeFQN, feature string) *symbols.Symbol {
	typ := m.symbolByFQN(typeFQN)
	if typ == nil {
		return nil
	}
	member, _ := m.LookupMember(typ, feature)
	return member
}

func chainRoot(n ast.Node) ast.Node {
	for {
		chain, ok := n.(*ast.FeatureChainExpr)
		if !ok {
			return n
		}
		n = chain.Operand
	}
}

func (m *Model) referenceNotEvaluable(sym *symbols.Symbol) (string, bool) {
	if sym == nil {
		return "the reference does not resolve", true
	}
	if resolved, ok := m.resolver.ResolveAliasTarget(sym); ok && resolved != nil {
		sym = resolved
	}
	owner := m.ownerOf(sym)
	if IsMetadataType(owner) {
		return "", false
	}
	if isFeaturingType(owner) {
		return fmt.Sprintf(
			"%s is featured within type %s and is not model-level evaluable",
			m.fqnOf(sym), m.fqnOf(owner)), true
	}
	usage, ok := sym.Decl.(*ast.Usage)
	if !ok || usage.Value == nil {
		return "", false
	}
	if !m.ModelLevelEvaluable(sym.OwnerScope, usage.Value) {
		return fmt.Sprintf("%s is not model-level evaluable", m.fqnOf(sym)), true
	}
	return "", false
}

func unresolvedReference(span source.Span, reason string) *symbols.FilterPredicate {
	return &symbols.FilterPredicate{
		Op:              symbols.FilterUnsupported,
		UnsupportedKind: symbols.FilterUnsupportedNotBoolean,
		Reason:          reason,
		Span:            span,
	}
}

func describeResultType(sym *symbols.Symbol) string {
	if sym == nil {
		return "an unsupported expression"
	}
	if name := simpleSymbolName(sym); name != "" {
		return "a " + name + " result"
	}
	return "a non-Boolean result"
}

func isFeaturingType(sym *symbols.Symbol) bool {
	if sym == nil {
		return false
	}
	switch sym.Kind {
	case symbols.SymbolPartDef, symbols.SymbolAttributeDef,
		symbols.SymbolItemDef, symbols.SymbolOccurrenceDef,
		symbols.SymbolIndividualDef, symbols.SymbolViewDef,
		symbols.SymbolViewpointDef, symbols.SymbolRenderingDef,
		symbols.SymbolConcernDef, symbols.SymbolConnectionDef,
		symbols.SymbolFlowDef, symbols.SymbolPortDef,
		symbols.SymbolInterfaceDef, symbols.SymbolAllocationDef,
		symbols.SymbolActionDef, symbols.SymbolStateDef,
		symbols.SymbolCalcDef, symbols.SymbolConstraintDef,
		symbols.SymbolRequirementDef, symbols.SymbolCaseDef,
		symbols.SymbolAnalysisCaseDef, symbols.SymbolVerificationCaseDef,
		symbols.SymbolUseCaseDef, symbols.SymbolPartUsage,
		symbols.SymbolAttributeUsage, symbols.SymbolItemUsage,
		symbols.SymbolOccurrenceUsage, symbols.SymbolIndividualUsage,
		symbols.SymbolMetadataUsage, symbols.SymbolViewUsage,
		symbols.SymbolViewpointUsage, symbols.SymbolRenderingUsage,
		symbols.SymbolConcernUsage, symbols.SymbolConnectionUsage,
		symbols.SymbolSuccessionUsage,
		symbols.SymbolFlowUsage, symbols.SymbolPortUsage,
		symbols.SymbolInterfaceUsage, symbols.SymbolAllocationUsage,
		symbols.SymbolActionUsage, symbols.SymbolStateUsage,
		symbols.SymbolCalcUsage, symbols.SymbolConstraintUsage,
		symbols.SymbolRequirementUsage, symbols.SymbolCaseUsage,
		symbols.SymbolAnalysisCaseUsage, symbols.SymbolVerificationCaseUsage,
		symbols.SymbolUseCaseUsage, symbols.SymbolKerMLType:
		return true
	}
	return false
}

// evalPredicate runs a compiled condition against one candidate element.
func (m *Model) evalPredicate(p *symbols.FilterPredicate, cand *symbols.Symbol) (symbols.FilterValue, error) {
	switch p.Op {
	case symbols.FilterUnsupported:
		return symbols.FilterValue{}, &FilterError{Err: ErrFilterUnevaluable, Reason: p.Reason, Span: p.Span}

	case symbols.FilterConst:
		return p.Value, nil

	case symbols.FilterClassify:
		return boolValue(m.annotatedBy(cand, p.TypeFQN)), nil

	case symbols.FilterMetaClassify:
		return boolValue(m.metaclassConforms(cand, p.TypeFQN)), nil

	case symbols.FilterFeature:
		return m.featureValue(cand, p)

	case symbols.FilterNot:
		v, err := m.evalBool(p.Operands[0], cand)
		if err != nil {
			return symbols.FilterValue{}, err
		}
		if v.Kind == symbols.FilterValueEmpty {
			return v, nil
		}
		return boolValue(!v.Bool), nil

	case symbols.FilterAnd, symbols.FilterOr, symbols.FilterXor, symbols.FilterImplies:
		left, err := m.evalBool(p.Operands[0], cand)
		if err != nil {
			return symbols.FilterValue{}, err
		}
		// Nothing to decide from yields nothing, leaving the candidate
		// unselected rather than the condition rejected.
		if left.Kind == symbols.FilterValueEmpty {
			return left, nil
		}
		// `and`, `or` and `implies` are decided by their left operand alone
		// where it settles the answer, so the right one is not evaluated. This
		// is what makes the guarded form filters are written in work: in
		// `@Safety and (as Safety).level > 4` the feature is only read from an
		// element the guard established has that annotation to read it from.
		if decided, ok := shortCircuit(p.Op, left.Bool); ok {
			return boolValue(decided), nil
		}
		right, err := m.evalBool(p.Operands[1], cand)
		if err != nil {
			return symbols.FilterValue{}, err
		}
		if right.Kind == symbols.FilterValueEmpty {
			return right, nil
		}
		return boolValue(evalFilterBool(p.Op, left.Bool, right.Bool)), nil

	case symbols.FilterEq, symbols.FilterNeq, symbols.FilterLt, symbols.FilterLe,
		symbols.FilterGt, symbols.FilterGe:
		return m.evalComparison(p, cand)
	}
	return symbols.FilterValue{}, &FilterError{Err: ErrFilterUnevaluable, Reason: "unknown filter operation", Span: p.Span}
}

// evalBool evaluates an operand a boolean operator needs: a boolean, or nothing
// at all, which the operator propagates.
func (m *Model) evalBool(p *symbols.FilterPredicate, cand *symbols.Symbol) (symbols.FilterValue, error) {
	v, err := m.evalPredicate(p, cand)
	if err != nil {
		return symbols.FilterValue{}, err
	}
	if v.Kind != symbols.FilterValueBool && v.Kind != symbols.FilterValueEmpty {
		return symbols.FilterValue{}, &FilterError{Err: ErrFilterNotBoolean, Reason: describeValueKind(v.Kind), Span: p.Span}
	}
	return v, nil
}

// shortCircuit reports the value a boolean operator's left operand settles on
// its own, and whether it does.
func shortCircuit(op symbols.FilterOp, left bool) (bool, bool) {
	switch op {
	case symbols.FilterAnd:
		return false, !left
	case symbols.FilterOr:
		return true, left
	case symbols.FilterImplies:
		return true, !left
	default: // symbols.FilterXor needs both operands
		return false, false
	}
}

// evalFilterBool applies a boolean operator to both operands, for the cases the
// left one does not settle.
func evalFilterBool(op symbols.FilterOp, l, r bool) bool {
	switch op {
	case symbols.FilterAnd:
		return l && r
	case symbols.FilterOr:
		return l || r
	case symbols.FilterXor:
		return l != r
	default: // symbols.FilterImplies
		return !l || r
	}
}

// evalComparison compares two operands of a filter condition: numbers and
// booleans by value, strings by text, and elements — an enumeration literal, or
// an annotation feature bound to one — by identity.
func (m *Model) evalComparison(p *symbols.FilterPredicate, cand *symbols.Symbol) (symbols.FilterValue, error) {
	left, err := m.evalPredicate(p.Operands[0], cand)
	if err != nil {
		return symbols.FilterValue{}, err
	}
	right, err := m.evalPredicate(p.Operands[1], cand)
	if err != nil {
		return symbols.FilterValue{}, err
	}
	equality := p.Op == symbols.FilterEq || p.Op == symbols.FilterNeq
	if left.Kind == symbols.FilterValueEmpty || right.Kind == symbols.FilterValueEmpty {
		if !equality { // an ordering needs a value on each side to compare
			return emptyValue(), nil
		}
		// `==` is declared over `[0..1]`: nothing equals only nothing.
		both := left.Kind == right.Kind
		return boolValue(both == (p.Op == symbols.FilterEq)), nil
	}
	if equality && (left.Kind == symbols.FilterValueRef || right.Kind == symbols.FilterValueRef ||
		left.Kind == symbols.FilterValueString || right.Kind == symbols.FilterValueString ||
		left.Kind == symbols.FilterValueBool || right.Kind == symbols.FilterValueBool) {
		if left.Kind != right.Kind {
			return symbols.FilterValue{}, &FilterError{
				Err:    ErrFilterUnevaluable,
				Reason: fmt.Sprintf("comparing %s with %s", describeValueKind(left.Kind), describeValueKind(right.Kind)),
				Span:   p.Span,
			}
		}
		same := left == right
		return boolValue(same == (p.Op == symbols.FilterEq)), nil
	}
	if left.Kind == symbols.FilterValueQuantity || right.Kind == symbols.FilterValueQuantity {
		return m.compareQuantityValues(p, left, right)
	}
	l, lok := numericValue(left)
	r, rok := numericValue(right)
	if !lok || !rok {
		return symbols.FilterValue{}, &FilterError{
			Err:    ErrFilterUnevaluable,
			Reason: fmt.Sprintf("comparing %s with %s", describeValueKind(left.Kind), describeValueKind(right.Kind)),
			Span:   p.Span,
		}
	}
	switch p.Op {
	case symbols.FilterEq:
		return boolValue(l == r), nil
	case symbols.FilterNeq:
		return boolValue(l != r), nil
	case symbols.FilterLt:
		return boolValue(l < r), nil
	case symbols.FilterLe:
		return boolValue(l <= r), nil
	case symbols.FilterGt:
		return boolValue(l > r), nil
	default: // symbols.FilterGe
		return boolValue(l >= r), nil
	}
}

// compareQuantityValues compares a quantity with another quantity or a bare
// number in the left operand's unit; incommensurable units are unevaluable,
// never an inequality decided over magnitudes. Points on two measurement
// scales, or on one and in a unit, meet only through the scale's anchor, a
// value the runtime reads; the filter leaves them unevaluated.
func (m *Model) compareQuantityValues(p *symbols.FilterPredicate, left, right symbols.FilterValue) (symbols.FilterValue, error) {
	lq, lok := asQuantity(left)
	rq, rok := asQuantity(right)
	if !lok || !rok {
		return symbols.FilterValue{}, &FilterError{
			Err:    ErrFilterUnevaluable,
			Reason: fmt.Sprintf("comparing %s with %s", describeValueKind(left.Kind), describeValueKind(right.Kind)),
			Span:   p.Span,
		}
	}
	lscale, lpoint := m.MeasurementScaleOf(lq.Unit.Term)
	rscale, rpoint := m.MeasurementScaleOf(rq.Unit.Term)
	if (lpoint || rpoint) && lscale != rscale && !isBareZero(*lq) && !isBareZero(*rq) {
		scale, other := lscale, rq.Unit
		if scale == nil {
			scale, other = rscale, lq.Unit
		}
		return symbols.FilterValue{}, &FilterError{
			Err:    ErrFilterUnevaluable,
			Reason: fmt.Sprintf("comparing a point on the measurement scale %s with %s: the scale's anchor is a value only the runtime reads", m.fqnOf(scale), other),
			Span:   p.Span,
		}
	}
	c, err := CompareMagnitudes(*lq, *rq)
	if err != nil {
		return symbols.FilterValue{}, &FilterError{Err: ErrFilterUnevaluable, Reason: err.Error(), Span: p.Span}
	}
	switch p.Op {
	case symbols.FilterEq:
		return boolValue(c == 0), nil
	case symbols.FilterNeq:
		return boolValue(c != 0), nil
	case symbols.FilterLt:
		return boolValue(c < 0), nil
	case symbols.FilterLe:
		return boolValue(c <= 0), nil
	case symbols.FilterGt:
		return boolValue(c > 0), nil
	default: // symbols.FilterGe
		return boolValue(c >= 0), nil
	}
}

// featureValue reads a feature a filter condition asks of a candidate: from an
// annotation binding it, else from the candidate itself when the predicate's type
// is a reflective metaclass (KerML 1.1 §8.2.4).
func (m *Model) featureValue(cand *symbols.Symbol, p *symbols.FilterPredicate) (symbols.FilterValue, error) {
	v, err := m.annotationFeatureValue(cand, p)
	if err != nil || v.Kind != symbols.FilterValueEmpty {
		return v, err
	}
	return m.metaclassFeatureValue(cand, p)
}

// metaclassFeatureValue answers a metaclass feature from the candidate element.
// A feature of a metaclass it is an instance of but whose value we cannot derive
// is unevaluable, which is observably not false.
func (m *Model) metaclassFeatureValue(cand *symbols.Symbol, p *symbols.FilterPredicate) (symbols.FilterValue, error) {
	if !isReflectiveMetaclassFQN(p.TypeFQN) {
		return emptyValue(), nil
	}
	meta := m.metaclassOf(cand)
	if meta == nil {
		return symbols.FilterValue{}, &FilterError{
			Err:    ErrFilterUnevaluable,
			Reason: fmt.Sprintf("what %s is an instance of is not known, so %s::%s cannot be read from it", candidateName(cand), p.TypeFQN, p.Feature),
			Span:   p.Span,
		}
	}
	if !m.metaclassConforms(cand, p.TypeFQN) {
		return emptyValue(), nil // not an instance of that metaclass: nothing to read
	}
	if v, ok := m.reflectiveFeatureValue(cand, p.Feature); ok {
		return v, nil
	}
	return symbols.FilterValue{}, &FilterError{
		Err:    ErrFilterUnevaluable,
		Reason: fmt.Sprintf("the metaclass feature %s::%s is not derived from a declaration", p.TypeFQN, p.Feature),
		Span:   p.Span,
	}
}

// isReflectiveMetaclassFQN reports whether a name is a metaclass of the abstract
// syntax, which the standard library declares under KerML and SysML.
func isReflectiveMetaclassFQN(fqn string) bool {
	return strings.HasPrefix(fqn, "KerML::") || strings.HasPrefix(fqn, "SysML::")
}

// candidateName names a candidate for a diagnostic message.
func candidateName(cand *symbols.Symbol) string {
	if name := simpleSymbolName(cand); name != "" {
		return name
	}
	return "the candidate element"
}

// annotationFeatureValue reads the value the candidate's annotation of the
// predicate's metadata type binds its feature to. A feature nothing binds, or one
// read from an annotation the candidate does not carry, has an empty value
// sequence, which leaves the comparison reading it false rather than unevaluable.
func (m *Model) annotationFeatureValue(cand *symbols.Symbol, p *symbols.FilterPredicate) (symbols.FilterValue, error) {
	typ := m.symbolByFQN(p.TypeFQN)
	for _, a := range m.annotationsOf(cand) {
		if !m.annotationConforms(a, typ, p.TypeFQN) {
			continue
		}
		v, ok := a.values[p.Feature]
		if !ok {
			continue
		}
		if v.Kind == symbols.FilterValueUnknown {
			return symbols.FilterValue{}, &FilterError{
				Err:    ErrFilterUnevaluable,
				Reason: fmt.Sprintf("the value %s::%s is bound to is not a constant", p.TypeFQN, p.Feature),
				Span:   p.Span,
			}
		}
		return v, nil
	}
	return emptyValue(), nil
}

// annotatedBy reports whether the candidate is annotated by metadata conforming
// to the named type: `@Safety` holds for an element annotated with a metadata
// type specializing Safety as well as with Safety itself. The candidate's own
// metaclass counts as such an annotation, which is what makes `@SysML::PartUsage`
// select the part usages among the candidates.
func (m *Model) annotatedBy(cand *symbols.Symbol, typeFQN string) bool {
	typ := m.symbolByFQN(typeFQN)
	for _, a := range m.annotationsOf(cand) {
		if m.annotationConforms(a, typ, typeFQN) {
			return true
		}
	}
	return m.metaclassConforms(cand, typeFQN)
}

// metaclassConforms reports whether the candidate's own metaclass — the KerML
// metaclass of the declaration, `SysML::PartUsage` for a part usage — conforms to
// the named type. It is what `@@T` tests.
func (m *Model) metaclassConforms(cand *symbols.Symbol, typeFQN string) bool {
	meta := m.metaclassOf(cand)
	if meta == nil {
		return false
	}
	if typ := m.symbolByFQN(typeFQN); typ != nil {
		return m.Conforms(meta, typ) || m.conformsByName(meta, typeFQN)
	}
	// The metaclass library is not loaded, so conformance can only be judged on
	// the name the candidate's metaclass has.
	return simpleName(typeFQN) == simpleSymbolName(meta)
}

// MetaclassConforms reports whether a candidate's metaclass conforms to a type.
func (m *Model) MetaclassConforms(cand *symbols.Symbol, typeFQN string) bool {
	return m.metaclassConforms(cand, typeFQN)
}

// annotationConforms reports whether an annotation's metadata type conforms to
// the type a condition names. The types are compared as symbols where both are
// indexed, and by qualified name otherwise.
func (m *Model) annotationConforms(a annotation, typ *symbols.Symbol, typeFQN string) bool {
	if a.typ != nil && typ != nil && m.Conforms(a.typ, typ) {
		return true
	}
	return a.typ != nil && m.conformsByName(a.typ, typeFQN)
}

// indexedElement returns the symbol the index holds for sym's element, which is
// the one its annotations are recorded against across index generations. A
// body-local declaration is not indexed and bears an unqualified name, so it is
// judged as itself rather than as a namesake of it.
func (m *Model) indexedElement(sym *symbols.Symbol) *symbols.Symbol {
	if sym == nil || bodyLocalSymbol(sym) {
		return sym
	}
	if indexed := m.symbolByFQN(m.fqnOf(sym)); indexed != nil && indexed.Kind == sym.Kind {
		return indexed
	}
	return sym
}

// bodyLocalSymbol reports whether sym is declared inside a body, whose names
// exist only within it.
func bodyLocalSymbol(sym *symbols.Symbol) bool {
	for scope := sym.OwnerScope; scope != nil; scope = scope.Parent() {
		if scope.BodyLocal() {
			return true
		}
	}
	return false
}

// conformsByName reports whether sym or a supertype of it carries the qualified
// name typeFQN. A workspace that reindexes holds one element as more than one
// symbol across generations, and a name identifies one element (symbolByFQN
// answers nothing for an ambiguous one), so the name decides what identity cannot.
func (m *Model) conformsByName(sym *symbols.Symbol, typeFQN string) bool {
	if sym == nil || typeFQN == "" {
		return false
	}
	if m.fqnOf(sym) == typeFQN {
		return true
	}
	for _, sup := range m.AllSupertypes(sym) {
		if m.fqnOf(sup) == typeFQN {
			return true
		}
	}
	return false
}

// symbolByFQN returns the single element registered under a qualified name, or
// nil when the name is unknown or names more than one element.
func (m *Model) symbolByFQN(fqn string) *symbols.Symbol {
	if fqn == "" || m.resolver == nil || m.resolver.Index() == nil {
		return nil
	}
	if sym, ok := m.filterTypes[fqn]; ok {
		return sym
	}
	var found *symbols.Symbol
	if syms := m.resolver.Index().LookupQualified(fqn); len(syms) == 1 {
		found = syms[0]
	}
	m.filterTypes[fqn] = found
	return found
}

// FilterProblem is a fault in a filter condition itself, found without any
// candidate to evaluate it for: a part of it the evaluator cannot decide, or one
// that yields something other than a boolean where the condition needs a truth
// value. Reason describes the fault for a diagnostic message, and Span locates
// the part of the condition at fault.
type FilterProblem struct {
	// Kind distinguishes a specification fault from an evaluator limitation.
	Kind   FilterProblemKind
	Reason string
	Span   source.Span
}

// FilterProblemKind classifies a filter condition fault.
type FilterProblemKind uint8

const (
	FilterProblemNotBoolean FilterProblemKind = iota
	FilterProblemNotEvaluable
	FilterProblemUnsupported
)

// CheckElementFilter reports the faults of a filter condition, in the order they
// appear in it. It is what the validation pass reports: the same compiled
// predicate the enumeration evaluates is examined, so a condition diagnosed here
// is exactly one whose verdict is not trusted there.
//
// Only faults that hold for every candidate are reported. Whether an annotation
// actually binds a feature is a property of the candidate, not of the condition,
// so it surfaces as an unevaluable verdict during enumeration rather than here.
func (m *Model) CheckElementFilter(f symbols.ElementFilter) []FilterProblem {
	pred := m.CompileElementFilter(f)
	if pred == nil {
		return []FilterProblem{{Kind: FilterProblemNotEvaluable, Reason: "the condition is empty", Span: f.Span}}
	}
	// The constraints are on the membership stating the condition (KerML 8.2.4),
	// so each fault is reported there, once.
	var out []FilterProblem
	seen := map[FilterProblemKind]bool{}
	for _, p := range m.appendFilterProblems(nil, pred, true) {
		if seen[p.Kind] {
			continue
		}
		seen[p.Kind] = true
		p.Span = f.Span
		out = append(out, p)
	}
	for _, p := range out {
		if p.Kind != FilterProblemNotBoolean {
			continue
		}
		return dropUnsupportedProblems(out)
	}
	return out
}

// appendFilterProblems collects the faults of a compiled condition. wantBool
// says whether the position the predicate occupies needs a truth value: the
// condition as a whole does, as does every operand of a boolean operator.
func (m *Model) appendFilterProblems(out []FilterProblem, p *symbols.FilterPredicate, wantBool bool) []FilterProblem {
	if p == nil {
		return out
	}
	if p.Op == symbols.FilterUnsupported {
		if p.UnsupportedKind == symbols.FilterUnsupportedNotBoolean {
			if wantBool {
				out = append(out, FilterProblem{
					Kind:   FilterProblemNotBoolean,
					Reason: p.Reason,
					Span:   p.Span,
				})
			}
			return out
		}
		kind := FilterProblemNotEvaluable
		if p.UnsupportedKind == symbols.FilterUnsupportedEvaluator {
			kind = FilterProblemUnsupported
		}
		out = append(out, FilterProblem{Kind: kind, Reason: p.Reason, Span: p.Span})
		if wantBool && m.filterYieldsBool(p) == filterNotBool {
			out = append(out, FilterProblem{
				Kind:   FilterProblemNotBoolean,
				Reason: describeResultType(p.ResultType),
				Span:   p.Span,
			})
		}
		return out
	}
	if wantBool && m.filterYieldsBool(p) == filterNotBool {
		out = append(out, FilterProblem{
			Kind:   FilterProblemNotBoolean,
			Reason: describeValueKind(p.Value.Kind),
			Span:   p.Span,
		})
	}
	operandsWantBool := isFilterBoolOp(p.Op)
	for _, operand := range p.Operands {
		out = m.appendFilterProblems(out, operand, operandsWantBool)
	}
	return out
}

// dropUnsupportedProblems keeps specification faults when a Boolean fault
// already explains why the condition cannot be accepted.
func dropUnsupportedProblems(problems []FilterProblem) []FilterProblem {
	filtered := make([]FilterProblem, 0, len(problems))
	for _, problem := range problems {
		if problem.Kind != FilterProblemUnsupported {
			filtered = append(filtered, problem)
		}
	}
	return filtered
}

// filterBoolness is what is known about the kind of value a compiled condition
// yields before it is run against a candidate.
type filterBoolness uint8

const (
	filterBoolUnknown filterBoolness = iota
	filterIsBool
	filterNotBool
)

// filterYieldsBool reports what is known about a predicate's value kind. The
// value of an annotation feature is only known once a candidate is at hand, so
// its declared result type is used when available.
func (m *Model) filterYieldsBool(p *symbols.FilterPredicate) filterBoolness {
	switch p.Op {
	case symbols.FilterClassify, symbols.FilterMetaClassify, symbols.FilterNot,
		symbols.FilterAnd, symbols.FilterOr, symbols.FilterXor, symbols.FilterImplies,
		symbols.FilterEq, symbols.FilterNeq, symbols.FilterLt, symbols.FilterLe,
		symbols.FilterGt, symbols.FilterGe:
		return filterIsBool
	case symbols.FilterConst:
		if p.Value.Kind == symbols.FilterValueBool {
			return filterIsBool
		}
		if p.Value.Kind == symbols.FilterValueRef && p.ResultType != nil {
			switch prim := m.PrimTypeOf(p.ResultType); prim {
			case PrimBoolean:
				return filterIsBool
			case PrimUnknown:
				return filterBoolUnknown
			default:
				return filterNotBool
			}
		}
		if p.Value.Kind == symbols.FilterValueRef {
			return filterBoolUnknown
		}
		return filterNotBool
	case symbols.FilterFeature, symbols.FilterUnsupported:
		if p.ResultType != nil {
			switch prim := m.PrimTypeOf(p.ResultType); prim {
			case PrimBoolean:
				return filterIsBool
			case PrimUnknown:
				return filterBoolUnknown
			default:
				return filterNotBool
			}
		}
		return filterBoolUnknown
	default:
		return filterBoolUnknown
	}
}

// isFilterBoolOp reports whether an operation needs boolean operands.
func isFilterBoolOp(op symbols.FilterOp) bool {
	switch op {
	case symbols.FilterNot, symbols.FilterAnd, symbols.FilterOr,
		symbols.FilterXor, symbols.FilterImplies:
		return true
	default:
		return false
	}
}

// unsupported builds the predicate node for a condition the evaluator cannot
// decide, carrying the reason a diagnostic reports.
func unsupported(span source.Span, reason string) *symbols.FilterPredicate {
	return &symbols.FilterPredicate{
		Op:              symbols.FilterUnsupported,
		UnsupportedKind: symbols.FilterUnsupportedNotEvaluable,
		Reason:          reason,
		Span:            span,
	}
}

func evaluatorUnsupported(span source.Span, reason string, resultType *symbols.Symbol) *symbols.FilterPredicate {
	return &symbols.FilterPredicate{
		Op:              symbols.FilterUnsupported,
		UnsupportedKind: symbols.FilterUnsupportedEvaluator,
		Reason:          reason,
		ResultType:      resultType,
		Span:            span,
	}
}

// binaryFilterOp maps a binary operator of a filter condition to its predicate
// operation. Only the operators compileOperator admits reach here.
func binaryFilterOp(op ast.OperatorKind) symbols.FilterOp {
	switch op {
	case ast.OpAnd, ast.OpConditionalAnd:
		return symbols.FilterAnd
	case ast.OpOr, ast.OpConditionalOr:
		return symbols.FilterOr
	case ast.OpXor:
		return symbols.FilterXor
	case ast.OpImplies:
		return symbols.FilterImplies
	case ast.OpEq:
		return symbols.FilterEq
	case ast.OpNeq:
		return symbols.FilterNeq
	case ast.OpLt:
		return symbols.FilterLt
	case ast.OpLe:
		return symbols.FilterLe
	case ast.OpGt:
		return symbols.FilterGt
	default: // ast.OpGe
		return symbols.FilterGe
	}
}

// constValue converts a folded constant to the form a filter predicate holds.
func constValue(v Value) symbols.FilterValue {
	switch v.Kind {
	case ValInt:
		return symbols.FilterValue{Kind: symbols.FilterValueInt, Int: v.Int}
	case ValReal:
		return symbols.FilterValue{Kind: symbols.FilterValueReal, Real: v.Real}
	case ValBool:
		return symbols.FilterValue{Kind: symbols.FilterValueBool, Bool: v.Bool}
	default:
		return symbols.FilterValue{}
	}
}

// quantityValue converts a folded quantity to the form a filter predicate
// holds; one whose unit cancelled is the bare constant it folded to.
func quantityValue(q Quantity) symbols.FilterValue {
	if q.Unit.None() {
		return constValue(q.Num)
	}
	return symbols.FilterValue{Kind: symbols.FilterValueQuantity, Quantity: &q}
}

// QuantityOf returns the quantity a FilterValueQuantity carries.
func QuantityOf(v symbols.FilterValue) (*Quantity, bool) {
	if v.Kind != symbols.FilterValueQuantity {
		return nil, false
	}
	q, ok := v.Quantity.(*Quantity)
	return q, ok && q != nil
}

// asQuantity views a value as a quantity for comparison: a quantity as itself,
// a bare number as a magnitude of dimension one.
func asQuantity(v symbols.FilterValue) (*Quantity, bool) {
	if q, ok := QuantityOf(v); ok {
		return q, true
	}
	if n, ok := numericValue(v); ok {
		return &Quantity{Num: Value{Kind: ValReal, Real: n}, Unit: UnitOne()}, true
	}
	return nil, false
}

func boolValue(b bool) symbols.FilterValue {
	return symbols.FilterValue{Kind: symbols.FilterValueBool, Bool: b}
}

// emptyValue is the empty sequence, the value of a feature bound to nothing.
func emptyValue() symbols.FilterValue {
	return symbols.FilterValue{Kind: symbols.FilterValueEmpty}
}

// numericValue returns a value as a float64 for comparison, and whether it is a
// number at all.
func numericValue(v symbols.FilterValue) (float64, bool) {
	switch v.Kind {
	case symbols.FilterValueInt:
		return float64(v.Int), true
	case symbols.FilterValueReal:
		return v.Real, true
	default:
		return 0, false
	}
}

// describeValueKind names a value kind for a diagnostic message.
func describeValueKind(k symbols.FilterValueKind) string {
	switch k {
	case symbols.FilterValueBool:
		return "a boolean"
	case symbols.FilterValueInt:
		return "an integer"
	case symbols.FilterValueReal:
		return "a real"
	case symbols.FilterValueString:
		return "a string"
	case symbols.FilterValueRef:
		return "an element reference"
	case symbols.FilterValueEmpty:
		return "nothing"
	case symbols.FilterValueInstance:
		return "a constructed instance"
	case symbols.FilterValueQuantity:
		return "a quantity"
	default:
		return "no value"
	}
}

// describeNode names a syntactic form for a diagnostic message.
func describeNode(n ast.Node) string {
	switch n.(type) {
	case nil:
		return "an empty condition"
	case *ast.CastExpr:
		return "a cast on its own"
	case *ast.InvocationExpr:
		return "an invocation"
	case *ast.IndexExpr:
		return "an indexed expression"
	case *ast.BodyExpr:
		return "a body expression"
	case *ast.ErrorNode:
		return "a malformed expression"
	default:
		return "this expression"
	}
}

// isSelfReference reports whether a node is the `self` keyword used as the
// operand of a classification, which names the element being filtered.
func isSelfReference(n ast.Node) bool {
	ref, ok := n.(*ast.FeatureReference)
	if !ok || ref.Name == nil || len(ref.Name.Parts) != 1 {
		return false
	}
	return ref.Name.Parts[0].Text == "self"
}

// resolveFQN resolves a qualified name written in scope to the name the element
// it names is indexed under.
func (m *Model) resolveFQN(scope *symbols.Scope, qn *ast.QualifiedName) (string, bool) {
	sym, ok := m.resolver.ResolveQualified(scope, qn)
	if !ok || sym == nil {
		return "", false
	}
	fqn := m.fqnOf(sym)
	return fqn, fqn != ""
}

// spanOf is a node's span, or the zero span for a missing node.
func spanOf(n ast.Node) source.Span {
	if n == nil {
		return source.Span{}
	}
	return n.Span()
}

// qnText renders a qualified name as written.
func qnText(qn *ast.QualifiedName) string {
	if qn == nil {
		return ""
	}
	parts := make([]string, 0, len(qn.Parts))
	for _, seg := range qn.Parts {
		parts = append(parts, seg.Text)
	}
	return strings.Join(parts, "::")
}

// simpleName is the last segment of a qualified name.
func simpleName(fqn string) string {
	if i := strings.LastIndex(fqn, "::"); i >= 0 {
		return fqn[i+2:]
	}
	return fqn
}

// simpleSymbolName is a symbol's own name, without the qualification an indexed
// symbol's name may carry.
func simpleSymbolName(sym *symbols.Symbol) string {
	if sym == nil {
		return ""
	}
	return simpleName(sym.Name)
}

// ownerSymbol returns the element a symbol is a member of.
func ownerSymbol(sym *symbols.Symbol) *symbols.Symbol {
	if sym == nil || sym.OwnerScope == nil {
		return nil
	}
	return sym.OwnerScope.Owner()
}

// ownerOf is the element declaring sym: its owning scope's owner, or — for a
// symbol restored from a cache record, which carries no scope — the namespace its
// indexed name names.
func (m *Model) ownerOf(sym *symbols.Symbol) *symbols.Symbol {
	if owner := ownerSymbol(sym); owner != nil {
		return owner
	}
	if sym == nil || m.resolver == nil || m.resolver.Index() == nil {
		return nil
	}
	fqn := m.fqnOf(sym)
	i := strings.LastIndex(fqn, "::")
	if i < 0 {
		return nil
	}
	if owners := m.resolver.Index().LookupQualified(fqn[:i]); len(owners) == 1 {
		return owners[0]
	}
	return nil
}

// IsMetadataType reports whether a symbol is a metadata definition or a KerML
// metaclass — the types an annotation can have.
func IsMetadataType(sym *symbols.Symbol) bool {
	if sym == nil {
		return false
	}
	return sym.Kind == symbols.SymbolMetadataDef || sym.Kind == symbols.SymbolMetaclass
}

// unquote reads the text a string literal spells, so a filter constant matches
// the same string the runtime evaluates the literal to.
func unquote(s string) string {
	return lexer.StringValue(s)
}
