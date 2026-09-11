package solve

// The differential agreement gate pins every variable of a translated element to
// the concrete value the evaluator reads, and requires the solver's verdict on
// that query to be the evaluator's: sat when the conditions hold, unsat when
// they do not, and unsat guards where the evaluator refuses to divide by zero.
// A solver `unknown`, an evaluator error and an unmet assumption are classified
// and counted rather than relaxed away. No AST is mutated: the pins are
// assertions of a query built beside the translated one.

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"sort"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// diffOutcome classifies one differential check.
type diffOutcome int

const (
	// diffAgreed: the solver's verdict on the pinned query is the evaluator's.
	diffAgreed diffOutcome = iota
	// diffDisagreed: it is not, which is a bug in one of the two layers.
	diffDisagreed
	// diffUnknown: the solver did not decide the pinned query.
	diffUnknown
	// diffEvalRefused: the evaluator returned a typed error rather than a
	// verdict, so there is no verdict to compare.
	diffEvalRefused
	// diffNotTranslated: the element is outside the translatable subset.
	diffNotTranslated
	// diffNoValues: no concrete value is declared for one of the variables.
	diffNoValues
	// diffAssumptionUnmet: the assignment falsifies an assumption, which the
	// query asserts and the evaluator trusts.
	diffAssumptionUnmet
)

var diffOutcomeNames = map[diffOutcome]string{
	diffAgreed:          "agreed",
	diffDisagreed:       "disagreed",
	diffUnknown:         "unknown",
	diffEvalRefused:     "evaluator-refused",
	diffNotTranslated:   "skipped-by-refusal",
	diffNoValues:        "skipped-without-values",
	diffAssumptionUnmet: "assumption-unmet",
}

func (o diffOutcome) String() string {
	if name, ok := diffOutcomeNames[o]; ok {
		return name
	}
	return "?"
}

// diffSummary counts what a run of the gate found, so coverage drift over a
// corpus is reviewable rather than invisible.
type diffSummary struct {
	label    string
	files    int
	elements int
	counts   map[diffOutcome]int
	// refusals counts the refusal reasons, most common first in the report.
	refusals map[string]int
}

// newSummary starts a summary of a run named by label.
func newSummary(label string) *diffSummary {
	return &diffSummary{label: label, counts: map[diffOutcome]int{}, refusals: map[string]int{}}
}

// count records one element's outcome.
func (s *diffSummary) count(o diffOutcome) {
	s.elements++
	s.counts[o]++
}

// refused records why an element was not translated.
func (s *diffSummary) refused(reason string) {
	s.count(diffNotTranslated)
	s.refusals[reason]++
}

// translated is the number of elements the translation accepted, whether or not
// concrete values were found for them.
func (s *diffSummary) translated() int {
	return s.elements - s.counts[diffNotTranslated]
}

// report logs the counts, which is what makes the gate's coverage reviewable.
func (s *diffSummary) report(t *testing.T) {
	t.Helper()
	t.Logf("differential gate %s: %d files, %d elements: %d translated, %d %s, "+
		"%d agreed, %d disagreed, %d unknown, %d evaluator-refused, %d assumption-unmet, %d without values",
		s.label, s.files, s.elements, s.translated(),
		s.counts[diffNotTranslated], diffNotTranslated,
		s.counts[diffAgreed], s.counts[diffDisagreed], s.counts[diffUnknown],
		s.counts[diffEvalRefused], s.counts[diffAssumptionUnmet], s.counts[diffNoValues])
	for _, reason := range topReasons(s.refusals) {
		t.Logf("  refused %3d× %s", s.refusals[reason], reason)
	}
}

// topReasons orders refusal reasons by how often they were reported, then by
// text, so the report is deterministic.
func topReasons(reasons map[string]int) []string {
	out := make([]string, 0, len(reasons))
	for reason := range reasons {
		out = append(out, reason)
	}
	sort.Slice(out, func(i, j int) bool {
		if reasons[out[i]] != reasons[out[j]] {
			return reasons[out[i]] > reasons[out[j]]
		}
		return out[i] < out[j]
	})
	return out
}

