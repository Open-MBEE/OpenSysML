package jupyter

import (
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/Open-MBEE/OpenSysML/internal/exec/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/frontend/repl"
	_ "github.com/Open-MBEE/OpenSysML/internal/frontend/repl/replext/all" // the kernel links every REPL extension
)

// recorder is an Output that keeps what a cell produced.
type recorder struct {
	streams  []string
	displays []MIMEBundle
	results  []MIMEBundle
	errors   []string
}

func (r *recorder) Stream(name, text string)                  { r.streams = append(r.streams, name+": "+text) }
func (r *recorder) Display(data MIMEBundle, _ map[string]any) { r.displays = append(r.displays, data) }
func (r *recorder) Result(data MIMEBundle)                    { r.results = append(r.results, data) }
func (r *recorder) Error(name, value string, _ []string) {
	r.errors = append(r.errors, name+": "+value)
}

func (r *recorder) text() string { return strings.Join(r.streams, "") }

func newEngine(t *testing.T) *REPLEngine {
	t.Helper()
	return NewREPLEngine(repl.NewSession())
}

func mustRun(t *testing.T, e *REPLEngine, code string) *recorder {
	t.Helper()
	out := &recorder{}
	if err := e.Execute(code, out); err != nil {
		t.Fatalf("Execute(%q) = %v", code, err)
	}
	return out
}

func mustFail(t *testing.T, e *REPLEngine, code string) *ExecError {
	t.Helper()
	err := e.Execute(code, &recorder{})
	var failure *ExecError
	if !errors.As(err, &failure) {
		t.Fatalf("Execute(%q) = %v, want an ExecError", code, err)
	}
	return failure
}

func TestCellsAccumulateIntoOneSession(t *testing.T) {
	e := newEngine(t)
	out := mustRun(t, e, "private import ScalarValues::*;\npackage Demo {\n  part def Wheel { attribute diameter : Real = 16.0; }\n}")
	if !strings.Contains(out.text(), "✓ package Demo") {
		t.Errorf("declaration output = %q", out.text())
	}
	out = mustRun(t, e, "Demo::Wheel::diameter + 1")
	if len(out.results) != 1 || !strings.Contains(out.results[0][mimeText].(string), "= 17.0") {
		t.Errorf("expression result = %v", out.results)
	}
	out = mustRun(t, e, "%eval Demo::Wheel::diameter")
	if !strings.Contains(out.text(), "= 16.0") {
		t.Errorf("%%eval output = %q", out.text())
	}
}

func TestACellMayMixDeclarationsAndExpressions(t *testing.T) {
	e := newEngine(t)
	out := mustRun(t, e, "private import ScalarValues::*;\npackage Demo {\n  part def Wheel { attribute diameter : Real = 16.0; }\n}\n1 + 2\nDemo::Wheel::diameter + 1")
	if !strings.Contains(out.text(), "✓ package Demo") {
		t.Errorf("declaration output = %q", out.text())
	}
	if len(out.results) != 2 {
		t.Fatalf("results = %v, want one per expression", out.results)
	}
	if first := out.results[0][mimeText].(string); !strings.Contains(first, "= 3") {
		t.Errorf("first result = %q", first)
	}
	if second := out.results[1][mimeText].(string); !strings.Contains(second, "= 17.0") {
		t.Errorf("second result = %q", second)
	}
}

func TestACellRunsItsStatementsInOrderAndStopsAtTheFirstFailure(t *testing.T) {
	e := newEngine(t)
	out := &recorder{}
	err := e.Execute("part def A;\n%print A\n%print Nope\n%print A", out)
	var failure *ExecError
	if !errors.As(err, &failure) || failure.Name != "CommandError" {
		t.Fatalf("Execute = %v, want a CommandError for the missing name", err)
	}
	if got := out.text(); strings.Count(got, "part def A") != 2 {
		t.Errorf("output before the failure = %q, want the declaration and one %%print", got)
	}
}

func TestFailuresAreNamedByWhatFailed(t *testing.T) {
	cases := map[string]string{
		"part def B { attribute x : Nope; }": "SubmissionError",
		"%nosuchcommand":                     "UnknownCommand",
		"%print Nope":                        "CommandError",
		"%render nothing":                    "RenderError",
		"%render":                            "UsageError",
		"%viz":                               "UsageError",
		"%viz --view nope Demo":              "UsageError",
		"%viz --style nosuch Demo":           "UsageError",
		"%viz nothing":                       "RenderError",
		"%render-document":                   "UsageError",
		"1 +":                                "SubmissionError",
	}
	for code, want := range cases {
		// Each in its own session: as at the prompt, a snippet that did not parse
		// stays in the buffer and holds back the deeper checks of what follows.
		e := newEngine(t)
		if got := mustFail(t, e, code); got.Name != want {
			t.Errorf("Execute(%q) failed as %s (%s), want %s", code, got.Name, got.Value, want)
		} else if got.Value == "" || len(got.Traceback) == 0 {
			t.Errorf("Execute(%q) failed without a value or traceback: %+v", code, got)
		}
	}
}

