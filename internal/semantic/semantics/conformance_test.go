package semantics

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// errFalse is a false verdict, the way the runtime returns one.
var errFalse = errors.New("condition evaluated to false")

// fakeEvaluator answers a concern by name: a concern named in fails is false of
// every element, one in broken cannot be evaluated, any other holds. It records
// what it was asked, so a test can state which element a concern was checked
// against and in what order.
type fakeEvaluator struct {
	fails  map[string]bool
	broken map[string]bool
	asked  []string
}

func (e *fakeEvaluator) EvaluateConcern(concern, element *symbols.Symbol) (bool, error) {
	e.asked = append(e.asked, fmt.Sprintf("%s/%s", localName(concern.Name), localName(element.Name)))
	switch {
	case e.broken[localName(concern.Name)]:
		return false, errors.New("no condition to evaluate")
	case e.fails[localName(concern.Name)]:
		return false, errFalse
	}
	return true, nil
}

func (e *fakeEvaluator) IsViolation(err error) bool { return errors.Is(err, errFalse) }

func localName(name string) string {
	if i := strings.LastIndex(name, "::"); i >= 0 {
		return name[i+len("::"):]
	}
	return name
}

// conformance evaluates view's conformance with eval, which may be nil.
func conformance(t *testing.T, m *Model, view *symbols.Symbol, eval ConcernEvaluator) *ViewConformance {
	t.Helper()
	report, err := m.ViewConformance(view, eval)
	if err != nil {
		t.Fatalf("ViewConformance(%s): unexpected error %v", view.Name, err)
	}
	return report
}

// onlyViewpoint is the single viewpoint conformance of a report.
func onlyViewpoint(t *testing.T, report *ViewConformance) ViewpointConformance {
	t.Helper()
	if len(report.Viewpoints) != 1 {
		t.Fatalf("satisfied viewpoints = %d, want 1", len(report.Viewpoints))
	}
	return report.Viewpoints[0]
}

func wantVerdict(t *testing.T, what string, got, expect Verdict, reason string) {
	t.Helper()
	if got != expect {
		t.Fatalf("%s = %v (%s), want %v", what, got, reason, expect)
	}
}

// concernNames is the framed concerns of a viewpoint conformance, in order.
func concernNames(vp ViewpointConformance) []string {
	names := make([]string, 0, len(vp.Concerns))
	for _, c := range vp.Concerns {
		names = append(names, c.Name)
	}
	return names
}

const conformanceLib = `
	part def Vehicle { attribute mass : ScalarValues::Real = 1200.0; }
	part def Wheel;
	part vehicle : Vehicle;
	part wheel : Wheel;
	concern def MassConcern { subject s : Vehicle; require constraint { s.mass < 2000.0 } }
	concern def CostConcern { subject s : Vehicle; require constraint { s.mass > 1.0 } }
`

// A view framing every concern its viewpoint frames conforms, and each concern
// is evaluated against the exposed elements its subject admits.
func TestViewConformanceFramedAndHolding(t *testing.T) {
	m, root := buildModel(t, conformanceLib+`
		viewpoint def VP { frame concern mass : MassConcern; }
		viewpoint vp : VP;
		view v { expose vehicle; expose wheel; satisfy vp; frame concern mass : MassConcern; }
	`)
	eval := &fakeEvaluator{}
	report := conformance(t, m, sym(t, root, "v"), eval)
	vp := onlyViewpoint(t, report)
	wantVerdict(t, "verdict of v", report.Verdict, VerdictConforms, vp.Reason)
	wantNames(t, "concerns evaluated", eval.asked, []string{"mass/vehicle"})
}

// A concern the viewpoint frames and the view does not is a non-conformance,
// reported as such rather than left silent.
func TestViewConformanceMissingConcernIsViolated(t *testing.T) {
	m, root := buildModel(t, conformanceLib+`
		viewpoint def VP { frame concern mass : MassConcern; frame concern cost : CostConcern; }
		viewpoint vp : VP;
		view v { expose vehicle; satisfy vp; frame concern mass : MassConcern; }
	`)
	report := conformance(t, m, sym(t, root, "v"), &fakeEvaluator{})
	vp := onlyViewpoint(t, report)
	wantNames(t, "concerns of vp", concernNames(vp), []string{"mass", "cost"})
	wantVerdict(t, "verdict of cost", vp.Concerns[1].Verdict, VerdictViolated, vp.Concerns[1].Reason)
	if !strings.Contains(vp.Concerns[1].Reason, "not by the view") {
		t.Fatalf("reason for cost = %q, want it to say the view does not frame it", vp.Concerns[1].Reason)
	}
	wantVerdict(t, "verdict of v", report.Verdict, VerdictViolated, "a concern is not framed")
}

