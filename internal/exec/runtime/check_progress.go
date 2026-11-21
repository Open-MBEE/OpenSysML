package runtime

import (
	"context"
	"errors"
	"fmt"
	"slices"
)

// ProgressStatus distinguishes a proof, a counterexample, lack of activation, and an incomplete search.
type ProgressStatus string

const (
	ProgressHolds        ProgressStatus = "holds"
	ProgressStalled      ProgressStatus = "stalled"
	ProgressVacuous      ProgressStatus = "vacuous"
	ProgressInconclusive ProgressStatus = "inconclusive"
)

// ErrProgressTimed means a started invocation declared a horizon or advanced its clock during search.
var ErrProgressTimed = errors.New("progress checking requires untimed execution without a clock horizon")

// ErrProgressMonitor means that no named pending-obligation predicate was provided.
var ErrProgressMonitor = errors.New("progress checking requires a named pending predicate")

// ProgressStep identifies a native explored edge by canonical stable-state digests and move identity.
type ProgressStep struct {
	From, To string
	Move     string
}

// ProgressReport reports one request-to-response obligation. A stalled report
// has a prefix to a pending terminal (empty Cycle) or a repeatable pending walk.
type ProgressReport struct {
	Status             ProgressStatus
	Search             *CheckReport
	Activated, Cleared bool
	Prefix, Cycle      []ProgressStep
}

// CheckProgress asks whether every model schedule leaves the pending region.
// The caller supplies a deterministic Boolean function of the canonical state alone,
// false before acceptance and true until an authorized response. No fairness is assumed.
func CheckProgress(stop context.Context, fresh func() (*Context, error), start Starter, budget CheckBudget, pending CheckProperty) (*ProgressReport, error) {
	if pending.Name == "" || pending.Holds == nil {
		return nil, ErrProgressMonitor
	}
	g := &progressGraph{pending: pending, nodes: make(map[stateKey]*progressNode)}
	search, err := check(stop, fresh, start, budget, CheckOptions{}, nil, g)
	if err != nil {
		return nil, err
	}
	return g.report(stop, search)
}

type progressGraph struct {
	pending    CheckProperty
	startClock float64
	incoming   ProgressStep
	nodes      map[stateKey]*progressNode
	order      []stateKey
	cleared    bool
}

type progressNode struct {
	pending, terminal bool
	parent            ProgressStep
	edges             []ProgressStep
}

func (g *progressGraph) observe(c *checker, key stateKey, visited bool) error {
	edge := g.incoming
	edge.To = string(key)
	if !visited {
		pending, err := c.ctx.everyStatementOrder(func() (bool, error) {
			return g.pending.Holds(c.ctx, c.inv)
		})
		if err != nil {
			return fmt.Errorf("progress monitor %q: %w", g.pending.Name, err)
		}
		g.nodes[key] = &progressNode{pending: pending, parent: edge}
		g.order = append(g.order, key)
	}
	if previous := g.nodes[stateKey(edge.From)]; previous != nil {
		previous.edges = append(previous.edges, edge)
		if previous.pending && !g.nodes[key].pending {
			g.cleared = true
		}
	}
	return nil
}

func (g *progressGraph) prefix(key stateKey) []ProgressStep {
	var path []ProgressStep
	for edge := g.nodes[key].parent; edge.From != ""; edge = g.nodes[stateKey(edge.From)].parent {
		path = append(path, edge)
	}
	slices.Reverse(path)
	return path
}

func (g *progressGraph) report(stop context.Context, search *CheckReport) (*ProgressReport, error) {
	r := &ProgressReport{Search: search, Cleared: g.cleared}
	for _, key := range g.order {
		n := g.nodes[key]
		if n.pending {
			r.Activated = true
			if n.terminal {
				r.Status, r.Prefix = ProgressStalled, g.prefix(key)
				return r, nil
			}
		}
	}
	cycle, err := g.pendingCycle(stop)
	if err != nil {
		return nil, err
	}
	switch {
	case len(cycle) != 0:
		r.Status, r.Prefix, r.Cycle = ProgressStalled, g.prefix(stateKey(cycle[0].From)), cycle
	case len(search.BoundsHit)+len(search.Unenumerated)+len(search.Violations) != 0:
		r.Status = ProgressInconclusive
	case !r.Activated:
		r.Status = ProgressVacuous
	default:
		r.Status = ProgressHolds
	}
	return r, nil
}

// pendingCycle searches only pending edges; an exit to a response does not
// prevent a failing cycle because no rule forces that exit to be chosen.
func (g *progressGraph) pendingCycle(stop context.Context) ([]ProgressStep, error) {
	type frame struct {
		key  stateKey
		next int
		via  ProgressStep
	}
	done := make(map[stateKey]bool)
	active := make(map[stateKey]int)
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
			to := stateKey(edge.To)
			if !g.nodes[to].pending || done[to] {
				continue
			}
			if entry, found := active[to]; found {
				cycle := make([]ProgressStep, 0, len(stack)-entry)
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
