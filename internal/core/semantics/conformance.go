package semantics

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// Viewpoint conformance (SysML v2 7.24): a concern is a requirement, so whether
// a viewpoint's framed concerns hold of what a view exposes is a requirement
// evaluation. The verdict rules are tool-defined — the spec leaves verification
// verdicts non-normative — and are stated in docs/project/spec-compliance.md.

// ErrNotAViewpoint reports a `satisfy` in a view body whose target is no
// viewpoint, so it asserts no viewpoint conformance.
var ErrNotAViewpoint = errors.New("not a viewpoint")

// Verdict is the outcome of a conformance question. A verdict is never guessed:
// what could not be evaluated is VerdictUnevaluable with a reason, never a pass.
type Verdict int

const (
	// VerdictConforms is a question answered in the affirmative.
	VerdictConforms Verdict = iota
	// VerdictViolated is a question answered in the negative: a concern the view
	// does not frame, or a condition that evaluated to false.
	VerdictViolated
	// VerdictUnevaluable is a question no answer could be reached for.
	VerdictUnevaluable
	// VerdictNotEvaluated is a structurally conforming concern whose conditions
	// were not evaluated, because the caller supplied no evaluator.
	VerdictNotEvaluated
)

func (v Verdict) String() string {
	switch v {
	case VerdictConforms:
		return "conforms"
	case VerdictViolated:
		return "violated"
	case VerdictUnevaluable:
		return "unevaluable"
	case VerdictNotEvaluated:
		return "not evaluated"
	}
	return "unknown"
}

// ConcernEvaluator evaluates a framed concern's conditions with one exposed
// element bound as its subject. It is implemented over the runtime's requirement
// engine, which the semantic model cannot import.
type ConcernEvaluator interface {
	EvaluateConcern(concern, element *symbols.Symbol) (bool, error)
}

// ViolationReporter tells whether an evaluator's error is a false verdict rather
// than a failure to evaluate. Without it, every error reads as a failure.
type ViolationReporter interface {
	IsViolation(err error) bool
}

// ConcernCheck is the verdict of one framed concern against one exposed element.
type ConcernCheck struct {
	Element *symbols.Symbol
	Holds   bool
	Err     error
}

// ConcernConformance is what became of one concern the viewpoint frames.
type ConcernConformance struct {
	// Concern is the viewpoint's framing and Target the concern it names, nil
	// when the framing names nothing resolvable.
	Concern *symbols.Symbol
	Target  *symbols.Symbol
	// Name is how the framed concern is written, for reporting.
	Name string
	// FramedBy is the view's framing of the same concern and FramedIn the view
	// declaring it. Both are nil when the view does not frame the concern.
	FramedBy *symbols.Symbol
	FramedIn *symbols.Symbol
	// Checks are the per-element verdicts, in exposed-element order.
	Checks  []ConcernCheck
	Verdict Verdict
	Reason  string
}

// PartyBinding is a stakeholder or actor of a viewpoint or view and what it is
// bound to. Reason is empty when the binding resolves.
type PartyBinding struct {
	Party  *symbols.Symbol
	Kind   string // "stakeholder" or "actor"
	Name   string
	Owner  *symbols.Symbol
	Bound  *symbols.Symbol
	Reason string
}

// ViewpointConformance is what became of one `satisfy` in a view's body.
type ViewpointConformance struct {
	// Satisfy is the satisfy usage, Ref its target as written and Viewpoint the
	// viewpoint it resolves to, nil when that is unresolved or no viewpoint.
	Satisfy   *symbols.Symbol
	Ref       string
	Viewpoint *symbols.Symbol
	// SatisfiedIn is the view declaring the satisfy: the view itself or one it
	// specializes.
	SatisfiedIn *symbols.Symbol
	Concerns    []ConcernConformance
	Parties     []PartyBinding
	Verdict     Verdict
	Reason      string
}

// ViewConformance is a view's conformance to the viewpoints it satisfies.
type ViewConformance struct {
	View       *symbols.Symbol
	Exposed    []*symbols.Symbol
	Viewpoints []ViewpointConformance
	Verdict    Verdict
}

