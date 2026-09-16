package runtime

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/lexer"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// ValidationStatus is what checking one assertion on one object decided.
type ValidationStatus int

const (
	// ValidationHolds is the assertion evaluating to true on the object.
	ValidationHolds ValidationStatus = iota
	// ValidationViolated is the model answering the assertion false.
	ValidationViolated
	// ValidationUndecided is an assertion that could not be evaluated.
	ValidationUndecided
)

// String names the status for a report a machine reads.
func (s ValidationStatus) String() string {
	switch s {
	case ValidationHolds:
		return "holds"
	case ValidationViolated:
		return "violated"
	default:
		return "undecided"
	}
}

// AssertionKind is the kind of assertion an object verdict is about.
type AssertionKind string

const (
	AssertionConstraint   AssertionKind = "constraint"
	AssertionRequirement  AssertionKind = "requirement"
	AssertionSatisfaction AssertionKind = "satisfaction"
)

// ObjectVerdict is one assertion checked on one object of a validated tree.
type ObjectVerdict struct {
	Kind AssertionKind
	// Element is the asserting element: the constraint, requirement or satisfy usage.
	Element *symbols.Symbol
	// Requirement is the requirement a requirement or satisfaction verdict is
	// about, whose verification cases a caller may report beside it.
	Requirement *symbols.Symbol
	// Text is the assertion as written, so an anonymous one can be named.
	Text string
	// Subject is the object evaluated against and Path the features walked to it
	// from the root, a collection element indexed as wheels[2]; empty for the root.
	Subject *Instance
	Path    []string
	Status  ValidationStatus
	// Err is the violation, or what left the assertion undecided; nil when it holds.
	Err error
}

// ValidationReport is what validating an object and the objects it holds found.
type ValidationReport struct {
	Root     *Instance
	Verdicts []ObjectVerdict
	// Bounded is true when the walk left nesting unreached — deeper than it
	// descends, or past its budget — so the unreached objects are unvalidated.
	Bounded bool
	// Unread are the feature values that could not be read, whose objects went unvalidated.
	Unread []error
}

// Valid reports whether the object is shown valid: it states at least one assertion,
// every assertion holds and every held object was reached.
func (r ValidationReport) Valid() bool {
	return len(r.Verdicts) > 0 && r.Status() == ValidationHolds && r.Complete()
}

// Complete reports whether the walk reached every object the root holds.
func (r ValidationReport) Complete() bool {
	return !r.Bounded && len(r.Unread) == 0
}

// Status is the status the report is judged by: one violation makes the object
// invalid whatever else is undecided, and one undecided assertion leaves it unshown.
func (r ValidationReport) Status() ValidationStatus {
	switch {
	case r.Count(ValidationViolated) > 0:
		return ValidationViolated
	case r.Count(ValidationUndecided) > 0:
		return ValidationUndecided
	}
	return ValidationHolds
}

// Count is how many verdicts have status.
func (r ValidationReport) Count(status ValidationStatus) int {
	n := 0
	for _, v := range r.Verdicts {
		if v.Status == status {
			n++
		}
	}
	return n
}

// validatedObject is one object the walk reached: its path from the root (a
// collection element indexed) the first time, and every feature it was read through.
type validatedObject struct {
	inst     *Instance
	path     []string
	holdings []holding
}

// holding is one feature an object was read through, by name and declaration,
// on the object holding it; a shared object has one per feature.
type holding struct {
	parent  *validatedObject
	name    string
	through *symbols.Symbol
	owner   *symbols.Symbol
}

// validationWalk collects the objects an object holds, depth first, under the
// bounds a materialization walk uses.
type validationWalk struct {
	ctx     *Context
	onPath  map[*symbols.Symbol]bool
	visited map[int64]*validatedObject
	read    map[*FeatureValue]bool
	budget  int
	bounded bool
	unread  []error
	objects []*validatedObject
}

