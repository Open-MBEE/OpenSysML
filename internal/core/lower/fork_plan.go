package lower

import (
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
)

// ForkPlan is where a fork's branches lead: one per orthogonal region of Owner,
// each region starting at its branch's target after the branch's effect.
type ForkPlan struct {
	Owner    *ast.StateNode
	Branches map[*ast.StateRegion]*Transition
}

// Targets is the state each region starts in.
func (p *ForkPlan) Targets() map[*ast.StateRegion]*ast.StateNode {
	targets := make(map[*ast.StateRegion]*ast.StateNode, len(p.Branches))
	for region, branch := range p.Branches {
		targets[region] = branch.Target.(*ast.StateNode)
	}
	return targets
}

// planForks checks every fork's outgoing branches and records, per fork, where
// each region starts, and per region, the forks that enter it.
func (g *StateGraph) planForks() error {
	for _, ps := range g.Pseudostates {
		if ps.Kind != ast.PseudostateFork {
			continue
		}
		plan, err := g.planFork(ps)
		if err != nil {
			return err
		}
		g.ForkPlans[ps] = plan
		for region := range plan.Branches {
			g.ForkEntered[region] = append(g.ForkEntered[region], ps)
		}
	}
	return nil
}

// planFork builds one fork's plan: at least two branches, neither triggered nor
// guarded, each into a state of a distinct orthogonal region, all regions of one
// composite state.
func (g *StateGraph) planFork(fork *ast.PseudostateNode) (*ForkPlan, error) {
	branches := g.Transitions[fork]
	if len(branches) < 2 {
		return nil, fmt.Errorf("fork %s needs at least two outgoing transitions, found %d", fork.Name, len(branches))
	}
	plan := &ForkPlan{Branches: make(map[*ast.StateRegion]*Transition, len(branches))}
	for _, branch := range branches {
		if branch.Trigger != nil {
			return nil, fmt.Errorf("fork %s: outgoing transitions cannot have triggers", fork.Name)
		}
		if branch.Guard != nil {
			return nil, fmt.Errorf("fork %s: outgoing transitions cannot be guarded", fork.Name)
		}
		target, ok := branch.Target.(*ast.StateNode)
		if !ok {
			return nil, fmt.Errorf("fork %s: branch target must be a state, got %s", fork.Name, DescribeMember(branch.Target))
		}
		region, ok := g.RegionOf[target]
		if !ok {
			return nil, fmt.Errorf("fork %s: branch target %s is not in an orthogonal region", fork.Name, target.Name)
		}
		if existing, dup := plan.Branches[region]; dup {
			return nil, fmt.Errorf("fork %s: branches %s and %s are in the same region", fork.Name, existing.Target.(*ast.StateNode).Name, target.Name)
		}
		plan.Branches[region] = branch
		owner := g.RegionOwner[region]
		if owner == nil {
			return nil, fmt.Errorf("fork %s: branch target %s is in a top-level region and has no owning composite state", fork.Name, target.Name)
		}
		if plan.Owner != nil && plan.Owner != owner {
			return nil, fmt.Errorf("fork %s: branches span more than one composite state", fork.Name)
		}
		plan.Owner = owner
	}
	return plan, nil
}

// ForkStarted reports whether a fork enters region, so it may start there
// without an entry transition of its own.
func (g *StateGraph) ForkStarted(region *ast.StateRegion) bool {
	return len(g.ForkEntered[region]) > 0
}

// checkForkOnlyRegion checks that a region without an entry transition is only
// ever started by a fork's branch: no other way into its composite state may
// leave the region to start by default.
func (g *StateGraph) checkForkOnlyRegion(owner *ast.StateNode, region *ast.StateRegion) error {
	entry := g.defaultEntryInto(owner, region)
	if entry == "" {
		return nil
	}
	return fmt.Errorf("region %s in state %s has no initial state: only a fork's branch may start it, but %s enters %s without one; write `entry; then <state>;` inside the region",
		region.Name, owner.Name, entry, owner.Name)
}

// defaultEntryInto describes a way into owner that starts region by default,
// naming no state inside it, or is empty when only forks' branches enter owner.
func (g *StateGraph) defaultEntryInto(owner *ast.StateNode, region *ast.StateRegion) string {
	for _, ps := range g.Pseudostates {
		if plan := g.ForkPlans[ps]; plan != nil && plan.Owner == owner && plan.Branches[region] == nil {
			return "fork " + ps.Name
		}
	}
	for _, body := range g.entryBodies() {
		if from := g.bodyState(body); from != nil && g.within(owner, from) {
			continue
		}
		for _, t := range g.EntryTransitions[body] {
			if g.entersByDefault(owner, region, nil, t.Target) {
				return "the entry transition naming " + t.Target.Name
			}
		}
	}
	for _, source := range g.States {
		for _, t := range g.Transitions[source] {
			if target := g.defaultStart(owner, region, source, t.Target, nil); target != nil {
				return fmt.Sprintf("the transition from %s to %s", source.Name, vertexName(target))
			}
		}
	}
	return ""
}

