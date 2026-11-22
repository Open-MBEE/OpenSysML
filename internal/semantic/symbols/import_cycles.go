package symbols

import (
	"maps"
	"slices"
	"sort"
)

// noteReadFrom records that the importer's re-exports, by an import doc
// states, read the namespace target.
func (idx *Index) noteReadFrom(importer, doc, target string) {
	have := idx.readFrom.at(importer)[doc]
	if at, found := slices.BinarySearch(have, target); !found {
		writableMap(idx.readFrom, importer)[doc] = slices.Insert(slices.Clone(have), at, target)
	}
}

// forgetReadFrom drops what the importer read by the imports doc stated, which
// goes with the re-exports doc claimed.
func (idx *Index) forgetReadFrom(importer, doc string) {
	if _, ok := idx.readFrom.at(importer)[doc]; !ok {
		return
	}
	byDoc := writableMap(idx.readFrom, importer)
	delete(byDoc, doc)
	if len(byDoc) == 0 {
		idx.readFrom.del(importer)
	}
}

// importGraph maps each importing namespace to the namespaces its wildcard
// imports name, as last expanded when settled and else as they resolve now,
// and to the ones its re-exports were read from.
func (idx *Index) importGraph(settled bool) map[string][]string {
	graph := make(map[string][]string)
	for _, importer := range idx.readFrom.keys() {
		byDoc := idx.readFrom.at(importer)
		for _, doc := range slices.Sorted(maps.Keys(byDoc)) {
			for _, target := range byDoc[doc] {
				if target != importer && !slices.Contains(graph[importer], target) {
					graph[importer] = append(graph[importer], target)
				}
			}
		}
	}
	edges := func(importer string, imports []resolvedImport) {
		for _, imp := range imports {
			if imp.fqn != "" && imp.fqn != importer && !slices.Contains(graph[importer], imp.fqn) {
				graph[importer] = append(graph[importer], imp.fqn)
			}
		}
	}
	if settled {
		for _, importer := range idx.lastTargets.keys() {
			edges(importer, idx.lastTargets.at(importer))
		}
		return graph
	}
	for _, importer := range idx.wildcardMeta.keys() {
		edges(importer, idx.resolveImports(importer))
	}
	return graph
}

// importComponents returns the strongly connected components of graph, each
// before every component that imports from it (Tarjan), in name order within.
func importComponents(graph map[string][]string) [][]string {
	nodes := make([]string, 0, len(graph))
	for importer := range graph {
		nodes = append(nodes, importer)
	}
	sort.Strings(nodes)

	type frame struct {
		node string
		next int
	}
	index := make(map[string]int, len(nodes))
	low := make(map[string]int, len(nodes))
	onStack := make(map[string]bool)
	var stack []string
	var out [][]string
	for _, root := range nodes {
		if _, seen := index[root]; seen {
			continue
		}
		work := []frame{{node: root}}
		index[root], low[root] = len(index), len(index)
		stack, onStack[root] = append(stack, root), true
		for len(work) > 0 {
			top := &work[len(work)-1]
			if targets := graph[top.node]; top.next < len(targets) {
				target := targets[top.next]
				top.next++
				if _, seen := index[target]; !seen {
					index[target], low[target] = len(index), len(index)
					stack, onStack[target] = append(stack, target), true
					work = append(work, frame{node: target})
				} else if onStack[target] {
					low[top.node] = min(low[top.node], index[target])
				}
				continue
			}
			node := top.node
			work = work[:len(work)-1]
			if len(work) > 0 {
				parent := work[len(work)-1].node
				low[parent] = min(low[parent], low[node])
			}
			if low[node] != index[node] {
				continue
			}
			var component []string
			for {
				member := stack[len(stack)-1]
				stack = stack[:len(stack)-1]
				onStack[member] = false
				component = append(component, member)
				if member == node {
					break
				}
			}
			sort.Strings(component)
			out = append(out, component)
		}
	}
	return out
}

// noteImportCycles records the import cycles of graph and where each namespace
// stands in it, and returns the namespaces whose cycle is not the one they had:
// a route keeps the namespaces on a cycle with the one holding it.
//
// Unless settled, a cycle only gains namespaces: while re-exports dropped are
// still being derived again, an import naming a namespace by one of them does
// not resolve, which is not the cycle through it having broken.
func (idx *Index) noteImportCycles(graph map[string][]string, settled bool) []string {
	components := importComponents(graph)
	order := make(map[string]int, len(graph))
	cycles := make(map[string][]string)
	for i, component := range components {
		for _, ns := range component {
			order[ns] = i
		}
		if len(component) < 2 {
			continue
		}
		cycle := component
		if !settled {
			for _, ns := range component {
				for _, other := range idx.cyclic.at(ns) {
					if _, found := slices.BinarySearch(cycle, other); !found {
						cycle = append(slices.Clone(cycle), other)
						sort.Strings(cycle)
					}
				}
			}
		}
		for _, ns := range cycle {
			cycles[ns] = cycle
		}
	}
	var changed []string
	if settled {
		for _, ns := range idx.cyclic.keys() {
			if _, still := cycles[ns]; !still {
				idx.cyclic.del(ns)
				changed = append(changed, ns)
			}
		}
	}
	for _, ns := range slices.Sorted(maps.Keys(cycles)) {
		if !slices.Equal(idx.cyclic.at(ns), cycles[ns]) {
			idx.cyclic.set(ns, cycles[ns])
			changed = append(changed, ns)
		}
	}
	idx.nsOrder = order
	return changed
}

// onCycleWith reports whether a and b are on one cycle of wildcard imports, so
// that reading either can come back to the other.
func (idx *Index) onCycleWith(a, b string) bool {
	_, found := slices.BinarySearch(idx.cyclic.at(a), b)
	return found
}

// settleImportCycles brings the recorded cycles up to date with the imports as
// they resolve, and reports whether it dropped re-exports to be derived again.
// With settled set, no re-export is left to derive, so the imports resolve as
// they will and a cycle they no longer make is forgotten.
//
// A route names the namespaces it came through that were on a cycle with its
// holder when it was read, so where a cycle formed, grew or broke, the routes
// of its namespaces and of every namespace reading through them go.
func (idx *Index) settleImportCycles(graph map[string][]string, settled bool) bool {
	changed := idx.noteImportCycles(graph, settled)
	if len(changed) == 0 {
		return false
	}
	importers := make(map[string][]string)
	for importer, targets := range graph {
		for _, target := range targets {
			importers[target] = append(importers[target], importer)
		}
	}
	stale := make(map[string]bool)
	for work := changed; len(work) > 0; {
		ns := work[len(work)-1]
		work = work[:len(work)-1]
		if stale[ns] {
			continue
		}
		stale[ns] = true
		work = append(work, importers[ns]...)
	}
	dropped := false
	for _, ns := range slices.Sorted(maps.Keys(stale)) {
		if _, expanded := idx.lastTargets.get(ns); expanded {
			idx.purgeReexportsUnder(ns)
			idx.lastTargets.del(ns)
			dropped = true
		}
	}
	return dropped
}

// mayReach reports whether reading the namespace from can come to one of the
// namespaces named: none it can reach stands after it in the import order.
func (idx *Index) mayReach(from string, namespaces []string) bool {
	at, ok := idx.nsOrder[from]
	if !ok {
		return false // it imports nothing
	}
	for _, ns := range namespaces {
		if other, ok := idx.nsOrder[ns]; ok && other <= at && ns != from {
			return true
		}
	}
	return false
}
