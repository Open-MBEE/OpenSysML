package semantics

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// A view's rendering (SysML v2 8.3.26 ViewRenderingMembership, 10.2 Views and
// Viewpoints) is the rendering its body names with `render` or declares as a
// `rendering` member. The notation states which rendering a view uses; how that
// rendering is carried out is left to the tool.

// ViewRendering is one rendering member of a view, and the rendering it names.
type ViewRendering struct {
	// Member is the `render`/`rendering` member itself.
	Member *symbols.Symbol
	// Ref is the rendering as written, empty for a member naming none.
	Ref string
	// Rendering is what Ref resolves to, nil when it names nothing or does not
	// resolve.
	Rendering *symbols.Symbol
	// DeclaredIn is the view declaring the member: the view itself, or one it
	// specializes.
	DeclaredIn *symbols.Symbol
}

// ViewRenderings returns the rendering members of view: its own in declaration
// order, followed by those of the views it specializes, once each. A view
// stating no rendering returns none, which is no error; a non-view is
// ErrNotAView. An abstract member states no rendering — every view inherits the
// standard library's `abstract ref rendering viewRendering` — so it is left out.
func (m *Model) ViewRenderings(view *symbols.Symbol) ([]ViewRendering, error) {
	if view == nil || !IsView(view) {
		return nil, ErrNotAView
	}
	var out []ViewRendering
	seen := map[*symbols.Symbol]bool{}
	add := func(owner *symbols.Symbol) {
		for _, member := range usageMembersOfKind(owner, ast.UsageViewRendering, ast.UsageRendering) {
			if seen[member] || isAbstractUsage(member) {
				continue
			}
			seen[member] = true
			target, ref := m.RenderingTarget(member)
			out = append(out, ViewRendering{Member: member, Ref: ref, Rendering: target, DeclaredIn: owner})
		}
	}
	add(view)
	for _, super := range m.AllSupertypes(view) {
		if IsView(super) {
			add(super)
		}
	}
	return out, nil
}

// RenderingTarget returns the rendering a `render`/`rendering` member names and
// the reference as written: the rendering it references (`render asTreeDiagram;`)
// or the rendering definition it is typed by (`render rendering r : AsTree;`).
func (m *Model) RenderingTarget(member *symbols.Symbol) (*symbols.Symbol, string) {
	if member == nil {
		return nil, ""
	}
	for _, rel := range RelationshipsOf(member) {
		if rel == nil || rel.Target == nil {
			continue
		}
		switch rel.Kind {
		case ast.RelReferences, ast.RelTyping, ast.RelSubsets, ast.RelRedefines:
		default:
			continue
		}
		return m.relationshipTarget(member, rel), refName(rel.Target)
	}
	return nil, ""
}

// isAbstractUsage reports whether a usage was declared abstract.
func isAbstractUsage(sym *symbols.Symbol) bool {
	usage, ok := sym.Decl.(*ast.Usage)
	return ok && usage.IsAbstract
}
