package solve

import (
	"fmt"
	"math"
	"math/big"
	"sort"
	"strconv"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/lexer"
	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// Subject is the element a query is about: what a verdict on it would name, and
// whether it asserts that its conditions do not hold.
type Subject struct {
	// Kind is the kind of element: "constraint", "requirement" or
	// "satisfaction".
	Kind string

	// Name is the element as a verdict about it would name it.
	Name string

	// Symbol is the element's declaration, used for provenance.
	Symbol *symbols.Symbol

	// Negated is set for `assert not …`, which denies the conjunction of the
	// required conditions rather than asserting each one.
	Negated bool
}

// Constraint translates the conditions sym states as a constraint, inherited ones
// included, in the order the evaluator checks them. scope stands in for sym's
// own scope when sym declares none.
func Constraint(ctx *runtime.Context, sym *symbols.Symbol, scope *symbols.Scope) (*Query, error) {
	return ConstraintWith(ctx, sym, scope, nil)
}

// ConstraintWith translates a constraint with values already fixed, so the solver
// synthesises only what the model leaves free. With no pins it is Constraint.
func ConstraintWith(ctx *runtime.Context, sym *symbols.Symbol, scope *symbols.Scope, pins []Pin) (*Query, error) {
	if err := runtime.RequireConstraint(sym); err != nil {
		return nil, err
	}
	subject := Subject{
		Kind:    "constraint",
		Name:    sym.Name,
		Symbol:  sym,
		Negated: runtime.NegatedDecl(sym),
	}
	return TranslateWith(ctx, subject, ctx.ConditionsOf(sym, scope), pins)
}

// Requirement translates the conditions sym states as a requirement, its
// assumptions included as assumed rather than required.
func Requirement(ctx *runtime.Context, sym *symbols.Symbol, scope *symbols.Scope) (*Query, error) {
	return RequirementWith(ctx, sym, scope, nil)
}

// RequirementWith translates a requirement with values already fixed. With no
// pins it is Requirement.
func RequirementWith(ctx *runtime.Context, sym *symbols.Symbol, scope *symbols.Scope, pins []Pin) (*Query, error) {
	if err := runtime.RequireRequirement(sym); err != nil {
		return nil, err
	}
	subject := Subject{
		Kind:    "requirement",
		Name:    sym.Name,
		Symbol:  sym,
		Negated: runtime.NegatedDecl(sym),
	}
	return TranslateWith(ctx, subject, ctx.ConditionsOf(sym, scope), pins)
}

// Satisfaction translates the conditions an `assert satisfy` checks: those of the
// requirement it names, read through that requirement's own parameters, since the
// query asks what they permit rather than what the subject holds.
func Satisfaction(ctx *runtime.Context, assertion *runtime.SatisfyAssertion) (*Query, error) {
	return SatisfactionWith(ctx, assertion, nil)
}

// SatisfactionWith translates an `assert satisfy` with values already fixed. With
// no pins it is Satisfaction.
func SatisfactionWith(ctx *runtime.Context, assertion *runtime.SatisfyAssertion, pins []Pin) (*Query, error) {
	if assertion == nil || assertion.Symbol == nil {
		return nil, fmt.Errorf("satisfaction: %w", ErrNoConditions)
	}
	subject := Subject{
		Kind:    "satisfaction",
		Name:    assertion.Text(),
		Symbol:  assertion.Symbol,
		Negated: assertion.Negated,
	}
	return TranslateWith(ctx, subject, ctx.ConditionsOf(assertion.Symbol, assertion.Symbol.OwnerScope), pins)
}

// TranslateWith builds a query whose pinned features are asserted equal to the
// values given, leaving the rest free. A value that cannot be fixed refuses, as
// an untranslatable condition does; a value no condition reads is reported in
// Query.Unread rather than dropped. One refusal fails the whole translation,
// since a query missing a conjunct would answer about conditions it does not
// hold.
func TranslateWith(ctx *runtime.Context, subject Subject, conds []runtime.Condition, pins []Pin) (*Query, error) {
	if ctx == nil {
		return nil, fmt.Errorf("solve: no runtime context")
	}
	if len(conds) == 0 {
		return nil, fmt.Errorf("%s %s: %w", subject.Kind, subject.Name, ErrNoConditions)
	}
	t := newTranslator(ctx, subject)
	if err := t.translate(conds); err != nil {
		return nil, err
	}
	if err := t.fix(pins); err != nil {
		return nil, err
	}
	return t.query(), nil
}

// translator holds the state of one translation: the variables and sorts the
// conditions turned out to need, the assertions built so far, and which
// condition is being translated, for a refusal's provenance.
type translator struct {
	ctx     *runtime.Context
	model   *semantics.Model
	subject Subject

	// features are the features a condition may name, as the evaluator sees them:
	// a redefinition masks what it redefines, so both read one value.
	features map[string]*symbols.Symbol

	vars    map[string]*Var
	sorts   map[string]Sort
	domains []Assertion
	guards  []Assertion
	asserts []Assertion

	// pins are the equalities fixing values the model already holds, and pinned
	// records them for a report; unread holds the values no variable reads.
	pins   []Assertion
	pinned []PinnedValue
	unread []Unread

	// guarded remembers the divisors already asserted non-zero, by their term, so
	// a divisor read twice is guarded once.
	guarded map[string]bool

	// baseUnits names the base units magnitudes of a dimension are scaled to, as
	// the quantities written in the conditions reduce to them.
	baseUnits map[string]string

	// branched counts the enclosing contexts a subexpression may go unevaluated in,
	// where a definedness assertion over the whole query would not be equivalent.
	branched int

	// objectives are the translated objectives, in the order they are optimized.
	objectives []Objective

	// within is the objective whose own conditions are being translated: a member
	// of its own it names (its `best`) is that objective's, not another's.
	within *symbols.Symbol

	nonlinear bool
	intDiv    bool

	condLabel string
	condFile  string
}

// newTranslator starts a translation of what subject states.
func newTranslator(ctx *runtime.Context, subject Subject) *translator {
	return &translator{
		ctx:       ctx,
		model:     ctx.Semantics(),
		subject:   subject,
		features:  effectiveFeatures(ctx, subject.Symbol),
		vars:      map[string]*Var{},
		sorts:     map[string]Sort{},
		guarded:   map[string]bool{},
		baseUnits: map[string]string{},
	}
}

// effectiveFeatures maps the name a condition may use to the feature it reads,
// from the same flattened schema the evaluator resolves a name through.
func effectiveFeatures(ctx *runtime.Context, sym *symbols.Symbol) map[string]*symbols.Symbol {
	features := ctx.FeaturesOf(sym)
	if len(features) == 0 {
		return nil
	}
	out := make(map[string]*symbols.Symbol, len(features))
	for _, feat := range features {
		if feat.Name == "" || feat.Symbol == nil {
			continue
		}
		out[feat.Name] = feat.Symbol
	}
	return out
}

// translate builds an assertion per condition, in the order the evaluator checks
// them. A negated element instead denies the conjunction of its required
// conditions, as evaluating it negates their verdict as a whole.
func (t *translator) translate(conds []runtime.Condition) error {
	var required []*Term
	var labels []string
	for _, cond := range conds {
		owner, span := conditionOrigin(cond)
		t.condLabel = cond.Label()
		t.condFile = ""
		if owner != nil {
			t.condFile = owner.DocName
		}
		term, err := t.condition(cond)
		if err != nil {
			return err
		}
		if t.subject.Negated && cond.Required {
			required = append(required, term)
			labels = append(labels, cond.Label())
			continue
		}
		role := RoleAssumed
		if cond.Required {
			role = RoleRequired
		}
		t.asserts = append(t.asserts, Assertion{Term: term, From: t.provenance(role, owner, span)})
	}
	if !t.subject.Negated {
		return nil
	}
	if len(required) == 0 {
		return fmt.Errorf("%s %s: %w to deny", t.subject.Kind, t.subject.Name, ErrNoConditions)
	}
	t.condLabel = "not (" + strings.Join(labels, " and ") + ")"
	from := t.provenance(RoleDenied, t.subject.Symbol, declSpan(t.subject.Symbol))
	if t.subject.Symbol != nil {
		from.File = t.subject.Symbol.DocName
		from.Location = t.ctx.SourceLocation(from.File, from.Span)
	}
	t.asserts = append(t.asserts, Assertion{Term: Not(And(required...)), From: from})
	return nil
}

// query assembles the translated parts, declarations ordered by name and domain
// assertions before the conditions, which is what makes a script deterministic.
func (t *translator) query() *Query {
	q := &Query{
		Kind:            t.subject.Kind,
		Element:         t.subject.Name,
		Negated:         t.subject.Negated,
		Nonlinear:       t.nonlinear,
		IntegerDivision: t.intDiv,
	}
	for _, v := range t.vars {
		if v.Unit == "" {
			v.Unit = t.baseUnits[v.Dimension]
		}
		q.Vars = append(q.Vars, v)
	}
	sortVars(q.Vars)
	for _, s := range t.sorts {
		q.Sorts = append(q.Sorts, s)
	}
	sortSorts(q.Sorts)
	domains := append([]Assertion(nil), t.domains...)
	sortAssertions(domains)
	q.Assertions = append(domains, t.pinnedAssertions(len(domains))...)
	q.Assertions = append(q.Assertions, t.guards...)
	q.Assertions = append(q.Assertions, t.asserts...)
	q.Pinned = t.pinned
	q.Unread = t.unread
	for _, obj := range t.objectives {
		if obj.Unit == "" {
			obj.Unit = t.baseUnits[obj.Dimension]
		}
		q.Objectives = append(q.Objectives, obj)
	}
	return q
}

// pinnedAssertions orders the fixed values by the variable they fix, and records
// where each one's assertion sits, so a core naming it names the fixed value. The
// assertions and the records they come from are ordered as one, since the record
// is what an unsat core is read back through.
func (t *translator) pinnedAssertions(offset int) []Assertion {
	order := make([]int, len(t.pinned))
	for i := range order {
		order[i] = i
	}
	sort.SliceStable(order, func(i, j int) bool {
		return t.pinned[order[i]].Var.Name < t.pinned[order[j]].Var.Name
	})
	assertions := make([]Assertion, 0, len(order))
	pinned := make([]PinnedValue, 0, len(order))
	for at, i := range order {
		assertions = append(assertions, t.pins[i])
		record := t.pinned[i]
		record.Index = offset + at
		pinned = append(pinned, record)
	}
	t.pinned = pinned
	return assertions
}

// condition translates one condition, applying the negation it was written with.
// A group stands for the conjunction of its conditions, so negating a group
// negates that conjunction.
func (t *translator) condition(cond runtime.Condition) (*Term, error) {
	if cond.Statement != nil {
		return nil, t.refuse(cond.Statement, "body statement", "OpenSysML does not execute a statement in a constraint body")
	}
	if cond.Conflict != nil {
		return nil, t.refuse(cond.Conflict.Node, "conflicting result expression", "only one owned or inherited result expression is allowed")
	}
	if cond.Negated {
		t.branched++
		defer func() { t.branched-- }()
	}
	if cond.Group != nil {
		parts := make([]*Term, 0, len(cond.Group))
		for _, sub := range cond.Group {
			term, err := t.condition(sub)
			if err != nil {
				return nil, err
			}
			parts = append(parts, term)
		}
		return negate(And(parts...), cond.Negated), nil
	}
	if cond.Expr == nil {
		return nil, t.refuse(nil, "empty condition", "it states no expression")
	}
	term, err := t.expr(cond.Expr, cond.Scope)
	if err != nil {
		return nil, err
	}
	if term.Sort.Kind != SortBool {
		return nil, t.refuse(cond.Expr, "condition", "it yields "+term.Sort.Name+" rather than a boolean")
	}
	return negate(term, cond.Negated), nil
}

// negate applies a written negation.
func negate(term *Term, negated bool) *Term {
	if negated {
		return Not(term)
	}
	return term
}

// expr translates an expression, resolving names in scope.
func (t *translator) expr(node ast.Node, scope *symbols.Scope) (*Term, error) {
	switch n := node.(type) {
	case *ast.LiteralBool:
		return BoolTerm(n.Value), nil
	case *ast.LiteralInteger:
		val, err := strconv.ParseInt(n.Value, 10, 64)
		if err != nil {
			return nil, t.refuse(n, "integer literal "+n.Value, "it does not fit a 64-bit integer")
		}
		return IntTerm(val), nil
	case *ast.LiteralReal:
		rat, ok := new(big.Rat).SetString(n.Value)
		if !ok {
			return nil, t.refuse(n, "real literal "+n.Value, "it is not an exact rational")
		}
		return RealTerm(rat), nil
	case *ast.LiteralString:
		return StringTerm(unquote(n.Value)), nil
	case *ast.FeatureReference, *ast.FeatureChainExpr:
		return t.reference(node, scope)
	case *ast.IndexExpr:
		if !n.Bracket {
			return nil, t.refuse(n, "index expression", "indexing a sequence is outside the subset")
		}
		return t.quantity(n, scope)
	case *ast.OperatorExpr:
		return t.operator(n, scope)
	case *ast.SequenceExpr:
		if len(n.Elements) == 1 {
			return t.expr(n.Elements[0], scope)
		}
		return nil, t.refuse(n, "sequence", "a collection is outside the subset")
	}
	return nil, t.refuse(node, describe(node), "it is outside the subset")
}

// recordBaseUnits remembers the base units a dimension's magnitudes are scaled
// to, so a model reports one in the units it is expressed in rather than bare.
func (t *translator) recordBaseUnits(dim semantics.Dimension, unit semantics.UnitTerm) {
	key := dimensionUnits(dim)
	if key == "" || t.baseUnits[key] != "" {
		return
	}
	t.baseUnits[key] = baseUnitsText(unit)
}

// baseUnitsText names a reduced unit's base units as declared ("gram",
// "metre·second^-1"), its scale factor left out since a magnitude scaled to them
// carries none.
func baseUnitsText(unit semantics.UnitTerm) string {
	out := ""
	for _, f := range unit.Factors {
		if out != "" {
			out += "·"
		}
		out += leafName(f.Unit.Name)
		if f.Exponent != 1 {
			out += fmt.Sprintf("^%g", f.Exponent)
		}
	}
	return out
}

// leafName is the declared name at the end of a qualified one.
func leafName(name string) string {
	if i := strings.LastIndex(name, "::"); i >= 0 {
		return name[i+2:]
	}
	return name
}

// quantity translates `magnitude [unit]`, scaling the magnitude to the base units
// its unit reduces to so that magnitudes of one dimension are comparable.
func (t *translator) quantity(n *ast.IndexExpr, scope *symbols.Scope) (*Term, error) {
	unit, err := t.model.UnitTermOfExpr(scope, n.Index)
	if err != nil {
		return nil, t.refuse(n, "quantity", err.Error())
	}
	unit = unit.Normalized()
	dim, ok := t.model.DimensionOfExpr(scope, n)
	if !ok {
		return nil, t.refuse(n, "quantity", "the dimension of its unit is not determined statically")
	}
	t.recordBaseUnits(dim, unit)
	scale, ok := ratOfScale(unit.Scale)
	if !ok {
		return nil, t.refuse(n, "quantity", "its unit reduces to a scale factor that is not an exact ratio")
	}
	magnitude, err := t.expr(n.Operand, scope)
	if err != nil {
		return nil, err
	}
	if !magnitude.Sort.Numeric() {
		return nil, t.refuse(n, "quantity", "its magnitude yields "+magnitude.Sort.Name+" rather than a number")
	}
	realMagnitude := ToReal(magnitude)
	if realMagnitude.Op == OpReal {
		return RealTerm(new(big.Rat).Mul(realMagnitude.Real, scale)), nil
	}
	if scale.Cmp(big.NewRat(1, 1)) == 0 {
		return realMagnitude, nil
	}
	return Binary(OpMul, Real, realMagnitude, RealTerm(scale)), nil
}

// ratOfScale converts a unit's scale factor to an exact ratio.
func ratOfScale(scale semantics.Scale) (*big.Rat, bool) {
	num, den := new(big.Rat), new(big.Rat)
	if math.IsInf(scale.Num, 0) || math.IsNaN(scale.Num) || num.SetFloat64(scale.Num) == nil {
		return nil, false
	}
	if math.IsInf(scale.Den, 0) || math.IsNaN(scale.Den) || den.SetFloat64(scale.Den) == nil || scale.Den == 0 {
		return nil, false
	}
	return num.Quo(num, den), true
}

// msgOperatorPrefix names the operator a refusal is about.
const msgOperatorPrefix = "operator `"

// operator translates an operator application, refusing one whose meaning this
// term language does not carry.
func (t *translator) operator(n *ast.OperatorExpr, scope *symbols.Scope) (*Term, error) {
	switch n.Operator {
	case ast.OpNot:
		return t.unaryBool(n, scope)
	case ast.OpAnd, ast.OpConditionalAnd:
		return t.binaryBool(n, scope, OpAnd)
	case ast.OpOr, ast.OpConditionalOr:
		return t.binaryBool(n, scope, OpOr)
	case ast.OpXor:
		return t.binaryBool(n, scope, OpXor)
	case ast.OpImplies:
		return t.binaryBool(n, scope, OpImplies)
	case ast.OpEq:
		return t.equality(n, scope, OpEq)
	case ast.OpNeq:
		return t.equality(n, scope, OpNe)
	case ast.OpLt:
		return t.comparison(n, scope, OpLt)
	case ast.OpLe:
		return t.comparison(n, scope, OpLe)
	case ast.OpGt:
		return t.comparison(n, scope, OpGt)
	case ast.OpGe:
		return t.comparison(n, scope, OpGe)
	case ast.OpAdd:
		return t.additive(n, scope, OpAdd)
	case ast.OpSub:
		return t.additive(n, scope, OpSub)
	case ast.OpMul:
		return t.multiplicative(n, scope, OpMul)
	case ast.OpDiv:
		return t.multiplicative(n, scope, OpDiv)
	case ast.OpMod:
		return t.remainder(n, scope)
	case ast.OpNeg, ast.OpPos:
		return t.unaryNumber(n, scope)
	case ast.OpConditional:
		return t.conditional(n, scope)
	}
	return nil, t.refuse(n, msgOperatorPrefix+n.Operator.String()+"`", operatorReason(n.Operator))
}

// operatorReason says why an operator outside the subset is outside it.
func operatorReason(op ast.OperatorKind) string {
	switch op {
	case ast.OpPow:
		return "exponentiation is outside the arithmetic encoded here"
	case ast.OpRange:
		return "a range is a collection"
	case ast.OpIndex, ast.OpAll:
		return "a collection operation is outside the subset"
	case ast.OpEqEqEq, ast.OpNeqEqEq:
		return "identity compares objects rather than values"
	case ast.OpHasType, ast.OpIsType, ast.OpAs, ast.OpMeta, ast.OpAt, ast.OpMetaAt:
		return "classification and metadata are not encoded as terms"
	case ast.OpNullCoalesce:
		return "a null value has no term"
	case ast.OpBitNot:
		return "bitwise negation is outside the subset"
	}
	return "it is outside the subset"
}

// unaryBool translates `not e`, whose operand is read in a negated position.
func (t *translator) unaryBool(n *ast.OperatorExpr, scope *symbols.Scope) (*Term, error) {
	t.branched++
	defer func() { t.branched-- }()
	arg, err := t.operandOfSort(n, scope, 0, Bool)
	if err != nil {
		return nil, err
	}
	return Not(arg), nil
}

// binaryBool translates a boolean connective. Every connective but `and` may
// leave an operand unevaluated, or reads it negated, so its operands are branched.
func (t *translator) binaryBool(n *ast.OperatorExpr, scope *symbols.Scope, op Op) (*Term, error) {
	if op != OpAnd {
		t.branched++
		defer func() { t.branched-- }()
	}
	left, err := t.operandOfSort(n, scope, 0, Bool)
	if err != nil {
		return nil, err
	}
	right, err := t.operandOfSort(n, scope, 1, Bool)
	if err != nil {
		return nil, err
	}
	switch op {
	case OpAnd:
		return And(left, right), nil
	case OpOr:
		return Or(left, right), nil
	}
	return Binary(op, Bool, left, right), nil
}

// equality translates `==` or `!=` between two values of the same sort.
func (t *translator) equality(n *ast.OperatorExpr, scope *symbols.Scope, op Op) (*Term, error) {
	left, right, err := t.operands(n, scope)
	if err != nil {
		return nil, err
	}
	left, right = promote(left, right)
	if !left.Sort.Equal(right.Sort) {
		return nil, t.refuse(n, msgOperatorPrefix+n.Operator.String()+"`",
			fmt.Sprintf("its operands yield %s and %s", left.Sort.Name, right.Sort.Name))
	}
	if left.Sort.Numeric() {
		if err := t.sameDimension(n, scope); err != nil {
			return nil, err
		}
	}
	return Binary(op, Bool, left, right), nil
}

// comparison translates an ordering comparison between two numbers.
func (t *translator) comparison(n *ast.OperatorExpr, scope *symbols.Scope, op Op) (*Term, error) {
	left, right, err := t.numericOperands(n, scope)
	if err != nil {
		return nil, err
	}
	if err := t.sameDimension(n, scope); err != nil {
		return nil, err
	}
	return Binary(op, Bool, left, right), nil
}

// additive translates `+` or `-` between two numbers of the same dimension.
func (t *translator) additive(n *ast.OperatorExpr, scope *symbols.Scope, op Op) (*Term, error) {
	left, right, err := t.numericOperands(n, scope)
	if err != nil {
		return nil, err
	}
	if err := t.sameDimension(n, scope); err != nil {
		return nil, err
	}
	return Binary(op, left.Sort, left, right), nil
}

// multiplicative translates `*` or `/`. A quotient is a Real whatever its
// operand sorts, as the evaluator's is, with a non-zero divisor asserted.
func (t *translator) multiplicative(n *ast.OperatorExpr, scope *symbols.Scope, op Op) (*Term, error) {
	if op == OpDiv {
		if folded, ok := t.folded(n); ok {
			return folded, nil
		}
	}
	left, right, err := t.numericOperands(n, scope)
	if err != nil {
		return nil, err
	}
	if op == OpDiv {
		if err := t.divisor(n, right); err != nil {
			return nil, err
		}
		if left.Sort.Kind == SortInt && right.Sort.Kind == SortInt {
			return RatioDiv(left, right), nil
		}
		return Binary(OpDiv, Real, ToReal(left), ToReal(right)), nil
	}
	if !left.Literal() && !right.Literal() {
		t.nonlinear = true
	}
	return Binary(OpMul, left.Sort, left, right), nil
}

// remainder translates `%` on integers as the remainder truncating division
// leaves, which takes the sign of the dividend as the evaluator's does.
func (t *translator) remainder(n *ast.OperatorExpr, scope *symbols.Scope) (*Term, error) {
	if folded, ok := t.folded(n); ok {
		return folded, nil
	}
	left, right, err := t.numericOperands(n, scope)
	if err != nil {
		return nil, err
	}
	if left.Sort.Kind != SortInt || right.Sort.Kind != SortInt {
		return nil, t.refuse(n, "operator `%` on real numbers",
			"the evaluator computes it in floating point, which this encoding does not model")
	}
	if err := t.divisor(n, right); err != nil {
		return nil, err
	}
	t.intDiv = true
	return TruncRem(left, right), nil
}

// divisor refuses a literal zero divisor, as the evaluator reports division by
// zero, and asserts a computed one to be non-zero: SMT-LIB's division is total,
// so a solver could otherwise satisfy a condition by dividing by zero. A divisor
// that is not a literal makes the arithmetic nonlinear.
func (t *translator) divisor(n *ast.OperatorExpr, divisor *Term) error {
	if divisor.Literal() {
		if isZero(divisor) {
			return t.refuse(n, msgOperatorPrefix+n.Operator.String()+"` by zero",
				"the evaluator reports division by zero")
		}
		return nil
	}
	if !t.hoistable() {
		return t.refuse(n, msgOperatorPrefix+n.Operator.String()+"` by a computed divisor",
			"asserting the divisor non-zero would deny assignments the evaluator accepts, "+
				"as this division may go unevaluated")
	}
	t.nonlinear = true
	t.guard(Binary(OpNe, Bool, divisor, zeroOf(divisor.Sort)), n)
	return nil
}

// hoistable reports whether a definedness assertion over the whole query says
// what the evaluator says: only where the expression is always evaluated and read
// unnegated, since a division the evaluator never performs cannot constrain it.
func (t *translator) hoistable() bool {
	return t.branched == 0 && !t.subject.Negated
}

// guard asserts a side condition a translated condition needs to mean what the
// evaluator means, once per distinct term.
func (t *translator) guard(term *Term, node ast.Node) {
	key := writeTerm(term)
	if t.guarded[key] {
		return
	}
	t.guarded[key] = true
	t.guards = append(t.guards, Assertion{Term: term, From: t.provenance(RoleDefined, nil, node.Span())})
}

// folded returns what the evaluator's constant folder answers for a constant
// expression: it keeps a constant `7 / 2` a real quotient rather than truncating
// it, so the encoding must answer the same.
func (t *translator) folded(n *ast.OperatorExpr) (*Term, bool) {
	val, ok := t.model.Eval(n)
	if !ok {
		return nil, false
	}
	switch val.Kind {
	case semantics.ValInt:
		return IntTerm(val.Int), true
	case semantics.ValReal:
		rat := new(big.Rat).SetFloat64(val.Real)
		if rat == nil {
			return nil, false
		}
		return RealTerm(rat), true
	}
	return nil, false
}

// isZero reports whether a numeric literal is zero.
func isZero(term *Term) bool {
	switch term.Op {
	case OpInt:
		return term.Int == 0
	case OpReal:
		return term.Real.Sign() == 0
	}
	return false
}

// zeroOf is the zero of a numeric sort.
func zeroOf(sort Sort) *Term {
	if sort.Kind == SortInt {
		return IntTerm(0)
	}
	return RealTerm(new(big.Rat))
}

// unaryNumber translates unary `-` and `+`.
func (t *translator) unaryNumber(n *ast.OperatorExpr, scope *symbols.Scope) (*Term, error) {
	if len(n.Operands) != 1 {
		return nil, t.refuse(n, msgOperatorPrefix+n.Operator.String()+"`", "it takes one operand")
	}
	arg, err := t.expr(n.Operands[0], scope)
	if err != nil {
		return nil, err
	}
	if !arg.Sort.Numeric() {
		return nil, t.refuse(n, msgOperatorPrefix+n.Operator.String()+"`",
			"its operand yields "+arg.Sort.Name+" rather than a number")
	}
	if n.Operator == ast.OpPos {
		return arg, nil
	}
	return negated(arg), nil
}

// negated is unary `-`, folded over a literal so a negative literal stays one:
// a literal divisor is what keeps a division linear.
func negated(arg *Term) *Term {
	switch arg.Op {
	case OpInt:
		return IntTerm(-arg.Int)
	case OpReal:
		return RealTerm(new(big.Rat).Neg(arg.Real))
	}
	return Unary(OpNeg, arg.Sort, arg)
}

// conditional translates `if c ? a else b`, whose branches must share a sort.
func (t *translator) conditional(n *ast.OperatorExpr, scope *symbols.Scope) (*Term, error) {
	if len(n.Operands) != 3 {
		return nil, t.refuse(n, "conditional expression", "it takes a condition and two branches")
	}
	cond, err := t.operandOfSort(n, scope, 0, Bool)
	if err != nil {
		return nil, err
	}
	t.branched++
	defer func() { t.branched-- }()
	then, err := t.expr(n.Operands[1], scope)
	if err != nil {
		return nil, err
	}
	otherwise, err := t.expr(n.Operands[2], scope)
	if err != nil {
		return nil, err
	}
	then, otherwise = promote(then, otherwise)
	if !then.Sort.Equal(otherwise.Sort) {
		return nil, t.refuse(n, "conditional expression",
			fmt.Sprintf("its branches yield %s and %s", then.Sort.Name, otherwise.Sort.Name))
	}
	return Ite(cond, then, otherwise), nil
}

// operands translates the two operands of a binary operator.
func (t *translator) operands(n *ast.OperatorExpr, scope *symbols.Scope) (*Term, *Term, error) {
	if len(n.Operands) != 2 {
		return nil, nil, t.refuse(n, msgOperatorPrefix+n.Operator.String()+"`", "it takes two operands")
	}
	left, err := t.expr(n.Operands[0], scope)
	if err != nil {
		return nil, nil, err
	}
	right, err := t.expr(n.Operands[1], scope)
	if err != nil {
		return nil, nil, err
	}
	return left, right, nil
}

// numericOperands translates two operands that must both be numbers, widening an
// integer one when the other is real.
func (t *translator) numericOperands(n *ast.OperatorExpr, scope *symbols.Scope) (*Term, *Term, error) {
	left, right, err := t.operands(n, scope)
	if err != nil {
		return nil, nil, err
	}
	if !left.Sort.Numeric() || !right.Sort.Numeric() {
		return nil, nil, t.refuse(n, msgOperatorPrefix+n.Operator.String()+"`",
			fmt.Sprintf("its operands yield %s and %s rather than numbers", left.Sort.Name, right.Sort.Name))
	}
	left, right = promote(left, right)
	return left, right, nil
}

// operandOfSort translates one operand and requires the sort given.
func (t *translator) operandOfSort(n *ast.OperatorExpr, scope *symbols.Scope, i int, want Sort) (*Term, error) {
	if i >= len(n.Operands) {
		return nil, t.refuse(n, msgOperatorPrefix+n.Operator.String()+"`", "it is missing an operand")
	}
	term, err := t.expr(n.Operands[i], scope)
	if err != nil {
		return nil, err
	}
	if !term.Sort.Equal(want) {
		return nil, t.refuse(n.Operands[i], "operand of `"+n.Operator.String()+"`",
			"it yields "+term.Sort.Name+" rather than "+want.Name)
	}
	return term, nil
}

// promote widens an integer term to a real one when the other side is real.
func promote(left, right *Term) (*Term, *Term) {
	if left.Sort.Kind == SortInt && right.Sort.Kind == SortReal {
		return ToReal(left), right
	}
	if left.Sort.Kind == SortReal && right.Sort.Kind == SortInt {
		return left, ToReal(right)
	}
	return left, right
}

// sameDimension refuses an operator whose operands are magnitudes of different
// dimensions, which the evaluator reports as incommensurable units. A dimension
// that is not statically determined counts as dimensionless, as a bare number is.
func (t *translator) sameDimension(n *ast.OperatorExpr, scope *symbols.Scope) error {
	left := t.dimensionOf(scope, n.Operands[0])
	right := t.dimensionOf(scope, n.Operands[1])
	if left.Term.Commensurable(right.Term) {
		return nil
	}
	return t.refuse(n, msgOperatorPrefix+n.Operator.String()+"`",
		fmt.Sprintf("incommensurable units: %s against %s", dimensionText(left), dimensionText(right)))
}

// dimensionOf returns the dimension of an expression, dimensionless when it is
// not statically determined.
func (t *translator) dimensionOf(scope *symbols.Scope, node ast.Node) semantics.Dimension {
	if dim, ok := t.model.DimensionOfExpr(scope, node); ok {
		return dim
	}
	return semantics.Dimension{Term: semantics.UnitTerm{Scale: semantics.UnitScale(1)}}
}

// dimensionText names a dimension for a message, naming a dimensionless one.
func dimensionText(dim semantics.Dimension) string {
	if dim.Term.Dimensionless() {
		return "a plain number"
	}
	return dim.String()
}

// unquote strips the quotes a string literal's raw text carries, as the
// evaluator does.
func unquote(text string) string {
	return lexer.StringValue(text)
}

// describe names a node as the notation writes it, for a refusal.
func describe(node ast.Node) string {
	switch node.(type) {
	case nil:
		return "empty expression"
	case *ast.NullExpr:
		return "`null`"
	case *ast.LiteralInfinity:
		return "`*`"
	case *ast.InvocationExpr:
		return "invocation"
	case *ast.CollectExpr:
		return "`->` collect expression"
	case *ast.SelectExpr:
		return "`->select` expression"
	case *ast.BodyExpr:
		return "body expression"
	case *ast.MetadataAccessExpr:
		return "metadata access"
	case *ast.CastExpr:
		return "cast"
	case *ast.ConstructorExpr:
		return "constructor"
	}
	return fmt.Sprintf("expression %T", node)
}

// conditionOrigin returns the element that declared a condition and where it was
// written, descending into a group's first condition, which carries the scope a
// group has none of.
func conditionOrigin(cond runtime.Condition) (*symbols.Symbol, source.Span) {
	if cond.Group != nil {
		if len(cond.Group) == 0 {
			return nil, source.Span{}
		}
		return conditionOrigin(cond.Group[0])
	}
	owner := cond.Owner()
	span := declSpan(owner)
	if cond.Expr != nil {
		span = cond.Expr.Span()
	}
	if cond.Statement != nil {
		span = cond.Statement.Span()
	}
	if cond.Conflict != nil {
		span = cond.Conflict.Node.Span()
	}
	return owner, span
}

// declSpan is where a symbol was declared, empty for none.
func declSpan(sym *symbols.Symbol) source.Span {
	if sym == nil {
		return source.Span{}
	}
	return sym.DeclSpan
}

// provenance records what the assertion being built came from.
func (t *translator) provenance(role Role, owner *symbols.Symbol, span source.Span) Provenance {
	return Provenance{
		Kind:      t.subject.Kind,
		Element:   t.subject.Name,
		Condition: t.condLabel,
		Role:      role,
		Declared:  owner,
		File:      t.condFile,
		Span:      span,
		Location:  t.ctx.SourceLocation(t.condFile, span),
	}
}

// refuse reports that a construct is outside the translatable subset, naming the
// condition it appeared in and where it was written.
func (t *translator) refuse(node ast.Node, construct, reason string) error {
	span := source.Span{}
	if node != nil {
		span = node.Span()
	}
	return &NotTranslatableError{
		Construct: construct,
		Reason:    reason,
		Element:   t.subject.Kind + " " + t.subject.Name,
		Condition: t.condLabel,
		File:      t.condFile,
		Span:      span,
		Location:  t.ctx.SourceLocation(t.condFile, span),
	}
}