// RequireObject is ErrNotAnObject unless sym has objects to validate: a definition
// or usage that is neither a namespace nor a data value (attribute, enumeration).
func RequireObject(sym *symbols.Symbol) error {
	switch sym.Kind {
	case symbols.SymbolAttributeDef, symbols.SymbolAttributeUsage,
		symbols.SymbolEnumerationDef, symbols.SymbolEnumerationUsage,
		symbols.SymbolConnectorEnd, symbols.SymbolCrossFeature, symbols.SymbolMultiplicity:
		return notAnObject(sym)
	}
	if sym.Kind.IsDefinition() || sym.IsFeature() {
		return nil
	}
	return notAnObject(sym)
}

func notAnObject(sym *symbols.Symbol) error {
	kind := sym.Notation()
	return fmt.Errorf("%w: %s is %s %s, which has no object to validate", ErrNotAnObject, sym.Name, articleFor(kind), kind)
}

// ValidateObject checks every assertion about root and the objects it holds: asserted
// constraints, carried requirements, and satisfactions (in scopes or the types) about them.
func (ctx *Context) ValidateObject(root *Instance, scopes []*symbols.Scope) (ValidationReport, error) {
	return ctx.validateObjectWithin(root, scopes, maxMaterializeBudget)
}

// validateObjectWithin is ValidateObject with room for budget feature reads and
// objects reached beyond the root.
func (ctx *Context) validateObjectWithin(root *Instance, scopes []*symbols.Scope, budget int) (ValidationReport, error) {
	if root == nil {
		return ValidationReport{}, errors.New("validate: no object")
	}
	if err := ctx.checkNotDestroyed(root); err != nil {
		return ValidationReport{Root: root}, err
	}
	w := ctx.walkHeldObjects(root, budget)
	report := ValidationReport{Root: root, Bounded: w.bounded, Unread: w.unread}
	// Objects of one shape reading only declared values share one verdict.
	defer ctx.ShareVerdicts()()
	// Every object's carried assertions are read first, since a satisfaction one
	// states may be about any object of the tree; verdicts then go out object by object.
	carried := make([][]ObjectVerdict, len(w.objects))
	position := make(map[int64]int, len(w.objects))
	var stated []*SatisfyAssertion
	for i, obj := range w.objects {
		var assertions []*SatisfyAssertion
		carried[i], assertions = ctx.carriedVerdicts(obj)
		stated = append(stated, assertions...)
		position[obj.inst.ID] = i
	}
	for _, scope := range scopes {
		stated = append(stated, ctx.SatisfyAssertionsIn(scope)...)
	}
	stated = distinctAssertions(stated)
	// A satisfaction resolving below the object `by` names is about that nested
	// object, so it is filed with the object it turned out to be about.
	satisfied := make([][]ObjectVerdict, len(w.objects))
	decided := map[satisfactionKey]bool{}
	for i, obj := range w.objects {
		for _, v := range ctx.satisfactionVerdicts(obj, stated) {
			key := satisfactionKey{v.Element, v.Subject.ID}
			if decided[key] {
				continue
			}
			decided[key] = true
			at := i
			if j, ok := position[v.Subject.ID]; ok {
				at = j
			}
			satisfied[at] = append(satisfied[at], v)
		}
	}
	for i := range w.objects {
		report.Verdicts = append(report.Verdicts, carried[i]...)
		report.Verdicts = append(report.Verdicts, satisfied[i]...)
	}
	return report, nil
}

// satisfactionKey is one assertion decided about one object, so an assertion
// resolving to the same nested object from two holders is reported once.
type satisfactionKey struct {
	assertion *symbols.Symbol
	subject   int64
}

// distinctAssertions keeps the first of each assertion, so one stated by a type
// and again in a scope is checked once.
func distinctAssertions(assertions []*SatisfyAssertion) []*SatisfyAssertion {
	var out []*SatisfyAssertion
	seen := map[*symbols.Symbol]bool{}
	for _, a := range assertions {
		if a == nil || a.Symbol == nil || seen[a.Symbol] {
			continue
		}
		seen[a.Symbol] = true
		out = append(out, a)
	}
	return out
}