// A concern whose condition is false of an exposed element is a violation, one
// check per element the subject admits.
func TestViewConformanceFalseConditionIsViolated(t *testing.T) {
	m, root := buildModel(t, conformanceLib+`
		viewpoint def VP { frame concern mass : MassConcern; }
		viewpoint vp : VP;
		part other : Vehicle;
		view v { expose vehicle; expose other; satisfy vp; frame concern mass : MassConcern; }
	`)
	eval := &fakeEvaluator{fails: map[string]bool{"mass": true}}
	report := conformance(t, m, sym(t, root, "v"), eval)
	vp := onlyViewpoint(t, report)
	wantVerdict(t, "verdict of mass", vp.Concerns[0].Verdict, VerdictViolated, vp.Concerns[0].Reason)
	wantNames(t, "concerns evaluated", eval.asked, []string{"mass/vehicle", "mass/other"})
	if len(vp.Concerns[0].Checks) != 2 {
		t.Fatalf("checks = %d, want one per exposed vehicle", len(vp.Concerns[0].Checks))
	}
}

// A concern whose condition cannot be evaluated is unevaluable with a reason: it
// never reads as a pass.
func TestViewConformanceUnevaluableConcern(t *testing.T) {
	m, root := buildModel(t, conformanceLib+`
		viewpoint def VP { frame concern mass : MassConcern; }
		viewpoint vp : VP;
		view v { expose vehicle; satisfy vp; frame concern mass : MassConcern; }
	`)
	eval := &fakeEvaluator{broken: map[string]bool{"mass": true}}
	vp := onlyViewpoint(t, conformance(t, m, sym(t, root, "v"), eval))
	wantVerdict(t, "verdict of mass", vp.Concerns[0].Verdict, VerdictUnevaluable, vp.Concerns[0].Reason)
	if !strings.Contains(vp.Concerns[0].Reason, "no condition") {
		t.Fatalf("reason for mass = %q, want it to say why", vp.Concerns[0].Reason)
	}
}

// A view exposing no element the concern's subject admits is unevaluable, saying
// which subject found nothing.
func TestViewConformanceNoSubjectIsUnevaluable(t *testing.T) {
	m, root := buildModel(t, conformanceLib+`
		viewpoint def VP { frame concern mass : MassConcern; }
		viewpoint vp : VP;
		view v { expose wheel; satisfy vp; frame concern mass : MassConcern; }
	`)
	eval := &fakeEvaluator{}
	vp := onlyViewpoint(t, conformance(t, m, sym(t, root, "v"), eval))
	wantVerdict(t, "verdict of mass", vp.Concerns[0].Verdict, VerdictUnevaluable, vp.Concerns[0].Reason)
	if !strings.Contains(vp.Concerns[0].Reason, "Vehicle") {
		t.Fatalf("reason for mass = %q, want the subject type named", vp.Concerns[0].Reason)
	}
	if len(eval.asked) != 0 {
		t.Fatalf("evaluated %v, want nothing the subject does not admit", eval.asked)
	}
}

// A view exposing nothing says so rather than naming a subject type.
func TestViewConformanceExposingNothingIsUnevaluable(t *testing.T) {
	m, root := buildModel(t, conformanceLib+`
		viewpoint def VP { frame concern mass : MassConcern; }
		viewpoint vp : VP;
		view v { satisfy vp; frame concern mass : MassConcern; }
	`)
	vp := onlyViewpoint(t, conformance(t, m, sym(t, root, "v"), &fakeEvaluator{}))
	wantVerdict(t, "verdict of mass", vp.Concerns[0].Verdict, VerdictUnevaluable, vp.Concerns[0].Reason)
	if !strings.Contains(vp.Concerns[0].Reason, "exposes nothing") {
		t.Fatalf("reason for mass = %q, want it to say the view exposes nothing", vp.Concerns[0].Reason)
	}
}

