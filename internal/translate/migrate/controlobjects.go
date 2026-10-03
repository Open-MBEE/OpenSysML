package migrate

import (
	"slices"
	"strconv"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/translate/xmi/sysmlv1"
)

// carriesObjects reports whether n is a node object flows may pass through as
// written features: a fork, join or merge.
func carriesObjects(n *sysmlv1.Element) bool {
	switch n.Type {
	case "ForkNode", "JoinNode", "MergeNode":
		return true
	}
	return false
}

// settleThrough marks the forks, joins and merges whose object flows are written
// through features of the node: inputObjectN for each flow into it, outputObjectN for
// each flow out of it. A node keeps its flows routed from their producing pins when one
// of them comes from or leads to an end no such flow may name.
func (a *activity) settleThrough() {
	for _, n := range a.nodes {
		if carriesObjects(n) && !a.dataNode[n] && !a.sink[n] && len(a.objectEdges(n, "target")) > 0 {
			a.through[n] = true
		}
	}
	for changed := true; changed; {
		changed = false
		for _, n := range a.nodes {
			if a.through[n] && !a.passable(n) {
				delete(a.through, n)
				changed = true
			}
		}
	}
}

// objectEdges lists the object flows whose end named role is n, in edge order.
func (a *activity) objectEdges(n *sysmlv1.Element, role string) []*sysmlv1.Element {
	var out []*sysmlv1.Element
	for _, e := range a.edges {
		if e.Type == "ObjectFlow" && a.m.model.Ref(e, role) == n {
			out = append(out, e)
		}
	}
	return out
}

// passable reports whether every object flow into n comes from a pin producing its
// value or from a node passing it through, and every one out of n leads to an input
// pin it is written to or to such a node.
func (a *activity) passable(n *sysmlv1.Element) bool {
	for _, e := range a.objectEdges(n, "target") {
		if !a.objectFrom(a.m.model.Ref(e, "source")) {
			return false
		}
	}
	for _, e := range a.objectEdges(n, "source") {
		if a.awaited[e] || !a.objectTo(a.m.model.Ref(e, "target")) {
			return false
		}
	}
	return true
}

// objectFrom reports whether a value written at src may travel a flow out of it.
func (a *activity) objectFrom(src *sysmlv1.Element) bool {
	if src == nil {
		return false
	}
	if a.through[src] {
		return true
	}
	n := src.Parent
	if nodeKind(src) != nodePin || n == nil || nodeKind(n) != nodeAction || !slices.Contains(a.nodes, n) ||
		a.starved[n] != nil || a.streams(src) || !a.writesOutput(src) || !a.producesAt(src) || a.unassigned(src) {
		return false
	}
	callee, p := a.calleeOutput(src)
	return callee == nil || p != nil && !a.m.dryOutputs(callee)[p]
}

// objectTo reports whether a flow may deliver a value to tgt.
func (a *activity) objectTo(tgt *sysmlv1.Element) bool {
	if tgt == nil {
		return false
	}
	if a.through[tgt] {
		return true
	}
	n := tgt.Parent
	return nodeKind(tgt) == nodePin && n != nil && nodeKind(n) == nodeAction && slices.Contains(a.nodes, n) &&
		a.starved[n] == nil && !a.streams(tgt) && a.writesInput(tgt)
}

// objectIndex is the position of e among the edges entering (role "target") or
// leaving (role "source") its node n, counting from 1, as the node lists them.
func (a *activity) objectIndex(e, n *sysmlv1.Element, role string) int {
	list := "incoming"
	if role == "source" {
		list = "outgoing"
	}
	edges := a.m.model.Refs(n, list)
	if len(edges) == 0 {
		for _, x := range a.edges {
			if a.m.model.Ref(x, role) == n {
				edges = append(edges, x)
			}
		}
	}
	return slices.Index(edges, e) + 1
}

// objectFeature names the feature of n the object flow e enters or leaves it by.
func (a *activity) objectFeature(e, n *sysmlv1.Element) string {
	if a.m.model.Ref(e, "target") == n {
		return "inputObject" + strconv.Itoa(a.objectIndex(e, n, "target"))
	}
	return "outputObject" + strconv.Itoa(a.objectIndex(e, n, "source"))
}

// objectSourceType is the classifier what leaves src is typed by.
func (a *activity) objectSourceType(src *sysmlv1.Element, seen map[*sysmlv1.Element]bool) *sysmlv1.Element {
	if a.through[src] {
		return a.objectOutType(src, seen)
	}
	return a.endType(src)
}

