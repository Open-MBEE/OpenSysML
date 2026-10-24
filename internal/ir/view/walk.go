package view

import (
	"fmt"
	"strconv"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// What a tree or interconnection rendering walks below each exposed element is
// bounded three ways, all applied by the one walk: the view's filter conditions
// prune the members they reject (and their subtrees), the view's depth — or the
// caller's, which overrides it — cuts the walk so many levels below each root,
// and a collapsed Layout folds an element's children away.

// SetDepth overrides the depth every view states for the renders that follow:
// each exposed element is drawn with depth levels of nested elements below it.
// 0 restores the model's say.
func (r *Renderer) SetDepth(depth int) {
	if depth < 0 {
		depth = 0
	}
	r.depth = depth
}

// walkBound is how many levels below an exposed root a walk of view draws: the
// caller's depth, else the view's, each capped by the cycle guard treeDepth.
func (r *Renderer) walkBound(view *symbols.Symbol) int {
	bound := r.treeDepth()
	if depth := r.statedDepth(view); depth > 0 && depth < bound {
		return depth
	}
	return bound
}

// statedDepth is the depth the caller or the view states, 0 for neither.
func (r *Renderer) statedDepth(view *symbols.Symbol) int {
	if r.depth > 0 {
		return r.depth
	}
	if view == nil {
		return 0
	}
	return r.model.ViewDepth(view)
}

// walkMembers is containedMembers less the members view's filter conditions
// reject: the conditions that admit what a view exposes prune its walk too, so
// the roots and what is drawn beneath them cannot disagree. A view stating no
// condition walks every member.
func (r *Renderer) walkMembers(view, sym *symbols.Symbol) []*symbols.Symbol {
	members := r.effectiveMembers(sym)
	filters := r.walkFilters(view)
	if len(filters) == 0 {
		return members
	}
	kept := members[:0:0]
	for _, member := range members {
		if r.admits(filters, member) {
			kept = append(kept, member)
		}
	}
	return kept
}

// walkFilters is the filter conditions governing view's walk, read once per view.
func (r *Renderer) walkFilters(view *symbols.Symbol) []symbols.ElementFilter {
	if view == nil {
		return nil
	}
	if filters, ok := r.filters[view]; ok {
		return filters
	}
	filters := r.model.ViewExposureConditions(view)
	if r.filters == nil {
		r.filters = map[*symbols.Symbol][]symbols.ElementFilter{}
	}
	r.filters[view] = filters
	return filters
}

// admits reports whether every filter admits sym, through the admission the
// exposed set itself passes.
func (r *Renderer) admits(filters []symbols.ElementFilter, sym *symbols.Symbol) bool {
	for _, filter := range filters {
		if !r.model.SatisfiesElementFilter(filter, sym) {
			return false
		}
	}
	return true
}

// collapsedIn reports whether a Layout in view folds sym's children away — what
// collapsed reads off the drawn node, for a walk that draws nothing.
func (r *Renderer) collapsedIn(view, sym *symbols.Symbol) bool {
	site, ok := r.model.LayoutOf(view, sym)
	return ok && site.Layout != nil && site.Layout.Collapsed
}

// collapsed reports whether the Layout drawn for node folds its children away.
func collapsed(node *Node) bool {
	return node != nil && node.Geometry != nil && node.Geometry.Collapsed
}

// cutNote is the detail a node at the depth bound carries when members lie below it.
func cutNote(bound int) string {
	return fmt.Sprintf("nested deeper than %d %s; not shown", bound, plural(bound, "level", "levels"))
}

// noteDepthCuts reports, once, how many elements a stated depth kept the walk
// from drawing; the cycle guard alone reports nothing, as before.
func (r *Renderer) noteDepthCuts(view *symbols.Symbol, out *Rendering) {
	if out.depthCut == 0 || r.statedDepth(view) == 0 {
		return
	}
	bound := r.walkBound(view)
	out.Notices = append(out.Notices, fmt.Sprintf("%d %s nested deeper than %d %s not shown",
		out.depthCut, plural(out.depthCut, "element", "elements"), bound, plural(bound, "level", "levels")))
}

// DepthError is a depth option naming no non-negative integer.
type DepthError struct {
	Value string
}

func (e *DepthError) Error() string {
	return fmt.Sprintf("depth %q is no non-negative integer; a depth is how many levels of nested elements to draw below each exposed element, 0 for every level", e.Value)
}

// ParseDepth reads a depth option: a non-negative integer, 0 for every level.
func ParseDepth(value string) (int, error) {
	depth, err := strconv.Atoi(value)
	if err != nil || depth < 0 {
		return 0, &DepthError{Value: value}
	}
	return depth, nil
}

// foldCollapsed folds the collapsed nodes under root as the tree and
// interconnection walks fold theirs: what a collapsed node nests is not drawn,
// its ports stay, and an edge at a node no longer drawn goes with it.
func foldCollapsed(root *Node, out *Rendering) {
	pruned := map[string]bool{}
	var prune func(*Node)
	prune = func(node *Node) {
		pruned[node.ID] = true
		for _, child := range node.Children {
			prune(child)
		}
	}
	var walk func(*Node)
	walk = func(node *Node) {
		if collapsed(node) {
			for _, child := range node.Children {
				prune(child)
			}
			node.Children = nil
			return
		}
		for _, child := range node.Children {
			walk(child)
		}
	}
	walk(root)
	if len(pruned) == 0 {
		return
	}
	kept := out.Edges[:0]
	for _, edge := range out.Edges {
		if !pruned[edge.From] && !pruned[edge.To] {
			kept = append(kept, edge)
		}
	}
	out.Edges = kept
}