// A satisfy inherited from a view definition is the view's own claim, and the
// definition it came from is reported.
func TestViewConformanceInheritedSatisfy(t *testing.T) {
	m, root := buildModel(t, conformanceLib+`
		viewpoint def VP { frame concern mass : MassConcern; }
		viewpoint vp : VP;
		view def StructureView { satisfy vp; }
		view v : StructureView { expose vehicle; frame concern mass : MassConcern; }
	`)
	vp := onlyViewpoint(t, conformance(t, m, sym(t, root, "v"), &fakeEvaluator{}))
	if vp.SatisfiedIn != sym(t, root, "StructureView") {
		t.Fatalf("satisfy declared in %v, want StructureView", vp.SatisfiedIn)
	}
	wantVerdict(t, "verdict of mass", vp.Concerns[0].Verdict, VerdictConforms, vp.Concerns[0].Reason)
}

// A framing inherited from a view definition frames for the view.
func TestViewConformanceInheritedFraming(t *testing.T) {
	m, root := buildModel(t, conformanceLib+`
		viewpoint def VP { frame concern mass : MassConcern; }
		viewpoint vp : VP;
		view def StructureView { frame concern mass : MassConcern; }
		view v : StructureView { expose vehicle; satisfy vp; }
	`)
	vp := onlyViewpoint(t, conformance(t, m, sym(t, root, "v"), &fakeEvaluator{}))
	wantVerdict(t, "verdict of mass", vp.Concerns[0].Verdict, VerdictConforms, vp.Concerns[0].Reason)
	if vp.Concerns[0].FramedIn != sym(t, root, "StructureView") {
		t.Fatalf("mass framed in %v, want the view definition StructureView that declares it", vp.Concerns[0].FramedIn)
	}
}

// A concern framed by a nested view frames for its container (tool-defined: the
// container's conformance is what the view tree as a whole addresses).
func TestViewConformanceNestedViewFraming(t *testing.T) {
	m, root := buildModel(t, conformanceLib+`
		viewpoint def VP { frame concern mass : MassConcern; }
		viewpoint vp : VP;
		view v { expose vehicle; satisfy vp; view inner { frame concern mass : MassConcern; } }
	`)
	vp := onlyViewpoint(t, conformance(t, m, sym(t, root, "v"), &fakeEvaluator{}))
	wantVerdict(t, "verdict of mass", vp.Concerns[0].Verdict, VerdictConforms, vp.Concerns[0].Reason)
	if vp.Concerns[0].FramedIn == nil || localName(vp.Concerns[0].FramedIn.Name) != "inner" {
		t.Fatalf("mass framed in %v, want the nested view inner", vp.Concerns[0].FramedIn)
	}
}

// A concern a viewpoint inherits is framed by the viewpoint too, so the view
// must frame it.
func TestViewConformanceInheritedViewpointFraming(t *testing.T) {
	m, root := buildModel(t, conformanceLib+`
		viewpoint def BaseVP { frame concern mass : MassConcern; }
		viewpoint def VP :> BaseVP { frame concern cost : CostConcern; }
		viewpoint vp : VP;
		view v { expose vehicle; satisfy vp; frame concern cost : CostConcern; }
	`)
	vp := onlyViewpoint(t, conformance(t, m, sym(t, root, "v"), &fakeEvaluator{}))
	wantNames(t, "concerns of vp", concernNames(vp), []string{"cost", "mass"})
	wantVerdict(t, "verdict of mass", vp.Concerns[1].Verdict, VerdictViolated, vp.Concerns[1].Reason)
}

// A bare `frame <concern>;` references a concern rather than declaring one, and
// matches the viewpoint's framing of that same concern.
func TestViewConformanceReferenceFramingMatches(t *testing.T) {
	m, root := buildModel(t, `
		part def Vehicle;
		part vehicle : Vehicle;
		concern modularity { subject s : Vehicle; require constraint { true } }
		viewpoint perspective { frame modularity; }
		view v { expose vehicle; satisfy perspective; frame modularity; }
	`)
	eval := &fakeEvaluator{}
	vp := onlyViewpoint(t, conformance(t, m, sym(t, root, "v"), eval))
	wantVerdict(t, "verdict of modularity", vp.Concerns[0].Verdict, VerdictConforms, vp.Concerns[0].Reason)
	// The conditions evaluated are the referenced concern's, not the framing's.
	wantNames(t, "concerns evaluated", eval.asked, []string{"modularity/vehicle"})
}

