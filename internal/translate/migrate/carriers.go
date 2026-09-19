package migrate

import (
	"strconv"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/translate/xmi/sysmlv1"
)

// carrier is the signal every transition into a state accepts, kept in an item of the
// state def so the state's entry and do behaviors take their parameters from it.
type carrier struct {
	holder string
	sig    *sysmlv1.Element
	attrs  []*sysmlv1.Element
}

// carriers declares, for each state whose entry or do behavior takes parameters, the item
// holding the incoming signal whose properties match them by position, type, order and multiplicity.
// Internal transitions enter no state, so they neither settle the signal nor rule it out.
func (m *migration) carriers(sm *sysmlv1.Element, used map[string]bool) {
	incoming := map[*sysmlv1.Element][]*sysmlv1.Element{}
	var states []*sysmlv1.Element
	var walk func(e *sysmlv1.Element)
	walk = func(e *sysmlv1.Element) {
		if e != sm && (isBehavior(e) || e.Type == "StateMachine") {
			return
		}
		switch {
		case e.Type == "Transition" && e.Role == "transition" && e.Attrs["kind"] != "internal":
			if tgt := m.model.Ref(e, "target"); tgt != nil {
				incoming[tgt] = append(incoming[tgt], e)
			}
		case e.Type == "State" && e.Role == "subvertex":
			states = append(states, e)
		}
		for _, c := range e.Children {
			walk(c)
		}
	}
	walk(sm)
	for _, a := range sm.Owned("ownedAttribute") {
		used[m.nameFor(a)] = true
	}
	for _, v := range states {
		behaviors := m.parameterizedBehaviors(v)
		if len(behaviors) == 0 {
			continue
		}
		sig, why := m.carrierSignal(v, incoming[v])
		if why == "" {
			why = m.carrierMatch(sig, behaviors)
		}
		if why != "" {
			m.carrierNotes[v] = why
			continue
		}
		for _, t := range incoming[v] {
			if eff := firstOwned(t, "effect"); eff != nil {
				for _, p := range eff.Owned("ownedParameter") {
					used[m.nameFor(p)] = true
				}
			}
		}
		used[m.nameFor(sig)] = true
		holder := freshIn(used, lowerFirst(m.nameFor(sig)))
		m.w.line("item " + writeName(holder) + " : " + m.ref(sig, sm) + ";")
		m.carrierOf[v] = &carrier{holder: holder, sig: sig, attrs: m.signalAttributes(sig)}
		m.add(v, Approximated, "", "the parameters of its entry and do actions take the attributes of "+
			m.nameFor(sig)+", the signal every transition into it accepts and keeps in "+holder+
			", as a simulation passes a signal's properties to the parameters of a state's behaviors matching them by position and type")
	}
}

// parameterizedBehaviors lists the entry and do behaviors a state owns that
// take parameters, which a carrier would value.
func (m *migration) parameterizedBehaviors(v *sysmlv1.Element) []*sysmlv1.Element {
	var out []*sysmlv1.Element
	for _, role := range []string{"entry", "doActivity"} {
		b := firstOwned(v, role)
		if b != nil && b.Parent == v && len(inParameters(b)) > 0 {
			out = append(out, b)
		}
	}
	return out
}

// carrierSignal returns the one signal every transition into state v accepts, or why
// there is none.
func (m *migration) carrierSignal(v *sysmlv1.Element, incoming []*sysmlv1.Element) (*sysmlv1.Element, string) {
	if len(incoming) == 0 {
		return nil, "no transition enters the state"
	}
	if r := v.Parent; r != nil {
		for _, sib := range r.Owned("subvertex") {
			if k := pseudoKind(sib); k == "shallowHistory" || k == "deepHistory" {
				return nil, "the region's " + k + " pseudostate re-enters the state with no signal"
			}
		}
	}
	var sig *sysmlv1.Element
	for _, t := range incoming {
		src := m.model.Ref(t, "source")
		if src != nil && src.Type == "Pseudostate" {
			return nil, "the transition from the " + pseudoKind(src) + " pseudostate " + describe(src) + " enters the state with no signal of its own"
		}
		triggers := t.Owned("trigger")
		if len(triggers) == 0 {
			return nil, "the transition from " + describe(src) + " enters the state with no trigger"
		}
		for _, tr := range triggers {
			ev := m.model.Ref(tr, "event")
			if ev == nil || ev.Type != "SignalEvent" {
				return nil, "the transition from " + describe(src) + " accepts " + eventKind(ev) + ", which carries no signal"
			}
			if note, ok := m.signalOf(ev); !ok {
				return nil, "the transition from " + describe(src) + " accepts a signal with no v2 declaration: " + note
			}
			s := m.model.Ref(ev, "signal")
			if sig != nil && s != sig {
				return nil, "the transitions into the state accept different signals, " + m.nameFor(sig) + " and " + m.nameFor(s)
			}
			sig = s
		}
		if eff := firstOwned(t, "effect"); eff != nil {
			switch {
			case eff.Parent != t:
				return nil, "the effect of the transition from " + describe(src) + " is written once, as its own action def, which cannot keep the accepted signal"
			case eff.Type != "Activity" && eff.Type != "OpaqueBehavior" && eff.Type != "FunctionBehavior":
				return nil, "the effect of the transition from " + describe(src) + " is " + aOrAn(eff.Type) + ", which has no action form to keep the accepted signal in"
			}
		}
	}
	if cat, _ := m.classify(sig); cat != catItemDef {
		return nil, "the signal " + m.nameFor(sig) + " is written as " + aOrAn(cat.keyword()) + ", which no item holds"
	}
	return sig, ""
}

