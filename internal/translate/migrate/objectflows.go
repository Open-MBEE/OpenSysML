package migrate

import (
	"slices"

	"github.com/Open-MBEE/OpenSysML/internal/translate/xmi/sysmlv1"
)

// successionFlow reports whether the object flow e is written as a succession flow,
// which orders its target action after its source as well as carrying the value:
// it joins pins of two actions of the activity that fire, under no guard but true,
// from no streaming parameter to none, a value travels it, and it is the succession
// its target waits on; or it enters or leaves a node passing objects through as the
// succession between its ends, under no guard. Settled once the data waits are.
func (a *activity) successionFlow(e *sysmlv1.Element) bool {
	if v, ok := a.succFlow[e]; ok {
		return v
	}
	src, tgt := a.m.model.Ref(e, "source"), a.m.model.Ref(e, "target")
	var v bool
	switch {
	case a.through[src] || a.through[tgt]:
		v = e.Type == "ObjectFlow" && !realGuard(e) && !a.dataOnly[e] && slices.Contains(a.succ[ownerNode(src)], e)
	default:
		v = a.joinsActions(e) && !realGuard(e) && slices.Contains(a.succ[src.Parent], e) && a.carriesValue(e)
	}
	a.succFlow[e] = v
	return v
}

// joinsActions reports whether e is an object flow from a pin of one action of the
// activity to a pin of another, neither of which never fires.
func (a *activity) joinsActions(e *sysmlv1.Element) bool {
	src, tgt := a.m.model.Ref(e, "source"), a.m.model.Ref(e, "target")
	if e.Type != "ObjectFlow" || src == nil || tgt == nil || nodeKind(src) != nodePin || nodeKind(tgt) != nodePin {
		return false
	}
	from, to := src.Parent, tgt.Parent
	return from != nil && to != nil && from != to && nodeKind(from) == nodeAction && nodeKind(to) == nodeAction &&
		slices.Contains(a.nodes, from) && slices.Contains(a.nodes, to) && a.starved[from] == nil && a.starved[to] == nil
}

// carriesValue reports whether a value travels the object flow e between two pins
// as a flow written between them, from no streaming parameter to none.
func (a *activity) carriesValue(e *sysmlv1.Element) bool {
	src, tgt := a.m.model.Ref(e, "source"), a.m.model.Ref(e, "target")
	if a.edgeSelf[e] || !slices.Equal(a.edgeSources[e], []*sysmlv1.Element{src}) || a.streams(src) || a.streams(tgt) {
		return false
	}
	if callee, p := a.calleeOutput(src); callee != nil && (p == nil || a.m.dryOutputs(callee)[p]) {
		return false
	}
	if !a.writesOutput(src) || !a.writesInput(tgt) {
		return false
	}
	return a.producesAt(src) && a.m.conform(a.endType(src), a.endType(tgt))
}

// writesOutput reports whether the output pin is declared as a parameter of its
// action that a flow names, where its action writes a value to it.
func (a *activity) writesOutput(pin *sysmlv1.Element) bool {
	n := pin.Parent
	if !slices.Contains(outputPins(n), pin) {
		return false
	}
	switch n.Type {
	case "CallBehaviorAction", "CallOperationAction", "OpaqueAction", "ValueSpecificationAction":
		return true
	case "ReadStructuralFeatureAction":
		return a.readWritten(n)
	}
	return false
}

// writesInput reports whether the input pin is declared as a parameter of its
// action that a flow names, not folded into what the action writes.
func (a *activity) writesInput(pin *sysmlv1.Element) bool {
	n := pin.Parent
	if !slices.Contains(inputPins(n), pin) {
		return false
	}
	switch n.Type {
	case "CallOperationAction":
		return firstOwned(n, "target") != pin && a.hasParameter(n, pin)
	case "CallBehaviorAction":
		return a.hasParameter(n, pin)
	case "OpaqueAction", "AddStructuralFeatureValueAction", "SendSignalAction":
		return true
	case "ReadStructuralFeatureAction":
		if !a.readWritten(n) {
			return false
		}
		_, _, folded := a.objectOf(pin)
		return !a.selfFed[pin] && len(a.sources[pin]) > 0 && !folded
	}
	return false
}

// readWritten reports whether a read of a structural feature is written with its
// result, as readFeature writes it: its feature is declared and the object read has it.
func (a *activity) readWritten(n *sysmlv1.Element) bool {
	f := a.m.model.Ref(n, "structuralFeature")
	if f == nil || !a.m.written(f) || a.m.nameOf(f) == "" || len(n.Owned("result")) == 0 {
		return false
	}
	_, t, ok := a.readObject(firstOwned(n, "object"))
	return !ok || a.m.mayHaveFeature(t, f)
}

