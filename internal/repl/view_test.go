package repl

import (
	"errors"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/passes"
	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

const viewModel = `package Demo {
    part def Vehicle;
    part def Wheel;
    part v : Vehicle;
    view def Overview;
    view summary : Overview {
        expose Demo::Vehicle;
        expose Demo::v;
        view detail {
            expose Demo::Wheel;
        }
    }
    view empty : Overview;
}`

func viewSession(t *testing.T) *Session {
	t.Helper()
	s := NewSession()
	res := s.Submit(viewModel)
	for _, d := range res.Diagnostics {
		if d.Severity == passes.SeverityError {
			t.Fatalf("model did not load: %v", res.Diagnostics)
		}
	}
	return s
}

func TestViewListsWhatItExposes(t *testing.T) {
	s := viewSession(t)
	out, _, err := s.RunMeta("%view Demo::summary")
	if err != nil {
		t.Fatal(err)
	}
	text := strings.Join(out, "\n")
	for _, want := range []string{"view Demo::summary", "exposes", "Demo::Vehicle", "Demo::v"} {
		if !strings.Contains(text, want) {
			t.Errorf("%%view output is missing %q:\n%s", want, text)
		}
	}
}

func TestViewListsNestedViews(t *testing.T) {
	s := viewSession(t)
	out, _, err := s.RunMeta("%view Demo::summary")
	if err != nil {
		t.Fatal(err)
	}
	text := strings.Join(out, "\n")
	if !strings.Contains(text, "nested views") || !strings.Contains(text, "detail") {
		t.Errorf("%%view did not report the nested view:\n%s", text)
	}
	// The nested view's own exposed set is asked of it directly, and is not
	// folded into its parent's.
	if strings.Contains(text, "Demo::Wheel") {
		t.Errorf("a nested view's exposures leaked into its parent's:\n%s", text)
	}
	out, _, err = s.RunMeta("%view Demo::summary::detail")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.Join(out, "\n"), "Demo::Wheel") {
		t.Errorf("the nested view did not report its own exposures: %v", out)
	}
}

func TestViewExposingNothingIsNoError(t *testing.T) {
	s := viewSession(t)
	out, _, err := s.RunMeta("%view Demo::empty")
	if err != nil {
		t.Fatalf("a view exposing nothing errored: %v", err)
	}
	if !strings.Contains(strings.Join(out, "\n"), "exposes nothing") {
		t.Errorf("out = %v, want it to say the view exposes nothing", out)
	}
}

func TestViewOfANonViewIsTyped(t *testing.T) {
	s := viewSession(t)
	if _, err := s.View("Demo::Vehicle"); !errors.Is(err, semantics.ErrNotAView) {
		t.Errorf("err = %v, want semantics.ErrNotAView", err)
	}
	// At the prompt it is a line, as a mistyped constraint or instance name is.
	out, _, err := s.RunMeta("%view Demo::Vehicle")
	if err != nil {
		t.Fatalf("a non-view should not fail the command: %v", err)
	}
	if text := strings.Join(out, "\n"); !strings.HasPrefix(text, "error: ") {
		t.Errorf("out = %v, want an error line", out)
	}
}

func TestViewOfAnUnknownNameReports(t *testing.T) {
	s := viewSession(t)
	out, _, err := s.RunMeta("%view Demo::Nope")
	if err != nil {
		t.Fatalf("an unknown name should not fail the command: %v", err)
	}
	if text := strings.Join(out, "\n"); !strings.HasPrefix(text, "error: ") {
		t.Errorf("view of an unknown name did not report anything: %v", out)
	}
}

func TestViewWithoutANameShowsUsage(t *testing.T) {
	s := viewSession(t)
	out, _, err := s.RunMeta("%view")
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 1 || !strings.HasPrefix(out[0], "usage:") {
		t.Errorf("out = %v, want the usage line", out)
	}
}

func TestViewIsInHelpAndCompletion(t *testing.T) {
	if !strings.Contains(strings.Join(helpText(), "\n"), "%view") {
		t.Error("the view command is dispatched but not in help")
	}
	found := false
	for _, name := range metaCommands() {
		if name == "%view" {
			found = true
		}
	}
	if !found {
		t.Error("the view command is not in the command table")
	}
}

