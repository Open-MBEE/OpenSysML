package lower

import (
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// StatementOrder is how the statements of one list may run: in runs separated by
// the statements that keep their place, each run's in any order, of which only
// those reordering two dependent statements differ.
type StatementOrder struct {
	fixed []bool
	// dependent holds for two statements that may not commute; shared, for two
	// that both may touch what another performance does.
	dependent, shared [][]bool
	before            [][]bool
	skipped           []bool
}

// BodyStatementOrder is the order of stmts, statements of node's body in graph
// or of a block within it.
func BodyStatementOrder(graph *ActionGraph, node ast.Node, stmts []Statement) *StatementOrder {
	return bodyStatementOrder(graph, node, stmts, nil)
}

// CalcBodyStatementOrder is the order of a calculation body's statements,
// including any `then` successions the lowered body states.
func CalcBodyStatementOrder(scope *symbols.Scope, members []ast.Node, stmts []Statement) *StatementOrder {
	before, matched := statementPrecedence(members, stmts)
	order := bodyStatementOrder(&ActionGraph{Scope: scope}, nil, stmts, before)
	order.skipSuccessions(stmts, matched)
	return order
}

func statementPrecedence(members []ast.Node, stmts []Statement) ([][]bool, map[*ast.SuccessionEdge]bool) {
	before := make([][]bool, len(stmts))
	for i := range before {
		before[i] = make([]bool, len(stmts))
	}
	index := make(map[ast.Node]int, len(stmts))
	for i, stmt := range stmts {
		if _, result := stmt.(Return); result {
			continue
		}
		if unsupported, ok := stmt.(Unsupported); ok {
			if _, succession := unsupported.Node.(*ast.SuccessionEdge); succession {
				continue
			}
		}
		if node := statementNode(stmt); node != nil {
			index[node] = i
		}
	}
	matched := make(map[*ast.SuccessionEdge]bool)
	for _, member := range members {
		edge, ok := unwrapMembership(member).(*ast.SuccessionEdge)
		if !ok {
			continue
		}
		from, fromOK := calcSuccessionEnd(edge.SourceMember, edge.Source, stmts, index)
		to, toOK := calcSuccessionEnd(edge.TargetMember, edge.Target, stmts, index)
		if fromOK && toOK && from != to {
			before[from][to] = true
			matched[edge] = true
		}
	}
	closePrecedence(before)
	cycle := false
	for i := range before {
		if before[i][i] {
			cycle = true
			break
		}
	}
	if cycle {
		for i := range before {
			clear(before[i])
		}
		clear(matched)
	}
	return before, matched
}

// ConstraintBodyStatementOrder is the order of a constraint body's statements.
func ConstraintBodyStatementOrder(scope *symbols.Scope, stmts []Statement) *StatementOrder {
	return BodyStatementOrder(&ActionGraph{Scope: scope}, nil, stmts)
}

// ConstraintBodyWithOrder returns a constraint body's statements with its
// successions between statements recorded as precedence.
func ConstraintBodyWithOrder(scope *symbols.Scope, members []ast.Node, stmts []Statement) ([]Statement, *StatementOrder) {
	stmts = discardMatchedSuccessions(members, stmts)
	order := constraintStatementOrder(scope, members, stmts)
	constraintNestedOrders(stmts)
	return stmts, order
}

func discardMatchedSuccessions(members []ast.Node, stmts []Statement) []Statement {
	_, matched := statementPrecedence(members, stmts)
	if len(matched) == 0 {
		return stmts
	}
	filtered := make([]Statement, 0, len(stmts))
	for _, stmt := range stmts {
		unsupported, ok := stmt.(Unsupported)
		edge, succession := unsupported.Node.(*ast.SuccessionEdge)
		if ok && succession && matched[edge] {
			continue
		}
		filtered = append(filtered, stmt)
	}
	return filtered
}

func bodyStatementOrder(graph *ActionGraph, node ast.Node, stmts []Statement, before [][]bool) *StatementOrder {
	declared := declaredFeatures(graph)
	n := len(stmts)
	o := &StatementOrder{
		fixed:     make([]bool, n),
		dependent: make([][]bool, n),
		shared:    make([][]bool, n),
		before:    cloneMatrix(before, n),
	}
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
			if o.before[i][j] || o.before[j][i] {
				dep = true
			}
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

// Skipped reports whether i is a succession represented by this order, not a
// statement to execute.
func (o *StatementOrder) Skipped(i int) bool {
	return i >= 0 && i < len(o.skipped) && o.skipped[i]
}

// HasSkipped reports whether this order carries any non-executable successions.
func (o *StatementOrder) HasSkipped() bool {
	for _, skipped := range o.skipped {
		if skipped {
			return true
		}
	}
	return false
}

func (o *StatementOrder) skipSuccessions(stmts []Statement, matched map[*ast.SuccessionEdge]bool) {
	o.skipped = make([]bool, len(stmts))
	for i, stmt := range stmts {
		unsupported, ok := stmt.(Unsupported)
		if !ok {
			continue
		}
		edge, ok := unsupported.Node.(*ast.SuccessionEdge)
		o.skipped[i] = ok && matched[edge]
	}
}

// Reorders reports whether two of the statements may run in either order with
// different results: two dependent ones in one run, or with divided set (another
// performance interleaving between statements), two that both touch what it does.
func (o *StatementOrder) Reorders(divided bool) bool {
	for i := range o.fixed {
		if o.Skipped(i) {
			continue
		}
		for j := i + 1; j < len(o.fixed); j++ {
			if !o.Skipped(j) && o.before[j][i] {
				return true
			}
		}
	}
	for i := range o.fixed {
		if o.Skipped(i) {
			continue
		}
		for j := i + 1; j < len(o.fixed); j++ {
			if o.Skipped(j) {
				continue
			}
			if o.fixed[j] {
				break
			}
			if !o.fixed[i] && o.dependentPair(i, j, divided) &&
				!o.before[i][j] && !o.before[j][i] {
				return true
			}
		}
	}
	return false
}

// HasReversePrecedence reports whether an explicit succession runs against declaration order.
func (o *StatementOrder) HasReversePrecedence() bool {
	for i := range o.fixed {
		for j := i + 1; j < len(o.fixed); j++ {
			if o.before[j][i] {
				return true
			}
		}
	}
	return false
}

func (o *StatementOrder) dependentPair(i, j int, divided bool) bool {
	return o.dependent[i][j] || o.before[i][j] || o.before[j][i] || divided && o.shared[i][j]
}

// run is the statements not done in the run the first of them is in: that one
// alone where it keeps its place.
func (o *StatementOrder) run(done []bool) []int {
	var out []int
	for i, d := range done {
		if d || o.Skipped(i) {
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
	if o.hasPrecedence(remaining) {
		var next []int
		for _, s := range remaining {
			ready := true
			for _, predecessor := range remaining {
				if o.before[predecessor][s] && !done[predecessor] {
					ready = false
					break
				}
			}
			if ready {
				next = append(next, s)
			}
		}
		for _, s := range next {
			independent := true
			for _, other := range remaining {
				if other != s && (o.dependentPair(s, other, divided) ||
					o.before[s][other] || o.before[other][s]) {
					independent = false
					break
				}
			}
			if independent {
				return []int{s}
			}
		}
		return next
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

// NextFixed lists the declaration-least ready statement, respecting explicit
// precedence without exposing a choice point under a fixed scheduling policy.
func (o *StatementOrder) NextFixed(done, blocked []bool, divided bool) []int {
	remaining := o.run(done)
	if len(remaining) < 2 {
		return remaining
	}
	if !o.hasPrecedence(remaining) {
		return remaining[:1]
	}
	for _, s := range remaining {
		ready := true
		for _, predecessor := range remaining {
			if o.before[predecessor][s] && !done[predecessor] {
				ready = false
				break
			}
		}
		if ready {
			return []int{s}
		}
	}
	return nil
}

func (o *StatementOrder) hasPrecedence(remaining []int) bool {
	present := make([]bool, len(o.before))
	for _, i := range remaining {
		present[i] = true
	}
	for _, i := range remaining {
		for _, j := range remaining {
			if o.before[i][j] && present[j] {
				return true
			}
		}
	}
	return false
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

func cloneMatrix(matrix [][]bool, n int) [][]bool {
	out := make([][]bool, n)
	for i := range out {
		out[i] = make([]bool, n)
		if i < len(matrix) {
			copy(out[i], matrix[i])
		}
	}
	return out
}

func closePrecedence(before [][]bool) {
	for k := range before {
		for i := range before {
			if !before[i][k] {
				continue
			}
			for j := range before {
				before[i][j] = before[i][j] || before[k][j]
			}
		}
	}
}
