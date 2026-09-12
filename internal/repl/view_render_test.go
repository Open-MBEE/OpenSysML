package repl

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/passes"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/view"
)

// A view stating no rendering renders as a containment tree, of what it exposes.
func TestRenderDefaultsToATree(t *testing.T) {
	out, _, err := viewSession(t).RunMeta("%render Demo::summary")
	if err != nil {
		t.Fatal(err)
	}
	text := strings.Join(out, "\n")
	for _, want := range []string{
		"Demo::summary - tree rendering",
		"the view states no rendering",
		"part def Demo::Vehicle",
		"part Demo::v (Vehicle)",
		"view Demo::summary::detail",
		"part def Demo::Wheel",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("%%render output is missing %q:\n%s", want, text)
		}
	}
}

// The Mermaid form is asked for by name, and is the same rendering.
func TestRenderWritesMermaidWhenAskedFor(t *testing.T) {
	out, _, err := viewSession(t).RunMeta("%render Demo::summary mermaid")
	if err != nil {
		t.Fatal(err)
	}
	text := strings.Join(out, "\n")
	if !strings.HasPrefix(text, "%% Demo::summary") || !strings.Contains(text, "flowchart TD") {
		t.Errorf("%%render mermaid = %q, want a Mermaid flowchart", text)
	}
}

// The DOT form is asked for by name, is the same rendering as a digraph, and is
// offered by the usage and help text.
func TestRenderWritesDotWhenAskedFor(t *testing.T) {
	s := viewSession(t)
	out, _, err := s.RunMeta("%render Demo::summary dot")
	if err != nil {
		t.Fatal(err)
	}
	text := strings.Join(out, "\n")
	for _, want := range []string{
		"// view: Demo::summary\n// kind: tree\n",
		"// layout: dot\n",
		`digraph "Demo::summary" {`,
		`label="part def Demo::Vehicle"`,
		`label="view Demo::summary::detail"`,
		"[arrowhead=none];",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("%%render dot is missing %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, "flowchart") {
		t.Errorf("%%render dot wrote Mermaid:\n%s", text)
	}
	wants(t, run(t, s, "%render"), "usage: %render <name> [text|mermaid|markdown|dot]")
	wants(t, run(t, s, "%render Demo::summary svg"), `unknown form "svg"`, "[text|mermaid|markdown|dot]")
	wants(t, run(t, s, "%help"), "%render <name> [form]", "Graphviz DOT")
}

// A view exposing nothing renders an empty artifact and says so.
func TestRenderOfAViewExposingNothingSaysSo(t *testing.T) {
	out, _, err := viewSession(t).RunMeta("%render Demo::empty")
	if err != nil {
		t.Fatalf("a view exposing nothing failed the command: %v", err)
	}
	if text := strings.Join(out, "\n"); !strings.Contains(text, "the rendering is empty") {
		t.Errorf("out = %v, want it to say the rendering is empty", out)
	}
}

// A name that is no view is the typed error %view returns, and a line at the
// prompt rather than a failed command.
func TestRenderOfANonViewIsTyped(t *testing.T) {
	s := viewSession(t)
	if _, err := s.ViewRendering("Demo::Vehicle"); !errors.Is(err, semantics.ErrNotAView) {
		t.Errorf("err = %v, want semantics.ErrNotAView", err)
	}
	out, _, err := s.RunMeta("%render Demo::Vehicle")
	if err != nil {
		t.Fatalf("a non-view should not fail the command: %v", err)
	}
	if text := strings.Join(out, "\n"); !strings.HasPrefix(text, "error: ") {
		t.Errorf("out = %v, want an error line", out)
	}
}

// A rendering kind this build does not produce names the kind and the view,
// rather than rendering something else.
func TestRenderOfAnUnsupportedKindNamesIt(t *testing.T) {
	s := viewSession(t)
	res := s.Submit(`package Shaped {
    private import StandardViewDefinitions::*;
    view shapes : GeometryView {
        expose Demo::Vehicle;
    }
}`)
	for _, d := range res.Diagnostics {
		if d.Severity == passes.SeverityError {
			t.Fatalf("the view did not load: %v", res.Diagnostics)
		}
	}
	_, err := s.ViewRendering("Shaped::shapes")
	if !errors.Is(err, view.ErrUnsupportedKind) {
		t.Fatalf("err = %v, want view.ErrUnsupportedKind", err)
	}
	for _, want := range []string{"Shaped::shapes", "geometry"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("err = %q, want it to name %q", err, want)
		}
	}
}