// ViewConformance evaluates whether view conforms to the viewpoints its body
// satisfies: every concern a viewpoint frames must be framed by the view and
// must hold of what the view exposes. A nil evaluator answers the structural
// question alone; a non-view is ErrNotAView, a view satisfying nothing no error.
func (m *Model) ViewConformance(view *symbols.Symbol, eval ConcernEvaluator) (*ViewConformance, error) {
	if view == nil || !IsView(view) {
		return nil, ErrNotAView
	}
	exposed, err := m.ExposedElements(view)
	if err != nil {
		return nil, err
	}
	out := &ViewConformance{View: view, Exposed: exposed, Verdict: VerdictConforms}
	framings := m.viewFramings(view)
	for _, sat := range m.SatisfyMembersOf(view) {
		out.Viewpoints = append(out.Viewpoints, m.viewpointConformance(view, sat, framings, exposed, eval))
	}
	for _, vp := range out.Viewpoints {
		out.Verdict = worse(out.Verdict, vp.Verdict)
	}
	return out, nil
}

// SatisfyMembersOf returns the viewpoint-claiming `satisfy` usages in a view's
// body followed by those of the views it specializes, in declaration order and
// once each: a satisfy is inherited the way an expose is.
func (m *Model) SatisfyMembersOf(view *symbols.Symbol) []*symbols.Symbol {
	var out []*symbols.Symbol
	seen := map[*symbols.Symbol]bool{}
	add := func(owner *symbols.Symbol) {
		for _, sym := range usageMembersOfKind(owner, ast.UsageSatisfy) {
			if !seen[sym] && IsViewpointSatisfy(sym) {
				seen[sym] = true
				out = append(out, sym)
			}
		}
	}
	add(view)
	for _, super := range m.AllSupertypes(view) {
		if IsView(super) {
			add(super)
		}
	}
	return out
}

// FramedConcernsOf returns the `frame` concern usages a viewpoint or view
// declares followed by those it inherits, in declaration order and once each.
func (m *Model) FramedConcernsOf(sym *symbols.Symbol) []*symbols.Symbol {
	if sym == nil {
		return nil
	}
	var out []*symbols.Symbol
	seen := map[*symbols.Symbol]bool{}
	add := func(owner *symbols.Symbol) {
		for _, fc := range usageMembersOfKind(owner, ast.UsageFramedConcern) {
			if !seen[fc] {
				seen[fc] = true
				out = append(out, fc)
			}
		}
	}
	add(sym)
	for _, super := range m.AllSupertypes(sym) {
		add(super)
	}
	return out
}

// ConcernEvaluationTarget returns the element whose conditions answer a framing:
// the framing when it declares a concern of its own, else the concern it names,
// whose conditions a bare `frame <concern>;` does not restate.
func (m *Model) ConcernEvaluationTarget(fc *symbols.Symbol) *symbols.Symbol {
	if fc == nil {
		return nil
	}
	if declaresOwnConcern(fc) {
		return fc
	}
	if target := m.FramedConcernTarget(fc); target != nil {
		return target
	}
	return fc
}

// declaresOwnConcern reports whether a framing declares a concern rather than
// referencing one: it names a type, or states members of its own.
func declaresOwnConcern(fc *symbols.Symbol) bool {
	usage, ok := fc.Decl.(*ast.Usage)
	if !ok {
		return false
	}
	if len(usage.Members) > 0 {
		return true
	}
	for _, rel := range usage.Relationships {
		if rel != nil && rel.Kind == ast.RelTyping {
			return true
		}
	}
	return false
}

// FramedConcernTarget returns the concern a `frame` member frames: the concern
// definition it is typed by, or the concern usage it references — the element
// named, not what that element in turn specializes, whose values it may mask. It
// is nil when the framing names no concern.
func (m *Model) FramedConcernTarget(fc *symbols.Symbol) *symbols.Symbol {
	if fc == nil {
		return nil
	}
	if target := m.declaredConcernTarget(fc); target != fc {
		return target
	}
	return nil
}

// declaredConcernTarget resolves the concern sym names through a relationship it
// declares, skipping the implicit base an untyped framing would take.
func (m *Model) declaredConcernTarget(sym *symbols.Symbol) *symbols.Symbol {
	for _, rel := range RelationshipsOf(sym) {
		if rel == nil || rel.Target == nil {
			continue
		}
		switch rel.Kind {
		case ast.RelTyping, ast.RelSubsets, ast.RelRedefines, ast.RelReferences:
		default:
			continue
		}
		target := m.relationshipTarget(sym, rel)
		if target == nil || target == sym || !isConcern(target) {
			continue
		}
		return target
	}
	return nil
}