// walkHeldObjects reaches the objects root holds, directly or through them, with
// room for budget feature reads and objects reached beyond the root.
func (ctx *Context) walkHeldObjects(root *Instance, budget int) *validationWalk {
	obj := &validatedObject{inst: root}
	w := &validationWalk{
		ctx:     ctx,
		onPath:  map[*symbols.Symbol]bool{root.Type: true},
		visited: map[int64]*validatedObject{root.ID: obj},
		read:    map[*FeatureValue]bool{},
		budget:  budget,
	}
	w.walk(obj, 0)
	return w
}

func (w *validationWalk) walk(obj *validatedObject, depth int) {
	w.objects = append(w.objects, obj)
	inst := obj.inst
	if w.ctx.checkNotDestroyed(inst) != nil {
		return
	}
	for _, of := range w.ctx.FeaturesOfObject(inst) {
		feat := of.Feature
		if of.Name == "" || !holdsObjects(feat) {
			continue
		}
		// A part of a type being expanded above, or deeper than the walk descends,
		// is cut only where reading it would begin objects of its own.
		if held := w.ctx.CompositeTypeOf(feat); held != nil && (depth >= maxMaterializeDepth || w.onPath[held]) && w.makesObjects(inst, of) {
			w.bounded = true
			continue
		}
		if shared := inst.FeatureValues[of.Name]; shared != nil {
			if w.read[shared] {
				continue
			}
			w.read[shared] = true
		}
		// The budget is charged per read and per object reached, and checked only
		// where the walk would spend it, so what was reached is validated whole.
		if w.budget <= 0 {
			w.bounded = true
			return
		}
		w.budget--
		fv, err := inst.GetFeatureValue(w.ctx, of.Name)
		if err != nil {
			w.unread = append(w.unread, fmt.Errorf("%s: %w", strings.Join(append(obj.path, of.Name), "."), err))
			continue
		}
		for _, child := range w.heldChildren(fv, lexer.NameText(of.Name)) {
			held := holding{parent: obj, name: of.Name, through: feat.Symbol, owner: feat.OwnerType}
			if reached, ok := w.visited[child.inst.ID]; ok {
				reached.holdings = append(reached.holdings, held)
				continue
			}
			if w.budget <= 0 {
				w.bounded = true
				return
			}
			w.budget--
			reached := child.validatedObject
			w.visited[reached.inst.ID] = reached
			reached.holdings = []holding{held}
			reached.path = append(append([]string(nil), obj.path...), child.segment)
			w.onPath[reached.inst.Type] = true
			w.walk(reached, depth+1)
			delete(w.onPath, reached.inst.Type)
		}
	}
}

// makesObjects reports whether reading a composite feature would materialize
// objects of its own: it is unread, and its lower bound or its body demands one.
func (w *validationWalk) makesObjects(inst *Instance, of ObjectFeature) bool {
	if fv := inst.FeatureValues[of.Name]; fv != nil && fv.Materialized {
		return false
	}
	lower := of.Feature.Multiplicity.Lower
	return !lower.Known || lower.Infinite || lower.Value > 0 || w.ctx.bodyBindsAFeature(of.Feature)
}

// heldChild is an object a feature value holds and the segment naming it there:
// the feature's name, indexed for a collection's element.
type heldChild struct {
	*validatedObject
	segment string
}

// heldChildren lists the objects a feature value holds, each under its segment.
func (w *validationWalk) heldChildren(fv *FeatureValue, segment string) []heldChild {
	var out []heldChild
	reach := func(val Value, segment string) {
		id, ok := val.Object()
		if !ok || w.ctx.HoldsNoValue(val) {
			return
		}
		if child, ok := w.ctx.Instance(id); ok {
			out = append(out, heldChild{&validatedObject{inst: child}, segment})
		}
	}
	if fv.Values.Kind == ValInvalid {
		reach(fv.Value, segment)
		return out
	}
	var elements []Value
	switch fv.Values.Kind {
	case ValSequence:
		if fv.Values.Sequence() != nil {
			elements = fv.Values.Sequence().Elements()
		}
	case ValSet:
		if fv.Values.Set() != nil {
			elements = fv.Values.Set().Elements()
		}
	}
	for i, val := range elements {
		reach(val, fmt.Sprintf("%s[%d]", segment, i+1))
	}
	return out
}

