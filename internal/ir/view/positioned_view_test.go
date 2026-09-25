package view

import (
	"strings"
	"testing"
)

// A note stated in a nested view's body is drawn in that view alone: the
// rendering of the enclosing view holds no `note:` box for it, while the nested
// view's own rendering draws it free.
func TestNotesScopedToTheViewStatingThem(t *testing.T) {
	outer := render(t, "positioned-views.sysml", "PositionedViews::outerView")
	dot, err := outer.DOT()
	if err != nil {
		t.Fatalf("outer DOT: %v", err)
	}
	if strings.Contains(dot, `"note:`) {
		t.Errorf("outer rendering draws a note the nested view stated:\n%s", dot)
	}
	nested := render(t, "positioned-views.sysml", "Scoped::Housing::detail")
	dot, err = nested.DOT()
	if err != nil {
		t.Fatalf("nested DOT: %v", err)
	}
	if !strings.Contains(dot, `"note:0"`) {
		t.Errorf("nested view's own rendering draws no note box:\n%s", dot)
	}
}

// An anchored note whose node a partial positioned drawing omits draws no box:
// the note is dropped with a notice, as the edges at the node are.
func TestAnchoredNoteOnAnOmittedNodeDrawsNoBox(t *testing.T) {
	rendering := render(t, "positioned-views.sysml", "PositionedViews::partialView")
	dot, err := rendering.DOT()
	if err != nil {
		t.Fatalf("DOT: %v", err)
	}
	if strings.Contains(dot, `"note:`) {
		t.Errorf("a note is drawn for the omitted member:\n%s", dot)
	}
	if !strings.Contains(dot, "no note is drawn") {
		t.Errorf("DOT reports no dropped note:\n%s", dot)
	}
}

// An exposed element the containment tree already draws below another exposed
// element is not appended as a root of its own: member is drawn once, under
// Whole, in every form the rendering feeds.
func TestExposedMemberIsNoSecondRoot(t *testing.T) {
	rendering := render(t, "positioned-views.sysml", "PositionedViews::dupView")
	if len(rendering.Roots) != 1 {
		t.Fatalf("roots = %d, want the one owner; member is its child", len(rendering.Roots))
	}
	dot, err := rendering.DOT()
	if err != nil {
		t.Fatalf("DOT: %v", err)
	}
	if got := strings.Count(dot, "member"); got != 1 {
		t.Errorf("member is declared %d times, want once:\n%s", got, dot)
	}
}

// A positioned or cameo drawing heads a node by the minimal suffix of its
// qualified name that distinguishes it: scattered roots of unrelated
// namespaces by their endings, not the whole name.
func TestPositionedDrawingLabelsSimply(t *testing.T) {
	rendering := render(t, "positioned-views.sysml", "PositionedViews::cameoView")
	dot, err := rendering.DOTWith(Options{Style: StyleCameo})
	if err != nil {
		t.Fatalf("DOT: %v", err)
	}
	for _, want := range []string{"<b>A::X</b>", "<b>B::X</b>", "<b>Y</b>"} {
		if !strings.Contains(dot, want) {
			t.Errorf("DOT lacks a simple head %s:\n%s", want, dot)
		}
	}
	for _, stale := range []string{"Named::A::X", "Named::B::X", "Elsewhere::Y"} {
		if strings.Contains(dot, stale) {
			t.Errorf("DOT still heads %s in full:\n%s", stale, dot)
		}
	}
}
