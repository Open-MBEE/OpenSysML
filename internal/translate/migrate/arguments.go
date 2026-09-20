package migrate

import (
	"slices"

	"github.com/Open-MBEE/OpenSysML/internal/translate/xmi/sysmlv1"
)

// refusal says why a node is written as a placeholder that carries the token and
// performs nothing, and with which verdict; refused is false when the node is
// written. The writer asks, and so does the pass finding nodes that produce no value.
func (a *activity) refusal(n *sysmlv1.Element) (why string, v Verdict, refused bool) {
	switch n.Type {
	case "ValueSpecificationAction":
		v := firstOwned(n, "value")
		if v == nil {
			return "the action has no value", Unmapped, true
		}
		var ok bool
		var note string
		if results := n.Owned("result"); len(results) == 0 {
			_, ok, note = a.m.behaviorValue(v, n)
		} else {
			_, ok, note = a.m.typedBehaviorValue(v, results[0], n)
		}
		if !ok {
			return "the value " + describeValue(v) + " is not written: " + note, Approximated, true
		}
	case "CallBehaviorAction":
		b := a.m.model.Ref(n, "behavior")
		if b == nil {
			return joinNotes(a.m.dangling(n, "behavior"), "the action calls no behavior"), Unmapped, true
		}
		if op := a.m.methodOf[b]; op != nil {
			b = op
		}
		if !a.m.written(b) {
			return "the behavior " + qualifiedName(b) + " it calls has no v2 declaration", Unmapped, true
		}
		switch cat, _ := a.m.classify(b); cat {
		case catCalcDef:
			return "", Mapped, false
		case catActionDef:
		default:
			return "the behavior " + qualifiedName(b) + " is written as a " + cat.keyword() + ", which an action cannot call", Unmapped, true
		}
		if p, why := a.unarguedParameter(inputPins(n), b); p != nil {
			return why, Approximated, true
		}
		if c := a.m.contextOf(b); c != nil {
			if expr, cnote := a.contextArgument(c); expr == "" {
				return a.uncontexted(b, cnote), Approximated, true
			}
		}
	case "CallOperationAction":
		op := a.m.model.Ref(n, "operation")
		if op == nil {
			return joinNotes(a.m.dangling(n, "operation"), "the action calls no operation"), Unmapped, true
		}
		if !a.m.written(op) {
			return "the operation " + qualifiedName(op) + " it calls has no v2 declaration", Unmapped, true
		}
		t := firstOwned(n, "target")
		ins := slices.DeleteFunc(inputPins(n), func(p *sysmlv1.Element) bool { return p == t })
		if p, why := a.unarguedParameter(ins, op); p != nil {
			return why, Approximated, true
		}
	case "SendSignalAction":
		sig := a.m.model.Ref(n, "signal")
		if sig == nil || !a.m.written(sig) {
			return "", Mapped, false
		}
		attrs := a.m.signalAttributes(sig)
		pins := n.Owned("argument")
		for i, attr := range attrs {
			if !requiresValue(attr) {
				continue
			}
			if i >= len(pins) {
				return a.unargued(sig, attr), Approximated, true
			}
			if dry := a.valueless(pins[i]); dry != nil {
				return a.dryArgument(pins[i], sig, attr, dry), Approximated, true
			}
			if pt, at := a.misfit(pins[i], attr); pt != nil {
				return a.misfitArgument(pins[i], sig, attr, pt, at), Approximated, true
			}
		}
	}
	return "", Mapped, false
}

// deaden marks the nodes written as placeholders, whose result pins carry no
// value, until no further call turns on a value none of them produces.
func (a *activity) deaden() {
	for changed := true; changed; {
		changed = false
		for _, n := range a.nodes {
			if a.dead[n] || nodeKind(n) != nodeAction {
				continue
			}
			if _, _, refused := a.refusal(n); refused {
				a.dead[n] = true
				changed = true
			}
		}
	}
}

// produces reports whether a node's output pins may carry a value: a refused call
// and an opaque action, whose pins stay unwritten, produce none.
func (a *activity) produces(n *sysmlv1.Element) bool {
	return !a.dead[n] && n.Type != "OpaqueAction"
}