// diffElement is one candidate the gate checks: what to translate, and what to
// evaluate.
type diffElement struct {
	// file names the document it was declared in.
	file string

	// kind is "constraint", "requirement" or "satisfaction".
	kind string

	// name is the element as a verdict about it would name it.
	name string

	// sym is the declaration, nil for none.
	sym *symbols.Symbol

	// scope stands in for sym's own scope when it declares none.
	scope *symbols.Scope

	// assertion is the satisfaction assertion, set only for kind
	// "satisfaction".
	assertion *runtime.SatisfyAssertion

	// host is the declaration the element is stated inside, whose object carries
	// the values its conditions read; nil when a package states it.
	host *symbols.Symbol
}

// label names the element as a report quotes it.
func (e diffElement) label() string { return e.kind + " " + e.name }

// translate translates the element as the surfaces that solve it do.
func (e diffElement) translate(ctx *runtime.Context) (*Query, error) {
	switch e.kind {
	case "constraint":
		return Constraint(ctx, e.sym, e.scope)
	case "requirement":
		return Requirement(ctx, e.sym, e.scope)
	case "satisfaction":
		return Satisfaction(ctx, e.assertion)
	}
	return nil, fmt.Errorf("differential: unknown element kind %q", e.kind)
}

// subject is the element as the translation names it, which is also how a
// variable's name is built.
func (e diffElement) subject() Subject {
	switch e.kind {
	case "satisfaction":
		return Subject{Kind: e.kind, Name: e.assertion.Text(), Symbol: e.assertion.Symbol, Negated: e.assertion.Negated}
	default:
		return Subject{Kind: e.kind, Name: e.sym.Name, Symbol: e.sym, Negated: runtime.NegatedDecl(e.sym)}
	}
}

// conditions are the conditions the evaluator checks and the translation reads,
// which is one list for both layers.
func (e diffElement) conditions(ctx *runtime.Context) []runtime.Condition {
	if e.kind == "satisfaction" {
		return ctx.ConditionsOf(e.assertion.Symbol, e.assertion.Symbol.OwnerScope)
	}
	return ctx.ConditionsOf(e.sym, e.scope)
}

// verdict is what the evaluator answered about an element: whether its
// conditions hold, the object it turned out to be about, and the typed error it
// returned instead of a verdict.
type verdict struct {
	holds   bool
	err     error
	subject *runtime.Instance
}

// violated reports whether the evaluator answered that the conditions do not
// hold, which a required condition failing reports as a violation error.
func (v verdict) violated() bool {
	return !v.holds && errors.Is(v.err, runtime.ErrViolated)
}

// refused reports whether the evaluator returned no verdict at all.
func (v verdict) refused() bool {
	return v.err != nil && !errors.Is(v.err, runtime.ErrViolated)
}

// text renders the verdict as a report quotes it.
func (v verdict) text() string {
	switch {
	case v.err == nil && v.holds:
		return "the conditions hold"
	case v.violated():
		return "the conditions do not hold: " + v.err.Error()
	case v.err != nil:
		return "no verdict: " + v.err.Error()
	default:
		return "the conditions do not hold"
	}
}

// evaluate evaluates the element through the runtime evaluator, which stays the
// normative answer the solver is compared against.
func (e diffElement) evaluate(ctx *runtime.Context) verdict {
	switch e.kind {
	case "constraint":
		result, err := ctx.CheckConstraintOn(e.sym, e.scope, e.object(ctx))
		return verdict{holds: result.Holds, err: err, subject: result.Subject}
	case "requirement":
		result, err := ctx.CheckRequirementOn(e.sym, e.scope, e.object(ctx))
		return verdict{holds: result.Holds, err: err, subject: result.Subject}
	case "satisfaction":
		result, err := ctx.CheckSatisfactionOn(e.assertion, nil)
		return verdict{holds: result.Holds, err: err, subject: result.Subject}
	}
	return verdict{err: fmt.Errorf("differential: unknown element kind %q", e.kind)}
}