func TestQuitDoesNotEndTheKernel(t *testing.T) {
	e := newEngine(t)
	out := mustRun(t, e, "%quit")
	if !strings.Contains(out.text(), "shutdown") {
		t.Errorf("%%quit printed %q, want a pointer to the notebook's shutdown", out.text())
	}
	mustRun(t, e, "part def A;")
}

const viewModel = `private import ScalarValues::*;
private import Views::*;
package Demo {
  part def Wheel { attribute diameter : Real = 16.0; }
  part w : Wheel;
}
view def Table { render asElementTable; }
view parts : Table { expose Demo::*; }
view def Inter { render asInterconnectionDiagram; }
view diag : Inter { expose Demo::*; }
`

func TestRenderedViewsAreShownInTheirForm(t *testing.T) {
	e := newEngine(t)
	mustRun(t, e, viewModel)
	cases := []struct {
		code string
		mime string
		mark string
	}{
		{"%render parts markdown", mimeMarkdown, "| Element |"},
		{"%render parts csv", mimeCSV, "Element,Kind"},
		{"%render parts tsv", mimeTSV, "Element\tKind"},
		{"%render diag mermaid", mimeMermaid, "flowchart"},
		{"%render diag dot", mimeDot, "digraph"},
		{"%render diag", mimeText, "interconnection"},
	}
	for _, tc := range cases {
		out := mustRun(t, e, tc.code)
		if len(out.displays) != 1 {
			t.Errorf("%s produced %d displays, want 1", tc.code, len(out.displays))
			continue
		}
		data := out.displays[0]
		if _, ok := data[mimeText]; !ok {
			t.Errorf("%s has no text/plain fallback", tc.code)
		}
		if got, _ := data[tc.mime].(string); !strings.Contains(got, tc.mark) {
			t.Errorf("%s %s = %q, want it to contain %q", tc.code, tc.mime, got, tc.mark)
		}
	}
}

func TestInstanceFeaturesAsJSONAreShownAsJSON(t *testing.T) {
	e := newEngine(t)
	mustRun(t, e, viewModel)
	mustRun(t, e, "%instantiate Demo::w")
	out := mustRun(t, e, "%features Demo::w json")
	if len(out.displays) != 1 {
		t.Fatalf("%d displays, want 1", len(out.displays))
	}
	if _, ok := out.displays[0][mimeJSON].(map[string]any); !ok {
		t.Errorf("application/json = %T, want a decoded object", out.displays[0][mimeJSON])
	}
}

func TestDocumentsRenderAsMarkdown(t *testing.T) {
	e := newEngine(t)
	mustRun(t, e, `package Hello {
  private import DocumentQueries::*;
  private import ScalarValues::*;
  part def HelloReport :> Document {
    attribute redefines title = "Wheels";
    part greeting : Paragraph { attribute redefines text = "Every wheel."; }
  }
}`)
	out := mustRun(t, e, "%render-document Hello::HelloReport")
	if len(out.displays) != 1 {
		t.Fatalf("%d displays, want 1: %q", len(out.displays), out.text())
	}
	if got, _ := out.displays[0][mimeMarkdown].(string); !strings.Contains(got, "Wheels") {
		t.Errorf("text/markdown = %q", got)
	}
}

func TestDocumentsRenderAsHTMLOnRequest(t *testing.T) {
	e := newEngine(t)
	mustRun(t, e, `package Hello {
  private import DocumentQueries::*;
  private import ScalarValues::*;
  part def HelloReport :> Document {
    attribute redefines title = "Wheels";
    part greeting : Paragraph { attribute redefines text = "Every wheel."; }
  }
}`)
	out := mustRun(t, e, "%render-document Hello::HelloReport html")
	if len(out.displays) != 1 {
		t.Fatalf("%d displays, want 1: %q", len(out.displays), out.text())
	}
	got, _ := out.displays[0][mimeHTML].(string)
	if !strings.Contains(got, "Wheels") || !strings.Contains(got, "<p") {
		t.Errorf("text/html = %q", got)
	}
	if strings.Contains(got, "<html") {
		t.Error("text/html is a whole page, not a fragment for the notebook's page")
	}
	if _, markdown := out.displays[0][mimeMarkdown]; markdown {
		t.Error("the HTML display also carries text/markdown")
	}
	// The form words still read before it.
	out = mustRun(t, e, "%render-document Hello::HelloReport mermaid html")
	if got, _ := out.displays[0][mimeHTML].(string); !strings.Contains(got, "Wheels") {
		t.Errorf("text/html with a form = %q", got)
	}
}