// producesAt reports whether an output pin may carry a value: its node must
// produce one, and a call's pin must stand for a parameter its callee gives a value.
func (a *activity) producesAt(pin *sysmlv1.Element) bool {
	n := pin.Parent
	if n == nil || !a.produces(n) {
		return false
	}
	callee, p := a.calleeOutput(pin)
	if callee != nil && p == nil {
		return false
	}
	return p == nil || !a.m.dryOutputs(callee)[p]
}

// calleeOutput returns the activity a call node's output pin takes its value from and
// the out parameter it stands for, by position; the callee alone when none is left for it.
func (a *activity) calleeOutput(pin *sysmlv1.Element) (callee, param *sysmlv1.Element) {
	n := pin.Parent
	switch n.Type {
	case "CallBehaviorAction":
		callee = a.m.model.Ref(n, "behavior")
	case "CallOperationAction":
		if op := a.m.model.Ref(n, "operation"); op != nil {
			callee = a.m.model.Ref(op, "method")
		}
	}
	if callee == nil || callee.Type != "Activity" {
		return nil, nil
	}
	i := slices.Index(append(n.Owned("result"), n.Owned("outputValue")...), pin)
	for _, p := range callee.Owned("ownedParameter") {
		switch p.Attrs["direction"] {
		case "out", "return", "inout":
			if i == 0 {
				return callee, p
			}
			i--
		}
	}
	return callee, nil
}

// dryOutputs lists the out parameters of an activity no value reaches: no flow
// leads to their nodes, or only nodes producing no value feed them. The graph is
// linked as for writing, without writing; a call cycle finds its own outputs produced.
func (m *migration) dryOutputs(b *sysmlv1.Element) map[*sysmlv1.Element]bool {
	if dry, settled := m.dryOut[b]; settled {
		return dry
	}
	m.dryOut[b] = nil
	def := b
	if op := m.methodOf[b]; op != nil {
		def = op
	}
	a := m.newActivity(b, def)
	a.link()
	a.resolveData()
	a.deaden()
	a.awaitData()
	dry := map[*sysmlv1.Element]bool{}
	for _, p := range b.Owned("ownedParameter") {
		switch p.Attrs["direction"] {
		case "out", "return", "inout":
			if a.dryParameter(p) {
				dry[p] = true
			}
		}
	}
	m.dryOut[b] = dry
	return dry
}

// dryParameter reports whether no value reaches the nodes of an out parameter.
func (a *activity) dryParameter(p *sysmlv1.Element) bool {
	for _, e := range a.edges {
		tgt := a.m.model.Ref(e, "target")
		if tgt == nil || nodeKind(tgt) != nodeParam || a.m.model.Ref(tgt, "parameter") != p {
			continue
		}
		if a.edgeSelf[e] {
			return false
		}
		for _, s := range a.edgeSources[e] {
			if nodeKind(s) != nodePin || a.producesAt(s) {
				return false
			}
		}
	}
	return true
}

// dryFlow reports whether no value travels an object flow: every pin it comes from
// produces none, and every parameter takes none.
func (a *activity) dryFlow(e *sysmlv1.Element) bool {
	if a.edgeSelf[e] || len(a.edgeSources[e]) == 0 {
		return false
	}
	for _, s := range a.edgeSources[e] {
		switch {
		case nodeKind(s) == nodeParam && a.m.unvalued[a.m.model.Ref(s, "parameter")]:
		case nodeKind(s) == nodePin && (a.inert[s.Parent] || !a.producesAt(s)):
		default:
			return false
		}
	}
	return true
}

// valueless returns a node whose pin feeds a pin that no value reaches: every
// pin flowing into it belongs to a node that produces none and every parameter
// takes none. A pin nothing feeds, or only such parameters do, never fires instead.
func (a *activity) valueless(pin *sysmlv1.Element) *sysmlv1.Element {
	if pin.Type == "ValuePin" && firstOwned(pin, "value") != nil || a.selfFed[pin] {
		return nil
	}
	var dry *sysmlv1.Element
	for _, s := range a.sources[pin] {
		switch {
		case nodeKind(s) == nodeParam && a.m.unvalued[a.m.model.Ref(s, "parameter")]:
		case nodeKind(s) == nodePin && !a.producesAt(s):
			dry = s.Parent
		default:
			return nil
		}
	}
	return dry
}