// A satisfy whose target is no viewpoint asserts no conformance, and is reported
// with what the target actually is.
func TestViewConformanceSatisfyTargetIsNoViewpoint(t *testing.T) {
	m, root := buildModel(t, conformanceLib+`
		requirement r { subject s : Vehicle; }
		view v { expose vehicle; satisfy r; }
	`)
	vp := onlyViewpoint(t, conformance(t, m, sym(t, root, "v"), &fakeEvaluator{}))
	wantVerdict(t, "verdict of satisfy r", vp.Verdict, VerdictUnevaluable, vp.Reason)
	if !strings.Contains(vp.Reason, ErrNotAViewpoint.Error()) {
		t.Fatalf("reason = %q, want it to say the target is not a viewpoint", vp.Reason)
	}
	if vp.Viewpoint != nil {
		t.Fatalf("viewpoint = %v, want none", vp.Viewpoint)
	}
}

// A stakeholder naming a type that resolves to nothing is reported, and makes
// the viewpoint's conformance unevaluable rather than a pass.
func TestViewConformanceUnresolvedStakeholder(t *testing.T) {
	m, root := buildModel(t, conformanceLib+`
		viewpoint def VP { stakeholder se : Missing; frame concern mass : MassConcern; }
		viewpoint vp : VP;
		view v { expose vehicle; satisfy vp; frame concern mass : MassConcern; }
	`)
	vp := onlyViewpoint(t, conformance(t, m, sym(t, root, "v"), &fakeEvaluator{}))
	var reasons []string
	for _, p := range vp.Parties {
		if p.Reason != "" {
			reasons = append(reasons, p.Reason)
		}
	}
	if len(reasons) != 1 || !strings.Contains(reasons[0], "Missing") {
		t.Fatalf("party reasons = %v, want the unresolved stakeholder named", reasons)
	}
	wantVerdict(t, "verdict of vp", vp.Verdict, VerdictUnevaluable, strings.Join(reasons, "; "))
}

// A stakeholder that resolves is no complaint, an untyped one included: it takes
// the standard library's Part.
func TestViewConformanceResolvedStakeholdersAreQuiet(t *testing.T) {
	m, root := buildModel(t, conformanceLib+`
		part def Engineer;
		viewpoint def VP { stakeholder se : Engineer; stakeholder anyone; frame concern mass : MassConcern; }
		viewpoint vp : VP;
		view v { expose vehicle; satisfy vp; frame concern mass : MassConcern; }
	`)
	vp := onlyViewpoint(t, conformance(t, m, sym(t, root, "v"), &fakeEvaluator{}))
	for _, p := range vp.Parties {
		if p.Reason != "" {
			t.Fatalf("party %s: unexpected reason %q", p.Name, p.Reason)
		}
	}
	wantVerdict(t, "verdict of vp", vp.Verdict, VerdictConforms, vp.Reason)
}

// Without an evaluator only the structural question is answered: a framed
// concern is not evaluated, and a missing one is still a violation.
func TestViewConformanceWithoutEvaluator(t *testing.T) {
	m, root := buildModel(t, conformanceLib+`
		viewpoint def VP { frame concern mass : MassConcern; frame concern cost : CostConcern; }
		viewpoint vp : VP;
		view v { expose vehicle; satisfy vp; frame concern mass : MassConcern; }
	`)
	vp := onlyViewpoint(t, conformance(t, m, sym(t, root, "v"), nil))
	wantVerdict(t, "verdict of mass", vp.Concerns[0].Verdict, VerdictNotEvaluated, vp.Concerns[0].Reason)
	wantVerdict(t, "verdict of cost", vp.Concerns[1].Verdict, VerdictViolated, vp.Concerns[1].Reason)
}

// A viewpoint framing no concern asks nothing of the view, so no conformance
// verdict is reachable: reporting one would be a pass over nothing checked.
func TestViewConformanceViewpointFramingNothing(t *testing.T) {
	m, root := buildModel(t, conformanceLib+`
		viewpoint def VP;
		viewpoint vp : VP;
		view v { expose vehicle; satisfy vp; }
	`)
	report := conformance(t, m, sym(t, root, "v"), &fakeEvaluator{})
	vp := onlyViewpoint(t, report)
	if len(vp.Concerns) != 0 {
		t.Fatalf("concerns = %v, want none", concernNames(vp))
	}
	wantVerdict(t, "verdict of v", report.Verdict, VerdictUnevaluable, vp.Reason)
}

