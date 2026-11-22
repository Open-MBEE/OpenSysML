package symbols

import (
	"maps"
	"slices"
	"sort"
)

// readGraph maps each importer to the namespaces its re-exports were read from
// and the ones its imports last named, as the index records them.
func (idx *Index) readGraph() map[string][]string {
	graph := make(map[string][]string)
	add := func(importer, target string) {
		if target == "" || target == importer {
			return
		}
		if at, found := slices.BinarySearch(graph[importer], target); !found {
			graph[importer] = slices.Insert(graph[importer], at, target)
		}
	}
	for _, key := range idx.reexportDocs.keys() {
		importer, _ := splitFQN(key.fqn)
		for _, claim := range idx.reexportDocs.at(key) {
			for _, hop := range claim.hops {
				add(importer, hop.from)
			}
		}
	}
	for _, importer := range idx.lastTargets.keys() {
		for _, imp := range idx.lastTargets.at(importer) {
			add(importer, imp.fqn)
		}
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
