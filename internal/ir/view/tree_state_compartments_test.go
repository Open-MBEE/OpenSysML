package view

import (
	"path/filepath"
	"strings"
	"testing"
)

// TestGoldenTreeStateCompartments folds a state definition's members into the
// figure's compartments: the entry, do and exit actions under `state actions`,
// the successions under `successions`, the substates under `states`.
func TestGoldenTreeStateCompartments(t *testing.T) {
	options := Options{Compartments: CompartmentsMost}
	for _, tc := range []struct {
		view, golden string
		rows         []string
	}{
		{"burnerTree", "state-forms-most", []string{"successions", "first running then off", "transitions"}},
		{"runningTree", "state-forms-running-most", []string{"state actions", "entry action ignite", "exit action", "do action", "states"}},
	} {
		rendering := render(t, "state-forms.sysml", "FurnaceViews::"+tc.view)
		for _, form := range []Form{FormDot, FormMermaid, FormPlantUML, FormD2, FormText} {
			out, err := rendering.WriteWith(form, options)
			if err != nil {
				t.Fatalf("write %s %s: %v", tc.view, form, err)
			}
			checkGolden(t, filepath.Join("testdata", tc.golden+"."+string(form)+".golden"), out)
		}
		text, err := rendering.WriteWith(FormText, options)
		if err != nil {
			t.Fatal(err)
		}
		for _, want := range tc.rows {
			if !strings.Contains(text, want) {
				t.Errorf("%s folded tree lacks %q:\n%s", tc.view, want, text)
			}
		}
		dot, err := rendering.WriteWith(FormDot, options)
		if err != nil {
			t.Fatal(err)
		}
		for _, want := range tc.rows {
			if !strings.Contains(dot, want) {
				t.Errorf("%s folded DOT lacks %q", tc.view, want)
			}
		}
	}
}