const conformanceModel = `package Demo {
    part def Vehicle { attribute mass : ScalarValues::Real = 1200.0; }
    part def Wheel;
    part vehicle : Vehicle;
    part wheel : Wheel;
    concern def MassBudget { subject s : Vehicle; require constraint { s.mass < 1000.0 } }
    concern def Modularity { subject s : Vehicle; require constraint { s.mass > 1.0 } }
    concern def Documented { subject s : Vehicle; }
    viewpoint def StructurePerspective {
        stakeholder engineer;
        frame concern budget : MassBudget;
        frame concern modularity : Modularity;
        frame concern documented : Documented;
    }
    viewpoint structure : StructurePerspective;
    view def StructureView { satisfy structure; }
    view report : StructureView {
        expose Demo::vehicle;
        expose Demo::wheel;
        frame concern modularity : Modularity;
        frame concern documented : Documented;
    }
}`

func conformanceSession(t *testing.T) *Session {
	t.Helper()
	s := NewSession()
	res := s.Submit(conformanceModel)
	for _, d := range res.Diagnostics {
		if d.Severity == passes.SeverityError {
			t.Fatalf("model did not load: %v", res.Diagnostics)
		}
	}
	return s
}

// %view reports the conformance of a view to the viewpoint it satisfies: a
// concern that holds, one that does not, one the view never framed, and where
// the satisfy came from.
func TestViewReportsViewpointConformance(t *testing.T) {
	out, _, err := conformanceSession(t).RunMeta("%view Demo::report")
	if err != nil {
		t.Fatal(err)
	}
	text := strings.Join(out, "\n")
	for _, want := range []string{
		"viewpoint conformance",
		"satisfy structure (from Demo::StructureView)",
		"concern budget: violated (framed by the viewpoint but not by the view)",
		"concern modularity: conforms",
		"concern documented: unevaluable",
		"states no condition to evaluate",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("%%view output is missing %q:\n%s", want, text)
		}
	}
	// Only the elements the concern's subject admits are checked.
	if strings.Contains(text, "Demo::wheel: ") {
		t.Errorf("a concern was checked against an element its subject does not admit:\n%s", text)
	}
}

// A concern whose condition is false of an exposed element names that element.
func TestViewReportsAViolatedConcernPerElement(t *testing.T) {
	s := conformanceSession(t)
	res := s.Submit(`package Budgeted {
    private import Demo::*;
    view budgeted : StructureView {
        expose Demo::vehicle;
        frame concern budget : MassBudget;
        frame concern modularity : Modularity;
        frame concern documented : Documented;
    }
}`)
	for _, d := range res.Diagnostics {
		if d.Severity == passes.SeverityError {
			t.Fatalf("the view did not load: %v", res.Diagnostics)
		}
	}
	out, err := s.View("Budgeted::budgeted")
	if err != nil {
		t.Fatal(err)
	}
	text := strings.Join(out, "\n")
	for _, want := range []string{"concern budget: violated", "Demo::vehicle: ", "s.mass < 1000.0"} {
		if !strings.Contains(text, want) {
			t.Errorf("%%view output is missing %q:\n%s", want, text)
		}
	}
}

// Tracing records the steps of a report's own evaluation too, so a trace does
// not depend on whether the session happened to hold the object checked.
func TestViewTracesAnObjectItMaterialized(t *testing.T) {
	s := conformanceSession(t)
	s.SetTracing(true)
	out, _, err := s.RunMeta("%view Demo::report")
	if err != nil {
		t.Fatal(err)
	}
	if len(s.instances) != 0 {
		t.Fatalf("the session held %v, so the report evaluated no object of its own", s.instances)
	}
	if !strings.Contains(strings.Join(out, "\n"), tracePrefix) {
		t.Errorf("%%view recorded no step while tracing:\n%s", strings.Join(out, "\n"))
	}
}

// A view satisfying nothing reports no conformance section at all.
func TestViewWithoutASatisfyReportsNoConformance(t *testing.T) {
	out, _, err := viewSession(t).RunMeta("%view Demo::summary")
	if err != nil {
		t.Fatal(err)
	}
	if text := strings.Join(out, "\n"); strings.Contains(text, "viewpoint conformance") {
		t.Errorf("a view satisfying no viewpoint reported conformance:\n%s", text)
	}
}