// carriedVerdicts checks each assertion declaration the object's types state about it once, inherited
// ones included and masked named ones left out; satisfaction assertions are returned for the subject search.
func (ctx *Context) carriedVerdicts(obj *validatedObject) ([]ObjectVerdict, []*SatisfyAssertion) {
	var verdicts []ObjectVerdict
	var stated []*SatisfyAssertion
	seen := map[*ast.Usage]bool{}
	for _, typ := range obj.inst.types() {
		var effective map[*symbols.Symbol]bool
		for _, member := range ctx.chainMembers(typ, typ.OwnerScope) {
			usage, ok := member.node.(*ast.Usage)
			if !ok || seen[usage] {
				continue
			}
			kind, asserted := assertionKindOf(usage)
			if !asserted {
				continue
			}
			sym := memberSymbol(member.scope, member.node)
			if sym == nil {
				continue
			}
			if sym.Name != "" {
				if effective == nil {
					effective = ctx.effectiveMembers(typ)
				}
				if !effective[sym] {
					continue
				}
			}
			seen[usage] = true
			switch kind {
			case AssertionSatisfaction:
				if a := ctx.satisfyAssertionOf(sym); a != nil {
					stated = append(stated, a)
				}
			case AssertionConstraint:
				result, err := ctx.CheckConstraintOn(sym, member.scope, obj.inst)
				verdicts = append(verdicts, ctx.objectVerdict(kind, sym, assertionText(usage, sym), obj, result, err))
			case AssertionRequirement:
				result, err := ctx.CheckRequirementOn(sym, member.scope, obj.inst)
				v := ctx.objectVerdict(kind, sym, assertionText(usage, sym), obj, result, err)
				v.Requirement = sym
				verdicts = append(verdicts, v)
			}
		}
	}
	return verdicts, stated
}

// assertionKindOf classifies a member asserting something about the object carrying
// it; an unasserted constraint usage is checked by name only, an assumed one never.
func assertionKindOf(usage *ast.Usage) (AssertionKind, bool) {
	switch usage.Kind {
	case ast.UsageConstraint:
		keyword := usage.PrefixKeyword
		if keyword == "" {
			keyword = usage.Keyword
		}
		switch keyword {
		case "assert", "inv":
			return AssertionConstraint, true
		}
		return "", false
	case ast.UsageRequirement:
		return AssertionRequirement, true
	case ast.UsageSatisfy:
		return AssertionSatisfaction, true
	default:
		return "", false
	}
}

// assertionText spells an asserting usage as written: its keywords and any name.
func assertionText(usage *ast.Usage, sym *symbols.Symbol) string {
	var parts []string
	if usage.PrefixKeyword != "" {
		parts = append(parts, usage.PrefixKeyword)
	}
	if usage.IsNegated {
		parts = append(parts, "not")
	}
	if usage.Keyword != "" {
		parts = append(parts, usage.Keyword)
	}
	if sym != nil && sym.Name != "" {
		parts = append(parts, lexer.NameText(sym.Name))
	}
	return strings.Join(parts, " ")
}

// satisfactionVerdicts checks each assertion obj is the subject of against it,
// in the order the assertions were stated.
func (ctx *Context) satisfactionVerdicts(obj *validatedObject, assertions []*SatisfyAssertion) []ObjectVerdict {
	var verdicts []ObjectVerdict
	for _, a := range assertions {
		if !ctx.subjectOf(a, obj) {
			continue
		}
		result, err := ctx.CheckSatisfactionOn(a, obj.inst)
		v := ctx.objectVerdict(AssertionSatisfaction, a.Symbol, a.Text(), obj, result, err)
		v.Requirement = a.AssertedRequirement()
		verdicts = append(verdicts, v)
	}
	return verdicts
}