// object is an instance of the declaration the element is stated inside, which
// is how the conformance corpus checks a condition: on the object carrying the
// values it reads. A declaration no object can be made of leaves the evaluator
// to resolve the element's own declarations.
func (e diffElement) object(ctx *runtime.Context) *runtime.Instance {
	if e.host == nil {
		return nil
	}
	inst, err := ctx.Instantiate(e.host)
	if err != nil {
		return nil
	}
	return inst
}

// errNoConcreteValue is reported for a variable no concrete value is declared
// for, which is a coverage gap rather than a disagreement.
var errNoConcreteValue = errors.New("no concrete value")

// values supplies the concrete value each variable of a query stands at.
type values interface {
	// valueOf returns the value v is pinned to, wrapping errNoConcreteValue when
	// the model declares none.
	valueOf(v *Var) (runtime.Value, error)

	// origin names where the values come from, for a report.
	origin() string
}

// declaredValues reads the value the evaluator reads for a variable: the value
// an effective feature of the element declares, else the value the object under
// check holds for it, walking a feature chain step by step. That is the
// evaluator's own order of resolution (runtime.EvalContext.evalName): a valued
// feature of the element masks the object's, which masks the declaration's.
type declaredValues struct {
	ctx *runtime.Context

	// features maps the qualified name a variable carries to the effective
	// feature of the element it stands for.
	features map[string]runtime.EffectiveFeature

	// subject is the object the evaluator reached its verdict about, nil when it
	// evaluated the declarations themselves.
	subject *runtime.Instance
}

// newDeclaredValues indexes the element's effective features by qualified name,
// which is how a query names the variable reading one.
func newDeclaredValues(ctx *runtime.Context, sym *symbols.Symbol, subject *runtime.Instance) *declaredValues {
	features := map[string]runtime.EffectiveFeature{}
	for _, feat := range ctx.FeaturesOf(sym) {
		if feat.Symbol == nil {
			continue
		}
		if fqn := symbols.FQNOf(feat.Symbol); fqn != "" {
			features[fqn] = feat
		}
	}
	return &declaredValues{ctx: ctx, features: features, subject: subject}
}

func (d *declaredValues) origin() string { return "declared" }

func (d *declaredValues) valueOf(v *Var) (runtime.Value, error) {
	steps := strings.Split(v.Name, ".")
	val, err := d.root(v, steps[0])
	if err != nil {
		return runtime.Value{}, err
	}
	for _, step := range steps[1:] {
		if val, err = d.step(val, step); err != nil {
			return runtime.Value{}, err
		}
	}
	return val, nil
}

// root reads the value of the feature a variable's name starts at, in the order
// the evaluator resolves a name in: a valued feature of the element, then the
// object under check, then the declaration the scope answers with.
func (d *declaredValues) root(v *Var, fqn string) (runtime.Value, error) {
	if feat, ok := d.features[fqn]; ok && feat.DefaultValue != nil {
		val, err := d.ctx.EvalWithScope(feat.DefaultValue, feat.DefaultScope())
		if err != nil {
			return runtime.Value{}, declaredValueError(fqn, err)
		}
		return val, nil
	}
	if val, err := d.held(simpleName(fqn)); err == nil {
		return val, nil
	}
	if usage, declared := declaredUsage(v, fqn); declared && usage.Value != nil {
		val, err := d.ctx.EvalWithScope(usage.Value, v.Symbol.OwnerScope)
		if err != nil {
			return runtime.Value{}, declaredValueError(fqn, err)
		}
		return val, nil
	}
	return runtime.Value{}, fmt.Errorf("%w: nothing declares one for %s", errNoConcreteValue, fqn)
}

// declaredValueError reports a declared value that does not evaluate. One that
// reads a feature holding no value — a case's subject the run supplies — is a
// concrete value nothing declares, not a disagreement.
func declaredValueError(fqn string, err error) error {
	if errors.Is(err, runtime.ErrNoValue) {
		return fmt.Errorf("%w: %s declares a value reading one that is unbound: %v", errNoConcreteValue, fqn, err)
	}
	return fmt.Errorf("%s declares a value that does not evaluate: %w", fqn, err)
}