// objectOutType is the classifier what n passes on is typed by: what flows into it,
// when every flow in agrees on it.
func (a *activity) objectOutType(n *sysmlv1.Element, seen map[*sysmlv1.Element]bool) *sysmlv1.Element {
	if seen[n] {
		return nil
	}
	seen[n] = true
	var t *sysmlv1.Element
	for i, e := range a.objectEdges(n, "target") {
		st := a.objectSourceType(a.m.model.Ref(e, "source"), seen)
		if i > 0 && st != t {
			return nil
		}
		t = st
	}
	return t
}

// objectFeatures writes the features n passes objects through by: an input for each
// object flow into it and an output for each one out of it, valued by the inputs, which
// a join gathers into one sequence and a merge passes on one at a time.
func (a *activity) objectFeatures(n *sysmlv1.Element) {
	if !a.through[n] {
		return
	}
	ins := a.objectEdges(n, "target")
	var names []string
	for _, e := range ins {
		name := a.objectFeature(e, n)
		names = append(names, name)
		a.m.w.line("in ref " + name + a.objectTyping(a.objectSourceType(a.m.model.Ref(e, "source"), map[*sysmlv1.Element]bool{})) + ";")
	}
	value := names[0]
	if len(names) > 1 {
		value = "(" + strings.Join(names, ", ") + ")"
	}
	unique := ""
	if n.Type == "JoinNode" && len(names) > 1 {
		unique = " nonunique"
	}
	typing := a.objectTyping(a.objectOutType(n, map[*sysmlv1.Element]bool{}))
	for _, e := range a.objectEdges(n, "source") {
		a.m.w.line("out ref " + a.objectFeature(e, n) + typing + unique + " = " + value + ";")
	}
}

// objectTyping writes the typing of a control node's object feature, nothing when untyped.
func (a *activity) objectTyping(t *sysmlv1.Element) string {
	if typ, _ := a.m.typeRef(t, a.def); typ != "" {
		return " : " + typ
	}
	return ""
}

// objectEnd writes how a flow names its end x at a node passing objects through, or at a pin.
func (a *activity) objectEnd(e, x *sysmlv1.Element) (string, bool) {
	if !a.through[x] {
		return a.pinRef(x)
	}
	name, ok := a.names[x]
	if !ok {
		return "", false
	}
	return writeName(name) + "." + a.objectFeature(e, x), true
}

// throughFlow writes an object flow into or out of a node passing objects through: a
// succession flow where it is the succession between its ends, a flow where a guarded
// succession or another edge orders them, or where it carries its value only.
func (a *activity) throughFlow(e, src, tgt *sysmlv1.Element) {
	from, okFrom := a.objectEnd(e, src)
	to, okTo := a.objectEnd(e, tgt)
	switch {
	case !okFrom:
		a.m.add(e, Unmapped, "", "the flow's source "+describe(src)+" has no v2 name")
		a.keepOrder(e)
		return
	case !okTo:
		a.m.add(e, Unmapped, "", "the flow's target "+describe(tgt)+" has no v2 name")
		a.keepOrder(e)
		return
	case !a.through[src] && a.inert[src.Parent]:
		a.m.w.line(flowNote + from + " to " + to + notWritten + describe(src.Parent) + " is not migrated and produces no value */")
		a.m.add(e, Approximated, "", "the flow is kept as a comment: its source "+describe(src.Parent)+" is not migrated, so no value reaches "+describe(tgt))
		a.keepOrder(e)
		return
	}
	st := a.objectSourceType(src, map[*sysmlv1.Element]bool{})
	if !a.through[tgt] && !a.m.conform(st, a.endType(tgt)) {
		a.m.w.line(flowNote + from + " to " + to + notWritten + qualifiedName(st) + " and " + qualifiedName(a.endType(tgt)) + " do not conform */")
		a.m.add(e, Approximated, "", "the flow is kept as a comment: its ends are typed by "+qualifiedName(st)+" and "+qualifiedName(a.endType(tgt))+", which do not conform")
		a.keepOrder(e)
		return
	}
	name := a.edgeFresh(e, spoken(from)+" to "+spoken(to))
	if name != "" {
		a.m.madeUp(e, writeName(name))
	}
	decl := "flow " + a.flowHead(name, nil) + from + " to " + to
	if a.successionFlow(e) {
		decl = "succession flow " + a.flowHead(name, st) + from + " to " + to
	}
	a.m.w.line(decl + ";")
	a.m.wroteEdgeAlso(e, a.def, "flow", nil, name)
	if a.dataOnly[e] {
		return
	}
	a.m.add(e, Mapped, a.m.edgeTarget(e), "")
}