// subjectOf reports whether obj is what an assertion's `by` names — an object of that
// feature, or one a chain reaches from its root — or, with no `by`, an object of the type stating it.
func (ctx *Context) subjectOf(a *SatisfyAssertion, obj *validatedObject) bool {
	if a.SubjectRef == "" {
		return a.Owner != nil && slices.ContainsFunc(obj.inst.types(), func(typ *symbols.Symbol) bool {
			return sameDeclaration(typ, a.Owner) || ctx.modelConforms(typ, a.Owner)
		})
	}
	if a.Subject == nil {
		return false
	}
	if a.SubjectChain == nil {
		return ctx.occursAs(obj, a.Subject)
	}
	return a.SubjectRoot != nil && ctx.reachedBy(obj, a.SubjectPath, a.SubjectRoot)
}

// reachedBy reports whether some chain of holdings named path leads up from obj
// to an object of root, trying every feature a shared object is held through.
func (ctx *Context) reachedBy(obj *validatedObject, path []string, root *symbols.Symbol) bool {
	if len(path) == 0 {
		return ctx.occursAs(obj, root)
	}
	if len(obj.holdings) == 0 {
		obj.holdings = ctx.holdersOf(obj.inst)
	}
	last := path[len(path)-1]
	for _, h := range obj.holdings {
		if h.name == last && ctx.reachedBy(h.parent, path[:len(path)-1], root) {
			return true
		}
	}
	return false
}

// holdersOf finds the features whose values hold inst, so a chain can be walked
// above a nested validated root.
func (ctx *Context) holdersOf(inst *Instance) []holding {
	ids := make([]int64, 0, len(ctx.instances))
	for id := range ctx.instances {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	var out []holding
	for _, id := range ids {
		candidate := ctx.instances[id]
		if candidate == nil || candidate == inst {
			continue
		}
		var holder *validatedObject
		for _, of := range ctx.FeaturesOfObject(candidate) {
			fv := candidate.FeatureValues[of.Name]
			if of.Name == "" || fv == nil || !slices.Contains(heldObjects(fv.HeldValue()), inst.ID) {
				continue
			}
			if holder == nil {
				holder = &validatedObject{inst: candidate}
			}
			out = append(out, holding{parent: holder, name: of.Name, through: of.Feature.Symbol, owner: of.Feature.OwnerType})
		}
	}
	return out
}

// occursAs reports whether obj is an object of sym: typed by it, or held by a
// feature declaring or redefining it.
func (ctx *Context) occursAs(obj *validatedObject, sym *symbols.Symbol) bool {
	declares := func(s *symbols.Symbol) bool { return sameDeclaration(s, sym) }
	if slices.ContainsFunc(obj.inst.types(), declares) {
		return true
	}
	for _, h := range obj.holdings {
		if declares(h.through) || slices.ContainsFunc(ctx.redefinedFeatures(h.through, h.owner), declares) {
			return true
		}
	}
	return false
}

// sameDeclaration reports whether two symbols stand for one declaration: one symbol,
// or the symbols two scope trees (an index's and a document's) built for one node.
func sameDeclaration(a, b *symbols.Symbol) bool {
	if a == b {
		return a != nil
	}
	return a != nil && b != nil && a.Decl != nil && a.Decl == b.Decl
}

// objectVerdict reports what a check on obj decided, about a nested object when
// the check resolved to one.
func (ctx *Context) objectVerdict(kind AssertionKind, sym *symbols.Symbol, text string, obj *validatedObject, result CheckResult, err error) ObjectVerdict {
	v := ObjectVerdict{Kind: kind, Element: sym, Text: text, Subject: obj.inst, Path: obj.path, Err: err}
	switch {
	case err == nil && result.Holds:
		v.Status = ValidationHolds
	case err == nil || isViolation(err):
		v.Status = ValidationViolated
	default:
		v.Status = ValidationUndecided
	}
	if result.Subject != nil && result.Subject != obj.inst && result.SubjectRoot == obj.inst {
		v.Subject = result.Subject
		v.Path = append(append([]string(nil), obj.path...), result.SubjectPath...)
	}
	return v
}

// isViolation reports whether err is the model answering false, not a failure to evaluate.
func isViolation(err error) bool {
	var violation *ViolationError
	return errors.As(err, &violation) || errors.Is(err, ErrViolated)
}
