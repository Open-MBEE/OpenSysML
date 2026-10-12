package view

import (
	"errors"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

func TestRenderNamedResolvesViewsAndPseudoViews(t *testing.T) {
	renderer, index := loadFixtures(t, "interconnection.sysml")
	lookup := func(name string) []*symbols.Symbol { return index.LookupQualified(name) }

	for _, name := range []string{"PlantViews::loopView", "#interconnection:Plant::Loop"} {
		rendering, err := RenderNamed(renderer, name, lookup)
		if err != nil {
			t.Fatalf("RenderNamed(%q): %v", name, err)
		}
		if rendering == nil || rendering.Kind != KindInterconnection {
			t.Errorf("RenderNamed(%q) = %+v, want an interconnection", name, rendering)
		}
	}
}

func TestRenderNamedClassifiesSelectionErrors(t *testing.T) {
	renderer, index := loadFixtures(t, "interconnection.sysml")
	lookup := func(name string) []*symbols.Symbol { return index.LookupQualified(name) }

	tests := []struct {
		name string
		want SelectionErrorKind
		text string
	}{
		{name: "PlantViews::missing", want: SelectionNotFound, text: "no view named PlantViews::missing"},
		{name: "#tree:Plant::Missing", want: SelectionNotFound, text: "names nothing in this model"},
		{name: "#unknown:Plant::Loop", want: SelectionInvalid, text: "is no pseudo-view"},
		{name: "#tree", want: SelectionInvalid, text: "is untargeted"},
		{name: "Plant::Loop", want: SelectionInvalid, text: "not a view"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := RenderNamed(renderer, test.name, lookup)
			var selectionErr *SelectionError
			if !errors.As(err, &selectionErr) {
				t.Fatalf("RenderNamed error = %v, want SelectionError", err)
			}
			if selectionErr.Kind != test.want {
				t.Errorf("SelectionError.Kind = %d, want %d (%v)", selectionErr.Kind, test.want, err)
			}
			if got := err.Error(); !strings.Contains(got, test.text) {
				t.Errorf("error = %q, want it to contain %q", got, test.text)
			}
		})
	}
}