func TestCompletionSpansTheWordAtTheCursorInRuneOffsets(t *testing.T) {
	e := newEngine(t)
	mustRun(t, e, "package Demo { part def Wheel; }")
	c := e.Complete("%ev", 3)
	if !contains(c.Matches, "%eval") || c.CursorStart != 0 || c.CursorEnd != 3 {
		t.Errorf("Complete(%%ev) = %+v", c)
	}
	// Completion is of the cursor's line, and the span is spelled in rune offsets
	// of the whole cell: the first line holds two-byte runes.
	code := "// äöü\n%eval Demo::Wh"
	cursor := len([]rune(code))
	c = e.Complete(code, cursor)
	if !contains(c.Matches, "Demo::Wheel") {
		t.Errorf("Complete(%q) = %+v", code, c)
	}
	if c.CursorEnd != cursor || c.CursorStart != cursor-len("Demo::Wh") {
		t.Errorf("Complete(%q) spans %d..%d, want %d..%d", code, c.CursorStart, c.CursorEnd, cursor-len("Demo::Wh"), cursor)
	}
	if c := e.Complete("%eval", 99); c.CursorEnd != 5 {
		t.Errorf("a cursor past the end is clamped: %+v", c)
	}
}

func TestIsCompleteFollowsTheContinuationRule(t *testing.T) {
	e := newEngine(t)
	cases := map[string]IsCompleteStatus{
		"":                    Complete,
		"package P {":         Incomplete,
		"package P {\n  part": Incomplete,
		"package P {}":        Complete,
		"%eval 1 +":           Complete,
		"part def A;\n%help":  Complete,
	}
	for code, want := range cases {
		status, indent := e.IsComplete(code)
		if status != want {
			t.Errorf("IsComplete(%q) = %s, want %s", code, status, want)
		}
		if (indent != "") != (want == Incomplete) {
			t.Errorf("IsComplete(%q) indent = %q", code, indent)
		}
	}
}

func TestInspectionPrintsTheNameUnderTheCursor(t *testing.T) {
	e := newEngine(t)
	mustRun(t, e, "private import ScalarValues::*;\npackage Demo { part def Wheel { attribute diameter : Real = 16.0; } }")
	in := e.Inspect("%eval Demo::Wheel::diameter + 1", 12, 0)
	if !in.Found || !strings.Contains(in.Data[mimeText].(string), "attribute diameter") {
		t.Errorf("Inspect = %+v", in)
	}
	if in := e.Inspect("zzz", 1, 0); in.Found {
		t.Errorf("an unknown name was found: %+v", in)
	}
	if in := e.Inspect("   ", 1, 0); in.Found {
		t.Errorf("whitespace was found: %+v", in)
	}
}

func TestNameAtReadsTheQualifiedNameAroundTheOffset(t *testing.T) {
	for code, want := range map[string]string{
		"%eval A::B::c + 1": "A::B::c",
		"x = Demo::w;":      "Demo::w",
		"ä::b":              "ä::b",
	} {
		at := strings.Index(code, "::") + 1
		if got := nameAt(code, at); got != want {
			t.Errorf("nameAt(%q, %d) = %q, want %q", code, at, got, want)
		}
	}
}

