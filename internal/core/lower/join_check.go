package lower

import (
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
)

// checkJoins refuses a join two of whose incoming transitions leave one region
// (UML: the transitions into a join originate in different orthogonal regions),
// so every segment fires when the join does.
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
	for _, state := range g.States {
		for _, trans := range g.Transitions[state] {
			if trans.Target == ast.Node(join) {
				sources = append(sources, state)
			}
		}
	}
	if len(sources) < 2 {
		return nil
	}
	owner := g.forkOwner(sources)
	seen := make(map[*ast.StateRegion]*ast.StateNode, len(sources))
	for _, source := range sources {
		region := g.enclosingRegion(source)
		if owner != nil {
			region = g.regionUnder(owner, source)
		}
		if region == nil {
			return fmt.Errorf("join %s: incoming transition leaves %s, which is not in an orthogonal region", join.Name, source.Name)
		}
		if other, dup := seen[region]; dup {
			if other == source {
				return fmt.Errorf("join %s: two incoming transitions leave %s", join.Name, source.Name)
			}
			return fmt.Errorf("join %s: incoming transitions leave %s and %s, in the same region", join.Name, other.Name, source.Name)
		}
		seen[region] = source
	}
	return nil
}