// realGuard reports whether e carries a guard other than the literal true, which
// UML reads as no guard.
func realGuard(e *sysmlv1.Element) bool {
	g := firstOwned(e, "guard")
	return g != nil && (g.Type != "LiteralBoolean" || g.Attrs["value"] != "true" && g.Attrs["value"] != "")
}

// guardedFlow reports whether e is an object flow between pins of two actions of the
// activity whose guard the succession between them is written with.
func (a *activity) guardedFlow(e *sysmlv1.Element) bool {
	return realGuard(e) && a.joinsActions(e)
}

// streams reports whether pin stands for a streaming parameter of the behavior its
// action calls: values pass it while the behavior runs, not when it completes.
func (a *activity) streams(pin *sysmlv1.Element) bool {
	if pin == nil || nodeKind(pin) != nodePin || pin.Parent == nil {
		return false
	}
	var callee *sysmlv1.Element
	switch pin.Parent.Type {
	case "CallBehaviorAction":
		callee = a.m.model.Ref(pin.Parent, "behavior")
	case "CallOperationAction":
		callee = a.m.model.Ref(pin.Parent, "operation")
	}
	if callee == nil {
		return false
	}
	pins, out := inputPins(pin.Parent), false
	if slices.Contains(outputPins(pin.Parent), pin) {
		pins, out = outputPins(pin.Parent), true
	}
	i := slices.Index(pins, pin)
	if i < 0 {
		return false
	}
	for _, p := range a.m.actionParameters(callee) {
		d := p.Attrs["direction"]
		if d == "inout" || out == (d == "out" || d == "return") {
			if i == 0 {
				return p.Attrs["isStream"] == "true"
			}
			i--
		}
	}
	return false
}

// flowEntry names what the succession of a succession flow into n is written to: the
// stamp or wait n's performance follows, past any join gathering its other edges; ""
// where the succession flow itself leads to n.
func (a *activity) flowEntry(n *sysmlv1.Element) string {
	if s, ok := a.before[n]; ok {
		return s.name
	}
	if w, ok := a.waits[n]; ok {
		return w.name
	}
	return ""
}

// orderedPrev counts the nodes before n an edge other than a succession flow leads
// from, which a join gathers when several.
func (a *activity) orderedPrev(n *sysmlv1.Element) int {
	count := 0
	for _, from := range a.prev[n] {
		for _, e := range a.succ[from] {
			if ownerNode(a.m.model.Ref(e, "target")) == n && !a.successionFlow(e) {
				count++
				break
			}
		}
	}
	return count
}

// keepOrder writes the succession a succession flow e that carries no written value
// stands for, so its target still follows its source.
func (a *activity) keepOrder(e *sysmlv1.Element) {
	if !a.successionFlow(e) {
		return
	}
	from, to := ownerNode(a.m.model.Ref(e, "source")), ownerNode(a.m.model.Ref(e, "target"))
	for _, x := range a.succ[from] {
		if ownerNode(a.m.model.Ref(x, "target")) == to && (!a.successionFlow(x) || a.flowEntry(to) != "") {
			return
		}
	}
	name, ok := a.names[from]
	entry := a.flowEntry(to)
	if entry == "" {
		entry = a.endpointIn(to)
	}
	if !ok || entry == "" {
		return
	}
	a.m.w.line(firstKw + writeName(name) + thenKw + entry + ";")
}

// hasParameter reports whether a call's input pin is written: an untyped call
// declares every pin, a typed one only those its callee has an in parameter for.
func (a *activity) hasParameter(n, pin *sysmlv1.Element) bool {
	callee := a.typedCallee(n)
	if callee == nil {
		return true
	}
	ins := 0
	for _, p := range a.m.actionParameters(callee) {
		switch p.Attrs["direction"] {
		case "out", "return":
		default:
			ins++
		}
	}
	t := firstOwned(n, "target")
	pins := slices.DeleteFunc(inputPins(n), func(p *sysmlv1.Element) bool { return n.Type == "CallOperationAction" && p == t })
	return slices.Index(pins, pin) < ins
}

// typedCallee returns the definition a call is written typed by, as callBehavior
// and callOperation write it; nil when its pins are declared untyped.
func (a *activity) typedCallee(n *sysmlv1.Element) *sysmlv1.Element {
	if _, _, refused := a.refusal(n); refused {
		return nil
	}
	if n.Type == "CallOperationAction" {
		return a.m.model.Ref(n, "operation")
	}
	b := a.m.model.Ref(n, "behavior")
	if b == nil || a.m.primitiveCalled(n) != nil {
		return nil
	}
	if op := a.m.methodOf[b]; op != nil {
		b = op
	}
	if cat, _ := a.m.classify(b); cat == catActionDef {
		return b
	}
	return nil
}
