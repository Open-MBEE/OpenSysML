package migrate

import (
	"slices"

	"github.com/Open-MBEE/OpenSysML/internal/core/xmi"
)

// awaitData turns the object flows into a control-flow-driven action into
// successions where v1's implicit join can be kept: the value has one producer in
// the activity, and the action lies on no loop that leaves the producer out, so
// the producer runs before the action on every pass. The other flows keep
// carrying their value only, with the reason recorded for the report.
func (a *activity) awaitData() {
	for _, e := range a.edges {
		if !a.dataOnly[e] || a.edgeSelf[e] || a.dryFlow(e) {
			continue
		}
		tgt := a.m.model.Ref(e, "target")
		if tgt == nil || nodeKind(tgt) != nodePin {
			continue
		}
		to := tgt.Parent
		srcs := a.edgeSources[e]
		if len(srcs) != 1 {
			if len(srcs) > 1 {
				a.dataWhy[e] = "the value has several producers, of which a pass runs one"
			}
			continue
		}
		from := ownerNode(srcs[0])
		if nodeKind(from) != nodeAction || from == to || !slices.Contains(a.nodes, from) {
			continue
		}
		if a.starvesWaiting(to, from) {
			a.dataWhy[e] = "the action lies on a loop that leaves " + describe(from) + " out, so waiting would starve its later passes"
			continue
		}
		delete(a.dataOnly, e)
		a.awaited[e] = true
		if slices.Contains(a.next[from], to) {
			continue
		}
		a.succ[from] = append(a.succ[from], e)
		a.next[from] = append(a.next[from], to)
		a.prev[to] = append(a.prev[to], from)
	}
}

// starvesWaiting reports whether a pass of to could wait for a value from a pass
// from does not run: a loop of successions leads from to back to to without
// passing a node from which from is surely reached again.
func (a *activity) starvesWaiting(to, from *xmi.Element) bool {
	refires := a.surelyReaching(from)
	seen := map[*xmi.Element]bool{}
	var walk func(*xmi.Element) bool
	walk = func(cur *xmi.Element) bool {
		for _, nx := range a.next[cur] {
			if nx == to {
				return true
			}
			if refires[nx] || seen[nx] {
				continue
			}
			seen[nx] = true
			if walk(nx) {
				return true
			}
		}
		return false
	}
	return walk(to)
}

// surelyReaching lists the nodes from which n is reached by successions that fire
// whenever their source does: unguarded edges out of anything but a decision.
func (a *activity) surelyReaching(n *xmi.Element) map[*xmi.Element]bool {
	reaching := map[*xmi.Element]bool{n: true}
	for changed := true; changed; {
		changed = false
		for src, edges := range a.succ {
			if reaching[src] || src.Type == "DecisionNode" {
				continue
			}
			for _, e := range edges {
				tgt := a.m.model.Ref(e, "target")
				if tgt != nil && firstOwned(e, "guard") == nil && reaching[ownerNode(tgt)] {
					reaching[src] = true
					changed = true
					break
				}
			}
		}
	}
	return reaching
}
