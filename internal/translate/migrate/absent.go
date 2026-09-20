package migrate

import (
	"slices"

	"github.com/Open-MBEE/OpenSysML/internal/translate/xmi/sysmlv1"
)

// admitAbsent marks, over every activity graph, the pins and parameters a value may
// fail to reach while v1 still runs the action; each is declared admitting no value.
func (m *migration) admitAbsent(activities []*sysmlv1.Element) {
	var graphs []*activity
	var gather func(act, def *sysmlv1.Element)
	gather = func(act, def *sysmlv1.Element) {
		a := m.newActivity(act, def)
		a.link()
		a.resolveData()
		a.deaden()
		graphs = append(graphs, a)
		for _, n := range a.nodes {
			if isStructured(n) {
				gather(n, n)
			}
		}
	}
	for _, act := range activities {
		def := act
		if op := m.methodOf[act]; op != nil {
			def = op
		}
		gather(act, def)
	}
	for changed := true; changed; {
		changed = false
		for _, a := range graphs {
			if a.admitAbsent() {
				changed = true
			}
		}
	}
}

// isStructured reports whether a node holds a graph of its own.
func isStructured(n *sysmlv1.Element) bool {
	switch n.Type {
	case "StructuredActivityNode", "ExpansionRegion", "LoopNode", "ConditionalNode", "SequenceNode":
		return true
	}
	return false
}

// admitAbsent marks one graph's pins fed by nothing carrying a value, the callee
// parameters its calls pass none or such a pin for, and the out parameters such
// pins feed; it reports whether any mark is new.
func (a *activity) admitAbsent() (changed bool) {
	for _, n := range a.nodes {
		if nodeKind(n) != nodeAction || a.dead[n] {
			continue
		}
		for _, pin := range inputPins(n) {
			if why := a.absentFeed(pin); why != "" && a.m.admitNone(pin, why) {
				changed = true
			}
		}
		callee, args := a.callArguments(n)
		if callee == nil {
			continue
		}
		i := 0
		for _, p := range a.m.actionParameters(callee) {
			if dir, _ := parameterDirection(p); dir == "out" {
				continue
			}
			if requiresValue(p) {
				why := ""
				switch {
				case i >= len(args):
					why = "the call " + describe(n) + " in " + qualifiedName(a.act) + " passes no argument for it"
				default:
					why = a.absentArgument(args[i])
				}
				if why != "" && a.m.admitNone(p, why+", and v1 runs the callee without one") {
					changed = true
				}
			}
			i++
		}
	}
	if a.act.Type != "Activity" {
		return changed
	}
	for _, p := range a.act.Owned("ownedParameter") {
		if dir, _ := parameterDirection(p); dir == "in" || !requiresValue(p) {
			continue
		}
		if why := a.absentResult(p); why != "" && a.m.admitNone(p, why) {
			changed = true
		}
	}
	return changed
}

// callArguments returns the behavior or operation a call node runs, when it is
// written as one an action performs, and the pins the call passes as arguments.
func (a *activity) callArguments(n *sysmlv1.Element) (callee *sysmlv1.Element, args []*sysmlv1.Element) {
	switch n.Type {
	case "CallBehaviorAction":
		callee = a.m.model.Ref(n, "behavior")
		if callee == nil {
			return nil, nil
		}
		if op := a.m.methodOf[callee]; op != nil {
			callee = op
		}
		args = inputPins(n)
	case "CallOperationAction":
		callee = a.m.model.Ref(n, "operation")
		t := firstOwned(n, "target")
		args = slices.DeleteFunc(inputPins(n), func(p *sysmlv1.Element) bool { return p == t })
	default:
		return nil, nil
	}
	if !a.m.written(callee) {
		return nil, nil
	}
	return callee, args
}

// absentArgument says why the pin a call passes may hold no value; "" when it holds one.
func (a *activity) absentArgument(pin *sysmlv1.Element) string {
	why, ok := a.m.admitsNone[pin]
	if !ok {
		return ""
	}
	holds := "may hold none"
	if a.valueless(pin) != nil {
		holds = "receives none"
	}
	return "the pin " + describe(pin) + " the call " + describe(pin.Parent) + " in " + qualifiedName(a.act) + " passes for it " + holds + ": " + why
}

// absentFeed says why a pin that must hold a value may receive none: only nodes
// producing none feed it, or a parameter or callee output admitting none does.
func (a *activity) absentFeed(pin *sysmlv1.Element) string {
	if pin.Type == "ValuePin" && firstOwned(pin, "value") != nil || a.selfFed[pin] || !requiresValue(pin) {
		return ""
	}
	if dry := a.valueless(pin); dry != nil {
		return describe(dry) + ", which feeds it, produces no value"
	}
	for _, s := range a.sources[pin] {
		if why := a.mayLack(s); why != "" {
			return why
		}
	}
	return ""
}

// absentResult says why an out parameter of the activity may be given no value:
// a pin admitting none flows into its node; "" when none does.
func (a *activity) absentResult(p *sysmlv1.Element) string {
	for _, e := range a.edges {
		tgt := a.m.model.Ref(e, "target")
		if tgt == nil || nodeKind(tgt) != nodeParam || a.m.model.Ref(tgt, "parameter") != p {
			continue
		}
		for _, s := range a.edgeSources[e] {
			if why := a.mayLack(s); why != "" {
				return why
			}
		}
	}
	return ""
}

// mayLack says why a producer may yield no value: it is the node of a parameter
// admitting none, or an output pin of a call whose callee's out parameter does.
func (a *activity) mayLack(s *sysmlv1.Element) string {
	switch nodeKind(s) {
	case nodeParam:
		p := a.m.model.Ref(s, "parameter")
		if p != nil && a.m.lacksValue(p) {
			return "the parameter " + a.m.nameFor(p) + " of " + qualifiedName(a.act) + ", which feeds it, admits no value"
		}
	case nodePin:
		if !a.producesAt(s) {
			return ""
		}
		if callee, p := a.calleeOutput(s); p != nil && a.m.lacksValue(p) {
			return "the parameter " + a.m.nameFor(p) + " of " + qualifiedName(callee) + ", which feeds it through " + describe(s) + ", admits no value"
		}
	}
	return ""
}

// admitNone records why a parameter or pin is declared admitting no value, marking
// the operation parameter a method's stands for too; it reports whether the mark is new.
func (m *migration) admitNone(p *sysmlv1.Element, why string) bool {
	if m.lacksValue(p) {
		return false
	}
	m.admitsNone[p] = why
	if op := m.realizes[p]; op != nil {
		m.admitsNone[op] = why
	}
	return true
}

// lacksValue reports whether a parameter or pin is declared admitting no value.
func (m *migration) lacksValue(p *sysmlv1.Element) bool {
	if _, ok := m.admitsNone[p]; ok {
		return true
	}
	if op := m.realizes[p]; op != nil {
		_, ok := m.admitsNone[op]
		return ok
	}
	return false
}