// A satisfy whose target is no viewpoint is reported as such, not silently
// treated as conformance.
func TestViewReportsASatisfyThatIsNoViewpoint(t *testing.T) {
	s := NewSession()
	s.Submit(`package Bad {
    part def Vehicle;
    part vehicle : Vehicle;
    requirement spec { subject s : Vehicle; }
    view v { expose Bad::vehicle; satisfy spec; }
}`)
	out, err := s.View("Bad::v")
	if err != nil {
		t.Fatal(err)
	}
	if text := strings.Join(out, "\n"); !strings.Contains(text, "not a viewpoint") {
		t.Errorf("out = %v, want it to say the satisfy target is not a viewpoint", out)
	}
}

// Reporting on a view creates nothing the session then holds, however often it
// is asked: the objects a report evaluates are its own, so the session's objects
// and the identity a later %instantiate is handed are the same either way.
func TestViewCreatesNoObjectOfItsOwn(t *testing.T) {
	nextIdentity := func(runs int) int64 {
		s := conformanceSession(t)
		for i := 0; i < runs; i++ {
			if _, err := s.View("Demo::report"); err != nil {
				t.Fatal(err)
			}
		}
		if len(s.instances) != 0 {
			t.Fatalf("%%view left the session holding %v", s.instances)
		}
		if out, _, err := s.RunMeta("%instantiate Demo::wheel"); err != nil {
			t.Fatalf("%%instantiate: %v (%v)", err, out)
		}
		return s.instances["Demo::wheel"].ID
	}
	if once, thrice := nextIdentity(1), nextIdentity(3); once != thrice {
		t.Errorf("identity after three %%view runs = %d, after one = %d: %%view leaks an object per run", thrice, once)
	}
}

// A check made after %view answers as it did before: a report leaves no object
// behind, so a view exposing two usages of one definition does not turn a later
// check of that definition into an ambiguity.
func TestViewLeavesNoAmbiguityForALaterCheck(t *testing.T) {
	s := conformanceSession(t)
	res := s.Submit(`package Checked {
    private import Demo::*;
    part def Car :> Vehicle { constraint light { mass > 1.0 } }
    part car : Car;
    part spare : Car;
    view carView : StructureView {
        expose Checked::car;
        expose Checked::spare;
        frame concern modularity : Modularity;
    }
}`)
	for _, d := range res.Diagnostics {
		if d.Severity == passes.SeverityError {
			t.Fatalf("the view did not load: %v", res.Diagnostics)
		}
	}
	for i := 0; i < 2; i++ {
		if _, err := s.View("Checked::carView"); err != nil {
			t.Fatal(err)
		}
	}
	out, _, err := s.RunMeta("%constraint Checked::Car::light")
	if err != nil {
		t.Fatal(err)
	}
	text := strings.Join(out, "\n")
	if strings.Contains(text, "carried by") || !strings.Contains(text, "passed") {
		t.Errorf("a check after two %%view runs = %q, want it to pass unambiguously", text)
	}
	list, _, err := s.RunMeta("%instances")
	if err != nil {
		t.Fatal(err)
	}
	if joined := strings.Join(list, "\n"); strings.Contains(joined, "Checked::") {
		t.Errorf("%%instances after two %%view runs lists objects the user never created:\n%s", joined)
	}
}

// %view evaluates the object the session already created, so what the user
// instantiated is what the verdict is about.
func TestViewEvaluatesTheSessionObject(t *testing.T) {
	s := conformanceSession(t)
	out, _, err := s.RunMeta("%instantiate Demo::vehicle")
	if err != nil {
		t.Fatalf("%%instantiate: %v (%v)", err, out)
	}
	created, ok := s.instances["Demo::vehicle"]
	if !ok {
		t.Fatalf("%%instantiate created no object: %v", out)
	}
	if _, err := s.View("Demo::report"); err != nil {
		t.Fatal(err)
	}
	if got := s.instances["Demo::vehicle"]; got != created {
		t.Errorf("%%view replaced the session object %d with %v", created.ID, got)
	}
}