// viewFraming is one framing a view makes, and the view that declared it.
type viewFraming struct {
	frame  *symbols.Symbol
	target *symbols.Symbol
	in     *symbols.Symbol
}

// viewFramings collects a view's own framings, then its supertypes', then those
// of the views nested in it — a nested view frames for its container, since the
// tree as a whole is what the container's conformance addresses (tool-defined).
func (m *Model) viewFramings(view *symbols.Symbol) []viewFraming {
	var out []viewFraming
	seen := map[*symbols.Symbol]bool{}
	var walk func(sym *symbols.Symbol, depth int)
	walk = func(sym *symbols.Symbol, depth int) {
		if sym == nil || seen[sym] || depth > 32 {
			return
		}
		seen[sym] = true
		out = append(out, m.framingsDeclaredBy(sym)...)
		nested, err := m.NestedViews(sym)
		if err != nil {
			return
		}
		for _, child := range nested {
			walk(child, depth+1)
		}
	}
	walk(view, 0)
	return out
}

// framingsDeclaredBy returns the framings of a view and of the views it
// specializes, each attributed to the view that declares it.
func (m *Model) framingsDeclaredBy(view *symbols.Symbol) []viewFraming {
	var out []viewFraming
	seen := map[*symbols.Symbol]bool{}
	add := func(owner *symbols.Symbol) {
		for _, fc := range usageMembersOfKind(owner, ast.UsageFramedConcern) {
			if seen[fc] {
				continue
			}
			seen[fc] = true
			out = append(out, viewFraming{frame: fc, target: m.FramedConcernTarget(fc), in: owner})
		}
	}
	add(view)
	for _, super := range m.AllSupertypes(view) {
		add(super)
	}
	return out
}

// viewpointConformance evaluates one satisfy member of a view.
func (m *Model) viewpointConformance(view, sat *symbols.Symbol, framings []viewFraming, exposed []*symbols.Symbol, eval ConcernEvaluator) ViewpointConformance {
	out := ViewpointConformance{Satisfy: sat, SatisfiedIn: sat.Owner()}
	target, ref := m.SatisfyTarget(sat)
	out.Ref = ref
	switch {
	case target == nil:
		out.Verdict = VerdictUnevaluable
		out.Reason = fmt.Sprintf("satisfy target %s does not resolve", quoteRef(ref))
		return out
	case !IsViewpoint(target):
		out.Verdict = VerdictUnevaluable
		out.Reason = fmt.Sprintf("satisfy target %s is a %s, %v", quoteRef(ref), target.Kind.String(), ErrNotAViewpoint)
		return out
	}
	out.Viewpoint = target
	out.Parties = m.partyBindings(target)
	out.Parties = append(out.Parties, m.partyBindings(view)...)
	out.Verdict = VerdictConforms
	for _, fc := range m.FramedConcernsOf(target) {
		out.Concerns = append(out.Concerns, m.concernConformance(fc, framings, exposed, eval))
	}
	// A viewpoint framing no concern asks nothing of the view, so conforming to
	// it would be a verdict reached by checking nothing.
	if len(out.Concerns) == 0 {
		out.Verdict = VerdictUnevaluable
		out.Reason = fmt.Sprintf("%s frames no concern", quoteRef(ref))
		return out
	}
	for _, c := range out.Concerns {
		out.Verdict = worse(out.Verdict, c.Verdict)
	}
	for _, p := range out.Parties {
		if p.Reason != "" {
			out.Verdict = worse(out.Verdict, VerdictUnevaluable)
		}
	}
	return out
}

// concernConformance answers whether the view frames one concern of the
// viewpoint and, when an evaluator is supplied, whether it holds of what the
// view exposes.
func (m *Model) concernConformance(fc *symbols.Symbol, framings []viewFraming, exposed []*symbols.Symbol, eval ConcernEvaluator) ConcernConformance {
	out := ConcernConformance{Concern: fc, Target: m.FramedConcernTarget(fc), Name: framedConcernName(fc)}
	if ref := m.unresolvedConcernRef(fc); ref != "" {
		out.Verdict = VerdictUnevaluable
		out.Reason = fmt.Sprintf("the framing names %s, which does not resolve", ref)
		return out
	}
	for _, f := range framings {
		if m.framesTheSame(out, f) {
			out.FramedBy, out.FramedIn = f.frame, f.in
			break
		}
	}
	if out.FramedBy == nil {
		out.Verdict = VerdictViolated
		out.Reason = "framed by the viewpoint but not by the view"
		return out
	}
	if eval == nil {
		out.Verdict = VerdictNotEvaluated
		return out
	}
	m.evaluateConcern(&out, exposed, eval)
	return out
}

