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
		idx.importsMoved(importer)
	}
}

// forgetReadFrom drops what the importer read by the imports doc stated, which
// goes with the re-exports doc claimed.
func (idx *Index) forgetReadFrom(importer, doc string) {
	if _, ok := idx.readFrom.at(importer)[doc]; !ok {
		return
	}
	idx.importsMoved(importer)
	byDoc := writableMap(idx.readFrom, importer)
	delete(byDoc, doc)
	if len(byDoc) == 0 {
		idx.readFrom.del(importer)
	}
}

// importsMoved notes that what importer read from or last named may have changed.
func (idx *Index) importsMoved(importer string) {
	idx.movedImporters = append(idx.movedImporters, importer)
}

// edgesOf lists the namespaces importer's re-exports read or its imports name.
func (idx *Index) edgesOf(importer string, imports []resolvedImport) []string {
	var edges []string
	byDoc := idx.readFrom.at(importer)
	for _, doc := range sortedKeys(byDoc) {
		for _, target := range byDoc[doc] {
			if target != importer && !slices.Contains(edges, target) {
				edges = append(edges, target)
			}
		}
	}
	for _, imp := range imports {
		if imp.fqn != "" && imp.fqn != importer && !slices.Contains(edges, imp.fqn) {
			edges = append(edges, imp.fqn)
		}
	}
	return edges
}

// importGraph maps each importer to the namespaces its wildcard imports name (as
// resolved, or as last expanded when resolved is nil) or its re-exports read.
func (idx *Index) importGraph(resolved []importerImports) map[string][]string {
	graph := make(map[string][]string, len(resolved))
	edges := func(importer string, imports []resolvedImport) {
		if targets := idx.edgesOf(importer, imports); len(targets) > 0 {
			graph[importer] = targets
		}
	}
	if resolved == nil {
		for _, importer := range idx.lastTargets.keys() {
			edges(importer, idx.lastTargets.at(importer))
		}
	}
	for _, stated := range resolved {
		edges(stated.importer, stated.imports)
	}
	for _, importer := range idx.readFrom.keys() {
		if _, done := graph[importer]; !done {
			edges(importer, nil)
		}
	}
	return graph
}

// recordedGraph returns the import graph last recorded when it is still the one
// the imports as resolved make: each importer reads as expanded, the moved too.
func (idx *Index) recordedGraph(resolved []importerImports) (map[string][]string, bool) {
	moved := idx.movedImporters
	idx.movedImporters = nil
	if idx.noted == nil {
		return nil, false
	}
	for _, stated := range resolved {
		last, expanded := idx.lastTargets.get(stated.importer)
		if !expanded {
			return nil, false
		}
		if same, _ := sameImports(last, stated.imports); !same {
			return nil, false
		}
	}
	for _, importer := range moved {
		if !slices.Equal(idx.edgesOf(importer, idx.lastTargets.at(importer)), idx.noted[importer]) {
			return nil, false
		}
	}
	return idx.noted, true
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

// noteLeafImports records graph when it differs from the one recorded only in
// imports gained by namespaces nothing imports from, which are on no cycle.
func (idx *Index) noteLeafImports(graph map[string][]string) bool {
	if idx.noted == nil || len(graph) < len(idx.noted) {
		return false
	}
	var moved []string
	kept := 0
	for importer, targets := range graph {
		old, had := idx.noted[importer]
		if had {
			kept++
		}
		if had && slices.Equal(old, targets) {
			continue
		}
		for _, target := range old {
			if !slices.Contains(targets, target) {
				return false // an import went: a cycle can have
			}
		}
		moved = append(moved, importer)
	}
	if kept != len(idx.noted) {
		return false
	}
	for _, importer := range moved {
		if idx.imported[importer] {
			return false
		}
		for _, other := range moved {
			if slices.Contains(graph[other], importer) {
				return false
			}
		}
	}
	if !idx.ownsOrder {
		idx.nsOrder, idx.imported, idx.ownsOrder = maps.Clone(idx.nsOrder), maps.Clone(idx.imported), true
	}
	for _, importer := range moved {
		level := 0
		for _, target := range graph[importer] {
			idx.imported[target] = true
			if _, known := idx.nsOrder[target]; !known {
				idx.nsOrder[target] = 0
			}
			level = max(level, idx.nsOrder[target])
		}
		idx.nsOrder[importer] = level + 1
	}
	idx.noted = graph
	return true
}

// noteImportCycles records graph's import cycles and returns the namespaces whose
// cycle changed. Unless settled a cycle only grows: see expandOnce.
func (idx *Index) noteImportCycles(graph map[string][]string, settled bool) []string {
	if (idx.notedExact || !settled) && maps.EqualFunc(graph, idx.noted, slices.Equal[[]string]) {
		return nil // as recorded
	}
	if (idx.notedExact || !settled) && idx.noteLeafImports(graph) {
		return nil
	}
	exact := true
	components := importComponents(graph)
	order := make(map[string]int, len(graph))
	imported := make(map[string]bool, len(graph))
	cycles := make(map[string][]string)
	for _, component := range components {
		level := 0 // one above every namespace the component reads from outside itself
		for _, ns := range component {
			for _, target := range graph[ns] {
				imported[target] = true
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
		}
		if len(component) < 2 {
			continue
		}
		cycle := component
		if !settled {
			for _, ns := range component {
				for _, other := range idx.cyclic.at(ns) {
					if _, found := slices.BinarySearch(cycle, other); !found {
						exact = false
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
	for _, ns := range idx.cyclic.keys() {
		if _, still := cycles[ns]; still {
			continue
		}
		if !settled {
			exact = false // kept until a round changes nothing
			continue
		}
		idx.cyclic.del(ns)
		changed = append(changed, ns)
	}
	for _, ns := range slices.Sorted(maps.Keys(cycles)) {
		if !slices.Equal(idx.cyclic.at(ns), cycles[ns]) {
			idx.cyclic.set(ns, cycles[ns])
			changed = append(changed, ns)
		}
	}
	idx.nsOrder, idx.imported, idx.ownsOrder = order, imported, true
	idx.noted, idx.notedExact = graph, exact
	return changed
}

// settleImportCycles records the cycles of graph and drops the re-exports of the
// namespaces whose cycle changed and of those reading through them, reporting any.
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
			idx.forgetExpansion(ns)
			dropped = true
		}
	}
	return dropped
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
