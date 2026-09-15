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

// planFork builds one fork's plan: at least two unguarded branches, each into a
// state of a distinct orthogonal region, all regions of one composite state.
func (g *StateGraph) planFork(fork *ast.PseudostateNode) (*ForkPlan, error) {
	branches := g.Transitions[fork]
	if len(branches) < 2 {
		return nil, fmt.Errorf("fork %s needs at least two outgoing transitions, found %d", fork.Name, len(branches))
	}
	plan := &ForkPlan{Branches: make(map[*ast.StateRegion]*Transition, len(branches))}
	for _, branch := range branches {
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
