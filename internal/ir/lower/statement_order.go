package lower

import "github.com/Open-MBEE/OpenSysML/internal/syntax/ast"

// The direct statements of an action body are subactions no succession orders
// (Actions.sysml `assignments`, `sendSubactions`, `ifSubactions`, `loops`), so the
// library admits every order of them. StatementOrder says which orders differ.

// StatementOrder is how the statements of one list may run: in runs separated by
// the statements that keep their place, each run's in any order, of which only
// those reordering two dependent statements differ.
type StatementOrder struct {
	fixed []bool
	// dependent holds for two statements that may not commute; shared, for two
	// that both may touch what another performance does.
	dependent, shared [][]bool
}

// BodyStatementOrder is the order of stmts, statements of node's body in graph
// or of a block within it.
func BodyStatementOrder(graph *ActionGraph, node ast.Node, stmts []Statement) *StatementOrder {
	declared := declaredFeatures(graph)
	n := len(stmts)
	o := &StatementOrder{fixed: make([]bool, n), dependent: make([][]bool, n), shared: make([][]bool, n)}
	footprints := make([]Footprint, n)
	chained := chainsStatements(graph, node, stmts)
	for i, stmt := range stmts {
		o.fixed[i] = chained || keepsPlace(stmt)
		b := &footprintBuilder{graph: graph, node: node, scope: nodeScopeOf(graph, node), declared: declared}
		b.statement(stmt)
		footprints[i] = b.footprint
		o.dependent[i], o.shared[i] = make([]bool, n), make([]bool, n)
	}
	for i := range stmts {
		for j := range i {
			dep := footprints[i].Dependent(footprints[j])
			shared := touchesShared(footprints[i]) && touchesShared(footprints[j])
			o.dependent[i][j], o.dependent[j][i] = dep, dep
			o.shared[i][j], o.shared[j][i] = shared, shared
		}
	}
	return o
}

// chainsStatements reports whether stmts is the body of an action written as one
// node, whose every statement after the first is a `then` continuation.
func chainsStatements(graph *ActionGraph, node ast.Node, stmts []Statement) bool {
	u, ok := node.(*ast.Usage)
	if !ok || !u.IsActionNode || graph == nil {
		return false
	}
	body := graph.Bodies[node]
	return len(body) > 0 && len(stmts) > 0 && &body[0] == &stmts[0]
}

// keepsPlace reports whether a statement keeps its place among the others: a
// declaration the statements after it read by name, a `return`, a reference
// performed (`perform`, as no unordered start either), or one the runtime refuses.
func keepsPlace(stmt Statement) bool {
	switch s := stmt.(type) {
	case Declare, DeclareUsage, Return, Unsupported:
		return true
	case Effect:
		return s.Kind == EffectPerform || s.Kind == EffectStart
	}
	return false
}

// Len is how many statements the order is over.
func (o *StatementOrder) Len() int { return len(o.fixed) }

// Reorders reports whether two of the statements may run in either order with
// different results: two dependent ones in one run, or with divided set (another
// performance interleaving between statements), two that both touch what it does.
func (o *StatementOrder) Reorders(divided bool) bool {
	for i := range o.fixed {
		for j := i + 1; j < len(o.fixed) && !o.fixed[j]; j++ {
			if !o.fixed[i] && o.dependentPair(i, j, divided) {
				return true
			}
		}
	}
	return false
}

func (o *StatementOrder) dependentPair(i, j int, divided bool) bool {
	return o.dependent[i][j] || divided && o.shared[i][j]
}

// run is the statements not done in the run the first of them is in: that one
// alone where it keeps its place.
func (o *StatementOrder) run(done []bool) []int {
	var out []int
	for i, d := range done {
		if d {
			continue
		}
		if o.fixed[i] {
			if len(out) == 0 {
				out = append(out, i)
			}
			return out
		}
		out = append(out, i)
	}
	return out
}

// Next lists, ascending, the statements that may run next after those done, so
// that the statements run in the least order, by position, of every class of
// orders equal up to swapping independent neighbours: each class is reached once.
// A statement blocked waits for one dependent on it to run first. Empty once all are done.
func (o *StatementOrder) Next(done, blocked []bool, divided bool) []int {
	remaining := o.run(done)
	if len(remaining) < 2 {
		return remaining
	}
	component := o.components(remaining, divided)
	var next []int
	for _, s := range remaining {
		if blocked[s] {
			continue
		}
		// Each statement that must not be a source of the class's order — one before
		// s, or blocked — needs a dependent path from one that may run before it.
		free := map[int]bool{}
		for _, r := range remaining {
			if r == s || r > s && !blocked[r] {
				free[component[r]] = true
			}
		}
		ok := true
		for _, r := range remaining {
			if r != s && (r < s || blocked[r]) && !free[component[r]] {
				ok = false
				break
			}
		}
		if ok {
			next = append(next, s)
		}
	}
	return next
}

// Rivals lists, ascending, the statements not done, besides s, that run with s and may
// not commute with it: those another may run between two moves of s, s once started.
func (o *StatementOrder) Rivals(s int, done []bool, divided bool) []int {
	var out []int
	for _, r := range o.run(done) {
		if r != s && o.dependentPair(r, s, divided) {
			out = append(out, r)
		}
	}
	return out
}

// components numbers the connected components of the dependence among remaining.
func (o *StatementOrder) components(remaining []int, divided bool) map[int]int {
	component := make(map[int]int, len(remaining))
	for _, start := range remaining {
		if _, seen := component[start]; seen {
			continue
		}
		component[start] = start
		stack := []int{start}
		for len(stack) > 0 {
			i := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			for _, j := range remaining {
				if _, seen := component[j]; !seen && o.dependentPair(i, j, divided) {
					component[j] = start
					stack = append(stack, j)
				}
			}
		}
	}
	return component
}

// Ran marks s run after those done, unblocking each statement left that depends
// on it and blocking each before it that does not.
func (o *StatementOrder) Ran(s int, done, blocked []bool, divided bool) {
	done[s] = true
	for r := range done {
		if done[r] {
			continue
		}
		switch {
		case o.dependentPair(r, s, divided):
			blocked[r] = false
		case r < s:
			blocked[r] = true
		}
	}
}
