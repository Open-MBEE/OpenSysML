package repl

import (
	"errors"
	"fmt"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
)

// dropReport records what one submission did to a declaration already in the
// buffer: it either merged into it (keeping the members the new body does not
// redeclare) or replaced it, in which case everything the superseded snippet
// declared and this submission does not is gone.
type dropReport struct {
	merged bool
	decl   string   // the declaration this submission re-declared, e.g. "package P"
	lost   []string // members no longer declared, e.g. "part def A"
	// gone names the declarations this submission superseded, qualified, e.g.
	// "P::A". Anything running against one of them (or something under it) is
	// stale; what the submission left alone is not.
	gone []string
	// reopened names a namespace a loaded file opened again, which stays a
	// declaration of its own rather than contributing to the earlier one.
	reopened string
}

// notice renders the report as the line the user is told about it, so no
// declaration ever disappears silently.
func (d dropReport) notice() string {
	if d.reopened != "" {
		name := notationName(d.reopened)
		return fmt.Sprintf("note: %s is opened by more than one loaded file; each opening stays a declaration of its own, so a member of one is not visible unqualified in the other — qualify it (%s::member)",
			name, name)
	}
	if d.decl == "" {
		return ""
	}
	switch {
	case d.merged && len(d.lost) == 0:
		return fmt.Sprintf("note: added to the existing %s (its other members are kept)", d.decl)
	case d.merged:
		return fmt.Sprintf("note: added to the existing %s, replacing %s", d.decl, strings.Join(d.lost, ", "))
	case len(d.lost) == 0:
		return fmt.Sprintf("note: replaced %s", d.decl)
	default:
		return fmt.Sprintf("note: replaced %s (%s no longer declared)", d.decl, strings.Join(d.lost, ", "))
	}
}

// dropNotices renders the reports of one submission, skipping the ones with
// nothing to say.
func dropNotices(drops []dropReport) []string {
	var out []string
	for _, d := range drops {
		if note := d.notice(); note != "" {
			out = append(out, note)
		}
	}
	return out
}

// replacedReport describes a snippet a submission supersedes: what it
// re-declared, and what went with it because the whole snippet was replaced —
// both the snippet's other top-level declarations and, for a re-declared
// namespace, the members its new body does not declare again.
func replacedReport(sn snippet, redeclared map[string]bool, newTop map[string]ast.Node) dropReport {
	root := parser.New(source.New(parseDocName(sn.origin), []byte(sn.src))).ParseFile()
	rep := dropReport{}
	if root == nil {
		return rep
	}
	for _, m := range root.Members {
		name := memberName(m)
		if name == "" {
			continue
		}
		desc := renderMember(m)
		if desc == "" {
			desc = name
		}
		// The whole snippet goes, so every name it declared is superseded — the
		// re-declared ones by the new text, the rest by being dropped with it.
		rep.gone = append(rep.gone, name)
		if redeclared[name] {
			if rep.decl == "" {
				rep.decl = desc
			}
			rep.lost = append(rep.lost, lostMembers(m, newTop[name])...)
			continue
		}
		rep.lost = append(rep.lost, desc)
	}
	return rep
}

// lostMembers lists the members of a re-declared namespace that its new body
// leaves undeclared, so replacing a body names what it took with it. A member
// re-declared as a namespace on both sides is descended into: only what neither
// body declares is lost.
func lostMembers(om, nm ast.Node) []string {
	if nm == nil {
		return nil
	}
	oldMembers, ok := bodyMembers(om)
	if !ok {
		return nil
	}
	// A new declaration with no body of its own declares none of the members,
	// so all of them are lost.
	newMembers, _ := bodyMembers(nm)
	var lost []string
	for _, m := range oldMembers {
		name := memberName(m)
		if name == "" {
			continue
		}
		_, kept := findNamed(newMembers, name)
		if kept == nil {
			desc := renderMember(m)
			if desc == "" {
				desc = name
			}
			lost = append(lost, desc)
			continue
		}
		lost = append(lost, lostMembers(m, kept)...)
	}
	return lost
}

// instancesDroppedNotice reports the instances a submission invalidated. A new
// document is a new AST, so the objects materialized from the previous one — and
// the IDs they were given — do not carry over.
func instancesDroppedNotice(n int) string {
	if n == 0 {
		return ""
	}
	return fmt.Sprintf("note: %s dropped because the declarations changed; re-run %%instantiate", instanceCount(n))
}

// instancesResetNotice reports the instances a reset took with it. The document
// they were materialized from is gone entirely, so there is nothing left to
// carry them over to.
func instancesResetNotice(n int) string {
	if n == 0 {
		return ""
	}
	return fmt.Sprintf("note: %s dropped because the session was reset; re-run %%instantiate", instanceCount(n))
}

// behaviorsRestartedNotice reports the behaviors a carry-over started again. An
// execution belongs to the analysis it started in, so a rebuilt model runs the
// behavior from its initial state rather than continuing it on stale values.
func behaviorsRestartedNotice(restarted []string) string {
	switch len(restarted) {
	case 0:
		return ""
	case 1:
		return fmt.Sprintf("note: the %s was restarted from its initial state because the model was rebuilt", restarted[0])
	default:
		return fmt.Sprintf("note: %d behaviors of carried-over objects were restarted from their initial states because the model was rebuilt", len(restarted))
	}
}

