package view

import (
	"errors"
	"strings"
	"testing"
)

// The edge routings are named, the default splines by the empty name too, and
// a name none has is refused on every form by an error naming the routings.
func TestLinesAreNamed(t *testing.T) {
	for name, want := range map[string]Lines{"": LinesSplines, "splines": LinesSplines, "ortho": LinesOrtho, "polyline": LinesPolyline} {
		if got, ok := ParseLines(name); !ok || got != want {
			t.Errorf("ParseLines(%q) = %q, %v, want %q", name, got, ok, want)
		}
	}
	for _, name := range []string{"curved", "ORTHO", "ORTHOLINE"} {
		if _, ok := ParseLines(name); ok {
			t.Errorf("ParseLines accepts %q", name)
		}
	}
	if got := LinesNames(); got != "splines, ortho, polyline" {
		t.Errorf("LinesNames() = %q", got)
	}
	rendering := render(t, "interconnection-ports.sysml", "ToasterViews::toasterView")
	for _, form := range Forms() {
		_, err := rendering.WriteWith(form, Options{Lines: Lines("curved")})
		var unknown *UnknownLinesError
		if !errors.As(err, &unknown) || !errors.Is(err, ErrUnknownLines) || unknown.Name != "curved" {
			t.Errorf("%s with lines \"curved\": err = %v, want an UnknownLinesError", form, err)
		} else if !strings.Contains(err.Error(), "splines, ortho, polyline") {
			t.Errorf("%s: %v does not name the routings", form, err)
		}
	}
	for _, write := range map[string]func(Options) (string, error){"DOT": rendering.DOTWith, "PlantUML": rendering.PlantUMLWith} {
		if _, err := write(Options{Lines: Lines("curved")}); !errors.Is(err, ErrUnknownLines) {
			t.Errorf("err = %v, want ErrUnknownLines", err)
		}
	}
}

// The DOT form routes its edges as the Lines asks: no `splines` attribute by
// default or under splines, `splines=ortho` and `splines=polyline` on the graph
// attribute list otherwise, on every rendering kind.
func TestDOTRoutesItsEdgesAsAsked(t *testing.T) {
	for _, fixture := range []struct{ file, view string }{
		{"interconnection-ports.sysml", "ToasterViews::toasterView"},
		{"action.sysml", "FlowViews::driveView"},
		{"state.sysml", "MachineViews::vehicleStates"},
	} {
		rendering := render(t, fixture.file, fixture.view)
		for _, lines := range []Lines{"", LinesSplines} {
			got, err := rendering.DOTWith(Options{Lines: lines})
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(got, "splines=") {
				t.Errorf("%s with lines %q sets splines:\n%s", fixture.view, lines, got)
			}
		}
		for lines, want := range map[Lines]string{LinesOrtho: "splines=ortho", LinesPolyline: "splines=polyline"} {
			got, err := rendering.DOTWith(Options{Lines: lines, Show: Shown{ElementEdgeLabels: true}})
			if err != nil {
				t.Fatal(err)
			}
			graph := got[strings.Index(got, "  graph ["):]
			graph = graph[:strings.Index(graph, "];")]
			if !strings.Contains(graph, ", "+want) || strings.Count(got, "splines=") != 1 {
				t.Errorf("%s with lines %s: graph attributes %q lack %s once:\n%s", fixture.view, lines, graph, want, got)
			}
			if strings.Contains(got, "not represented: lines") {
				t.Errorf("%s with lines %s is drawn, not noted:\n%s", fixture.view, lines, got)
			}
			var plain, external int
			for _, line := range strings.Split(got, "\n") {
				if !strings.Contains(line, " -> ") {
					continue
				}
				if strings.Contains(line, "[label=") || strings.Contains(line, ", label=") {
					plain++
				}
				if strings.Contains(line, "xlabel=") {
					external++
				}
			}
			if ortho := lines == LinesOrtho; ortho && plain > 0 || !ortho && external > 0 {
				t.Errorf("%s with lines %s labels %d edge(s) plainly and %d externally; ortho asks for xlabel alone, polyline for label alone:\n%s", fixture.view, lines, plain, external, got)
			}
			if ortho := lines == LinesOrtho; ortho && external == 0 && fixture.view == "ToasterViews::toasterView" {
				t.Errorf("%s with lines ortho keeps no edge label as an xlabel:\n%s", fixture.view, got)
			}
		}
	}
}

// The PlantUML form routes its edges as the Lines asks: no `linetype` by
// default, `skinparam linetype ortho` or `skinparam linetype polyline` after
// the direction otherwise, on every rendering kind.
func TestPlantUMLRoutesItsEdgesAsAsked(t *testing.T) {
	for _, fixture := range []struct{ file, view string }{
		{"interconnection-ports.sysml", "ToasterViews::toasterView"},
		{"action.sysml", "FlowViews::driveView"},
		{"state.sysml", "MachineViews::vehicleStates"},
	} {
		rendering := render(t, fixture.file, fixture.view)
		got, err := rendering.PlantUMLWith(Options{})
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(got, "linetype") {
			t.Errorf("%s by default sets linetype:\n%s", fixture.view, got)
		}
		for lines, want := range map[Lines]string{LinesOrtho: "skinparam linetype ortho\n", LinesPolyline: "skinparam linetype polyline\n"} {
			got, err := rendering.PlantUMLWith(Options{Lines: lines})
			if err != nil {
				t.Fatal(err)
			}
			if strings.Count(got, "linetype") != 1 || !strings.Contains(got, want) {
				t.Errorf("%s with lines %s lacks %q once:\n%s", fixture.view, lines, want, got)
			}
			if at, header := strings.Index(got, want), strings.Index(got, "hide stereotype\n"); at < header {
				t.Errorf("%s with lines %s routes before the style block:\n%s", fixture.view, lines, got)
			}
			if strings.Contains(got, "not represented: lines") {
				t.Errorf("%s with lines %s is drawn, not noted:\n%s", fixture.view, lines, got)
			}
		}
	}
}

// A form that routes no edge itself notes the routing asked for, as a comment
// in its own syntax, and the default silently; text, Markdown and the tables
// ignore it.
func TestFormsRoutingNoEdgeNoteTheRouting(t *testing.T) {
	rendering := render(t, "interconnection-ports.sysml", "ToasterViews::toasterView")
	for form, prefix := range map[Form]string{FormMermaid: "%% not represented: ", FormD2: "# not represented: "} {
		got, err := rendering.WriteWith(form, Options{Lines: LinesOrtho})
		if err != nil {
			t.Fatal(err)
		}
		want := prefix + "lines ortho; " + string(form) + " routes no edge itself, the dot and plantuml forms do\n"
		if !strings.Contains(got, want) {
			t.Errorf("%s with lines ortho lacks %q:\n%s", form, want, got)
		}
		if plain, err := rendering.WriteWith(form, Options{Lines: LinesSplines}); err != nil || strings.Contains(plain, "lines splines") {
			t.Errorf("%s with the default routing notes it (%v):\n%s", form, err, plain)
		}
	}
	routed, err := rendering.WriteWith(FormText, Options{Lines: LinesOrtho})
	if err != nil {
		t.Fatal(err)
	}
	if plain, _ := rendering.WriteWith(FormText, Options{}); routed != plain {
		t.Errorf("text changes under lines ortho:\n%s", routed)
	}
}