// A view stating the tabular rendering renders rows: aligned columns as text, a
// Markdown table as the machine-readable form, and a typed error for Mermaid.
func TestRenderOfATabularView(t *testing.T) {
	s := viewSession(t)
	res := s.Submit(`package Tabular {
    private import Views::*;
    view parts {
        expose Demo::Vehicle;
        render asElementTable;
    }
}`)
	for _, d := range res.Diagnostics {
		if d.Severity == passes.SeverityError {
			t.Fatalf("the view did not load: %v", res.Diagnostics)
		}
	}
	text := run(t, s, "%render Tabular::parts")
	for _, want := range []string{"Tabular::parts", "table rendering", "Element", "Declared in", "Demo::Vehicle"} {
		if !strings.Contains(text, want) {
			t.Errorf("the table is missing %q:\n%s", want, text)
		}
	}
	if markdown := run(t, s, "%render Tabular::parts markdown"); !strings.Contains(markdown, "| Element | Kind | Type | Declared in |") {
		t.Errorf("the Markdown form is no table:\n%s", markdown)
	}
	if out := run(t, s, "%render Tabular::parts mermaid"); !strings.Contains(out, "error: ") {
		t.Errorf("Mermaid of a table = %v, want an error line naming the form", out)
	}
}

func TestRenderOfAnUnknownNameReports(t *testing.T) {
	out, _, err := viewSession(t).RunMeta("%render Demo::Nope")
	if err != nil {
		t.Fatalf("an unknown name should not fail the command: %v", err)
	}
	if text := strings.Join(out, "\n"); !strings.HasPrefix(text, "error: ") {
		t.Errorf("out = %v, want an error line", out)
	}
}

func TestPseudoViewsRenderThroughTheSession(t *testing.T) {
	s := NewSession()
	res := s.Submit(`package Direct {
    port def Port;
    part def Network {
        port left : Port;
        port right : Port;
        connection link connect left to right;
    }
    state def Machine {
        entry; then idle;
        state idle;
    }
    action def Flow {
        first start;
        action finish;
        succession first start then finish;
    }
}`)
	for _, d := range res.Diagnostics {
		if d.Severity == passes.SeverityError {
			t.Fatalf("model did not load: %v", res.Diagnostics)
		}
	}
	cases := []struct {
		spec string
		kind view.Kind
		form view.Form
		want string
	}{
		{"#tree", view.KindTree, view.FormText, "part def Direct::Network"},
		{"#state:Direct::Machine", view.KindState, view.FormMermaid, "stateDiagram-v2"},
		{"#action:Flow", view.KindAction, view.FormMermaid, "flowchart TD"},
		{"#interconnection:Direct::Network", view.KindInterconnection, view.FormMermaid, "flowchart LR"},
		{"#table:Direct::Network", view.KindTable, view.FormMarkdown, "| Element | Kind | Type | Declared in |"},
	}
	for _, tc := range cases {
		t.Run(tc.spec, func(t *testing.T) {
			rendering, err := s.ViewRendering(tc.spec)
			if err != nil {
				t.Fatalf("ViewRendering(%q): %v", tc.spec, err)
			}
			if rendering.Kind != tc.kind || rendering.View != "" {
				t.Errorf("rendering = kind %q, view %q", rendering.Kind, rendering.View)
			}
			artifact, err := rendering.Write(tc.form)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(artifact, tc.want) {
				t.Errorf("%s is missing %q:\n%s", tc.spec, tc.want, artifact)
			}
			if strings.Contains(strings.SplitN(artifact, "\n", 2)[0], " — ") ||
				strings.HasPrefix(artifact, " - ") {
				t.Errorf("%s has an unnamed-view header:\n%s", tc.spec, artifact)
			}
		})
	}
	if text := run(t, s, "%render #tree"); !strings.HasPrefix(text, "tree rendering") {
		t.Errorf("%%render did not accept #tree:\n%s", text)
	}
}

