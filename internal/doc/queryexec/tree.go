package queryexec

import (
	"github.com/Open-MBEE/OpenSysML/internal/ir/queryplan"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// treeNode is one candidate row of a Tree: a source row, or an ancestor row
// kept only while a source row nests under it.
type treeNode struct {
	value    Value
	cells    []Cell
	source   bool
	parent   int
	children []int
}

// evaluateTree arranges the source rows as a containment tree in pre-order:
// each row nests under the nearest row containing it — the nearest owner
// among the rows, or the individual whose part is typed by it — and rows
// nobody contains are top-level. Rows of `ancestors` join the tree as
// intermediate levels where a source row nests under them.
func (e *executor) evaluateTree(expression queryplan.Expression) (sequence, error) {
	source, err := e.ownershipArgument(expression, "source")
	if err != nil {
		return sequence{}, err
	}
	var ancestors sequence
	if hasArgument(expression, "ancestors") {
		if ancestors, err = e.ownershipArgument(expression, "ancestors"); err != nil {
			return sequence{}, err
		}
	}
	nodes := make([]*treeNode, 0, len(source.values)+len(ancestors.values))
	index := make(map[rowKey]int, len(source.values)+len(ancestors.values))
	for i, value := range source.values {
		key := keyOfRow(value)
		if _, duplicate := index[key]; duplicate {
			continue
		}
		index[key] = len(nodes)
		node := &treeNode{value: value, source: true, parent: -1}
		if i < len(source.cells) {
			node.cells = cloneCells(source.cells[i])
		}
		nodes = append(nodes, node)
	}
	for _, value := range ancestors.values {
		key := keyOfRow(value)
		if _, present := index[key]; present {
			continue
		}
		index[key] = len(nodes)
		nodes = append(nodes, &treeNode{value: value, parent: -1, cells: emptyCells(len(source.columns))})
	}
	typed, err := e.typedNesting(expression, nodes, index)
	if err != nil {
		return sequence{}, err
	}
	for i, node := range nodes {
		parent := e.owningNode(node.value, index)
		if parent < 0 {
			parent = typed[i]
		}
		if parent >= 0 && parent != i && !reaches(nodes, parent, i) {
			node.parent = parent
			nodes[parent].children = append(nodes[parent].children, i)
		}
	}
	result := sequence{columns: append([]Column(nil), source.columns...)}
	if len(source.cells) > 0 {
		result.cells = make([][]Cell, 0, len(nodes))
	}
	result.depths = make([]int64, 0, len(nodes))
	var walk func(i int, depth int64)
	walk = func(i int, depth int64) {
		node := nodes[i]
		if !node.source && !holdsSource(nodes, i) {
			return
		}
		result.values = append(result.values, node.value)
		result.depths = append(result.depths, depth)
		if result.cells != nil {
			result.cells = append(result.cells, node.cells)
		}
		for _, child := range node.children {
			walk(child, depth+1)
		}
	}
	for i, node := range nodes {
		if node.parent < 0 {
			walk(i, 0)
		}
	}
	return result, nil
}

// owningNode is the nearest owner of a row among the tree's candidates, or -1.
func (e *executor) owningNode(row Value, index map[rowKey]int) int {
	seen := make(map[rowKey]struct{})
	for {
		owner, ok := e.ownerRow(row)
		if !ok {
			return -1
		}
		key := keyOfRow(owner)
		if _, cycle := seen[key]; cycle {
			return -1
		}
		seen[key] = struct{}{}
		if i, ok := index[key]; ok {
			return i
		}
		row = owner
	}
}

// typedNesting is, per candidate, the candidate whose individual part is typed
// by it (through individuals that are no candidates themselves), or -1: the
// nesting of an individual's parts, whose definitions need not own one another.
func (e *executor) typedNesting(
	expression queryplan.Expression,
	nodes []*treeNode,
	index map[rowKey]int,
) ([]int, error) {
	parents := make([]int, len(nodes))
	for i := range parents {
		parents[i] = -1
	}
	visited := make(map[rowKey]struct{})
	for i, node := range nodes {
		sym, ok := node.value.Element()
		if !ok || !individualDefinition(sym) {
			continue
		}
		queue := []*symbols.Symbol{sym}
		for len(queue) > 0 {
			next := queue[0]
			queue = queue[1:]
			for _, part := range e.individualParts(next) {
				key := keyOfRow(ElementValue(part))
				if j, candidate := index[key]; candidate {
					if parents[j] < 0 && j != i {
						parents[j] = i
					}
					continue
				}
				if _, seen := visited[key]; seen {
					continue
				}
				if !e.consumeVisit() {
					return nil, e.budgetError(expression)
				}
				visited[key] = struct{}{}
				queue = append(queue, part)
			}
		}
	}
	return parents, nil
}

// individualParts are the individual definitions typing the features an
// individual definition declares: the individuals its parts are.
func (e *executor) individualParts(sym *symbols.Symbol) []*symbols.Symbol {
	if sym.Scope == nil {
		return nil
	}
	var out []*symbols.Symbol
	for _, member := range sym.Scope.AllMembers() {
		if !member.IsFeature() {
			continue
		}
		for _, typ := range e.context.Model.FeatureTypeSet(member) {
			if typ != sym && individualDefinition(typ) {
				out = append(out, typ)
			}
		}
	}
	return out
}

// individualDefinition reports whether a symbol declares an individual
// definition (`individual part def`).
func individualDefinition(sym *symbols.Symbol) bool {
	if sym == nil {
		return false
	}
	definition, ok := sym.Decl.(*ast.Definition)
	return ok && definition.IsIndividual
}

// reaches reports whether node from is at or under node to, so linking to
// under from would close a cycle.
func reaches(nodes []*treeNode, from, to int) bool {
	for i := from; i >= 0; i = nodes[i].parent {
		if i == to {
			return true
		}
	}
	return false
}

// holdsSource reports whether a source row nests anywhere under node i.
func holdsSource(nodes []*treeNode, i int) bool {
	for _, child := range nodes[i].children {
		if nodes[child].source || holdsSource(nodes, child) {
			return true
		}
	}
	return false
}

// emptyCells are the cells of a row the projection never saw.
func emptyCells(columns int) []Cell {
	if columns == 0 {
		return nil
	}
	return make([]Cell, columns)
}