// declaredUsage is the declaration a variable reads directly, which is the
// variable's own symbol only when its name walks no feature chain.
func declaredUsage(v *Var, fqn string) (*ast.Usage, bool) {
	if v.Symbol == nil || v.Name != fqn {
		return nil, false
	}
	usage, ok := v.Symbol.Decl.(*ast.Usage)
	return usage, ok
}

// simpleName is the name a feature answers to, unqualified.
func simpleName(fqn string) string {
	if at := strings.LastIndex(fqn, "::"); at >= 0 {
		return fqn[at+2:]
	}
	return fqn
}

// held reads the value the object under check holds for a named feature.
func (d *declaredValues) held(name string) (runtime.Value, error) {
	if d.subject == nil {
		return runtime.Value{}, fmt.Errorf("%w: no object under check", errNoConcreteValue)
	}
	fv, err := d.subject.GetFeatureValue(d.ctx, name)
	if err != nil || fv == nil {
		return runtime.Value{}, fmt.Errorf("%w: object %d holds none for %s (%v)",
			errNoConcreteValue, d.subject.ID, name, err)
	}
	val := fv.HeldValue()
	if val.Kind == runtime.ValInvalid {
		return runtime.Value{}, fmt.Errorf("%w: object %d holds nothing for %s", errNoConcreteValue, d.subject.ID, name)
	}
	return val, nil
}

// step reads a feature of the object a chain's previous step named.
func (d *declaredValues) step(val runtime.Value, name string) (runtime.Value, error) {
	id, ok := val.Object()
	if !ok {
		return runtime.Value{}, fmt.Errorf("%w: %s is read through a value that is no object", errNoConcreteValue, name)
	}
	inst, ok := d.ctx.Instance(id)
	if !ok {
		return runtime.Value{}, fmt.Errorf("%w: object %d is not materialized", errNoConcreteValue, id)
	}
	fv, err := inst.GetFeatureValue(d.ctx, name)
	if err != nil || fv == nil {
		return runtime.Value{}, fmt.Errorf("%w: object %d holds none for %s (%v)", errNoConcreteValue, id, name, err)
	}
	return fv.HeldValue(), nil
}

// nodeValues reads, for each variable, the value the evaluator reads for the
// reference that variable came from: the condition's own node, evaluated in the
// condition's scope against the object under check. A variable the walk did not
// reach, or a reference the evaluator refused, falls back to declaredValues.
type nodeValues struct {
	byName   map[string]runtime.Value
	fallback *declaredValues
}

func (n *nodeValues) origin() string { return "evaluated" }

func (n *nodeValues) valueOf(v *Var) (runtime.Value, error) {
	if val, ok := n.byName[v.Name]; ok {
		return val, nil
	}
	return n.fallback.valueOf(v)
}

// newNodeValues evaluates every reference the conditions state, naming each by
// the variable the translation gives it.
func newNodeValues(ctx *runtime.Context, el diffElement, self *runtime.Instance) *nodeValues {
	out := &nodeValues{byName: map[string]runtime.Value{}, fallback: newDeclaredValues(ctx, valueHost(el), self)}
	subject := el.subject()
	var collect func(conds []runtime.Condition)
	collect = func(conds []runtime.Condition) {
		for _, cond := range conds {
			collect(cond.Group)
			if cond.Expr == nil || cond.Scope == nil {
				continue
			}
			for _, node := range referenceNodes(cond.Expr) {
				name, ok := varNameOfReference(ctx, subject, node, cond.Scope)
				if !ok {
					continue
				}
				val, err := ctx.EvalWithScopeOn(node, cond.Scope, self)
				if err != nil {
					continue
				}
				out.byName[name] = val
			}
		}
	}
	collect(el.conditions(ctx))
	return out
}

