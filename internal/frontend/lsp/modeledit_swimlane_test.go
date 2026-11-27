package lsp

import (
	"strings"
	"testing"
)

// One performer with a swimlane in each of two flows; folded folds the first flow, whose
// lane is the one the Layout about the performer places.
const swimlaneEditModel = `package Two {
	private import DiagramLayout::*;
	private import StandardViewDefinitions::*;
	action runA { action load; then action ship; }
	action runB { action pack; then action post; }
	part line { part robot { perform runA.load; perform runB.pack; } }
	view unfolded : ActionFlowView {
		expose runA;
		expose runB;
		metadata Layout about line::robot { x = 320; y = 120; width = 220; height = 180; }
	}
	view folded : ActionFlowView {
		expose runA;
		expose runB;
		metadata Layout about runA { collapsed = true; }
		metadata Layout about line::robot { x = 320; y = 120; width = 220; height = 180; }
	}
	view single : ActionFlowView {
		expose runA;
	}
}
`

// A client that predates the fitted mark drags any lane and names only its performer. The
// server cannot tell which lane that was, so it refuses rather than move another.
func TestApplyModelEditRefusesAPerformerLayoutNamingNoLane(t *testing.T) {
	for _, viewName := range []string{"Two::folded", "Two::unfolded"} {
		t.Run(viewName, func(t *testing.T) {
			s, docURI := renderServer(t, "two.sysml", swimlaneEditModel)
			drawn := render(t, s, docURI, viewName)
			legacy := ModelEditOperation{Kind: EditSetLayout, Target: "Two::line::robot", View: drawn.View,
				Layout: &ModelEditLayout{X: float(219), Y: float(120)}}
			fitted := legacy
			fitted.Within = "Two::runB"
			for name, op := range map[string]ModelEditOperation{"no lane": legacy, "the fitted lane": fitted} {
				out := applyModelEdit(t, s, docURI, drawn.Version, op)
				if out.Edit != nil || len(out.Refused) != 1 || out.Refused[0].Failure != "not-drawn" ||
					!strings.Contains(out.Refused[0].Message, "drag that swimlane instead") {
					t.Errorf("%s: result = %+v, want one not-drawn refusal and no edit", name, out)
				}
			}
		})
	}
}

// The request naming the action that holds the lane its Layout places moves that lane and
// keeps the size the Layout states.
func TestApplyModelEditPlacesTheSwimlaneItsLayoutPlaces(t *testing.T) {
	s, docURI := renderServer(t, "two.sysml", swimlaneEditModel)
	drawn := render(t, s, docURI, "Two::unfolded")
	out := applyModelEdit(t, s, docURI, drawn.Version, ModelEditOperation{Kind: EditSetLayout, Target: "Two::line::robot",
		View: drawn.View, Within: "Two::runA", Layout: &ModelEditLayout{X: float(219), Y: float(120), Width: float(220), Height: float(180)}})
	applied, redrawn := redraw(t, s, docURI, swimlaneEditModel, out, 2, drawn.View)
	if !strings.Contains(applied, "metadata Layout about line::robot { x = 219; y = 120; width = 220; height = 180; }") {
		t.Errorf("the lane's Layout was not rewritten in place:\n%s", applied)
	}
	var placed, fitted int
	for _, n := range redrawn.Nodes {
		switch {
		case n.Kind != "swimlane":
		case n.Fitted:
			fitted++
		case n.X != nil && *n.X == 219:
			placed++
		}
	}
	if placed != 1 || fitted != 1 {
		t.Errorf("lanes after the move: %d placed at x = 219 and %d fitted, want one of each", placed, fitted)
	}
}