// A framed concern whose reference does not resolve is unevaluable: the view
// cannot frame a concern that is unknown, so this is no framing miss.
func TestViewConformanceUnresolvedFramedConcern(t *testing.T) {
	m, root := buildModel(t, conformanceLib+`
		viewpoint def VP { frame concern ghost : NoSuchConcern; }
		viewpoint vp : VP;
		view v { expose vehicle; satisfy vp; }
	`)
	vp := onlyViewpoint(t, conformance(t, m, sym(t, root, "v"), &fakeEvaluator{}))
	c := vp.Concerns[0]
	wantVerdict(t, "verdict of ghost", c.Verdict, VerdictUnevaluable, c.Reason)
	if !strings.Contains(c.Reason, "NoSuchConcern") {
		t.Errorf("reason = %q, want it to name NoSuchConcern", c.Reason)
	}
}

// A framing referencing a concern usage frames that usage, not the definition it
// is typed by, whose values the usage may mask. Two usages of one definition are
// therefore different concerns.
func TestViewConformanceFramesTheNamedConcernUsage(t *testing.T) {
	m, root := buildModel(t, conformanceLib+`
		concern tight : MassConcern;
		concern loose : MassConcern;
		viewpoint def VP { frame tight; }
		viewpoint vp : VP;
		view framingTheUsage { expose vehicle; satisfy vp; frame tight; }
		view framingAnother { expose vehicle; satisfy vp; frame loose; }
	`)
	eval := &fakeEvaluator{}
	vp := onlyViewpoint(t, conformance(t, m, sym(t, root, "framingTheUsage"), eval))
	if got := vp.Concerns[0].Target; got != sym(t, root, "tight") {
		t.Fatalf("framed concern target = %v, want the tight usage", got)
	}
	wantVerdict(t, "verdict of tight", vp.Concerns[0].Verdict, VerdictConforms, vp.Concerns[0].Reason)
	wantNames(t, "concerns evaluated", eval.asked, []string{"tight/vehicle"})

	other := onlyViewpoint(t, conformance(t, m, sym(t, root, "framingAnother"), &fakeEvaluator{}))
	wantVerdict(t, "verdict of tight", other.Concerns[0].Verdict, VerdictViolated, other.Concerns[0].Reason)
}

// A satisfy stating a subject asserts a requirement of that subject, as the
// stdlib's `View` does with `satisfy requirement viewpointConformance by that`,
// so it is no claim of viewpoint conformance and is not reported as one.
func TestViewConformanceIgnoresSatisfyWithASubject(t *testing.T) {
	m, root := buildModel(t, conformanceLib+`
		requirement def VehicleSpecification { subject s : Vehicle; }
		requirement spec : VehicleSpecification;
		viewpoint def VP { frame concern mass : MassConcern; }
		viewpoint vp : VP;
		view v {
			expose vehicle;
			satisfy requirement conformance : VehicleSpecification by vehicle;
			satisfy spec by vehicle;
			satisfy vp;
			frame concern mass : MassConcern;
		}
	`)
	report := conformance(t, m, sym(t, root, "v"), &fakeEvaluator{})
	vp := onlyViewpoint(t, report)
	if vp.Viewpoint == nil || localName(vp.Viewpoint.Name) != "vp" {
		t.Fatalf("satisfied viewpoint = %v, want vp", vp.Viewpoint)
	}
	wantVerdict(t, "verdict of v", report.Verdict, VerdictConforms, vp.Reason)
}

// A view framing a concern without naming its type frames it under that name:
// the framings share no resolved concern, so the name is what matches, either
// way round.
func TestViewConformanceUntypedFramingMatchesByName(t *testing.T) {
	m, root := buildModel(t, conformanceLib+`
		viewpoint def VP { frame concern mass : MassConcern; }
		viewpoint vp : VP;
		viewpoint def UntypedVP { frame concern mass; }
		viewpoint untypedVp : UntypedVP;
		view untypedByTheView { expose vehicle; satisfy vp; frame concern mass; }
		view untypedByTheViewpoint { expose vehicle; satisfy untypedVp; frame concern mass : MassConcern; }
	`)
	for _, name := range []string{"untypedByTheView", "untypedByTheViewpoint"} {
		vp := onlyViewpoint(t, conformance(t, m, sym(t, root, name), &fakeEvaluator{}))
		if vp.Concerns[0].FramedBy == nil {
			t.Errorf("%s: mass reported as not framed (%s), want it framed under its name", name, vp.Concerns[0].Reason)
		}
	}
}

