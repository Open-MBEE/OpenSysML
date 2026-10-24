package symbols

import "testing"

// A view's `render tree;` is a RenderingUsage named by the rendering it
// references (SysML RenderingUsage::namingFeature), so the view binds the
// member `tree`; the declaration form keeps its own name.
func TestViewRenderingReferenceTakesTheRenderingsName(t *testing.T) {
	root := build(t, `package P {
		rendering def Tree;
		rendering tree : Tree;
		view def V { render tree; }
		view def W { render rendering t2 : Tree; }
	}`)

	pkg, _ := root.LookupLocal("P")
	view, _ := pkg.Scope.LookupLocal("V")
	tree, ok := view.Scope.LookupLocal("tree")
	if !ok {
		t.Fatalf("V members = %v, want tree", view.Scope.MemberNames())
	}
	if !tree.EffectiveName() {
		t.Errorf("V::tree naming = %v, want a name taken from its naming feature", tree.Naming)
	}
	other, _ := pkg.Scope.LookupLocal("W")
	if t2, ok := other.Scope.LookupLocal("t2"); !ok || t2.EffectiveName() {
		t.Fatalf("W members = %v, want the declared t2", other.Scope.MemberNames())
	}
}
