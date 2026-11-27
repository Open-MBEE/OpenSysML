// Package progress finds stalled obligations in an observed transition graph.
// It knows nothing about execution, state identity, or whether traversal was complete.
package progress

import (
	"context"
	"slices"
)

// Step is an edge between caller-supplied stable state identifiers.
type Step struct {
	From, To string
	Move     string
}

type node struct {
	pending, terminal bool
	parent            Step
	startupPicks      []int
	edges             []Step
}

// Graph retains the explored transitions and discovery order for stable witnesses.
type Graph struct {
	nodes   map[string]*node
	order   []string
	cleared bool
}

func New() *Graph {
	return &Graph{nodes: make(map[string]*node)}
}

// Observe records an edge, or a root if From is empty. Pending and startupPicks
// are needed only for a newly discovered state (and startupPicks only at a root).
func (g *Graph) Observe(edge Step, newState, pending bool, startupPicks []int) {
	if newState {
		n := &node{pending: pending, parent: edge}
		if edge.From == "" {
			n.startupPicks = slices.Clone(startupPicks)
		}
		g.nodes[edge.To] = n
		g.order = append(g.order, edge.To)
	}
	if previous := g.nodes[edge.From]; previous != nil {
		previous.edges = append(previous.edges, edge)
		if previous.pending && !g.nodes[edge.To].pending {
			g.cleared = true
		}
	}
}

// MarkTerminal marks a previously observed state with no further moves.
func (g *Graph) MarkTerminal(key string) {
	g.nodes[key].terminal = true
}

// Result describes what the graph demonstrated, without classifying an
// incomplete traversal as a pass. A nonempty Root belongs to a stalled witness.
type Result struct {
	Activated, Cleared bool
	Root               string
	StartupPicks       []int
	Prefix, Cycle      []Step
}

func (g *Graph) witness(r *Result, key string) {
	for edge := g.nodes[key].parent; edge.From != ""; edge = g.nodes[key].parent {
		r.Prefix = append(r.Prefix, edge)
		key = edge.From
	}
	slices.Reverse(r.Prefix)
	r.Root = key
	r.StartupPicks = slices.Clone(g.nodes[key].startupPicks)
}

// Analyze prefers a pending terminal, then looks for a repeatable pending cycle.
func (g *Graph) Analyze(stop context.Context) (Result, error) {
	r := Result{Cleared: g.cleared}
	for _, key := range g.order {
		n := g.nodes[key]
		if n.pending {
			r.Activated = true
			if n.terminal {
				g.witness(&r, key)
				return r, nil
			}
		}
	}
	cycle, err := g.pendingCycle(stop)
	if err != nil {
		return Result{}, err
	}
	if len(cycle) != 0 {
		r.Cycle = cycle
		g.witness(&r, cycle[0].From)
	}
	return r, nil
}

// pendingCycle searches only pending edges; an exit to a response does not
// prevent a failing cycle because no rule forces that exit to be chosen.
func (g *Graph) pendingCycle(stop context.Context) ([]Step, error) {
	type frame struct {
		key  string
		next int
		via  Step
	}
	done := make(map[string]bool)
	active := make(map[string]int)
	for _, root := range g.order {
		if done[root] || !g.nodes[root].pending {
			continue
		}
		stack := []frame{{key: root}}
		active[root] = 0
		for len(stack) > 0 {
			if err := stop.Err(); err != nil {
				return nil, err
			}
			f := &stack[len(stack)-1]
			edges := g.nodes[f.key].edges
			if f.next == len(edges) {
				done[f.key] = true
				delete(active, f.key)
				stack = stack[:len(stack)-1]
				continue
			}
			edge := edges[f.next]
			f.next++
			to := edge.To
			if !g.nodes[to].pending || done[to] {
				continue
			}
			if entry, found := active[to]; found {
				cycle := make([]Step, 0, len(stack)-entry)
				for _, path := range stack[entry+1:] {
					cycle = append(cycle, path.via)
				}
				return append(cycle, edge), nil
			}
			active[to] = len(stack)
			stack = append(stack, frame{key: to, via: edge})
		}
	}
	return nil, nil
}
