package migrate

import (
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/xmi/sysmlv1"
)

// instantValue names a TimeInstantValue attribute a behavior declares for an
// absolute time event, with the note the event's time carries.
type instantValue struct {
	name, note string
}

// signalOf says why a signal event's signal is not written as an accept: the
// event names none, or names one with no v2 declaration; ok when it is written.
func (m *migration) signalOf(ev *sysmlv1.Element) (note string, ok bool) {
	sig := m.model.Ref(ev, "signal")
	switch {
	case sig == nil:
		return joinNotes("the signal event names no signal", m.dangling(ev, "signal")), false
	case sig.IsProxy():
		return "the signal " + qualifiedName(sig) + " it names is a proxy for an element of another document, which has no v2 declaration here", false
	case !m.written(sig):
		return "the signal " + qualifiedName(sig) + " it names has no v2 declaration in the document", false
	}
	return "", true
}

// instants declares, in the body of behavior b, a TimeInstantValue attribute for
// each absolute time event a trigger of b refers to, which `accept at` reads;
// the triggers of a behavior nested in b declare theirs in their own body.
func (m *migration) instants(b *sysmlv1.Element, used map[string]bool) {
	var evs []*sysmlv1.Element
	seen := map[*sysmlv1.Element]bool{}
	var walk func(e *sysmlv1.Element)
	walk = func(e *sysmlv1.Element) {
		if e != b && isBehavior(e) {
			return
		}
		if e.Type == "Trigger" && e.Role == "trigger" {
			ev := m.model.Ref(e, "event")
			if ev != nil && ev.Type == "TimeEvent" && ev.Attrs["isRelative"] != "true" && !seen[ev] {
				seen[ev] = true
				evs = append(evs, ev)
			}
		}
		for _, c := range e.Children {
			walk(c)
		}
	}
	walk(b)
	for _, ev := range evs {
		d, ok, note := m.instantExpr(ev, b)
		if !ok {
			continue
		}
		name := lowerFirst(m.nameOf(ev))
		if name == "" {
			name = "instant"
		}
		name = freshIn(used, name)
		m.w.line("attribute " + writeName(name) + " : Time::TimeInstantValue = " + d + ";")
		if m.instant[b] == nil {
			m.instant[b] = map[*sysmlv1.Element]instantValue{}
		}
		m.instant[b][ev] = instantValue{name: name, note: note}
	}
}

// instantExpr writes the instant an absolute time event names as a
// TimeInstantValue in seconds from the start of the clock: a duration literal
// or an expression read in scope; else ok is false with why.
func (m *migration) instantExpr(ev, scope *sysmlv1.Element) (expr string, ok bool, note string) {
	d, ok, note := m.durationExpr(firstOwned(ev, "when"), scope)
	if !ok {
		return "", false, strings.Replace(note, "the duration", "the instant", 1)
	}
	note = strings.Replace(note, "the duration", "the instant", 1)
	return d + " [SI::s]", true, joinNotes(note, "the absolute time is an instant on the simulation clock, which starts at 0.0 [SI::s]")
}

// instantRef names the TimeInstantValue attribute an absolute time event's accept
// reads in scope, declared by the enclosing behavior; the note says why none is.
func (m *migration) instantRef(ev, scope *sysmlv1.Element) (name, note string, ok bool) {
	for cur := scope; cur != nil; cur = cur.Parent {
		if iv, ok := m.instant[cur][ev]; ok {
			return writeName(iv.name), iv.note, true
		}
	}
	if _, ok, note := m.instantExpr(ev, scope); !ok {
		return "", "the time event's time is not written: " + note, false
	}
	return "", "an absolute time event is accepted through a TimeInstantValue attribute, which only a state machine or activity declares for its own triggers", false
}