// varNameOfReference is the name the translation gives the variable a reference reads,
// reporting false for a reference it grounds as a value or refuses.
func varNameOfReference(ctx *runtime.Context, subject Subject, node ast.Node, scope *symbols.Scope) (string, bool) {
	t := &translator{
		ctx:       ctx,
		model:     ctx.Semantics(),
		subject:   subject,
		features:  effectiveFeatures(ctx, subject.Symbol),
		vars:      map[string]*Var{},
		sorts:     map[string]Sort{},
		guarded:   map[string]bool{},
		baseUnits: map[string]string{},
	}
	term, err := t.reference(node, scope)
	if err != nil || term.Op != OpVar || term.Var == nil {
		return "", false
	}
	return term.Var.Name, true
}

// referenceNodes are the references an expression reads, a feature chain taken
// whole: what the translation turns into one variable, the evaluator evaluates
// as one name.
func referenceNodes(node ast.Node) []ast.Node {
	switch n := node.(type) {
	case *ast.FeatureReference:
		return []ast.Node{n}
	case *ast.FeatureChainExpr:
		return []ast.Node{n}
	case *ast.OperatorExpr:
		var out []ast.Node
		for _, operand := range n.Operands {
			out = append(out, referenceNodes(operand)...)
		}
		return out
	case *ast.IndexExpr:
		return referenceNodes(n.Operand)
	}
	return nil
}

// pin is one variable fixed to a concrete value.
type pin struct {
	v     *Var
	value runtime.Value
	term  *Term
	text  string
}

// pins reads a concrete value for every variable of the query, refusing the
// whole element when one is missing: a query with a free variable answers a
// question the evaluator was not asked.
func pinsOf(q *Query, from values) ([]pin, error) {
	out := make([]pin, 0, len(q.Vars))
	for _, v := range q.Vars {
		val, err := from.valueOf(v)
		if err != nil {
			return nil, err
		}
		term, text, err := pinTerm(v, val)
		if err != nil {
			return nil, err
		}
		out = append(out, pin{v: v, value: val, term: term, text: text})
	}
	return out, nil
}

// pinTerm renders a runtime value as a term of the variable's sort, scaling a
// quantity to the base units the translation expresses magnitudes in.
func pinTerm(v *Var, val runtime.Value) (*Term, string, error) {
	switch v.Sort.Kind {
	case SortBool:
		if val.Kind == runtime.ValConst && val.Const.Kind == semantics.ValBool {
			return BoolTerm(val.Const.Bool), fmt.Sprintf("%t", val.Const.Bool), nil
		}
	case SortInt:
		if val.Kind == runtime.ValConst && val.Const.Kind == semantics.ValInt {
			return IntTerm(val.Const.Int), fmt.Sprintf("%d", val.Const.Int), nil
		}
	case SortReal:
		if rat, ok := ratOfValue(val); ok {
			return RealTerm(rat), renderRat(rat), nil
		}
	case SortString:
		if val.Kind == runtime.ValString {
			return StringTerm(val.Str()), `"` + val.Str() + `"`, nil
		}
	case SortDatatype:
		if name, ok := datatypeValueOf(v.Sort, val); ok {
			return ValueTerm(v.Sort, name), name, nil
		}
	}
	return nil, "", fmt.Errorf("%w: %s holds %s, which is no value of %s",
		errNoConcreteValue, v.Name, val.Kind, v.Sort.Name)
}

// ratOfValue reads a magnitude as the exact rational the translation asserts
// about: a quantity is scaled to its base units, as a quantity literal is.
func ratOfValue(val runtime.Value) (*big.Rat, bool) {
	switch val.Kind {
	case runtime.ValConst:
		return ratOfConst(val.Const)
	case runtime.ValQuantity:
		if val.Quantity() == nil {
			return nil, false
		}
		magnitude, ok := ratOfConst(val.Quantity().Num)
		if !ok {
			return nil, false
		}
		scale, ok := ratOfScale(val.Quantity().Unit.Term.Normalized().Scale)
		if !ok {
			return nil, false
		}
		return magnitude.Mul(magnitude, scale), true
	}
	return nil, false
}

