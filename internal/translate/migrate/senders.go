package migrate

import (
	"sort"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/translate/xmi/sysmlv1"
)

// senders indexes what posts each signal: the send actions the migration
// wrote, and the signal instances of the tool's UI prototype, which a button
// posts when it is pressed in the tool and the migration does not write. The
// accepts of a signal a button posts are noted once every send is written, since
// a send action is a sender only when it is written, not refused as a placeholder.
type senders struct {
	sent    map[*sysmlv1.Element]bool
	buttons map[*sysmlv1.Element]int
	accepts []uiAccept
}

// uiAccept is an accept of sig: the elements reported for it, which are noted
// when no send action of the document turns out to send sig.
type uiAccept struct {
	sig *sysmlv1.Element
	of  []*sysmlv1.Element
}

// recordSender notes e as a button of the tool's UI prototype posting a signal,
// when it is one.
func (m *migration) recordSender(e *sysmlv1.Element) {
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

// uiAccept records that the elements of are an accept of sig, to be noted by
// uiOnlyAccepts when a button of the tool's UI prototype posts sig or a signal
// specializing it and no send action is written for either.
func (m *migration) uiAccept(sig *sysmlv1.Element, of ...*sysmlv1.Element) {
	if sig == nil {
		return
	}
	if n, _ := m.buttonsPosting(sig); n > 0 {
		m.senders.accepts = append(m.senders.accepts, uiAccept{sig: sig, of: of})
	}
}

// uiOnlyAccepts notes, once every send action is written, each recorded accept
// whose signal only the tool's UI prototype posts.
func (m *migration) uiOnlyAccepts() {
	for _, acc := range m.senders.accepts {
		if m.sentByAction(acc.sig) {
			continue
		}
		note := m.unsentNote(acc.sig)
		for _, e := range acc.of {
			m.annotate(e, note, true)
		}
	}
}

// unsentNote says that the tool's UI prototype is what posts sig: no send action
// of the document sends it or a signal specializing it, only a button of the
// prototype, which the migration does not write, so an accept of it waits for a
// message nothing in the model posts. A signal nothing posts arrives from outside
// the model, in the tool and here alike, and gets no note.
func (m *migration) unsentNote(sig *sysmlv1.Element) string {
	n, special := m.buttonsPosting(sig)
	what := describe(sig)
	if len(special) > 0 {
		names := make([]string, len(special))
		for i, s := range special {
			names[i] = describe(s)
		}
		what += " or a signal specializing it (" + strings.Join(names, ", ") + ")"
	}
	return "no send action of the document sends " + what + ", which only " + count(n, "button") +
		" of the tool's UI prototype posts, which the migration does not write; an accept of it waits for a message nothing in the model posts"
}

// buttonsPosting counts the buttons of the tool's UI prototype posting sig or a
// signal specializing it, which an accept of sig takes too, and lists those
// specializing signals by id.
func (m *migration) buttonsPosting(sig *sysmlv1.Element) (n int, special []*sysmlv1.Element) {
	for posted, buttons := range m.senders.buttons {
		switch {
		case posted == sig:
			n += buttons
		case m.inherits(posted, sig):
			n += buttons
			special = append(special, posted)
		}
	}
	sort.Slice(special, func(i, j int) bool { return special[i].ID < special[j].ID })
	return n, special
}

// sentByAction reports whether a send action the migration wrote sends sig or a
// signal specializing it, either of which an accept of sig takes.
func (m *migration) sentByAction(sig *sysmlv1.Element) bool {
	if m.senders.sent[sig] {
		return true
	}
	for sent := range m.senders.sent {
		if m.inherits(sent, sig) {
			return true
		}
	}
	return false
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
// action of it calls, is written with: a duration constraint bounding it or one
// of its nodes, written as a wait before it, or an accept of a time event that is
// written. "" when none: a bound or a trigger the writer leaves as a placeholder
// is no wait.
func (m *migration) clockWait(b *sysmlv1.Element) string {
	return m.clockWaitIn(b, map[*sysmlv1.Element]bool{})
}

func (m *migration) clockWaitIn(b *sysmlv1.Element, seen map[*sysmlv1.Element]bool) string {
	if b == nil || b.Type != "Activity" || seen[b] {
		return ""
	}
	seen[b] = true
	if w := m.waitOf(b); w.ok {
		return "the duration constraint " + describe(w.dc) + " on " + describe(b)
	}
	found := ""
	m.walkActions(b, func(n *sysmlv1.Element) {
		if found != "" {
			return
		}
		switch n.Type {
		case "AcceptEventAction":
			if m.acceptsTime(n) {
				found = "the accept of a time event " + describe(n) + " in " + describe(b)
				return
			}
		case "CallBehaviorAction":
			found = m.calledWait(m.model.Ref(n, "behavior"), seen)
		case "CallOperationAction":
			if op := m.model.Ref(n, "operation"); op != nil {
				found = m.calledWait(m.bodyMethod(op), seen)
			}
		}
		if w := m.waitOf(n); found == "" && w.ok {
			found = "the duration constraint " + describe(w.dc) + " on " + describe(n) + " in " + describe(b)
		}
	})
	return found
}

// calledWait is the clock wait of a called behavior, which a call performs only
// when the behavior has a v2 declaration; a call of one without is a placeholder.
func (m *migration) calledWait(b *sysmlv1.Element, seen map[*sysmlv1.Element]bool) string {
	if !m.written(b) {
		return ""
	}
	return m.clockWaitIn(b, seen)
}

// acceptsTime reports whether the accept action n is written accepting a time
// event: its first trigger, the one written, names a time event whose time the
// writer spells.
func (m *migration) acceptsTime(n *sysmlv1.Element) bool {
	triggers := n.Owned("trigger")
	if len(triggers) == 0 {
		return false
	}
	ev := m.model.Ref(triggers[0], "event")
	if ev == nil || ev.Type != "TimeEvent" {
		return false
	}
	_, _, ok := m.acceptClause(ev, n.Parent, "")
	return ok
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