// An element whose name needs quotes is keyed as %instantiate keys it, by the
// name the index holds: the object the session created for it is the one the
// verdict is about, and no second copy appears under the quoted spelling.
func TestViewSharesTheObjectOfAQuotedName(t *testing.T) {
	s := conformanceSession(t)
	res := s.Submit(`package Quoted {
    private import Demo::*;
    part 'road car' : Vehicle;
    view quotedView : StructureView { expose Quoted::'road car'; frame concern modularity : Modularity; }
}`)
	for _, d := range res.Diagnostics {
		if d.Severity == passes.SeverityError {
			t.Fatalf("the view did not load: %v", res.Diagnostics)
		}
	}
	if out, _, err := s.RunMeta("%instantiate Quoted::'road car'"); err != nil {
		t.Fatalf("%%instantiate: %v (%v)", err, out)
	}
	created, ok := s.instances["Quoted::road car"]
	if !ok {
		t.Fatalf("%%instantiate created no object: %v", s.instances)
	}
	// The verdict is about this object, so editing it below the concern's bound
	// has to change it — which it only can if the report read this object.
	created.FeatureValues["mass"].Value = runtime.Value{
		Kind:  runtime.ValConst,
		Const: semantics.Value{Kind: semantics.ValReal, Real: 0.5},
	}
	out, err := s.View("Quoted::quotedView")
	if err != nil {
		t.Fatal(err)
	}
	if text := strings.Join(out, "\n"); !strings.Contains(text, "concern modularity: violated") {
		t.Errorf("%%view of an edited object = %q, want the verdict to be about that object", text)
	}
	if got := s.instances["Quoted::road car"]; got != created {
		t.Errorf("%%view replaced the session object %d with %v", created.ID, got)
	}
	list, _, err := s.RunMeta("%instances")
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Count(strings.Join(list, "\n"), "road car"); got != 1 {
		t.Errorf("objects of Quoted::'road car' = %d, want 1:\n%v", got, list)
	}
}

// Anything admits every element, so a concern whose subject states it is
// checked against each exposed element rather than reported unevaluable.
func TestViewChecksAConcernWhoseSubjectIsAnything(t *testing.T) {
	s := conformanceSession(t)
	res := s.Submit(`package Universal {
    private import Demo::*;
    concern def Named { subject s : Base::Anything; require constraint { 1.0 > 2.0 } }
    viewpoint def AnyPerspective { frame concern named : Named; }
    viewpoint anything : AnyPerspective;
    view anyView { expose Demo::wheel; satisfy anything; frame concern named : Named; }
}`)
	for _, d := range res.Diagnostics {
		if d.Severity == passes.SeverityError {
			t.Fatalf("the view did not load: %v", res.Diagnostics)
		}
	}
	out, err := s.View("Universal::anyView")
	if err != nil {
		t.Fatal(err)
	}
	text := strings.Join(out, "\n")
	if !strings.Contains(text, "concern named: violated") {
		t.Errorf("%%view of an Anything-subject concern = %q, want the condition checked", text)
	}
}

// The conformance section is deterministic: the same model reported twice reads
// the same.
func TestViewConformanceOutputIsDeterministic(t *testing.T) {
	s := conformanceSession(t)
	first, err := s.View("Demo::report")
	if err != nil {
		t.Fatal(err)
	}
	second, err := s.View("Demo::report")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(first, "\n") != strings.Join(second, "\n") {
		t.Errorf("%%view output differs between runs:\n%v\n%v", first, second)
	}
}

// falseWithoutError answers no without saying why, which the semantic layer
// reads as a violation with a reason of its own.
type falseWithoutError struct{}

func (falseWithoutError) EvaluateConcern(concern, element *symbols.Symbol) (bool, error) {
	return false, nil
}

func TestViewReportsAFailedCheckThatCarriesNoError(t *testing.T) {
	s := conformanceSession(t)
	sym, _, err := s.lookupSymbol("Demo::report")
	if err != nil {
		t.Fatal(err)
	}
	ctx, err := s.getOrCreateRuntime()
	if err != nil {
		t.Fatal(err)
	}
	report, err := ctx.Semantics().ViewConformance(sym, falseWithoutError{})
	if err != nil {
		t.Fatal(err)
	}
	text := strings.Join(s.conformanceLines(report), "\n")
	if strings.Contains(text, "<nil>") || strings.Contains(text, ": \n") {
		t.Errorf("a failed check with no error reads %q, want a reason", text)
	}
	if !strings.Contains(text, "a required condition does not hold") {
		t.Errorf("a failed check with no error reads %q, want the reason the model gives", text)
	}
}
