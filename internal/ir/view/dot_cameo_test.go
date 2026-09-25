package view

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
)

// The drawing styles: pilot and cameo, the empty name the pilot, anything
// else a typed error naming the styles there are.
func TestDrawingStyles(t *testing.T) {
	if got := DrawingStyleNames(); got != "pilot, cameo" {
		t.Errorf("DrawingStyleNames() = %q", got)
	}
	for _, name := range []string{"", "pilot", "cameo"} {
		want := DrawingStyle(name)
		if name == "" {
			want = StylePilot
		}
		if got, ok := ParseDrawingStyle(name); !ok || got != want {
			t.Errorf("ParseDrawingStyle(%q) = %q, %v; want %q", name, got, ok, want)
		}
	}
	if got, ok := ParseDrawingStyle("Cameo"); ok || got != "" {
		t.Errorf("ParseDrawingStyle(\"Cameo\") = %q, %v; want none", got, ok)
	}
	_, err := render(t, "state.sysml", "MachineViews::vehicleStates").DOTWith(Options{Style: "magicdraw"})
	var unknown *UnknownDrawingStyleError
	if !errors.As(err, &unknown) || unknown.Name != "magicdraw" || !errors.Is(err, ErrUnknownDrawingStyle) {
		t.Fatalf("DOTWith(magicdraw) error = %v; want an UnknownDrawingStyleError", err)
	}
	if want := `unknown drawing style "magicdraw"; the styles are pilot, cameo`; err.Error() != want {
		t.Errorf("error = %q, want %q", err, want)
	}
	for _, form := range Forms() {
		if got := form.TakesStyle(); got != (form == FormDot) {
			t.Errorf("%s.TakesStyle() = %v", form, got)
		}
	}
}

// The empty style and StylePilot write the same DOT, byte for byte, as the
// style-less call did: the Pilot look is the default and is unchanged.
func TestDOTPilotIsTheDefault(t *testing.T) {
	for _, tc := range []struct{ file, view string }{
		{"tree.sysml", "VehicleViews::vehicleView"},
		{"interconnection.sysml", "PlantViews::loopView"},
		{"state.sysml", "MachineViews::vehicleStates"},
		{"action.sysml", "FlowViews::driveView"},
		{"layout.sysml", "PlantViews::placedView"},
	} {
		rendering := render(t, tc.file, tc.view)
		plain, err := rendering.DOT()
		if err != nil {
			t.Fatalf("%s: DOT: %v", tc.view, err)
		}
		pilot, err := rendering.DOTWith(Options{Style: StylePilot})
		if err != nil {
			t.Fatalf("%s: DOTWith(pilot): %v", tc.view, err)
		}
		if pilot != plain {
			t.Errorf("%s: the pilot style differs from the default:\n%s\n---\n%s", tc.view, pilot, plain)
		}
		if strings.Contains(plain, dotFrameCluster) || strings.Contains(plain, cameoBlockFill) {
			t.Errorf("%s: the default DOT carries the Cameo look:\n%s", tc.view, plain)
		}
	}
}

// The Cameo goldens, one per kind the DOT form draws: each is well-formed DOT,
// framed, in Arial, with Cameo's gradient fills and thin dark borders, and
// keeps the default `dot` engine of an unplaced rendering.
func TestGoldenDOTCameo(t *testing.T) {
	cases := []struct {
		name, file, view, header, fill string
	}{
		{"tree", "tree.sysml", "VehicleViews::vehicleView", "<b>bdd</b> [Block] Vehicles::Vehicle [ vehicleView ]", cameoBlockFill},
		{"interconnection", "interconnection.sysml", "PlantViews::loopView", "<b>ibd</b> [Block] Loop [ loopView ]", cameoBlockFill},
		{"state", "state.sysml", "MachineViews::vehicleStates", "<b>stm</b> [State Machine] VehicleStates [ vehicleStates ]", cameoStateFill},
		{"action", "action.sysml", "FlowViews::driveView", "<b>act</b> [Activity] Drive [ driveView ]", cameoActionFill},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rendering := render(t, tc.file, tc.view)
			dot, err := rendering.WriteWith(FormDot, Options{Style: StyleCameo})
			if err != nil {
				t.Fatalf("write dot: %v", err)
			}
			checkGolden(t, filepath.Join("testdata", tc.name+".cameo.dot.golden"), dot)
			checkDOTSyntax(t, dot)
			for _, want := range []string{
				"// layout: dot\n",
				`subgraph "cluster_frame" {`,
				"label=<" + tc.header + ">;",
				`node [shape=box, style=filled, fillcolor="` + cameoBlockFill + `", gradientangle=0, color="` + cameoBlockLine + `", fontname="Arial", fontsize=11, fontcolor="` + cameoTextColor + `", penwidth=1];`,
				`edge [color="` + cameoEdgeColor + `", fontname="Arial", fontsize=9, fontcolor="` + cameoTextColor + `", penwidth=1, arrowhead=open];`,
				`fillcolor="` + tc.fill + `"`,
			} {
				if !strings.Contains(dot, want) {
					t.Errorf("cameo DOT lacks %q:\n%s", want, dot)
				}
			}
			if strings.Contains(dot, "Helvetica") || strings.Contains(dot, "#181818") {
				t.Errorf("cameo DOT keeps the Pilot look:\n%s", dot)
			}
		})
	}
}

