package symbols

import (
	"maps"
	"slices"
	"sort"
)

// importGraph maps each importer to the namespaces it reads from, in name order.
type importGraph map[string][]string

func (g importGraph) add(importer, target string) {
	if target == "" || target == importer {
		return
	}
	if at, found := slices.BinarySearch(g[importer], target); !found {
		g[importer] = slices.Insert(g[importer], at, target)
	}
}

// addHops adds the namespaces a re-export into importer was read from.
func (g importGraph) addHops(importer string, hops []importHop) {
	for i := range hops {
		g.add(importer, hops[i].from)
	}
}

// addLastTargets adds the namespaces the index's imports last named.
func (g importGraph) addLastTargets(idx *Index) {
	for _, importer := range idx.lastTargets.keys() {
		for _, imp := range idx.lastTargets.at(importer) {
			g.add(importer, imp.fqn)
		}
	}
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

// noteImportCycles records graph, its import cycles and the order of its
// namespaces, and returns the namespaces whose cycle changed, in name order.
func (idx *Index) noteImportCycles(graph map[string][]string) []string {
	order := make(map[string]int, len(graph))
	cycles := make(map[string][]string)
	for _, component := range importComponents(graph) {
		level := 0 // one above every namespace the component reads from outside itself
		for _, ns := range component {
			for _, target := range graph[ns] {
				if _, within := slices.BinarySearch(component, target); !within {
					level = max(level, order[target])
				}
			}
		}
		if len(component) > 1 || len(graph[component[0]]) > 0 {
			level++
		}
		for _, ns := range component {
			order[ns] = level
			if len(component) > 1 {
				cycles[ns] = component
			}
		}
	}
	var changed []string
	for _, ns := range idx.cyclic.keys() {
		if _, still := cycles[ns]; !still {
			idx.cyclic.del(ns)
			changed = append(changed, ns)
		}
	}
	for ns, cycle := range cycles {
		if !slices.Equal(idx.cyclic.at(ns), cycle) {
			idx.cyclic.set(ns, cycle)
			changed = append(changed, ns)
		}
	}
	sort.Strings(changed)
	readers := make(map[string][]string)
	for _, importer := range slices.Sorted(maps.Keys(graph)) {
		for _, target := range graph[importer] {
			readers[target] = append(readers[target], importer)
		}
	}
	idx.nsOrder, idx.noted, idx.readers = order, graph, readers
	return changed
}

// noteLeafReads records what the stale importers read when none of them is read
// from or on a cycle: no cycle can pass through them, so the others stand.
func (idx *Index) noteLeafReads(stale []string, reads map[string][]string) bool {
	for _, importer := range stale {
		if len(idx.readers[importer]) > 0 || len(idx.cyclic.at(importer)) > 0 {
			return false
		}
	}
	for _, targets := range reads {
		for _, target := range targets {
			if _, leaf := slices.BinarySearch(stale, target); leaf {
				return false
			}
		}
	}
	graph, readers, order := maps.Clone(idx.noted), maps.Clone(idx.readers), maps.Clone(idx.nsOrder)
	if graph == nil {
		graph, readers, order = make(map[string][]string), make(map[string][]string), make(map[string]int)
	}
	for _, importer := range stale {
		for _, target := range graph[importer] {
			kept := slices.DeleteFunc(slices.Clone(readers[target]), func(r string) bool { return r == importer })
			if len(kept) == 0 {
				delete(readers, target)
				// A namespace neither reading nor read from is not in the graph.
				if len(graph[target]) == 0 {
					delete(order, target)
				}
			} else {
				readers[target] = kept
			}
		}
		delete(graph, importer)
		delete(order, importer)
	}
	for _, importer := range stale {
		targets := reads[importer]
		if len(targets) == 0 {
			continue
		}
		graph[importer] = targets
		level := 0
		for _, target := range targets {
			if _, known := order[target]; !known {
				order[target] = 0 // a namespace read from is in the graph
			}
			level = max(level, order[target])
			at, _ := slices.BinarySearch(readers[target], importer)
			readers[target] = slices.Insert(slices.Clone(readers[target]), at, importer)
		}
		order[importer] = level + 1
	}
	idx.nsOrder, idx.noted, idx.readers = order, graph, readers
	return true
}

// mayReach reports whether reading the namespace from can come to one of the
// namespaces named: none it can reach stands above it in the import order.
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

// readBy lists, in order, the namespaces importer's re-exports were read from
// and the ones its imports last named.
func (idx *Index) readBy(importer string) []string {
	var targets []string
	add := func(target string) {
		if target == "" || target == importer {
			return
		}
		if at, found := slices.BinarySearch(targets, target); !found {
			targets = slices.Insert(targets, at, target)
		}
	}
	for _, fqn := range idx.childKeys(importer) {
		for _, sym := range idx.reexportedAt(fqn) {
			for _, claim := range idx.reexportDocs.at(reexportKey{fqn: fqn, sym: sym}) {
				for _, hop := range claim.hops {
					add(hop.from)
				}
			}
		}
	}
	for _, imp := range idx.lastTargets.at(importer) {
		add(imp.fqn)
	}
	return targets
}

// readSince lists what importer reads, from what it read since the cycles were
// recorded: with what it read before unless that was dropped, or all of it anew.
func (idx *Index) readSince(importer string, reads *expansionReads) []string {
	if reads.anew {
		return idx.readBy(importer)
	}
	targets := reads.read
	if !reads.dropped {
		targets = append(targets, idx.noted[importer]...)
	}
	for _, imp := range idx.lastTargets.at(importer) {
		if imp.fqn != "" && imp.fqn != importer {
			targets = append(targets, imp.fqn)
		}
	}
	slices.Sort(targets)
	return slices.Compact(targets)
}

// settleImportCycles records what the importers expanded since it last ran read,
// and reports whether it dropped re-exports whose cycle moved, to derive again.
func (idx *Index) settleImportCycles() bool {
	expanded := idx.expanded
	idx.expanded = nil
	reads := make(map[string][]string)
	var moved []string
	for importer, since := range expanded {
		if targets := idx.readSince(importer, since); !slices.Equal(targets, idx.noted[importer]) {
			reads[importer] = targets
			moved = append(moved, importer)
		}
	}
	if len(moved) == 0 {
		return false
	}
	sort.Strings(moved)
	if idx.noteLeafReads(moved, reads) {
		return false
	}
	graph := make(map[string][]string, len(idx.noted)+len(reads))
	for importer, targets := range idx.noted {
		if _, again := slices.BinarySearch(moved, importer); !again {
			graph[importer] = targets
		}
	}
	for importer, targets := range reads {
		if len(targets) > 0 {
			graph[importer] = targets
		}
	}
	rerouted := idx.noteImportCycles(graph)
	if len(rerouted) == 0 {
		return false
	}
	// A route continues from the ones it was read from, so their readers go too.
	stale := make(map[string]bool, len(rerouted))
	work := slices.Clone(rerouted)
	for _, ns := range rerouted {
		stale[ns] = true
	}
	for len(work) > 0 {
		ns := work[len(work)-1]
		work = work[:len(work)-1]
		for _, reader := range idx.readers[ns] {
			if !stale[reader] {
				stale[reader] = true
				work = append(work, reader)
			}
		}
	}
	for _, importer := range slices.Sorted(maps.Keys(stale)) {
		idx.purgeReexportsUnder(importer)
		idx.lastTargets.del(importer)
	}
	return true
}
