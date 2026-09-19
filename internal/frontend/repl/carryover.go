package repl

import (
	"slices"
	"sort"

	"github.com/Open-MBEE/OpenSysML/internal/exec/runtime"
)

// carried is one object the session materialized, together with the shapes it
// was materialized against, recorded while the resolution that produced it is
// still the current one. Comparing those shapes against the ones the next
// document resolves is what decides whether the object still means the same
// thing.
type carried struct {
	fqn     string
	obj     *runtime.Instance
	shapes  *runtime.Shapes
	unnamed bool // a later %instantiate of fqn displaced it; only its id reaches it
}

// carryover is the state of a session that a submission may be able to keep: the
// objects %instantiate materialized, and the shapes the debugging sessions were
// started against.
type carryover struct {
	prev    *runtime.Context
	objects []carried
	action  *runtime.Shapes
	state   *runtime.Shapes
}

// recordCarryover takes the shapes of everything the session holds, before the
// document they were resolved against is replaced. It must run before the new
// text is opened: afterwards the old resolution is no longer there to record.
func (s *Session) recordCarryover() carryover {
	over := carryover{prev: s.rtCtx}
	if s.rtCtx == nil {
		return over
	}
	fqns := make([]string, 0, len(s.instances))
	for fqn := range s.instances {
		fqns = append(fqns, fqn)
	}
	sort.Strings(fqns)
	for _, fqn := range fqns {
		obj := s.instances[fqn]
		over.objects = append(over.objects, carried{fqn: fqn, obj: obj, shapes: s.rtCtx.ShapesOf(obj)})
	}
	for _, u := range s.unnamed {
		over.objects = append(over.objects, carried{fqn: u.fqn, obj: u.obj, shapes: s.rtCtx.ShapesOf(u.obj), unnamed: true})
	}
	if s.actionExec != nil {
		over.action = s.rtCtx.ShapesOfType(s.actionExec.symbol)
	}
	if s.stateExec != nil {
		over.state = s.rtCtx.ShapesOfType(s.stateExec.symbol)
	}
	return over
}

// carryOverObjects rebinds the session's objects to the new document's
// declarations, keeping those whose shape the reload still resolves to — a reload
// of unchanged text keeps them all — and reporting the ones it had to drop.
func (s *Session) carryOverObjects(over carryover) []string {
	held := s.heldObjects()
	if held == 0 {
		return nil
	}
	kept := make(map[string]*runtime.Instance, len(over.objects))
	var keptUnnamed []unnamedObject
	ctx, err := s.getOrCreateRuntime()
	var restarted []string
	for _, c := range over.objects {
		if err != nil {
			continue
		}
		again, adoptErr := ctx.Adopt(over.prev, c.shapes, c.obj)
		if adoptErr != nil {
			continue
		}
		if c.unnamed {
			keptUnnamed = append(keptUnnamed, unnamedObject{fqn: c.fqn, obj: c.obj})
		} else {
			kept[c.fqn] = c.obj
		}
		restarted = append(restarted, again...)
	}
	// Anything not carried over is gone, including an object whose resolution was
	// not there to record.
	dropped := held - len(kept) - len(keptUnnamed)
	s.instances = kept
	s.unnamed = keptUnnamed
	s.given = slices.DeleteFunc(s.given, func(fqn string) bool { return kept[fqn] == nil })
	var notices []string
	if note := behaviorsRestartedNotice(restarted); note != "" {
		notices = append(notices, note)
	}
	if dropped == 0 {
		// Nothing went this time, so an earlier loss is no longer what a listing of
		// the survivors has to explain: it was reported when it happened.
		s.lost = instanceLoss{}
		return notices
	}
	s.lost = lossAtSubmission(dropped, s.version)
	return append(notices, instancesDroppedNotice(dropped))
}

// goneNames collects the declarations a submission superseded.
func goneNames(drops []dropReport) []string {
	var gone []string
	for _, d := range drops {
		gone = append(gone, d.gone...)
	}
	return gone
}
