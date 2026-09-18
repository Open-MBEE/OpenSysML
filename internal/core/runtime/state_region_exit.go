package runtime

import (
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
)

// exitFront is the front of a composite's regions being left, drawn under
// ChoiceExitOrder.
func (e *StateExecutor) exitFront(where string, span source.Span) *orderFront {
	return &orderFront{exec: e, kind: ChoiceExitOrder, where: where, span: span}
}

func exitRegionStateWrap(err error) error { return fmt.Errorf("exit region state: %w", err) }

func exitStateWrap(err error) error { return fmt.Errorf("exit state: %w", err) }

// addExitQueues puts one exit queue per active region of owner on the front, ahead
// of before, and records the configuration left for a history of the owner to
// restore. The regions are taken out of the active configuration here: the
// queues hold what is left to exit in each.
func (e *StateExecutor) addExitQueues(f *orderFront, before *orderQueue, owner *ast.StateNode, regions []*ast.StateRegion, wrap func(error) error) *queueGroup {
	group := &queueGroup{}
	for _, region := range regions {
		active, isActive := e.activeConfig.regionStates[region]
		if !isActive {
			continue
		}
		if active != owner {
			e.recordRegionHistory(region, active)
		}
		delete(e.activeConfig.regionStates, region)
		q := &orderQueue{region: region, group: group, wrap: wrap}
		group.left++
		f.insert(before, []*orderQueue{q})
		e.fillExitQueue(f, q, active, owner)
	}
	if group.left == 0 {
		return nil
	}
	return group
}

// fillExitQueue queues the exits of a region's active state and of the states
// between it and the owner, innermost first. A state on the way whose own regions
// are active has their queues put on the front ahead of q, as siblings, and its
// exit waits for them; a graph-only region owner, which logs no exit, leaves with
// the state below it.
func (e *StateExecutor) fillExitQueue(f *orderFront, q *orderQueue, active, owner *ast.StateNode) {
	current := active
	for current != nil && current != owner {
		unit := orderUnit{label: e.stateExitLabel(current), state: current}
		if regions, isComposite := e.graph.CompositeStates[current]; isComposite {
			if group := e.addExitQueues(f, q, current, regions, q.wrap); group != nil {
				unit.after = &group.unitGate
			}
		}
		leaving := []*ast.StateNode{current}
		for current = e.graph.ParentState[current]; current != nil && current != owner && e.leavesUnlogged(current); current = e.graph.ParentState[current] {
			leaving = append(leaving, current)
		}
		unit.run = func(*orderQueue) error {
			for _, state := range leaving {
				if err := e.exitOwn(state); err != nil {
					return err
				}
			}
			return nil
		}
		q.units = append(q.units, unit)
	}
}

// leavesUnlogged reports whether a state's exit is nothing a trace tells apart from
// the exit below it: a graph-only region owner with no exit behavior and no
// regions of its own.
func (e *StateExecutor) leavesUnlogged(state *ast.StateNode) bool {
	_, isComposite := e.graph.CompositeStates[state]
	return e.graph.HiddenStates[state] && !isComposite && len(e.behaviorsOf(state).Exit) == 0
}

// stateExitLabel names a state's exit as a unit, `left(exit)`, or the region a
// graph-only owner stands for.
func (e *StateExecutor) stateExitLabel(state *ast.StateNode) string {
	if e.graph.HiddenStates[state] {
		return regionName(e.graph.HiddenRegionOf[state])
	}
	return state.Name + "(exit)"
}