// unresolvedConcernRef names what a framing references and could not resolve, so
// the concern it frames is unknown rather than unframed by the view.
func (m *Model) unresolvedConcernRef(fc *symbols.Symbol) string {
	for _, rel := range RelationshipsOf(fc) {
		if rel == nil || rel.Target == nil {
			continue
		}
		switch rel.Kind {
		case ast.RelTyping, ast.RelSubsets, ast.RelReferences:
		default:
			continue
		}
		if m.relationshipTarget(fc, rel) == nil {
			return quoteRef(refName(rel.Target))
		}
	}
	return ""
}

// framesTheSame reports whether a view's framing frames the concern the
// viewpoint's does: the same concern, or one specializing the other. Where
// either side names no concern — an untyped `frame concern mass;` — the written
// name decides, which is all the two framings share (tool-defined).
func (m *Model) framesTheSame(c ConcernConformance, f viewFraming) bool {
	if c.Target != nil && f.target != nil {
		return c.Target == f.target || m.Conforms(f.target, c.Target) || m.Conforms(c.Target, f.target)
	}
	return c.Target == f.frame || (c.Name != "" && c.Name == framedConcernName(f.frame))
}

// evaluateConcern evaluates a framed concern against the exposed elements its
// subject admits, and must hold of every one of them (tool-defined). An exposed
// set with no element the subject admits is unevaluable, never a pass.
func (m *Model) evaluateConcern(out *ConcernConformance, exposed []*symbols.Symbol, eval ConcernEvaluator) {
	concern := m.ConcernEvaluationTarget(out.Concern)
	subjectType := m.concernSubjectType(concern)
	var evaluated int
	out.Verdict = VerdictConforms
	for _, elem := range exposed {
		if subjectType != nil && !m.Conforms(elem, subjectType) {
			continue
		}
		if !instantiable(elem) {
			continue
		}
		evaluated++
		holds, err := eval.EvaluateConcern(concern, elem)
		check := ConcernCheck{Element: elem, Holds: holds && err == nil, Err: err}
		out.Checks = append(out.Checks, check)
		switch {
		case err == nil && holds:
		case isViolation(eval, err) || (err == nil && !holds):
			out.Verdict = worse(out.Verdict, VerdictViolated)
			if out.Reason == "" {
				out.Reason = violationReason(err)
			}
		default:
			out.Verdict = worse(out.Verdict, VerdictUnevaluable)
			if out.Reason == "" {
				out.Reason = fmt.Sprintf("could not be evaluated: %v", err)
			}
		}
	}
	if evaluated == 0 {
		out.Verdict = VerdictUnevaluable
		out.Reason = m.noSubjectReason(subjectType, exposed)
	}
}

// noSubjectReason says why a concern had no exposed element to be evaluated
// against, which is never read as a pass.
func (m *Model) noSubjectReason(subjectType *symbols.Symbol, exposed []*symbols.Symbol) string {
	if len(exposed) == 0 {
		return "no subject to evaluate: the view exposes nothing"
	}
	if subjectType != nil {
		return fmt.Sprintf("no subject to evaluate: no exposed element is a %s", subjectType.Name)
	}
	return "no subject to evaluate: no exposed element is an object"
}

// violationReason names the condition a violation reported, so the verdict reads
// the way requirement evaluation already reports one.
func violationReason(err error) string {
	if err != nil {
		return err.Error()
	}
	return "a required condition does not hold"
}

// isViolation reports whether err is a verdict rather than a failure to reach
// one, which only the evaluator can tell.
func isViolation(eval ConcernEvaluator, err error) bool {
	if err == nil {
		return false
	}
	reporter, ok := eval.(ViolationReporter)
	return ok && reporter.IsViolation(err)
}