// defaultStart is the vertex a transition from source to target ends at when it
// enters owner in a way that starts region by default — owner itself or a state
// in another of its regions, reached from outside owner or from owner itself —
// following junctions, choices and joins to the states they lead to, and a fork
// to the composite state its branches enter or the states they end at; nil otherwise.
func (g *StateGraph) defaultStart(owner *ast.StateNode, region *ast.StateRegion, source *ast.StateNode, target ast.Node, seen map[ast.Node]bool) ast.Node {
	switch v := target.(type) {
	case *ast.StateNode:
		if g.entersByDefault(owner, region, source, v) {
			return v
		}
	case *ast.PseudostateNode:
		switch v.Kind {
		case ast.PseudostateShallowHistory, ast.PseudostateDeepHistory:
			if g.entersByDefault(owner, region, source, g.PseudostateOwner[v]) {
				return v
			}
		case ast.PseudostateFork:
			if g.forkStartsByDefault(owner, region, source, v) {
				return v
			}
		case ast.PseudostateJunction, ast.PseudostateChoice, ast.PseudostateJoin:
			if seen[v] {
				return nil
			}
			if seen == nil {
				seen = map[ast.Node]bool{}
			}
			seen[v] = true
			for _, t := range g.Transitions[v] {
				if end := g.defaultStart(owner, region, source, t.Target, seen); end != nil {
					return end
				}
			}
		}
	}
	return nil
}

// forkStartsByDefault reports whether fork's branches enter owner on their way to
// a state below it, or end at owner or in another of its regions, from source.
func (g *StateGraph) forkStartsByDefault(owner *ast.StateNode, region *ast.StateRegion, source *ast.StateNode, fork *ast.PseudostateNode) bool {
	plan := g.ForkPlans[fork]
	switch {
	case plan == nil, plan.Owner == owner:
		return false
	case g.within(owner, plan.Owner):
		return g.entersByDefault(owner, region, source, plan.Owner)
	}
	for _, branch := range plan.Branches {
		if g.entersByDefault(owner, region, source, branch.Target.(*ast.StateNode)) {
			return true
		}
	}
	return false
}

// entryBodies lists the bodies whose entry transitions are on record, in
// declaration order: the machine's own, then each state's and its regions'.
func (g *StateGraph) entryBodies() []ast.Node {
	bodies := []ast.Node{nil}
	for _, state := range g.States {
		bodies = append(bodies, state)
		for _, region := range g.CompositeStates[state] {
			bodies = append(bodies, region)
		}
	}
	return bodies
}

// bodyState is the state a body's entry transitions start inside: the state
// itself, a region's owner, nil for the machine's own body.
func (g *StateGraph) bodyState(body ast.Node) *ast.StateNode {
	switch b := body.(type) {
	case *ast.StateNode:
		return b
	case *ast.StateRegion:
		return g.RegionOwner[b]
	}
	return nil
}

// entersByDefault reports whether a move from source to target starts region by
// default: target is owner or lies in another of its regions, and the move comes
// from outside owner or from owner itself, so the whole of owner is entered.
func (g *StateGraph) entersByDefault(owner *ast.StateNode, region *ast.StateRegion, source, target *ast.StateNode) bool {
	if target == nil || !g.within(owner, target) {
		return false
	}
	if source != owner && g.within(owner, source) && target != owner {
		return false
	}
	return g.regionUnder(owner, target) != region
}

// within reports whether state is owner or lies below it.
func (g *StateGraph) within(owner, state *ast.StateNode) bool {
	for s := state; s != nil; s = g.ParentState[s] {
		if s == owner {
			return true
		}
	}
	return false
}

// regionUnder is the region of owner that state lies in, nil for owner itself.
func (g *StateGraph) regionUnder(owner, state *ast.StateNode) *ast.StateRegion {
	for s := state; s != nil && s != owner; s = g.ParentState[s] {
		if region := g.HiddenRegionOf[s]; region != nil && g.RegionOwner[region] == owner {
			return region
		}
		if region := g.RegionOf[s]; region != nil && g.RegionOwner[region] == owner {
			return region
		}
	}
	return nil
}