// datatypeValueOf names the value of a finite sort a runtime value is: an
// enumeration literal or a selected variant, by qualified name.
func datatypeValueOf(sort Sort, val runtime.Value) (string, bool) {
	var sym *symbols.Symbol
	switch val.Kind {
	case runtime.ValEnumLiteral:
		sym = val.Literal()
	case runtime.ValVariant:
		sym = val.Variant()
	}
	if sym == nil {
		return "", false
	}
	name := symbols.FQNOf(sym)
	if name == "" {
		name = sym.Name
	}
	for _, candidate := range sort.Values {
		if candidate == name {
			return name, true
		}
	}
	return "", false
}

// pinnedQuery builds the query the solver decides: the translated one with every
// variable pinned, optionally with its conditions denied rather than asserted.
// The translated query is left untouched.
func pinnedQuery(q *Query, pins []pin, deny bool) *Query {
	out := &Query{
		Kind:            q.Kind,
		Element:         q.Element,
		Negated:         q.Negated,
		Sorts:           q.Sorts,
		Vars:            q.Vars,
		Nonlinear:       q.Nonlinear,
		IntegerDivision: q.IntegerDivision,
	}
	context, checked := splitAssertions(q)
	out.Assertions = append(out.Assertions, context...)
	for _, p := range pins {
		out.Assertions = append(out.Assertions, pinAssertion(p))
	}
	switch {
	case len(checked) == 0:
	case deny:
		terms := make([]*Term, 0, len(checked))
		for _, a := range checked {
			terms = append(terms, a.Term)
		}
		from := checked[0].From
		from.Condition = "not (" + from.Condition + ")"
		out.Assertions = append(out.Assertions, Assertion{Term: Not(And(terms...)), From: from})
	default:
		out.Assertions = append(out.Assertions, checked...)
	}
	return out
}

// contextQuery is the query without the conditions under check: the declared
// domains and the well-definedness guards, with the assumptions when asked for.
// It is what decides whether an assignment is one the evaluator answered about.
func contextQuery(q *Query, pins []pin, assumptions bool) *Query {
	out := &Query{
		Kind:            q.Kind,
		Element:         q.Element,
		Negated:         q.Negated,
		Sorts:           q.Sorts,
		Vars:            q.Vars,
		Nonlinear:       q.Nonlinear,
		IntegerDivision: q.IntegerDivision,
	}
	for _, a := range q.Assertions {
		if a.From.Role == RoleDomain || a.From.Role == RoleDefined ||
			(assumptions && a.From.Role == RoleAssumed) {
			out.Assertions = append(out.Assertions, a)
		}
	}
	for _, p := range pins {
		out.Assertions = append(out.Assertions, pinAssertion(p))
	}
	return out
}

// splitAssertions partitions a query into the context an assignment must meet —
// declared domains, well-definedness guards and assumptions — and the conditions
// the verdict is about.
func splitAssertions(q *Query) (context, checked []Assertion) {
	for _, a := range q.Assertions {
		switch a.From.Role {
		case RoleRequired, RoleDenied:
			checked = append(checked, a)
		default:
			context = append(context, a)
		}
	}
	return context, checked
}

// pinAssertion asserts that a variable holds the value it is pinned to, which is
// a bound on its values rather than a condition the model wrote.
func pinAssertion(p pin) Assertion {
	left, right := promote(VarTerm(p.v), p.term)
	return Assertion{
		Term: Binary(OpEq, Bool, left, right),
		From: Provenance{
			Kind:      "assignment",
			Element:   p.v.Name,
			Condition: p.v.Name + " = " + p.text,
			Role:      RoleDomain,
			Declared:  p.v.Symbol,
			File:      p.v.File,
			Span:      p.v.Span,
			Location:  p.v.Location,
		},
	}
}

// hasRole reports whether the query asserts a term in that role.
func hasRole(q *Query, role Role) bool {
	for _, a := range q.Assertions {
		if a.From.Role == role {
			return true
		}
	}
	return false
}

// diffGate runs differential checks against one solver, accumulating a summary.
type diffGate struct {
	solver  *Solver
	summary *diffSummary

	// verbose logs every checked element, not only the disagreements.
	verbose bool
}