// concernSubjectType returns the type of the concern's subject, which decides
// which exposed elements it is about. It is nil for an absent or untyped
// subject, and the concern is then evaluated against every exposed element.
func (m *Model) concernSubjectType(concern *symbols.Symbol) *symbols.Symbol {
	for _, owner := range append([]*symbols.Symbol{concern}, m.AllSupertypes(concern)...) {
		if owner.Scope == nil {
			continue
		}
		for _, member := range declaredMembers(owner.Scope) {
			if !isSubject(member) {
				continue
			}
			if declared := m.declaredSubjectType(member); declared != nil {
				return declared
			}
		}
	}
	return nil
}

// declaredSubjectType resolves the type a subject parameter declares, ignoring
// the implicit standard-library base an untyped `subject;` takes.
func (m *Model) declaredSubjectType(subject *symbols.Symbol) *symbols.Symbol {
	for _, rel := range RelationshipsOf(subject) {
		if rel == nil || rel.Target == nil || rel.Kind != ast.RelTyping {
			continue
		}
		return m.relationshipTarget(subject, rel)
	}
	return nil
}

// partyBindings reports the stakeholders and actors of a viewpoint or view and
// whether each resolves: a party naming something unresolved is reported rather
// than passed over, since its concern is about somebody.
func (m *Model) partyBindings(sym *symbols.Symbol) []PartyBinding {
	var out []PartyBinding
	for _, owner := range append([]*symbols.Symbol{sym}, m.AllSupertypes(sym)...) {
		for _, party := range usageMembersOfKind(owner, ast.UsageStakeholder, ast.UsageActor) {
			out = append(out, m.partyBinding(party, owner))
		}
	}
	return out
}

// partyBinding resolves one stakeholder or actor.
func (m *Model) partyBinding(party, owner *symbols.Symbol) PartyBinding {
	kind := "actor"
	if usage, ok := party.Decl.(*ast.Usage); ok && usage.Kind == ast.UsageStakeholder {
		kind = "stakeholder"
	}
	name, _ := ast.EffectiveName(party.Decl.(*ast.Usage))
	if name == "" {
		name = party.Name
	}
	out := PartyBinding{Party: party, Kind: kind, Name: name, Owner: owner}
	for _, rel := range RelationshipsOf(party) {
		if rel == nil || rel.Target == nil {
			continue
		}
		switch rel.Kind {
		case ast.RelTyping, ast.RelSubsets, ast.RelRedefines, ast.RelReferences:
		default:
			continue
		}
		if target := m.relationshipTarget(party, rel); target != nil {
			out.Bound = target
			return out
		}
		out.Reason = fmt.Sprintf("%s %s names %s, which does not resolve", kind, name, quoteRef(refName(rel.Target)))
		return out
	}
	// An untyped party (`stakeholder engineer;`) is bound to the standard
	// library's Part, which is a binding like any other.
	return out
}

// SatisfyTarget returns the element a satisfy member names and the reference as
// written. A satisfy reference is recorded as a subsetting rather than a
// reference subsetting, so it has no effective name to read the target from.
func (m *Model) SatisfyTarget(sat *symbols.Symbol) (*symbols.Symbol, string) {
	for _, rel := range RelationshipsOf(sat) {
		if rel == nil || rel.Target == nil {
			continue
		}
		switch rel.Kind {
		case ast.RelSubsets, ast.RelReferences, ast.RelTyping:
		default:
			continue
		}
		return m.relationshipTarget(sat, rel), refName(rel.Target)
	}
	return nil, ""
}

// IsViewpointSatisfy reports whether a satisfy member claims conformance to
// what it names — `satisfy vp;`. A satisfy stating a subject asserts its
// requirement of that subject instead (`satisfy requirement r by that;`, as the
// stdlib View does), which is no claim about the view.
func IsViewpointSatisfy(sat *symbols.Symbol) bool {
	if sat == nil {
		return false
	}
	usage, ok := sat.Decl.(*ast.Usage)
	if !ok || usage.Kind != ast.UsageSatisfy {
		return false
	}
	names := false
	for _, rel := range usage.Relationships {
		if rel == nil {
			continue
		}
		switch rel.Kind {
		case ast.RelSubject:
			return false
		case ast.RelSubsets, ast.RelTyping:
			names = names || rel.Target != nil
		}
	}
	return names
}