// Satisfies and framings come back in declaration order even where a named
// declaration and an anonymous reference are mixed, which the symbol table keeps
// in separate member lists.
func TestViewConformanceOrdersMixedNamedAndAnonymousMembers(t *testing.T) {
	m, root := buildModel(t, conformanceLib+`
		concern alpha : MassConcern;
		concern omega : CostConcern;
		viewpoint def VP { frame alpha; frame concern middle : MassConcern; frame omega; }
		viewpoint vp : VP;
		viewpoint def OtherVP { frame concern only : CostConcern; }
		view v {
			expose vehicle;
			satisfy vp;
			satisfy requirement named : OtherVP;
			frame alpha;
			frame concern middle : MassConcern;
			frame omega;
		}
	`)
	report := conformance(t, m, sym(t, root, "v"), &fakeEvaluator{})
	if len(report.Viewpoints) != 2 {
		t.Fatalf("satisfied viewpoints = %d, want 2", len(report.Viewpoints))
	}
	wantNames(t, "satisfies of v", []string{report.Viewpoints[0].Ref, report.Viewpoints[1].Ref}, []string{"vp", "OtherVP"})
	wantNames(t, "concerns of vp", concernNames(report.Viewpoints[0]), []string{"alpha", "middle", "omega"})
}

// A view satisfying no viewpoint is no error and claims nothing.
func TestViewConformanceOfAViewSatisfyingNothing(t *testing.T) {
	m, root := buildModel(t, conformanceLib+`view v { expose vehicle; }`)
	report := conformance(t, m, sym(t, root, "v"), &fakeEvaluator{})
	if len(report.Viewpoints) != 0 {
		t.Fatalf("satisfied viewpoints = %d, want none", len(report.Viewpoints))
	}
	wantVerdict(t, "verdict of v", report.Verdict, VerdictConforms, "nothing is claimed")
}

// Asking a non-view is ErrNotAView, as ExposedElements is.
func TestViewConformanceOfANonView(t *testing.T) {
	m, root := buildModel(t, `part def Vehicle;`)
	if _, err := m.ViewConformance(sym(t, root, "Vehicle"), nil); !errors.Is(err, ErrNotAView) {
		t.Fatalf("ViewConformance(Vehicle) error = %v, want ErrNotAView", err)
	}
}

// Satisfies, concerns and per-element checks come back in declaration order,
// twice the same, so a report is deterministic.
func TestViewConformanceIsDeterministic(t *testing.T) {
	m, root := buildModel(t, conformanceLib+`
		part second : Vehicle;
		viewpoint def VP { frame concern mass : MassConcern; frame concern cost : CostConcern; }
		viewpoint vp : VP;
		viewpoint def OtherVP { frame concern cost2 : CostConcern; }
		viewpoint other : OtherVP;
		view v {
			expose vehicle; expose second;
			satisfy vp; satisfy other;
			frame concern mass : MassConcern;
			frame concern cost : CostConcern;
			frame concern cost2 : CostConcern;
		}
	`)
	view := sym(t, root, "v")
	var runs []string
	for range 2 {
		eval := &fakeEvaluator{}
		report := conformance(t, m, view, eval)
		var b strings.Builder
		for _, vp := range report.Viewpoints {
			fmt.Fprintf(&b, "%s:%v|", vp.Ref, concernNames(vp))
		}
		fmt.Fprintf(&b, "%v", eval.asked)
		runs = append(runs, b.String())
	}
	if runs[0] != runs[1] {
		t.Fatalf("report differs between runs:\n%s\n%s", runs[0], runs[1])
	}
	if want := "vp:[mass cost]|other:[cost2]|[mass/vehicle mass/second cost/vehicle cost/second cost2/vehicle cost2/second]"; runs[0] != want {
		t.Fatalf("report = %s, want %s", runs[0], want)
	}
}
