package migrate

import (
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/translate/xmi/sysmlv1"
)

// senders indexes what posts each signal: the send and broadcast actions of the
// model, and the signal instances of the tool's UI prototype, which a button
// posts when it is pressed in the tool and the migration does not write.
type senders struct {
	actions map[*sysmlv1.Element]bool
	buttons map[*sysmlv1.Element]int
}

// recordSender notes e as a sender of a signal, when it is one.
func (m *migration) recordSender(e *sysmlv1.Element) {
	switch e.Type {
	case "SendSignalAction", "BroadcastSignalAction":
		if sig := m.model.Ref(e, "signal"); sig != nil {
			m.senders.actions[sig] = true
		}
	}
	for _, s := range e.Stereotypes {
		if !simulationProvenance.applies(s, "SignalInstance") {
			continue
		}
		for _, id := range s.Tags["element"] {
			if sig := m.model.Lookup(id); sig != nil && sig.Type == "Signal" {
				m.senders.buttons[sig]++
			}
		}
	}
}

// unsentNote says that the tool's UI prototype is what posts sig: no send action
// of the document sends it, only a button of the prototype, which the migration
// does not write, so an accept of it waits for a message nothing in the model
// posts. "" when a send action names it or nothing at all does: a signal nothing
// posts arrives from outside the model, in the tool and here alike.
func (m *migration) unsentNote(sig *sysmlv1.Element) string {
	if sig == nil || m.senders.actions[sig] || m.senders.buttons[sig] == 0 {
		return ""
	}
	return "no send action of the document sends " + describe(sig) + ", which only " + count(m.senders.buttons[sig], "button") +
		" of the tool's UI prototype posts, which the migration does not write; an accept of it waits for a message nothing in the model posts"
}

// uiNote says that a configuration's UI is the tool's UI prototype, which the
// migration does not write: a run of the tool takes a user's input through it, a
// run here gets none from it. ids are the UI tag's values.
func (m *migration) uiNote(ids []string) string {
	var frames []string
	for _, id := range ids {
		e := m.model.Lookup(id)
		if e == nil {
			frames = append(frames, id)
			continue
		}
		frames = append(frames, uiFrameName(e))
	}
	return simConfig + "UI = " + strings.Join(frames, ", ") + " is the tool's UI prototype, through which a user of the tool's run posts signals and reads values; it has no v2 form, so a run here takes no input from it"
}

// uiFrameName names a UI prototype frame: its title in the tool, else the element.
func uiFrameName(e *sysmlv1.Element) string {
	for _, s := range e.Stereotypes {
		if toolProfile(s.Namespace) == "" {
			continue
		}
		for _, tag := range []string{"Title", "title"} {
			if vs := s.Tags[tag]; len(vs) == 1 && vs[0] != "" {
				return "'" + vs[0] + "'"
			}
		}
	}
	return describe(e)
}

// clockWait names the first wait on the clock the behavior b, or a behavior an
// action of it calls, is written with: a duration constraint bounding one of its
// nodes, written as a wait before it, or an accept of a time event. "" when none.
func (m *migration) clockWait(b *sysmlv1.Element) string {
	return m.clockWaitIn(b, map[*sysmlv1.Element]bool{})
}

func (m *migration) clockWaitIn(b *sysmlv1.Element, seen map[*sysmlv1.Element]bool) string {
	if b == nil || b.Type != "Activity" || seen[b] {
		return ""
	}
	seen[b] = true
	found := ""
	m.walkActions(b, func(n *sysmlv1.Element) {
		if found != "" {
			return
		}
		switch n.Type {
		case "AcceptEventAction":
			for _, t := range n.Owned("trigger") {
				if ev := m.model.Ref(t, "event"); ev != nil && ev.Type == "TimeEvent" {
					found = "the accept of a time event " + describe(n) + " in " + describe(b)
					return
				}
			}
		case "CallBehaviorAction":
			found = m.clockWaitIn(m.model.Ref(n, "behavior"), seen)
		}
		if found == "" && len(m.bounded[n]) > 0 {
			found = "the duration constraint " + describe(m.bounded[n][0]) + " on " + describe(n) + " in " + describe(b)
		}
	})
	return found
}

// instantWaitNote says that a behavior performed at an instant — a state's entry
// or exit action or a transition's effect, never its do action — waits for the
// clock, which a run refuses at the wait; "" for a do action or a behavior that
// does not wait.
func (m *migration) instantWaitNote(kw string, b, owner *sysmlv1.Element) string {
	what := kw
	switch {
	case owner.Type == "Transition":
		what = "transition effect"
	case kw == doAction:
		return ""
	}
	wait := m.clockWait(b)
	if wait == "" {
		return ""
	}
	return "it waits for the clock (" + wait + "), which a v2 " + what + ", performed whole at the instant it is triggered, may not: a run stops at the wait"
}
