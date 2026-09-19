package lower

import (
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// JoinPlan is where a join's incoming segments come from: Regions maps each to
// the orthogonal region of Owner it leaves, Owner being the innermost state all
// the sources lie below, nil when only the machine's own regions do.
type JoinPlan struct {
	Owner   *ast.StateNode
	Regions map[*Transition]*ast.StateRegion
}

// checkJoins refuses a join two of whose incoming transitions leave one region
// (UML: the transitions into a join originate in different orthogonal regions),
// so every segment fires when the join does, and records where each comes from.
func (g *StateGraph) checkJoins() error {
	for _, ps := range g.Pseudostates {
		if ps.Kind != ast.PseudostateJoin {
			continue
		}
		if err := g.checkJoin(ps); err != nil {
			return err
		}
	}
	return nil
}

func (g *StateGraph) checkJoin(join *ast.PseudostateNode) error {
	var sources []*ast.StateNode
	var segments []*Transition
	for _, state := range g.States {
		for _, trans := range g.Transitions[state] {
			if trans.Target == ast.Node(join) {
				sources = append(sources, state)
				segments = append(segments, trans)
			}
		}
	}
	if len(sources) < 2 {
		return nil
	}
	for _, source := range sources {
		if g.enclosingRegion(source) == nil {
			return fmt.Errorf("join %s: incoming transition leaves %s, which is not in an orthogonal region", join.Name, source.Name)
		}
	}
	plan := &JoinPlan{Owner: g.forkOwner(sources), Regions: make(map[*Transition]*ast.StateRegion, len(segments))}
	seen := make(map[*ast.StateRegion]*ast.StateNode, len(sources))
	for i, source := range sources {
		// The region is one of the owner's own — the machine's, for a nil owner —
		// not a region nested below it, which the owner's exit leaves as a whole.
		region := g.regionUnder(plan.Owner, source)
		if region == nil {
			return fmt.Errorf("join %s: incoming transitions leave regions of more than one orthogonal state", join.Name)
		}
		if other, dup := seen[region]; dup {
			if other == source {
				return fmt.Errorf("join %s: two incoming transitions leave %s", join.Name, source.Name)
			}
			return fmt.Errorf("join %s: incoming transitions leave %s and %s, in the same region", join.Name, other.Name, source.Name)
		}
		seen[region] = source
		plan.Regions[segments[i]] = region
	}
	g.JoinPlans[join] = plan
	return nil
}