func TestPseudoViewErrorsUseTheSessionLookupAndListAlternatives(t *testing.T) {
	s := viewSession(t)
	_, err := s.ViewRendering("#not-a-rendering")
	if err == nil {
		t.Fatal("an unsupported pseudo-view succeeded")
	}
	for _, spec := range view.PseudoViewSpecs() {
		if !strings.Contains(err.Error(), spec) {
			t.Errorf("error %q does not list %q", err, spec)
		}
	}

	_, _, lookupErr := s.lookupSymbol("Demo::Missing")
	_, err = s.ViewRendering("#state:Demo::Missing")
	if err == nil || lookupErr == nil || err.Error() != lookupErr.Error() {
		t.Errorf("pseudo-view error = %v, lookup error = %v", err, lookupErr)
	}
}

func TestTargetlessPseudoViewSpansLoadedDocuments(t *testing.T) {
	s := NewSession()
	res := s.SubmitFiles([]SourceFile{
		{Name: "second.kerml", Text: `package Second {
    class TwoA;
    class TwoB;
}`},
		{Name: "first.sysml", Text: `package First {
    part def OneA;
    part def OneB;
}`},
	})
	for _, d := range res.Diagnostics {
		if d.Severity == passes.SeverityError {
			t.Fatalf("model did not load: %v", res.Diagnostics)
		}
	}
	rendering, err := s.ViewRendering("#tree")
	if err != nil {
		t.Fatal(err)
	}
	text := rendering.Text()
	at := -1
	for _, want := range []string{"Second::TwoA", "Second::TwoB", "First::OneA", "First::OneB"} {
		next := strings.Index(text, want)
		if next < 0 {
			t.Fatalf("rendering does not span both documents; missing %s:\n%s", want, text)
		}
		if next <= at {
			t.Errorf("document order was not preserved at %s:\n%s", want, text)
		}
		at = next
	}
}

func TestViewsListsSessionViewsInDeclarationOrder(t *testing.T) {
	s := NewSession()
	res := s.SubmitFiles([]SourceFile{
		{Name: "first.sysml", Text: "package First { view k; }"},
		{Name: "second.sysml", Text: `package ViewsA {
    view z;
    view a;
}`},
	})
	for _, d := range res.Diagnostics {
		if d.Severity == passes.SeverityError {
			t.Fatalf("model did not load: %v", res.Diagnostics)
		}
	}
	views, err := s.Views()
	if err != nil {
		t.Fatal(err)
	}
	if len(views) != 3 || views[0].Name != "First::k" ||
		views[1].Name != "ViewsA::z" || views[2].Name != "ViewsA::a" {
		t.Errorf("views = %+v, want declaration order", views)
	}
}

func TestRenderMisuseShowsUsage(t *testing.T) {
	s := viewSession(t)
	for _, line := range []string{"%render", "%render Demo::summary svg"} {
		out, _, err := s.RunMeta(line)
		if err != nil {
			t.Fatal(err)
		}
		if len(out) != 1 || !strings.Contains(out[0], "%render <name>") {
			t.Errorf("%q = %v, want guidance naming the usage", line, out)
		}
	}
}

func TestRenderIsInHelpAndCompletion(t *testing.T) {
	if !strings.Contains(strings.Join(helpText(), "\n"), "%render") {
		t.Error("the render command is dispatched but not in help")
	}
	if !slices.Contains(metaCommands(), "%render") {
		t.Error("the render command is not in the command table")
	}
	s := viewSession(t)
	if got := s.Complete("%ren", len("%ren")); !slices.Contains(got.Candidates, "%render") {
		t.Errorf("completing %%ren offered %v, want %%render", got.Candidates)
	}
	// The view name completes as any name does, and the form after it does not.
	if got := s.Complete("%render Demo::sum", len("%render Demo::sum")); !slices.Contains(got.Candidates, "Demo::summary") {
		t.Errorf("completing a view name offered %v", got.Candidates)
	}
	for _, form := range []string{"text", "mermaid", "markdown"} {
		if got := s.Complete("%render Demo::summary ", len("%render Demo::summary ")); !slices.Contains(got.Candidates, form) {
			t.Errorf("completing the form offered %v, want %s", got.Candidates, form)
		}
	}
	// A form has been typed already, and a third argument is no command, so no
	// further form is offered.
	for _, head := range []string{"%render Demo::summary text ", "%render Demo::summary text mer"} {
		for _, form := range renderForms() {
			if got := s.Complete(head, len(head)); slices.Contains(got.Candidates, form) {
				t.Errorf("completing past the form offered %s: %v", form, got.Candidates)
			}
		}
	}
}