func TestInterruptStopsARunningCommandAndTheNextCellRuns(t *testing.T) {
	e := newEngine(t)
	if err := e.Session().SetBudgets(hugeBudgets(t, e)); err != nil {
		t.Fatal(err)
	}
	mustRun(t, e, "action spin {\n  first start;\n  merge m;\n  action a;\n  succession first start then m;\n  succession first m then a;\n  succession first a then m;\n}")
	mustRun(t, e, "%action spin")
	done := make(chan error, 1)
	go func() { done <- e.Execute("%continue", &recorder{}) }()
	time.Sleep(200 * time.Millisecond)
	e.Interrupt()
	select {
	case err := <-done:
		var failure *ExecError
		if !errors.As(err, &failure) || failure.Name != "KeyboardInterrupt" {
			t.Fatalf("interrupted %%continue = %v, want KeyboardInterrupt", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("the continue command did not stop within ten seconds of the interrupt")
	}
	mustRun(t, e, "1 + 1")
}

func contains(list []string, s string) bool {
	for _, item := range list {
		if item == s {
			return true
		}
	}
	return false
}

// hugeBudgets are the session's budgets with the action step budget raised so
// far that only an interrupt ends a spinning action.
func hugeBudgets(t *testing.T, e *REPLEngine) runtime.Budgets {
	t.Helper()
	b := e.Session().Budgets()
	b.MaxActionSteps = 1 << 40
	return b
}

const vizKernelModel = `package P {
  part def Wheel;
  part def Car { part fl : Wheel; part fr : Wheel; connect fl to fr; }
  part car : Car { part a : Wheel; part b : Wheel; connect a to b; }
  state def Lamp { state off; state on; transition off then on; }
  action def Start { action a; action b; first a then b; }
}
`

// %viz with no form shows a diagram: Mermaid, and the DOT the kind is drawn in,
// as SVG where Graphviz draws it; a form asked for is shown as %render shows it.
func TestVizShowsADiagram(t *testing.T) {
	e := newEngine(t)
	mustRun(t, e, vizKernelModel)
	cases := []struct {
		code string
		mark string
	}{
		{"%viz P::Car", "flowchart"},
		{"%viz P::car", "flowchart LR"},
		{"%viz --view STATE P::Lamp", "stateDiagram-v2"},
		{"%viz P::Lamp", "stateDiagram-v2"},
		{"%viz P::Start", "flowchart"},
		{"%viz --style LR --style ortholine P::Car P::Lamp", "flowchart LR"},
	}
	for _, c := range cases {
		out := mustRun(t, e, c.code)
		if len(out.displays) != 1 {
			t.Fatalf("%s displayed %d bundles, want 1", c.code, len(out.displays))
		}
		bundle := out.displays[0]
		mermaid, ok := bundle[mimeMermaid].(string)
		if !ok || !strings.Contains(mermaid, c.mark) {
			t.Errorf("%s: %s = %q, want a diagram holding %q", c.code, mimeMermaid, bundle[mimeMermaid], c.mark)
		}
		dot, ok := bundle[mimeDot].(string)
		if !ok || !strings.Contains(dot, "digraph") {
			t.Errorf("%s: %s = %q, want the DOT source beside the Mermaid", c.code, mimeDot, bundle[mimeDot])
		}
		if text, ok := bundle[mimeText].(string); !ok || text != mermaid {
			t.Errorf("%s: %s = %q, want the Mermaid source as the plain text", c.code, mimeText, bundle[mimeText])
		}
		if svg, ok := bundle[mimeSVG].(string); ok != dotDrawn() || ok && !strings.Contains(svg, "<svg") {
			t.Errorf("%s: %s present = %t, want %t (Graphviz drawn)", c.code, mimeSVG, ok, dotDrawn())
		}
	}
	out := mustRun(t, e, "%viz --style ortholine P::Car")
	if mermaid, _ := out.displays[0][mimeMermaid].(string); !strings.Contains(mermaid, "%% not represented: style ORTHOLINE (orthogonal line style) is not drawn") {
		t.Errorf("a pilot style not drawn was not noted in the diagram:\n%s", mermaid)
	}
	out = mustRun(t, e, "%viz --view SEQUENCE P::Car")
	if bundle := out.displays[0]; bundle[mimeMermaid] == nil || bundle[mimeDot] != nil {
		t.Errorf("a sequence diagram, which DOT does not draw, was shown as %v", bundle)
	}
	out = mustRun(t, e, "%viz text P::Car")
	if bundle := out.displays[0]; bundle[mimeMermaid] != nil || !strings.Contains(bundle[mimeText].(string), "tree rendering") {
		t.Errorf("the text form asked for was shown as %v", bundle)
	}
	out = mustRun(t, e, "%viz P::Car dot")
	if bundle := out.displays[0]; bundle[mimeDot] == nil || bundle[mimeMermaid] != nil {
		t.Errorf("the dot form asked for was shown as %v", bundle)
	}
	failure := mustFail(t, e, "%viz --view nope P::Car")
	if failure.Name != "UsageError" || !strings.Contains(failure.Value, `unknown view "nope"`) || !slices.ContainsFunc(failure.Traceback, func(line string) bool { return strings.HasPrefix(line, "usage: %viz") }) {
		t.Errorf("a usage problem failed as %+v", failure)
	}
	failure = mustFail(t, e, "%viz P::Car P::Nope")
	if failure.Name != "RenderError" || !strings.Contains(failure.Value, "P::Nope") {
		t.Errorf("an unresolved name failed as %+v", failure)
	}
}

// dotDrawn reports whether the kernel can draw DOT as SVG here.
func dotDrawn() bool {
	_, ok := drawDot("digraph { a -> b }")
	return ok
}