// eventKind names an event for a diagnostic, or its absence.
func eventKind(ev *sysmlv1.Element) string {
	if ev == nil {
		return "no event"
	}
	return aOrAn(ev.Type)
}

// aOrAn prefixes a noun with its indefinite article.
func aOrAn(noun string) string {
	if strings.ContainsRune("aeiouAEIOU", rune(noun[0])) {
		return "an " + noun
	}
	return "a " + noun
}

// carrierMatch says why the parameters of a state's behaviors do not take the attributes
// of sig, inherited ones included: their number, position, type, order or multiplicity differ.
func (m *migration) carrierMatch(sig *sysmlv1.Element, behaviors []*sysmlv1.Element) string {
	attrs := m.signalAttributes(sig)
	for _, b := range behaviors {
		params := inParameters(b)
		if len(params) != len(attrs) {
			return describe(b) + " takes " + count(len(params), "parameter") + ", but " + m.nameFor(sig) + " has " + count(len(attrs), "attribute")
		}
		for i, p := range params {
			a := attrs[i]
			if why := m.carrierPair(a, p); why != "" {
				return "the parameter " + m.nameFor(p) + " of " + describe(b) + " does not take the attribute " + m.nameFor(a) + " of " + m.nameFor(sig) + " at its position: " + why
			}
		}
	}
	return ""
}

// carrierPair says why attribute a cannot value parameter p: a type of a is not
// p's or a special of it, or their order or multiplicity differ.
func (m *migration) carrierPair(a, p *sysmlv1.Element) string {
	at, pt := m.model.Ref(a, "type"), m.model.Ref(p, "type")
	switch {
	case at == nil || pt == nil:
		if at != pt {
			return "one is untyped and the other is not"
		}
	case at != pt && !m.inherits(at, pt) && !m.sameScalar(at, pt):
		return "the attribute is typed by " + m.nameFor(at) + " and the parameter by " + m.nameFor(pt)
	}
	if a.Attrs["isOrdered"] != p.Attrs["isOrdered"] {
		return "one is ordered and the other is not"
	}
	am, _ := m.multiplicity(a)
	pm, _ := m.multiplicity(p)
	if am != pm {
		return "their multiplicities differ"
	}
	return ""
}

// sameScalar reports whether two types stand for the same scalar value type,
// as two references to the same primitive type through different libraries do.
func (m *migration) sameScalar(a, b *sysmlv1.Element) bool {
	sa := m.scalarBase(a)
	return sa != "" && sa == m.scalarBase(b)
}

// count writes n nouns, pluralised.
func count(n int, noun string) string {
	if n == 1 {
		return "1 " + noun
	}
	return strconv.Itoa(n) + " " + noun + "s"
}

// carrierBindings binds the parameters of a state's entry or do behavior to
// the attributes of the signal its carrier holds, and reports each binding.
func (m *migration) carrierBindings(v, b *sysmlv1.Element) map[*sysmlv1.Element]string {
	c := m.carrierOf[v]
	if c == nil {
		return nil
	}
	bound := map[*sysmlv1.Element]string{}
	for i, p := range inParameters(b) {
		bound[p] = writeName(c.holder) + "." + writeName(m.nameFor(c.attrs[i]))
	}
	return bound
}

// carrierWhy says why the parameters of a state's kw take no value from the
// signal a transition into the state accepts.
func (m *migration) carrierWhy(v *sysmlv1.Element, kw string) string {
	if kw == "exit action" {
		return "an exit runs before the effect of the transition leaving the state, which alone receives the accepted signal"
	}
	if why := m.carrierNotes[v]; why != "" {
		return "the signal the transitions into the state accept would value them, but " + why
	}
	return "only a transition's effect receives the accepted signal"
}

// keep writes the statement a transition's effect keeps the accepted signal
// with, for the carrier of the state it enters, or nothing.
func (m *migration) keep(tgt *sysmlv1.Element, sig *sysmlv1.Element, payload string) string {
	c := m.carrierOf[tgt]
	if c == nil || c.sig != sig || payload == "" {
		return ""
	}
	return "assign " + writeName(c.holder) + " := " + writeName(payload) + ";"
}
