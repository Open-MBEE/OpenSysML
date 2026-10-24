package view

import (
	"path/filepath"
	"testing"
)

// TestGoldenTreeGeneralization locks every graphical form of a positioned tree
// whose specialization edges follow Routes naming their generals, in Cameo-sized
// boxes that keep their keyword line beside a wrapped name.
func TestGoldenTreeGeneralization(t *testing.T) {
	rendering := render(t, "tree-generalization.sysml", "MicroscopeViews::positioned")
	for _, form := range []Form{FormDot, FormMermaid, FormPlantUML, FormD2, FormText} {
		out, err := rendering.Write(form)
		if err != nil {
			t.Fatalf("write %s: %v", form, err)
		}
		checkGolden(t, filepath.Join("testdata", "tree-generalization."+string(form)+".golden"), out)
	}
	cameo, err := rendering.DOTWith(Options{Style: StyleCameo})
	if err != nil {
		t.Fatalf("DOTWith(cameo): %v", err)
	}
	checkGolden(t, filepath.Join("testdata", "tree-generalization.cameo.dot.golden"), cameo)
	for _, view := range []string{"second", "unpositioned"} {
		dot, err := render(t, "tree-generalization.sysml", "MicroscopeViews::"+view).Write(FormDot)
		if err != nil {
			t.Fatalf("write %s: %v", view, err)
		}
		checkGolden(t, filepath.Join("testdata", "tree-generalization-"+view+".dot.golden"), dot)
	}
}

// TestTreeSpecializationRoutes checks that a Route about a specific element
// naming a general steers the edge to that general alone: each general of a
// subtype with two gets its own line, the same subtype follows a different
// Route in another view, and the unpositioned tree routes nothing.
func TestTreeSpecializationRoutes(t *testing.T) {
	routes := func(view string) map[string][]Point {
		rendering := render(t, "tree-generalization.sysml", "MicroscopeViews::"+view)
		names := map[string]string{}
		for _, root := range rendering.Roots {
			names[root.ID] = root.Name
		}
		out := map[string][]Point{}
		for _, edge := range rendering.Edges {
			if edge.Kind == EdgeSpecialization {
				out[names[edge.From]+" -> "+names[edge.To]] = edge.Route
			}
		}
		return out
	}
	positioned := routes("positioned")
	want := map[string][]Point{
		"Microscope::'Secondary Electron Detector' -> Microscope::Detector":     {{240, 160}, {240, 110}, {315, 110}, {315, 53}},
		"Microscope::'Backscattered Electron Detector' -> Microscope::Detector": {{440, 160}, {440, 110}, {315, 110}, {315, 53}},
		"Microscope::'Backscattered Electron Detector' -> Microscope::Sensor":   {{520, 160}, {520, 120}, {535, 120}, {535, 53}},
	}
	if len(positioned) != len(want) {
		t.Fatalf("positioned draws %d specialization edges, want %d: %v", len(positioned), len(want), positioned)
	}
	for edge, points := range want {
		if got := positioned[edge]; !samePoints(got, points) {
			t.Errorf("positioned route of %s = %v, want %v", edge, got, points)
		}
	}
	second := routes("second")
	if got := second["Microscope::'Backscattered Electron Detector' -> Microscope::Detector"]; !samePoints(got, []Point{{200, 140}, {200, 90}, {75, 90}, {75, 53}}) {
		t.Errorf("second route = %v, want the view's own Route", got)
	}
	if len(second) != 1 {
		t.Errorf("second draws %d specialization edges, want the one between its two exposed elements", len(second))
	}
	for edge, points := range routes("unpositioned") {
		if len(points) != 0 {
			t.Errorf("unpositioned routes %s through %v, want nothing", edge, points)
		}
	}
}

func samePoints(got, want []Point) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}
