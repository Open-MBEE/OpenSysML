package view

import (
	"math/bits"
	"slices"
	"strconv"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// treeQuery shares equivalent occurrence subtrees for a drawn-set query, never for emitted output.
type treeQuery struct {
	*treeQueryGraph
	r           *Renderer
	view        *symbols.Symbol
	bound       int
	nodes       map[treeQueryKey]treeQueryNode
	memberCache map[*symbols.Symbol][]*symbols.Symbol
	geometry    map[treeGeometryKey]*Geometry
}

type treeQueryGraph struct {
	component map[*symbols.Symbol]int
	reachable [][]uint64
	ids       map[*symbols.Symbol]int
	keys      map[symbols.ElementKey][]int
}

func (g *treeQueryGraph) covers(roots []*symbols.Symbol) bool {
	if g == nil {
		return false
	}
	for _, root := range roots {
		if _, ok := g.component[root]; !ok {
			return false
		}
	}
	return true
}

type treeQueryKey struct {
	sym, owner *symbols.Symbol
	qualified  bool
	path       string
}

type treeQueryNode struct {
	node  *Node
	depth int
}

type treeGeometryKey struct{ sym, owner *symbols.Symbol }

func (q *treeQuery) geometryOf(sym, owner *symbols.Symbol, out *Rendering) *Geometry {
	key := treeGeometryKey{sym, owner}
	if geometry, ok := q.geometry[key]; ok {
		return geometry
	}
	geometry := q.r.inheritedGeometryOf(q.view, owner, sym, out)
	q.geometry[key] = geometry
	return geometry
}

func (r *Renderer) treeQuery(view *symbols.Symbol, out *Rendering, roots []*symbols.Symbol) *treeQuery {
	if !out.query {
		return nil
	}
	q := &treeQuery{r: r, view: view, bound: r.walkBound(view), nodes: map[treeQueryKey]treeQueryNode{}, memberCache: map[*symbols.Symbol][]*symbols.Symbol{}, geometry: map[treeGeometryKey]*Geometry{}}
	unfiltered := len(r.walkFilters(view)) == 0
	if unfiltered && r.queryGraph.covers(roots) {
		q.treeQueryGraph = r.queryGraph
		return q
	}
	q.treeQueryGraph = &treeQueryGraph{component: map[*symbols.Symbol]int{}, ids: map[*symbols.Symbol]int{}, keys: map[symbols.ElementKey][]int{}}
	q.prepare(roots)
	if unfiltered && (r.queryGraph == nil || len(q.component) > len(r.queryGraph.component)) {
		r.queryGraph = q.treeQueryGraph
	}
	return q
}

func (q *treeQuery) members(sym *symbols.Symbol) []*symbols.Symbol {
	if members, ok := q.memberCache[sym]; ok {
		return members
	}
	if q.r.queryMembers == nil {
		q.r.queryMembers = map[*symbols.Symbol][]*symbols.Symbol{}
	}
	members, ok := q.r.queryMembers[sym]
	if !ok {
		members = q.r.effectiveMembers(sym)
		q.r.queryMembers[sym] = members
	}
	if filters := q.r.walkFilters(q.view); len(filters) > 0 {
		var kept []*symbols.Symbol
		for _, member := range members {
			if q.r.admits(filters, member) {
				kept = append(kept, member)
			}
		}
		members = kept
	}
	q.memberCache[sym] = members
	return members
}

// prepare condenses member cycles and computes reachable expansion tokens as
// bit sets. Types are tokens enter tests, not additional member walks.
func (q *treeQuery) prepare(roots []*symbols.Symbol) {
	order, low := map[*symbols.Symbol]int{}, map[*symbols.Symbol]int{}
	active := map[*symbols.Symbol]bool{}
	var stack []*symbols.Symbol
	var components [][]*symbols.Symbol
	var visit func(*symbols.Symbol)
	visit = func(sym *symbols.Symbol) {
		order[sym] = len(order) + 1
		low[sym] = order[sym]
		stack = append(stack, sym)
		active[sym] = true
		for _, member := range q.members(sym) {
			if order[member] == 0 {
				visit(member)
				low[sym] = min(low[sym], low[member])
			} else if active[member] {
				low[sym] = min(low[sym], order[member])
			}
		}
		if low[sym] != order[sym] {
			return
		}
		var component []*symbols.Symbol
		for {
			last := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			active[last] = false
			q.component[last] = len(components)
			component = append(component, last)
			if last == sym {
				break
			}
		}
		components = append(components, component)
	}
	for _, root := range roots {
		if order[root] == 0 {
			visit(root)
		}
	}
	types := map[*symbols.Symbol][]*symbols.Symbol{}
	for sym := range order {
		types[sym] = q.r.drawnTypes(sym)
		for _, token := range q.tokens(sym, types[sym]) {
			if _, ok := q.ids[token]; !ok {
				q.ids[token] = len(q.ids)
				key := symbols.KeyOf(token)
				q.keys[key] = append(q.keys[key], q.ids[token])
			}
		}
	}
	q.reachable = make([][]uint64, len(components))
	for component, syms := range components {
		bits := make([]uint64, (len(q.ids)+63)/64)
		for _, sym := range syms {
			for _, token := range q.tokens(sym, types[sym]) {
				id := q.ids[token]
				bits[id/64] |= 1 << (id % 64)
			}
			for _, member := range q.members(sym) {
				if child := q.component[member]; child != component {
					for word, value := range q.reachable[child] {
						bits[word] |= value
					}
				}
			}
		}
		q.reachable[component] = bits
	}
}

func (q *treeQuery) tokens(sym *symbols.Symbol, types []*symbols.Symbol) []*symbols.Symbol {
	return append(append([]*symbols.Symbol{sym}, types...), q.r.annotatingMembers(sym)...)
}

// exposedDescendants uses reachability only to reject impossible containers;
// the original guarded walk and fixpoint still decide every suppression.
func (q *treeQuery) exposedDescendants(exposed []*symbols.Symbol, annotations bool) map[symbols.ElementKey]bool {
	if len(exposed) < 2 {
		return nil
	}
	targets := map[int][]int{}
	for i, sym := range exposed {
		for _, id := range q.keys[symbols.KeyOf(sym)] {
			targets[id] = append(targets[id], i)
		}
	}
	containers := make([][]int, len(exposed))
	for j, sym := range exposed {
		seen := map[int]bool{}
		add := func(id int) {
			for _, i := range targets[id] {
				if i != j && !seen[i] {
					containers[i] = append(containers[i], j)
					seen[i] = true
				}
			}
		}
		reached := q.reachable[q.component[sym]]
		if len(targets) < len(reached) {
			for id := range targets {
				if reached[id/64]&(1<<(id%64)) != 0 {
					add(id)
				}
			}
			continue
		}
		for word, value := range reached {
			for value != 0 {
				id := word*64 + bits.TrailingZeros64(value)
				value &= value - 1
				add(id)
			}
		}
	}
	by := make([]map[symbols.ElementKey]bool, len(exposed))
	members := func(sym *symbols.Symbol) []*symbols.Symbol {
		if annotations {
			return append(slices.Clone(q.members(sym)), q.r.annotatingMembers(sym)...)
		}
		return q.members(sym)
	}
	suppressed := map[symbols.ElementKey]bool{}
	for range exposed {
		changed := false
		for i, elem := range exposed {
			key := symbols.KeyOf(elem)
			contained := false
			for _, j := range containers[i] {
				if suppressed[symbols.KeyOf(exposed[j])] {
					continue
				}
				if by[j] == nil {
					by[j] = q.r.treeDescendants(q.view, exposed[j], members)
				}
				if by[j][key] {
					contained = true
					break
				}
			}
			if contained != suppressed[key] {
				changed = true
				if contained {
					suppressed[key] = true
				} else {
					delete(suppressed, key)
				}
			}
		}
		if !changed {
			break
		}
	}
	return suppressed
}

// key retains exactly the ancestors reachable below sym; unrelated siblings cannot affect its walk.
func (q *treeQuery) key(sym, owner *symbols.Symbol, path expansion, qualified bool) treeQueryKey {
	var active []int
	reached := q.reachable[q.component[sym]]
	for ancestor := range path {
		id, ok := q.ids[ancestor]
		if !ok || reached[id/64]&(1<<(id%64)) == 0 {
			continue
		}
		active = append(active, id)
	}
	slices.Sort(active)
	var signature strings.Builder
	for _, id := range active {
		signature.WriteString(strconv.Itoa(id))
		signature.WriteByte(',')
	}
	return treeQueryKey{sym: sym, owner: owner, qualified: qualified, path: signature.String()}
}

// node reuses a completed walk with the same cycle barriers and more depth:
// it has already recorded everything this occurrence can draw.
func (q *treeQuery) node(key treeQueryKey, depth int) *Node {
	if previous, ok := q.nodes[key]; ok && previous.depth <= depth {
		return previous.node
	}
	return nil
}

func (q *treeQuery) remember(key treeQueryKey, depth int, node *Node) {
	if previous, ok := q.nodes[key]; !ok || depth < previous.depth {
		q.nodes[key] = treeQueryNode{node: node, depth: depth}
	}
}
