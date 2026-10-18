package lsp

import (
	"strings"
	"testing"
)

// A rendering with nothing to draw says why, so a client never shows an empty
// drawing as a success: the view's own reason, and the first error of a
// document that did not analyse cleanly, in the words the CLI uses.
func TestRenderWithNothingToDrawSaysWhy(t *testing.T) {
	s, docURI := renderServer(t, "empty.sysml", `package Cart {
	part def Wheel;
}

package EmptyViews {
	private import Views::*;
	private import SysML::Systems::*;

	view filteredOut {
		expose Cart::*;
		filter @ActionUsage;
	}

	view exposesNothing {
		private import Cart::*;
	}
}
`)
	out := render(t, s, docURI, "EmptyViews::filteredOut")
	if len(out.Nodes) != 0 || len(out.Edges) != 0 || len(out.Rows) != 0 {
		t.Fatalf("filteredOut drew %d nodes, %d edges, %d rows; want none", len(out.Nodes), len(out.Edges), len(out.Rows))
	}
	want := []string{"every element the view exposes is rejected by its filter @ActionUsage; the rendering is empty"}
	if strings.Join(out.Notices, "\n") != strings.Join(want, "\n") {
		t.Errorf("filteredOut notices = %q, want %q", out.Notices, want)
	}
	out = render(t, s, docURI, "EmptyViews::exposesNothing")
	want = []string{"the view exposes nothing; the rendering is empty"}
	if strings.Join(out.Notices, "\n") != strings.Join(want, "\n") {
		t.Errorf("exposesNothing notices = %q, want %q", out.Notices, want)
	}

	broken, brokenURI := renderServer(t, "broken.sysml", `package Cart {
	part def Wheel;
}

package BrokenViews {
	private import Views::*;

	view brokenView {
		expose ;
	}
}
`)
	out = render(t, broken, brokenURI, "BrokenViews::brokenView")
	if len(out.Nodes) != 0 || len(out.Edges) != 0 {
		t.Fatalf("brokenView drew %d nodes, %d edges; want none", len(out.Nodes), len(out.Edges))
	}
	if len(out.Notices) < 2 || !strings.HasPrefix(out.Notices[0], "broken.sysml did not analyse cleanly; nothing was rendered: 9:10: ") {
		t.Errorf("brokenView notices = %q, want the document's first error first", out.Notices)
	}
	kit, kitURI := renderServer(t, "kit.sysml", renderModel)
	drawn := render(t, kit, kitURI, "KitViews::widgetTree")
	for _, notice := range drawn.Notices {
		if strings.Contains(notice, "did not analyse cleanly") || strings.Contains(notice, "the rendering is empty") {
			t.Errorf("a drawn rendering carries an empty-rendering notice: %q", drawn.Notices)
		}
	}
}