// A performer with one lane in the view is placed by its name alone, as before the mark.
func TestApplyModelEditPlacesAPerformersOnlySwimlane(t *testing.T) {
	s, docURI := renderServer(t, "two.sysml", swimlaneEditModel)
	drawn := render(t, s, docURI, "Two::single")
	out := applyModelEdit(t, s, docURI, drawn.Version, ModelEditOperation{Kind: EditSetLayout, Target: "Two::line::robot",
		View: drawn.View, Layout: &ModelEditLayout{X: float(40), Y: float(60)}})
	applied, _ := redraw(t, s, docURI, swimlaneEditModel, out, 2, drawn.View)
	if !strings.Contains(applied, "Layout about Two::line::robot { x = 40; y = 60; }") {
		t.Errorf("the only lane was not placed:\n%s", applied)
	}
}

// One performer with a swimlane in a flow and in the flow nested in it, placed by an inline
// Layout; jig performs in one action only.
const nestedSwimlaneEditModel = `package F9 {
	private import DiagramLayout::*;
	action assemble {
		action fetch;
		action build { action cut; then action weld; }
		first fetch then build;
	}
	part arm {
		@Layout { x = 320; y = 120; width = 220; height = 180; }
		perform assemble.fetch;
		perform assemble.build.cut;
	}
	part jig { perform assemble.build.weld; }
}
`

// A pseudo-view states no view, so the Layout is written inline and a legacy request names
// neither the rendering nor the lane. With lanes in several actions the server refuses it,
// as it refuses the nested lane, which is drawn around its nodes there.
func TestApplyModelEditRefusesAnInlinePerformerLayoutNamingNoLane(t *testing.T) {
	const pseudo = "#action:F9::assemble"
	s, docURI := renderServer(t, "f9.sysml", nestedSwimlaneEditModel)
	drawn := render(t, s, docURI, pseudo)
	legacy := ModelEditOperation{Kind: EditSetLayout, Target: "F9::arm", Layout: &ModelEditLayout{X: float(219), Y: float(120)}}
	unrendered, unnamed, nested := legacy, legacy, legacy
	unrendered.Within = "F9::assemble"
	unnamed.Rendering = pseudo
	nested.Rendering, nested.Within = pseudo, "F9::assemble::build"
	for name, op := range map[string]ModelEditOperation{"legacy": legacy, "no rendering": unrendered, "no lane": unnamed, "the nested lane": nested} {
		out := applyModelEdit(t, s, docURI, drawn.Version, op)
		if out.Edit != nil || len(out.Refused) != 1 || out.Refused[0].Failure != "not-drawn" ||
			!strings.Contains(out.Refused[0].Message, "the action it is drawn in") {
			t.Errorf("%s: result = %+v, want one not-drawn refusal and no edit", name, out)
		}
	}
}

// The request naming the pseudo-view and the action holding the lane the inline Layout
// places there moves that lane and keeps its size: the outer lane, or the nested flow's
// lane where that flow is rendered alone. A performer of one action needs neither name.
func TestApplyModelEditPlacesASwimlaneInAPseudoView(t *testing.T) {
	for _, tc := range []struct{ name, pseudo, target, within, want string }{
		{"the outer lane", "#action:F9::assemble", "F9::arm", "F9::assemble", "@Layout { x = 219; y = 140; width = 220; height = 180; }"},
		{"the only lane", "#action:F9::assemble::build", "F9::arm", "F9::assemble::build", "@Layout { x = 219; y = 140; width = 220; height = 180; }"},
		{"a performer of one action", "#action:F9::assemble", "F9::jig", "", "@DiagramLayout::Layout { x = 219; y = 140; width = 220; height = 180; }"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, docURI := renderServer(t, "f9.sysml", nestedSwimlaneEditModel)
			drawn := render(t, s, docURI, tc.pseudo)
			op := ModelEditOperation{Kind: EditSetLayout, Target: tc.target, Within: tc.within,
				Layout: &ModelEditLayout{X: float(219), Y: float(140), Width: float(220), Height: float(180)}}
			if tc.within != "" {
				op.Rendering = tc.pseudo
			}
			out := applyModelEdit(t, s, docURI, drawn.Version, op)
			applied, _ := redraw(t, s, docURI, nestedSwimlaneEditModel, out, 2, tc.pseudo)
			if !strings.Contains(applied, tc.want) {
				t.Errorf("want %q in:\n%s", tc.want, applied)
			}
		})
	}
}
