package lsp

import "testing"

// A Layout binding only collapsed reaches the client as a collapsed node with
// no place, so the panel folds it and lays it out itself.
func TestRenderFoldOnlyLayoutCarriesCollapsedWithoutAPlace(t *testing.T) {
	const src = `package Kit {
	part def Widget {
		part cog : Cog {
			part tooth;
		}
	}
	part def Cog;
}

package KitViews {
	private import Views::*;
	private import StandardViewDefinitions::*;
	private import DiagramLayout::*;

	view folded : InterconnectionView {
		expose Kit::Widget;
		metadata Layout about Kit::Widget::cog { collapsed = true; }
	}
}
`
	s, docURI := renderServer(t, "kit.sysml", src)
	out := render(t, s, docURI, "KitViews::folded")
	var cog *RenderNode
	for i := range out.Nodes {
		switch out.Nodes[i].Name {
		case "cog":
			cog = &out.Nodes[i]
		case "tooth":
			t.Errorf("tooth is drawn under the collapsed cog")
		}
	}
	if cog == nil {
		t.Fatalf("cog is not drawn; nodes: %+v", out.Nodes)
	}
	if !cog.Collapsed || cog.X != nil || cog.Y != nil || cog.Width != nil || cog.Height != nil {
		t.Errorf("cog = %+v, want collapsed with no x, y, width or height", *cog)
	}
}
