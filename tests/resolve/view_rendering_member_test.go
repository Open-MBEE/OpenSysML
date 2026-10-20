package resolve_test

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// A view's `render tree;` is a RenderingUsage named by the rendering it
// references (SysML RenderingUsage::namingFeature): V::tree resolves to that
// member, whose reference still reaches the rendering outside the view.
func TestViewRenderingReferenceResolvesByTheRenderingsName(t *testing.T) {
	r, _, docRoot := resolvedDocNamed(t, "views.sysml", `package P {
		rendering def Tree;
		rendering tree : Tree;
		view def V {
			render tree;
		}
	}`)
	if len(r.Diagnostics) != 0 {
		t.Fatalf("resolution diagnostics: %v", r.Diagnostics)
	}
	pkg := local(t, docRoot, "P")
	rendering := local(t, pkg.Scope, "tree")
	view := local(t, pkg.Scope, "V")

	member, ok := r.ResolveQualified(docRoot, ast.QualifiedNameOf("P", "V", "tree"))
	if !ok {
		t.Fatalf("P::V::tree did not resolve; V members = %v", view.Scope.MemberNames())
	}
	usage, isUsage := member.Decl.(*ast.Usage)
	if !isUsage || usage.Kind != ast.UsageViewRendering || member == rendering {
		t.Fatalf("P::V::tree resolved to %T %q, want the view's rendering usage", member.Decl, member.Name)
	}
	if got := semantics.NewModel(r).ReferencedFeature(member); got != rendering {
		t.Errorf("`render tree;` references %v, want P::tree", got)
	}
}