// newGate discovers a solver, skipping loudly when there is none: the gate
// proves nothing without one, so it never reports agreement it did not check.
func newGate(t *testing.T, label string) *diffGate {
	t.Helper()
	return &diffGate{solver: requireSolver(t), summary: newSummary(label), verbose: testing.Verbose()}
}

// check runs the differential property for one element, counting the outcome.
func (g *diffGate) check(t *testing.T, ctx *runtime.Context, el diffElement) diffOutcome {
	t.Helper()
	q, err := el.translate(ctx)
	if err != nil {
		var refusal *NotTranslatableError
		switch {
		case errors.As(err, &refusal):
			g.summary.refused(refusal.Construct + ": " + refusal.Reason)
			return diffNotTranslated
		case errors.Is(err, ErrNotTranslatable), errors.Is(err, ErrNoConditions):
			g.summary.refused(err.Error())
			return diffNotTranslated
		}
		t.Errorf("%s: translating %s failed with an untyped error: %v", el.file, el.label(), err)
		g.summary.count(diffDisagreed)
		return diffDisagreed
	}

	answer := el.evaluate(ctx)
	pins, err := pinsOf(q, newNodeValues(ctx, el, answer.subject))
	if err != nil {
		if errors.Is(err, errNoConcreteValue) {
			if g.verbose {
				t.Logf("%s: %s: no assignment to check: %v", el.file, el.label(), err)
			}
			g.summary.count(diffNoValues)
			return diffNoValues
		}
		t.Errorf("%s: %s: reading the declared values failed: %v", el.file, el.label(), err)
		g.summary.count(diffDisagreed)
		return diffDisagreed
	}
	return g.compare(t, el, q, answer, pins)
}

// valueHost is the element whose effective features a variable's value is read
// through: the requirement's own for a satisfaction, since the assertion reads
// the requirement's parameters.
func valueHost(el diffElement) *symbols.Symbol {
	if el.kind == "satisfaction" && el.assertion != nil {
		return el.assertion.Symbol
	}
	return el.sym
}

// compare decides the differential property for one assignment and reports a
// disagreement with everything needed to debug it.
func (g *diffGate) compare(t *testing.T, el diffElement, q *Query, answer verdict, pins []pin) diffOutcome {
	t.Helper()

	// An assignment the declared domains or the well-definedness guards rule out
	// is one the evaluator does not answer about either.
	admissible, ok := g.status(t, el, contextQuery(q, pins, false))
	if !ok {
		return g.record(t, el, q, answer, pins, diffUnknown, "the solver did not decide the guards")
	}
	if admissible == StatusUnsat {
		if errors.Is(answer.err, runtime.ErrDivisionByZero) {
			return g.record(t, el, q, answer, pins, diffAgreed, "")
		}
		if answer.refused() {
			return g.record(t, el, q, answer, pins, diffEvalRefused,
				"the guards rule the assignment out and the evaluator reached no verdict")
		}
		return g.record(t, el, q, answer, pins, diffDisagreed,
			"the guards rule out an assignment the evaluator answered about")
	}
	if errors.Is(answer.err, runtime.ErrDivisionByZero) {
		return g.record(t, el, q, answer, pins, diffDisagreed,
			"the evaluator reports division by zero, but the guarded query admits the assignment")
	}

	// An assignment falsifying an assumption is outside what the query is about:
	// the query asserts an assumption the evaluator trusts.
	if hasRole(q, RoleAssumed) {
		assumed, ok := g.status(t, el, contextQuery(q, pins, true))
		if !ok {
			return g.record(t, el, q, answer, pins, diffUnknown, "the solver did not decide the assumptions")
		}
		if assumed == StatusUnsat {
			return g.record(t, el, q, answer, pins, diffAssumptionUnmet, "")
		}
	}

	holds, ok := g.status(t, el, pinnedQuery(q, pins, false))
	if !ok {
		return g.record(t, el, q, answer, pins, diffUnknown, "the solver did not decide the pinned query")
	}
	denied, ok := g.status(t, el, pinnedQuery(q, pins, true))
	if !ok {
		return g.record(t, el, q, answer, pins, diffUnknown, "the solver did not decide the denied query")
	}

	switch {
	case holds == StatusSat && denied == StatusSat:
		return g.record(t, el, q, answer, pins, diffDisagreed,
			"the pinned assignment decides neither verdict, so it does not pin every value the conditions read")
	case holds == StatusUnsat && denied == StatusUnsat:
		return g.record(t, el, q, answer, pins, diffDisagreed,
			"the pinned assignment admits no verdict at all")
	case answer.refused():
		return g.record(t, el, q, answer, pins, diffEvalRefused,
			fmt.Sprintf("the solver answered %s but the evaluator reached no verdict", holds))
	case holds == StatusSat:
		if answer.holds {
			return g.record(t, el, q, answer, pins, diffAgreed, "")
		}
		return g.record(t, el, q, answer, pins, diffDisagreed,
			"the solver satisfies the conditions for an assignment the evaluator says violates them")
	default:
		if !answer.holds {
			return g.record(t, el, q, answer, pins, diffAgreed, "")
		}
		return g.record(t, el, q, answer, pins, diffDisagreed,
			"the solver rules out an assignment the evaluator says satisfies the conditions")
	}
}

