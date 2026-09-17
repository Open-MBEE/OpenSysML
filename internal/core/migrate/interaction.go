package migrate

import (
	"strconv"

	"github.com/Open-MBEE/OpenSysML/internal/core/xmi"
)

// scenarioSend is one message of an interaction written as a send: the
// signal it carries and the part of the interaction's context that receives it.
type scenarioSend struct {
	msg    *xmi.Element
	signal *xmi.Element
	part   *xmi.Element
}

// interactionNote says why an interaction cannot be written as a scenario of
// sends: "" when every message is a signal sent to a part of the classifier
// that owns the interaction, in the order of its occurrences.
func (m *migration) interactionNote(e *xmi.Element) string {
	_, note := m.scenario(e)
	return note
}

// scenario resolves the messages of an interaction to the sends a scenario
// action def writes, in occurrence order.
func (m *migration) scenario(e *xmi.Element) ([]scenarioSend, string) {
	context := classifierOf(e)
	if context == nil {
		return nil, "the interaction belongs to no block whose parts its lifelines could stand for"
	}
	messages := e.Owned("message")
	if len(messages) == 0 {
		return nil, "the interaction has no message"
	}
	byOccurrence := map[*xmi.Element]int{}
	for i, f := range e.Owned("fragment") {
		byOccurrence[f] = i
	}
	sends := make([]scenarioSend, 0, len(messages))
	for _, msg := range messages {
		s, note := m.messageSend(msg, context, e)
		if note != "" {
			return nil, "the message " + describe(msg) + " " + note
		}
		sends = append(sends, s)
	}
	// Occurrences are listed in the order they happen; a message without a
	// send occurrence keeps its document order.
	order := func(s scenarioSend) int {
		if ev := m.model.Ref(s.msg, "sendEvent"); ev != nil {
			if i, ok := byOccurrence[ev]; ok {
				return i
			}
		}
		return len(byOccurrence) + indexOf(messages, s.msg)
	}
	for i := 1; i < len(sends); i++ {
		for j := i; j > 0 && order(sends[j]) < order(sends[j-1]); j-- {
			sends[j], sends[j-1] = sends[j-1], sends[j]
		}
	}
	return sends, ""
}

func indexOf(list []*xmi.Element, e *xmi.Element) int {
	for i, c := range list {
		if c == e {
			return i
		}
	}
	return -1
}

// messageSend resolves one message to a send: an asynchronous signal message
// whose signature is a migrated signal and whose receive occurrence covers a
// lifeline standing for a part of context.
func (m *migration) messageSend(msg, context, e *xmi.Element) (scenarioSend, string) {
	if sort := msg.Attrs["messageSort"]; sort != "asynchSignal" {
		if sort == "" {
			sort = "synchCall"
		}
		return scenarioSend{}, "is a " + sort + " message; only a signal send has a v2 form"
	}
	sig := m.model.Ref(msg, "signature")
	if sig == nil {
		return scenarioSend{}, "names no signal" + suffixNote(m.dangling(msg, "signature"))
	}
	if sig.Type != "Signal" || !m.written(sig) {
		return scenarioSend{}, "names " + describe(sig) + ", which is not a migrated signal"
	}
	recv := m.model.Ref(msg, "receiveEvent")
	if recv == nil {
		return scenarioSend{}, "has no receiving occurrence" + suffixNote(m.dangling(msg, "receiveEvent"))
	}
	line := m.model.Ref(recv, "covered")
	if line == nil || line.Type != "Lifeline" {
		return scenarioSend{}, "is received on no lifeline"
	}
	part := m.model.Ref(line, "represents")
	if part == nil {
		return scenarioSend{}, "is received on a lifeline standing for nothing" + suffixNote(m.dangling(line, "represents"))
	}
	if part.Type != "Property" || part.Parent != context || !m.written(part) {
		return scenarioSend{}, "is received on a lifeline standing for " + describe(part) + ", which is not a part of " + qualifiedName(context)
	}
	if firstOwned(line, "selector") != nil {
		return scenarioSend{}, "is received on a lifeline selecting one of several " + part.Name + ", which a send cannot address"
	}
	return scenarioSend{msg: msg, signal: sig, part: part}, ""
}

func suffixNote(s string) string {
	if s == "" {
		return ""
	}
	return " (" + s + ")"
}

// interactionBody writes an interaction as a scenario: its messages as sends
// to the parts their lifelines stand for, one after another.
func (m *migration) interactionBody(e *xmi.Element) {
	sends, note := m.scenario(e)
	if note != "" {
		// classifyBehavior does not let this happen; keep the body honest anyway.
		m.w.lines(commentLines("interaction not migrated — " + note))
		return
	}
	used := map[string]bool{"start": true, "done": true}
	prev := "start"
	for _, s := range sends {
		name := m.nameOf(s.msg)
		if name == "" {
			name = "send" + lowerFirst(m.nameFor(s.signal))
		}
		name = writeName(freshIn(used, name))
		args, anote := m.messageArguments(s.msg, s.signal, e)
		m.w.line("action " + name + " send new " + m.ref(s.signal, e) + "(" + args + ") to this." + writeName(m.nameOf(s.part)) + ";")
		m.w.line("first " + prev + " then " + name + ";")
		prev = name
		m.add(s.msg, verdictFor(anote), name, joinNotes("written as a send to the part "+m.nameOf(s.part), anote))
		for _, role := range []string{"sendEvent", "receiveEvent"} {
			if ev := m.model.Ref(s.msg, role); ev != nil {
				m.add(ev, Mapped, name, "the occurrence orders the send "+name)
			}
		}
	}
	m.w.line("first " + prev + " then done;")
	for _, l := range e.Owned("lifeline") {
		if part := m.model.Ref(l, "represents"); part != nil {
			m.add(l, Mapped, "this."+m.nameOf(part), "the lifeline stands for the part the sends address")
		} else {
			m.add(l, Skipped, "", "the lifeline receives no message")
		}
	}
	for _, f := range e.Owned("fragment") {
		if f.Type != "MessageOccurrenceSpecification" {
			m.unmapped(f, "a "+f.Type+" has no form in a scenario of sends")
		}
	}
	for _, r := range e.Owned("ownedRule") {
		m.unmapped(r, "a "+r.Type+" on an interaction has no form in a scenario of sends")
	}
	for _, o := range e.Owned("observation") {
		m.unmapped(o, "a "+o.Type+" has no v2 form")
	}
	m.add(e, Approximated, m.v2Name(e), "written as a scenario of "+strconv.Itoa(len(sends))+" sends, one per message in occurrence order; the lifelines' own behavior is not part of it")
}

// messageArguments writes a message's arguments as the signal's attribute
// values, by position against the signal's own attributes.
func (m *migration) messageArguments(msg, sig, scope *xmi.Element) (string, string) {
	args := msg.Owned("argument")
	if len(args) == 0 {
		return "", ""
	}
	attrs := sig.Owned("ownedAttribute")
	out := ""
	note := ""
	for i, arg := range args {
		if i >= len(attrs) {
			note = joinNotes(note, "the argument "+describeValue(arg)+" has no attribute of "+sig.Name+" to bind to and is dropped")
			continue
		}
		expr, ok, vnote := m.behaviorValue(arg, scope)
		if !ok {
			note = joinNotes(note, "the argument "+describeValue(arg)+" for "+attrs[i].Name+" is dropped: "+vnote)
			continue
		}
		if out != "" {
			out += ", "
		}
		out += writeName(m.nameOf(attrs[i])) + " = " + expr
		note = joinNotes(note, vnote)
	}
	return out, note
}