// unarguedParameter returns the first in or inout parameter the action def of a
// called behavior or operation declares that must hold a value (no default, lower
// bound above 0) but that no argument pin of the call stands for, or whose pin no
// value reaches, and says which; nil when every such parameter is served.
func (a *activity) unarguedParameter(args []*sysmlv1.Element, callee *sysmlv1.Element) (*sysmlv1.Element, string) {
	i := 0
	for _, p := range a.m.actionParameters(callee) {
		if dir, _ := parameterDirection(p); dir == "out" || dir == "return" {
			continue
		}
		if requiresValue(p) {
			if i >= len(args) {
				return p, a.unargued(callee, p)
			}
			if dry := a.valueless(args[i]); dry != nil {
				return p, a.dryArgument(args[i], callee, p, dry)
			}
		}
		i++
	}
	return nil, ""
}

// requiresValue reports whether a parameter or property must hold a value: it
// has no default and its lower bound is not 0.
func requiresValue(p *sysmlv1.Element) bool {
	if firstOwned(p, "defaultValue") != nil {
		return false
	}
	lv := firstOwned(p, "lowerValue")
	return lv == nil || boundValue(lv) != "0"
}

// unargued says why a call or send is a placeholder: the callee or signal requires
// an argument the action never passes, where v1 would run it holding no value.
func (a *activity) unargued(callee, p *sysmlv1.Element) string {
	who, what, does := "call", "parameter", "runs the callee"
	if callee.Type == "Signal" {
		who, what, does = "send", "attribute", "sends the signal"
	}
	return "the " + who + " passes no argument for the " + what + " " + a.m.nameFor(p) + " of " + qualifiedName(callee) + ", which must hold a value; v1 " + does + " without it, which v2 does not admit, so the action carries the token and performs nothing"
}

// dryArgument says why a call or send is a placeholder: the pin it passes for a
// required parameter or signal attribute is fed by flows no value travels.
func (a *activity) dryArgument(pin, callee, p, dry *sysmlv1.Element) string {
	what, does := "parameter", "runs the callee"
	if callee.Type == "Signal" {
		what, does = "attribute", "sends the signal"
	}
	return "the pin " + describe(pin) + " it passes for the " + what + " " + a.m.nameFor(p) + " of " + qualifiedName(callee) + ", which must hold a value, receives none: " + describe(dry) + ", which feeds it, produces no value; v1 " + does + " without it, which v2 does not admit, so the action carries the token and performs nothing"
}

// misfit returns the types of an argument pin and of the signal attribute it stands
// for when the attribute cannot take the pin's type; nil, nil when it can or is unknown.
func (a *activity) misfit(pin, attr *sysmlv1.Element) (pt, at *sysmlv1.Element) {
	pt, at = a.m.model.Ref(pin, "type"), a.m.model.Ref(attr, "type")
	if pt == nil || at == nil || pt == at || a.m.inherits(pt, at) || !a.m.written(at) {
		return nil, nil
	}
	return pt, at
}

// misfitArgument says why a send is a placeholder: the pin it passes for a
// required signal attribute holds a type the attribute cannot take.
func (a *activity) misfitArgument(pin, sig, attr, pt, at *sysmlv1.Element) string {
	return "the pin " + describe(pin) + " it passes for the attribute " + a.m.nameFor(attr) + " of " + qualifiedName(sig) + ", which must hold a value, is a " + qualifiedName(pt) + ", which " + a.m.nameFor(attr) + " : " + qualifiedName(at) + " cannot take; v1 sends the signal without it, which v2 does not admit, so the action carries the token and performs nothing"
}

// uncontexted says why a call is a placeholder: the caller holds no object the
// callee acts on, where v1 would run it on the caller's object, whose ports it lacks.
func (a *activity) uncontexted(callee *sysmlv1.Element, why string) string {
	return why + "; v1 runs " + qualifiedName(callee) + " on the caller's object, which lacks the ports it goes through, so the action carries the token and performs nothing"
}