// The Cameo state and action looks: a state's `do` compartment under a rule, a
// composite state as a rounded gradient container, the initial dot, the final
// bullseye, fork and join bars, the decision diamond, and open arrowheads.
func TestDOTCameoPseudonodes(t *testing.T) {
	state, err := render(t, "state.sysml", "MachineViews::vehicleStates").DOTWith(Options{Style: StyleCameo})
	if err != nil {
		t.Fatalf("DOT: %v", err)
	}
	for _, want := range []string{
		`"n5" [shape=point, fillcolor=black, label=""];`,
		`<hr/><tr><td align="left">initial</td></tr></table>>];`,
		"style=\"rounded,filled\";\n      fillcolor=\"" + cameoStateFill + "\";",
	} {
		if !strings.Contains(state, want) {
			t.Errorf("cameo state DOT lacks %q:\n%s", want, state)
		}
	}
	action, err := render(t, "action.sysml", "FlowViews::driveView").DOTWith(Options{Style: StyleCameo})
	if err != nil {
		t.Fatalf("DOT: %v", err)
	}
	for _, want := range []string{"shape=doublecircle", "shape=diamond", `fillcolor="` + cameoActionFill + `"`} {
		if !strings.Contains(action, want) {
			t.Errorf("cameo action DOT lacks %q:\n%s", want, action)
		}
	}
}

// A Style annotation wins over the style's defaults, in Cameo as in Pilot; a
// palette recolours a Cameo drawing the way it does a Pilot one, and the two
// options are independent.
func TestDOTCameoHonoursStylesAndPalettes(t *testing.T) {
	rendering := render(t, "layout.sysml", "PlantViews::placedView")
	cameo, err := rendering.DOTWith(Options{Style: StyleCameo})
	if err != nil {
		t.Fatalf("DOT: %v", err)
	}
	checkDOTSyntax(t, cameo)
	for _, want := range []string{
		"// layout: neato -n2\n",
		`fillcolor="#FFE8BD", color="#333333", fontname="Arial", fontsize=11, pos=`,
		`color="#0000FF", pos="400,730 400,730 450,680 450,680 450,680 500,730 500,730"`,
		`"note:1" [shape=note, fillcolor="` + cameoNoteFill + `", color="` + cameoLineColor + `", label=<<font point-size="9">«comment»</font><br/>anchored>, pos="700,745!", pin=true, width=1.3888888888888888, height=0.4166666666666667, fixedsize=true];`,
		`"note:1" -> "n2" [style=dashed, arrowhead=none];`,
	} {
		if !strings.Contains(cameo, want) {
			t.Errorf("cameo DOT lacks %q:\n%s", want, cameo)
		}
	}
	both, err := rendering.DOTWith(Options{Style: StyleCameo, Palette: PaletteOkabeIto})
	if err != nil {
		t.Fatalf("DOT: %v", err)
	}
	checkDOTSyntax(t, both)
	fills := familyFills{palette: PaletteOkabeIto}
	fills.collect(rendering.Roots[0])
	tank := rendering.Roots[0].Children[1]
	if want := `fillcolor="` + fills.fill(tank) + `"`; !strings.Contains(both, want) {
		t.Errorf("cameo DOT under okabe-ito lacks the palette fill %q:\n%s", want, both)
	}
	if !strings.Contains(both, `subgraph "cluster_frame"`) || !strings.Contains(both, `fontname="Arial"`) {
		t.Errorf("a palette undoes the Cameo look:\n%s", both)
	}
	// A Style's colours are the node's own, and stay over a palette's family fill.
	if !strings.Contains(both, `fillcolor="#FFE8BD", color="#333333", label=`) {
		t.Errorf("a palette overrides a Style's fill:\n%s", both)
	}
}

// Notes are drawn in the Pilot style too, and the forms that draw none say so.
func TestNotesAcrossForms(t *testing.T) {
	rendering := render(t, "layout.sysml", "PlantViews::placedView")
	dot, err := rendering.DOT()
	if err != nil {
		t.Fatalf("DOT: %v", err)
	}
	for _, want := range []string{
		`"note:0" [shape=note, label="always", pos="43.5,692!", pin=true, width=0.9305555555555556, height=0.5];`,
		`"note:2" [shape=note, label="free", pos="27,82!", pin=true, width=0.75, height=0.5];`,
		`"note:0" -> "n1" [style=dashed, arrowhead=none];`,
	} {
		if !strings.Contains(dot, want) {
			t.Errorf("DOT lacks %q:\n%s", want, dot)
		}
	}
	notice := "not represented: 3 note(s); the dot form draws notes"
	mermaid, plantuml := rendering.Mermaid(), rendering.Text()
	if !strings.Contains(mermaid, "%% "+notice) {
		t.Errorf("Mermaid drops the notes silently:\n%s", mermaid)
	}
	if puml, _ := rendering.PlantUML(); !strings.Contains(puml, "' "+notice) {
		t.Errorf("PlantUML drops the notes silently:\n%s", puml)
	}
	if !strings.Contains(plantuml, "notes:\n  \"always\" on pump at (10, 90)\n  \"anchored\" on tank at (650, 40) size 100×30\n  \"free\" at (0, 700)\n") {
		t.Errorf("text lacks the notes:\n%s", plantuml)
	}
	for _, form := range []Form{FormMermaid, FormPlantUML} {
		out, err := rendering.WriteWith(form, Options{Style: StyleCameo})
		if err != nil {
			t.Fatalf("%s: %v", form, err)
		}
		if !strings.Contains(out, "not represented: style cameo; only the DOT form draws a diagram in a style") {
			t.Errorf("%s takes the cameo style silently:\n%s", form, out)
		}
	}
}