// A view whose name is unrestricted is rendered, named and completed with its
// quotes: the name holding a space stays one argument, and the form is offered
// only once that name is closed.
func TestRenderOfAViewWithAnUnrestrictedName(t *testing.T) {
	s := viewSession(t)
	res := s.Submit(`package Quoted {
    view 'My Summary' {
        expose Demo::Vehicle;
    }
    view 'frame' {
        expose Demo::Wheel;
    }
}`)
	for _, d := range res.Diagnostics {
		if d.Severity == passes.SeverityError {
			t.Fatalf("the views did not load: %v", res.Diagnostics)
		}
	}
	if text := run(t, s, "%render Quoted::'My Summary'"); !strings.Contains(text, "Quoted::'My Summary' - tree rendering") {
		t.Errorf("a quoted view name is not rendered under its written name:\n%s", text)
	}
	// A name spelling a keyword is written with its quotes, so it can be typed back.
	if text := run(t, s, "%render Quoted::'frame'"); !strings.Contains(text, "Quoted::'frame' - tree rendering") {
		t.Errorf("a keyword view name is not written with its quotes:\n%s", text)
	}
	// A quoted name still being typed is not yet the form position, so the
	// space inside it offers no form.
	head := "%render Quoted::'My Sum"
	for _, form := range renderForms() {
		if got := s.Complete(head, len(head)); slices.Contains(got.Candidates, form) {
			t.Errorf("completing inside an unfinished quoted name offered the form %s: %v", form, got.Candidates)
		}
	}
	// A closed quoted name holding a space is one argument, so the form follows it.
	head = "%render Quoted::'My Summary' "
	if got := s.Complete(head, len(head)); !slices.Contains(got.Candidates, "mermaid") {
		t.Errorf("completing the form after a quoted name offered %v", got.Candidates)
	}
	// The notation's own escape closes no name, so a name holding a quote is
	// finished and the form follows it too.
	head = `%render Quoted::'it\'s' `
	if got := s.Complete(head, len(head)); !slices.Contains(got.Candidates, "mermaid") {
		t.Errorf("completing the form after an escaped quote offered %v", got.Candidates)
	}
}

// %render is read-only with respect to the session: rendering between two steps
// of an action debugging session creates no object, ends no session, and leaves
// the object identities and the submission buffer as they were.
func TestRenderBetweenStepsDisturbsNothing(t *testing.T) {
	s := loadFixture(t, "testdata/action_debug.sysml")
	res := s.Submit(`package DebugViews {
    private import Views::*;
    view tallyView : StandardViewDefinitions::ActionFlowView {
        expose Debug::tally;
    }
}`)
	for _, d := range res.Diagnostics {
		if d.Severity == passes.SeverityError {
			t.Fatalf("the view did not load: %v", res.Diagnostics)
		}
	}
	run(t, s, "%instantiate Debug::tally")
	if started := run(t, s, "%action Debug::tally"); !strings.Contains(started, "Started action executor") {
		t.Fatalf("%%action failed: %s", started)
	}
	wants(t, run(t, s, "%step"), "✓ Step complete")

	before := s.actionExec
	beforeExecutor := before.executor
	beforeRuntime := s.rtCtx
	beforeIdentities := map[string]int64{}
	for name, obj := range s.instances {
		beforeIdentities[name] = obj.ID
	}
	beforeSnippets, beforeVersion := len(s.snippets), s.version

	rendered := run(t, s, "%render DebugViews::tallyView")
	if !strings.Contains(rendered, "action rendering") {
		t.Fatalf("%%render did not render the action: %s", rendered)
	}

	if s.actionExec != before || s.actionExec.executor != beforeExecutor {
		t.Error("%render replaced the action debugging session")
	}
	if s.rtCtx != beforeRuntime {
		t.Error("%render rebuilt the runtime context")
	}
	if len(s.instances) != len(beforeIdentities) {
		t.Errorf("objects after %%render = %v, want the %d there were", s.instances, len(beforeIdentities))
	}
	for name, id := range beforeIdentities {
		if got, ok := s.instances[name]; !ok || got.ID != id {
			t.Errorf("the object %s is now %v, want identity %d", name, got, id)
		}
	}
	if len(s.snippets) != beforeSnippets || s.version != beforeVersion {
		t.Errorf("submissions after %%render = %d at version %d, want %d at %d",
			len(s.snippets), s.version, beforeSnippets, beforeVersion)
	}
	// The session it was rendered in the middle of still runs to completion.
	wants(t, run(t, s, "%continue"), "✓ Action completed", "total = 5")
}