// status solves a query, failing the test when the solver process itself fails,
// and reporting false for an answer the solver did not decide.
func (g *diffGate) status(t *testing.T, el diffElement, q *Query) (Status, bool) {
	t.Helper()
	result, err := g.solver.Solve(context.Background(), q)
	if err != nil {
		t.Fatalf("%s: %s: solving failed: %v\nscript:\n%s", el.file, el.label(), err, Script(q))
	}
	if result.Status == StatusUnknown {
		if g.verbose {
			t.Logf("%s: %s: the solver did not decide the query: %s", el.file, el.label(), result.Reason)
		}
		return result.Status, false
	}
	return result.Status, true
}

// record counts an outcome and reports a disagreement as an error, with the
// element, its conditions, the assignment, both verdicts and the exact script.
func (g *diffGate) record(t *testing.T, el diffElement, q *Query, answer verdict,
	pins []pin, outcome diffOutcome, why string) diffOutcome {
	t.Helper()
	g.summary.count(outcome)
	switch {
	case outcome == diffDisagreed:
		t.Errorf("differential disagreement: %s", diffReport(el, q, answer, pins, why))
	case g.verbose:
		t.Logf("%s %s: %s: %s", el.file, el.label(), outcome, diffReport(el, q, answer, pins, why))
	}
	return outcome
}

// diffReport renders everything a disagreement needs to be debugged: the
// element, the conditions as written, the value of every variable, what the
// evaluator answered, and the script the solver was given.
func diffReport(el diffElement, q *Query, answer verdict, pins []pin, why string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s in %s", el.label(), el.file)
	if el.sym != nil {
		if loc := el.sym.DocName; loc != "" && el.sym.DeclSpan.Offset > 0 {
			fmt.Fprintf(&b, " (%s)", loc)
		}
	}
	if why != "" {
		fmt.Fprintf(&b, "\n  %s", why)
	}
	b.WriteString("\n  conditions:")
	for _, a := range q.Assertions {
		if a.From.Role == RoleDomain && a.From.Kind == "assignment" {
			continue
		}
		fmt.Fprintf(&b, "\n    %s: %s", a.From.Role, a.From.Condition)
		if a.From.Location != "" {
			fmt.Fprintf(&b, " at %s", a.From.Location)
		}
	}
	b.WriteString("\n  assignment:")
	if len(pins) == 0 {
		b.WriteString(" (the conditions read no feature)")
	}
	for _, p := range pins {
		fmt.Fprintf(&b, "\n    %s = %s : %s", p.v.Name, p.text, p.v.Sort.Name)
		if p.v.Location != "" {
			fmt.Fprintf(&b, " declared at %s", p.v.Location)
		}
	}
	fmt.Fprintf(&b, "\n  evaluator: %s", answer.text())
	fmt.Fprintf(&b, "\n  script:\n%s", Script(pinnedQuery(q, pins, false)))
	fmt.Fprintf(&b, "  denied script:\n%s", Script(pinnedQuery(q, pins, true)))
	return b.String()
}
