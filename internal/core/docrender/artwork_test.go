package docrender

import (
	"errors"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/view"
)

const (
	telescopeFixture = "telescope_report.sysml"
	mathFixture      = "math_report.sysml"
)

// TestDiagramsListGraphShapedViews: the two graph-shaped diagrams of the
// telescope report, in document order, with the source the Markdown fence
// carries; the table-kind view is left out.
func TestDiagramsListGraphShapedViews(t *testing.T) {
	document := fixtureDocument(t, filepath.Join("testdata", telescopeFixture), "Observatory::MassReport")
	for _, form := range []view.Form{"", view.FormMermaid, view.FormDot} {
		diagrams, err := Diagrams(document, form)
		if err != nil {
			t.Fatalf("Diagrams(%q): %v", form, err)
		}
		if len(diagrams) != 2 || diagrams[0].Name != "imaging" || diagrams[1].Name != "states" {
			t.Fatalf("Diagrams(%q) = %+v, want imaging then states", form, diagrams)
		}
		markdown, err := Markdown(document, MarkdownOptions{DiagramForm: form})
		if err != nil {
			t.Fatal(err)
		}
		for _, diagram := range diagrams {
			if diagram.Source == "" || !strings.Contains(markdown, "\n"+diagram.Source+"\n```") {
				t.Errorf("Diagrams(%q): %s source is not the fenced source:\n%s", form, diagram.Name, diagram.Source)
			}
		}
	}
}

func TestDiagramsErrors(t *testing.T) {
	var typed *Error
	if _, err := Diagrams(nil, ""); !errors.As(err, &typed) || typed.Kind != ErrorNilDocument {
		t.Errorf("nil document: %v", err)
	}
	document := fixtureDocument(t, filepath.Join("testdata", telescopeFixture), "Observatory::MassReport")
	if _, err := Diagrams(document, "svg"); !errors.As(err, &typed) || typed.Kind != ErrorUnknownForm {
		t.Errorf("unknown form: %v", err)
	}
}

// TestFormulasListDistinctMath: every math run and formula block once, in
// document order, keyed as the HTML backend writes them.
func TestFormulasListDistinctMath(t *testing.T) {
	document := fixtureDocument(t, filepath.Join("testdata", mathFixture), "Optics::OpticsReport")
	formulas := Formulas(document)
	if len(formulas) == 0 {
		t.Fatal("no formulas")
	}
	seen := map[Formula]int{}
	for _, f := range formulas {
		seen[f]++
		if f.Source != strings.TrimSpace(f.Source) {
			t.Errorf("untrimmed formula %q", f.Source)
		}
	}
	for f, n := range seen {
		if n > 1 {
			t.Errorf("formula %+v listed %d times", f, n)
		}
	}
	want := []Formula{
		{Source: `m \propto D^{2.5}_{\text{eff}}`},
		{Source: "A = \\pi \\left(\\frac{D}{2}\\right)^2\n\n  = \\frac{\\pi D^2}{4}", Display: true},
		{Source: `\text{cost} = 10^6\,\$ \times D^{2.5} + $`, Display: true},
	}
	if !reflect.DeepEqual(formulas[:3], want) {
		t.Errorf("first formulas = %#v, want %#v", formulas[:3], want)
	}
	if got := want[1].TeX(); got != "A = \\pi \\left(\\frac{D}{2}\\right)^2\n= \\frac{\\pi D^2}{4}" {
		t.Errorf("display TeX = %q", got)
	}
	if got := want[2].TeX(); got != `\text{cost} = 10^6\,\$ \times D^{2.5} + \$` {
		t.Errorf("display TeX escapes a bare dollar: %q", got)
	}
	if got := (Formula{Source: " a\nb "}).TeX(); got != "a b" {
		t.Errorf("inline TeX = %q", got)
	}
	if Formulas(nil) != nil {
		t.Error("nil document lists formulas")
	}
}

// TestCaptionsFollowDocumentOrder: the captions of tables (grouped ones
// included), diagrams (table-kind included) and formulas, in the order the
// Markdown backend writes them as emphasized paragraphs; ordinary emphasized
// prose is not among them.
func TestCaptionsFollowDocumentOrder(t *testing.T) {
	document := fixtureDocument(t, filepath.Join("testdata", telescopeFixture), "Observatory::MassReport")
	want := []string{
		"Subsystems grouped by zone",
		"All subsystems by mass",
		"Mass margins (allocated - estimated)",
		"Subsystem notes",
		"Imaging chain interconnection",
		"Observatory states, left to right",
		"Type of telescope",
	}
	if got := Captions(document); !reflect.DeepEqual(got, want) {
		t.Errorf("Captions = %q, want %q", got, want)
	}
	markdown, err := Markdown(document, MarkdownOptions{})
	if err != nil {
		t.Fatal(err)
	}
	var emphasized []string
	for _, line := range strings.Split(markdown, "\n") {
		if strings.HasPrefix(line, "*") && !strings.HasPrefix(line, "**") && strings.HasSuffix(line, "*") {
			emphasized = append(emphasized, strings.Trim(line, "*"))
		}
	}
	if !reflect.DeepEqual(emphasized, want) {
		t.Errorf("emphasized lines = %q, want the captions %q", emphasized, want)
	}

	document = fixtureDocument(t, filepath.Join("testdata", mathFixture), "Optics::OpticsReport")
	if got := Captions(document); !reflect.DeepEqual(got, []string{"Collecting area of a circular mirror"}) {
		t.Errorf("formula captions = %q", got)
	}
	if Captions(nil) != nil {
		t.Error("nil document has captions")
	}
}