// usageMembersOfKind returns the usages of the given kinds declared directly in
// sym's body, in declaration order — including the anonymous ones, which is
// what `satisfy viewpoint;` and `frame Concern;` are.
func usageMembersOfKind(sym *symbols.Symbol, kinds ...ast.UsageKind) []*symbols.Symbol {
	if sym == nil || sym.Scope == nil {
		return nil
	}
	wanted := func(k ast.UsageKind) bool {
		for _, want := range kinds {
			if k == want {
				return true
			}
		}
		return false
	}
	var out []*symbols.Symbol
	for _, member := range declaredMembers(sym.Scope) {
		if usage, ok := member.Decl.(*ast.Usage); ok && wanted(usage.Kind) {
			out = append(out, member)
		}
	}
	return out
}

// declaredMembers returns the symbols declared directly in scope in declaration
// order. A reference member (`satisfy vp;`, `frame c;`) is anonymous, so the two
// member lists are merged by source position rather than concatenated.
func declaredMembers(scope *symbols.Scope) []*symbols.Symbol {
	var out []*symbols.Symbol
	seen := map[*symbols.Symbol]bool{}
	for _, list := range [][]*symbols.Symbol{scope.Members(), scope.AnonymousMembers()} {
		for _, member := range list {
			if !seen[member] {
				seen[member] = true
				out = append(out, member)
			}
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].DeclSpan.Offset < out[j].DeclSpan.Offset
	})
	return out
}

// framedConcernName is how a framing is written: its effective name, else the
// concern it names.
func framedConcernName(fc *symbols.Symbol) string {
	if fc == nil {
		return ""
	}
	if usage, ok := fc.Decl.(*ast.Usage); ok {
		if name, _ := ast.EffectiveName(usage); name != "" {
			return name
		}
		for _, rel := range usage.Relationships {
			if rel == nil || rel.Target == nil {
				continue
			}
			if name, _ := ast.TargetName(rel.Target); name != "" {
				return name
			}
		}
	}
	return fc.Name
}

// refName renders a reference as written, qualified as it was spelled.
func refName(node ast.Node) string {
	qn := ast.AsQualifiedName(node)
	if qn == nil {
		name, _ := ast.TargetName(node)
		return name
	}
	var b strings.Builder
	for i, part := range qn.Parts {
		if i > 0 {
			b.WriteString("::")
		}
		b.WriteString(part.Text)
	}
	return b.String()
}

// quoteRef renders a reference as written, naming an absent one.
func quoteRef(ref string) string {
	if ref == "" {
		return "<none>"
	}
	return ref
}

// IsViewpoint reports whether sym is a viewpoint usage or definition.
func IsViewpoint(sym *symbols.Symbol) bool {
	switch decl := sym.Decl.(type) {
	case *ast.Definition:
		return decl.Kind == ast.DefViewpoint
	case *ast.Usage:
		return decl.Kind == ast.UsageViewpoint
	}
	switch sym.Kind {
	case symbols.SymbolViewpointUsage, symbols.SymbolViewpointDef:
		return true
	}
	return false
}

// isConcern reports whether sym is a concern usage — a framing included — or a
// concern definition.
func isConcern(sym *symbols.Symbol) bool {
	switch decl := sym.Decl.(type) {
	case *ast.Definition:
		return decl.Kind == ast.DefConcern
	case *ast.Usage:
		return decl.Kind == ast.UsageConcern || decl.Kind == ast.UsageFramedConcern
	}
	return false
}

// isSubject reports whether sym is the subject parameter of a requirement body.
func isSubject(sym *symbols.Symbol) bool {
	switch decl := sym.Decl.(type) {
	case *ast.SubjectMember:
		return decl != nil
	case *ast.Usage:
		return decl.Kind == ast.UsageSubject
	}
	return false
}

// instantiable reports whether an exposed element is an object a concern can be
// evaluated against: a usage, not a definition or a package.
func instantiable(sym *symbols.Symbol) bool {
	_, ok := sym.Decl.(*ast.Usage)
	return ok
}

// worse returns the more severe of two verdicts, so a view's verdict is the
// worst of its viewpoints' and a viewpoint's the worst of its concerns'.
func worse(a, b Verdict) Verdict {
	rank := func(v Verdict) int {
		switch v {
		case VerdictConforms:
			return 0
		case VerdictNotEvaluated:
			return 1
		case VerdictUnevaluable:
			return 2
		case VerdictViolated:
			return 3
		}
		return 4
	}
	if rank(b) > rank(a) {
		return b
	}
	return a
}