// instanceLoss records the objects the session no longer holds and what took
// them, so a command that finds none says which it is: a session that never
// materialized anything, or one whose objects are gone.
type instanceLoss struct {
	count int
	// why is what took them, phrased for the notes below to read with.
	why string
}

// lossAtSubmission records objects a submission's changed declarations dropped.
func lossAtSubmission(n, version int) instanceLoss {
	return instanceLoss{count: n, why: fmt.Sprintf("when the declarations changed at submission %d", version)}
}

// lossOnReset records objects a reset dropped, which belongs to no submission.
func lossOnReset(n int) instanceLoss {
	return instanceLoss{count: n, why: "when the session was reset"}
}

// lossOnBudgets records objects dropped with the runtime context their IDs came
// from, which new run bounds replace.
func lossOnBudgets(n int) instanceLoss {
	return instanceLoss{count: n, why: "when the run bounds were changed"}
}

// goneNote explains an empty instance list that was not always empty, which
// %instances would otherwise report the same way as a fresh session.
func (l instanceLoss) goneNote() string {
	if l.count == 0 {
		return ""
	}
	return fmt.Sprintf("(no instances created; %s dropped %s — re-run %%instantiate)",
		instanceCount(l.count), l.why)
}

// partlyGoneNote reports the objects dropped alongside the ones that were kept,
// which a list of the survivors would not otherwise mention.
func (l instanceLoss) partlyGoneNote() string {
	if l.count == 0 {
		return ""
	}
	return fmt.Sprintf("(%s also dropped %s — re-run %%instantiate)",
		instanceCount(l.count), l.why)
}

// lostNote explains an object a command asked for that the session no longer
// holds.
func (l instanceLoss) lostNote() string {
	if l.count == 0 {
		return ""
	}
	return fmt.Sprintf("  %s dropped %s — re-run %%instantiate",
		instanceCount(l.count), l.why)
}

// instanceCount words a number of instances, as every drop notice reports it.
func instanceCount(n int) string {
	return countOf(n, "instance was", "instances were")
}

func countOf(n int, one, many string) string {
	if n == 1 {
		return "1 " + one
	}
	return fmt.Sprintf("%d %s", n, many)
}

// endedSession is why a debugging session is no longer there: the submission
// that invalidated it, so the next command can say what happened instead of only
// that nothing is active.
type endedSession struct {
	kind     string // "action" or "state machine"
	name     string // the behavior that was being debugged
	rootName string // the declaration whose resubmission ended it
	// objectGone records that what ended it was the loss of the object performing
	// the behavior, which rootName then names.
	objectGone bool
	// outside is the cause when no submission ended it — a reset or new run
	// bounds — which names no declaration.
	outside string
	version int // the submission that did it
}

// The causes outside any submission that end a debugging session.
const (
	sessionReset  = "the session was reset"
	boundsChanged = "the run bounds were changed"
)

// reason explains an ended debugging session in the past tense, for a command
// that found none active.
func (e *endedSession) reason() string {
	if e == nil {
		return ""
	}
	if e.outside != "" {
		return fmt.Sprintf("the %s session for %q ended when %s", e.kind, e.name, e.outside)
	}
	if e.objectGone {
		return fmt.Sprintf("the %s session for %q ended when the object %s performing it was dropped at submission %d",
			e.kind, e.name, e.rootName, e.version)
	}
	return fmt.Sprintf("the %s session for %q ended when %s was redeclared at submission %d",
		e.kind, e.name, e.rootName, e.version)
}

// noSessionMsg is what a debugging command reports when nothing is active: why
// the session it would have used is gone, when that is known, and how to start
// one either way.
func noSessionMsg(what string, ended *endedSession, start string) string {
	return "error: " + noSessionText(what, ended, start)
}

// noSessionText is the same message as an error's text, for a command that
// reports its failure as an error rather than as a line.
func noSessionText(what string, ended *endedSession, start string) string {
	msg := "no active " + what
	if reason := ended.reason(); reason != "" {
		msg += ": " + reason
	}
	if start == "" {
		return msg
	}
	return msg + fmt.Sprintf(" (use %s <name> first)", start)
}

// noActionSessionMsg reports that %step and friends have no action executor to
// drive, naming the submission that ended the last one.
func (s *Session) noActionSessionMsg() string {
	return noSessionMsg("action session", s.endedAction, "%action")
}

// noActionSessionErr is noActionSessionMsg as an error.
func (s *Session) noActionSessionErr() error {
	return errors.New(noSessionText("action session", s.endedAction, "%action"))
}

// noStateSessionErr is noStateSessionMsg as an error.
func (s *Session) noStateSessionErr() error {
	return errors.New(noSessionText("state machine session", s.endedState, "%state"))
}

// noStateSessionMsg is the same for the state machine debugger.
func (s *Session) noStateSessionMsg() string {
	return noSessionMsg("state machine session", s.endedState, "%state")
}

// noDebugSessionMsg answers a command that would drive either debugger,
// explaining whichever session ended most recently.
func (s *Session) noDebugSessionMsg() string {
	return noSessionMsg("debugging session", s.mostRecentlyEnded(), "")
}

// mostRecentlyEnded is the debugger session that ended last, nil when none did.
func (s *Session) mostRecentlyEnded() *endedSession {
	ended := s.endedAction
	if s.endedState != nil && (ended == nil || s.endedState.version >= ended.version) {
		ended = s.endedState
	}
	return ended
}
