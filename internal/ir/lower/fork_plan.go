package lower

import (
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
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
	targets := make([]*ast.StateNode, 0, len(branches))
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
		if g.HiddenRegionOf[target] != nil {
			return nil, fmt.Errorf("fork %s: branch target %s is an orthogonal region, not a state in one", fork.Name, target.Name)
		}
		if g.enclosingRegion(target) == nil {
			return nil, fmt.Errorf("fork %s: branch target %s is not in an orthogonal region", fork.Name, target.Name)
		}
		targets = append(targets, target)
	}
	plan := &ForkPlan{Owner: g.forkOwner(targets), Branches: make(map[*ast.StateRegion]*Transition, len(branches))}
	if plan.Owner == nil {
		if g.RegionOwner[g.enclosingRegion(targets[0])] == nil {
			return nil, fmt.Errorf("fork %s: branch target %s is in a top-level region and has no owning composite state", fork.Name, targets[0].Name)
		}
		return nil, fmt.Errorf("fork %s: branches span more than one composite state", fork.Name)
	}
	for i, branch := range branches {
		region := g.regionUnder(plan.Owner, targets[i])
		if region == nil {
			return nil, fmt.Errorf("fork %s: branch target %s is not in a region of %s", fork.Name, targets[i].Name, plan.Owner.Name)
		}
		if existing, dup := plan.Branches[region]; dup {
			return nil, fmt.Errorf("fork %s: branches %s and %s are in the same region", fork.Name, existing.Target.(*ast.StateNode).Name, targets[i].Name)
		}
		plan.Branches[region] = branch
	}
	return plan, nil
}

// forkOwner is the innermost orthogonal state every target lies below, nil when
// only the machine encloses them all.
func (g *StateGraph) forkOwner(targets []*ast.StateNode) *ast.StateNode {
	owner := g.ParentState[targets[0]]
	for _, target := range targets[1:] {
		for owner != nil && !g.within(owner, target) {
			owner = g.ParentState[owner]
		}
	}
	for owner != nil && len(g.CompositeStates[owner]) == 0 {
		owner = g.ParentState[owner]
	}
	return owner
}

// enclosingRegion is the innermost orthogonal region state lies in, nil if none.
func (g *StateGraph) enclosingRegion(state *ast.StateNode) *ast.StateRegion {
	for s := state; s != nil; s = g.ParentState[s] {
		if region := g.RegionOf[s]; region != nil {
			return region
		}
	}
	return nil
}

// ForkStarted reports whether a fork branch enters region.
func (g *StateGraph) ForkStarted(region *ast.StateRegion) bool {
	return len(g.ForkEntered[region]) > 0
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

// regionUnder is the region of owner that state lies in, nil for owner itself;
// a nil owner is the machine, whose regions are the top-level ones.
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
