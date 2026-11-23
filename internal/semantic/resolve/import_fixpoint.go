package resolve

import (
	"slices"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// enumFixpoint enumerates an import's members as a least fixpoint over the import graph (issue
// #633): a pass reads each edge once, one met again answers what it has, until none grows.
type enumFixpoint struct {
	gen    int                      // the route the fixpoint is over (Resolver.routeGen)
	over   []*symbols.Scope         // the namespaces that route excludes
	approx map[enumEdge]keptMembers // every edge's members as of the last pass that read it
	done   map[enumEdge]fixedEdge   // the edges read in the pass in progress
	active map[enumEdge]bool        // the edges being read
	// children, reading and brought are done, active and approx for the namespaces
	// read: what one brings is read once a pass for every import that takes it.
	children map[fixedChildren]fixedEdge
	reading  map[fixedChildren]bool
	brought  map[fixedChildren]keptMembers
	read     map[*symbols.Scope]readNamespace // each namespace read, and how
	finals   []fixedEdge
	// collided keeps collidedBelow's answers while collisionEpoch stays at epoch.
	collided map[collidedName]collidedAnswer
	epoch    int
	// changed: an edge grew in this pass; mixed: a namespace was read both ways;
	// settled: the last pass grew none.
	changed, mixed, settled bool
}

// fixedEdge is an edge's answer within a pass; final when no edge it read was
// still growing, and across when it is kept for later enumerations (keepEdge).
type fixedEdge struct {
	keptMembers
	edge          enumEdge
	final, across bool
	frame         *frame // the document frame the answer's reads were made in
}

// replayFixed makes the read in progress depend on what an answer of this pass
// read: the frame that made the reads already holds them.
func (r *Resolver) replayFixed(fixed fixedEdge) {
	if r.cur != fixed.frame {
		fixed.reads.replay(r)
	} else if top := r.topCapture(); top != nil && fixed.reads != nil {
		top.adopt(fixed.reads)
	}
	r.replayRoute(fixed.route)
}

// collidedName is a name asked of collidedBelow, and collidedAnswer what it
// answered: the memberships hiding it, what the walk read, and whether it was cut.
type collidedName struct {
	scope *symbols.Scope
	name  string
}

type collidedAnswer struct {
	hidden []importedMember
	reads  *readCapture
	cut    bool
}

// fixedChildren is a namespace's memberships as one kind of import takes them.
type fixedChildren struct {
	target       *symbols.Symbol
	all, unnamed bool
}

// fixpoint is the fixpoint in progress over the route in progress, if any.
func (r *Resolver) fixpoint() *enumFixpoint {
	if r.enumFix != nil && r.enumFix.gen == r.routeGen {
		return r.enumFix
	}
	return nil
}

// enumReading is reading within an enumeration: a fixpoint excludes only what its route
// already did, as the edge in progress or an earlier pass answers for a walk coming back.
func (r *Resolver) enumReading(scope *symbols.Scope, f func()) {
	if r.fixpoint() != nil {
		f()
		return
	}
	r.reading(scope, f)
}

// enter records how a namespace is read; one read in two ways makes the fixpoint mixed.
func (fix *enumFixpoint) enter(read readNamespace) {
	if was, entered := fix.read[read.scope]; entered && was != read {
		fix.mixed = true
	}
	fix.read[read.scope] = read
}

// settle records an edge's answer for the pass, noting whether it grew.
func (fix *enumFixpoint) settle(edge enumEdge, kept keptMembers, frame *frame, final, across bool) {
	fix.changed = fix.changed || grew(fix.approx[edge], kept)
	fix.approx[edge] = kept
	fixed := fixedEdge{keptMembers: kept, edge: edge, final: final, across: final && across, frame: frame}
	fix.done[edge] = fixed
	if final {
		fix.finals = append(fix.finals, fixed)
	}
}

// bring records what a namespace brings for the pass, noting whether it grew.
func (fix *enumFixpoint) bring(key fixedChildren, kept keptMembers, frame *frame, final bool) {
	fix.changed = fix.changed || grew(fix.brought[key], kept)
	fix.brought[key] = kept
	fix.children[key] = fixedEdge{keptMembers: kept, final: final, frame: frame}
}

// grew reports whether an answer holds more than the one before it did.
func grew(was, now keptMembers) bool {
	return len(now.members) != len(was.members) || len(now.route.cut) != len(was.route.cut) || len(now.route.entered) != len(was.route.entered)
}

// edgeInProgress answers for an import being read by the fixpoint in progress
// with what the last pass found it to bring; own is false for any other import.
func (r *Resolver) edgeInProgress(imp *ast.Import, includeUnnamed bool) (members []*symbols.Symbol, own bool) {
	fix := r.fixpoint()
	edge := enumEdge{imp: imp, unnamed: includeUnnamed}
	if fix == nil || !fix.active[edge] {
		return nil, false
	}
	r.enumApprox++
	was := fix.approx[edge]
	r.replayRoute(was.route)
	return was.members, true
}

// enumerateToFixpoint enumerates key by passes until no edge grows. It reports false for a namespace
// read in two ways (readNamespace): a walk through that one is no route of KerML 8.3.2.4.5.
func (r *Resolver) enumerateToFixpoint(key enumResult, admission func() func(*symbols.Symbol) bool) ([]*symbols.Symbol, bool) {
	outer := r.enumFix
	defer func() { r.enumFix = outer }()
	// An import in progress, an unresolved target or a filter condition makes
	// the answers this read's alone, not the next enumeration's to start from.
	share := r.enumShare
	if r.inCondition != 0 || len(r.resolvingImports) != 0 || len(r.importStack) != 0 {
		share = nil
	}
	var fix *enumFixpoint
	for pass := 0; ; pass++ {
		cut := r.enumCut
		mark := r.markRoute()
		var members []*symbols.Symbol
		reads := r.capturing(func() {
			r.reading(key.scope, func() {
				if pass == 0 {
					fix = r.fixpointOver(share)
					r.enumFix = fix
				}
				fix.begin()
				fix.gen = r.routeGen
				members = r.enumerateImported(key.scope, key.imp, key.unnamed, admission)
				fix.gen = -1
			})
		})
		route := r.routeDepsSince(mark, key.scope)
		// Each pass but the last grows an edge, and an edge grows by a member
		// or a namespace: more passes than that is no monotone enumeration.
		if fix.mixed || pass > (len(fix.approx)+len(fix.brought))*(len(fix.read)+1) {
			if share != nil {
				share.last = nil
			}
			return nil, false
		}
		if fix.changed {
			continue
		}
		for _, fixed := range fix.finals {
			r.keepEdge(fixed.edge, fixed.keptMembers, fixed.across)
		}
		if r.enumCut == cut && r.keepsEnumeration(route) {
			journalNew(r, r.enumResults, key, key.imp)
			r.enumResults[key] = keptMembers{members: members, reads: reads, route: route}
		}
		if share != nil && r.enumCut == cut {
			fix.settled = true
			share.last, share.epoch = fix, r.collisionEpoch
		}
		return members, true
	}
}

// fixShare is the fixpoint the enumerations of one namespace's imports share (collectImported):
// they exclude the same namespaces, so what the first found each to bring the next ones read.
type fixShare struct {
	last  *enumFixpoint
	epoch int // collisionEpoch as last settled
}

// sharingFixpoints runs f with its enumerations sharing a fixpoint.
func (r *Resolver) sharingFixpoints(f func()) {
	outer := r.enumShare
	r.enumShare = &fixShare{}
	defer func() { r.enumShare = outer }()
	f()
}

// fixpointOver is the fixpoint to enumerate with: the shared one when it settled over the same
// excluded namespaces and no namespace's collisions changed since, else a new one.
func (r *Resolver) fixpointOver(share *fixShare) *enumFixpoint {
	if share != nil && share.last != nil && share.epoch == r.collisionEpoch && slices.Equal(share.last.over, r.importPath) {
		return share.last
	}
	return &enumFixpoint{
		gen:      -1,
		over:     slices.Clone(r.importPath),
		approx:   map[enumEdge]keptMembers{},
		done:     map[enumEdge]fixedEdge{},
		active:   map[enumEdge]bool{},
		read:     map[*symbols.Scope]readNamespace{},
		children: map[fixedChildren]fixedEdge{},
		reading:  map[fixedChildren]bool{},
		brought:  map[fixedChildren]keptMembers{},
		collided: map[collidedName]collidedAnswer{},
		epoch:    r.collisionEpoch,
	}
}

// begin starts a pass: every edge and namespace is read anew, but for the first
// pass over a settled fixpoint, whose answers are already the least ones.
func (fix *enumFixpoint) begin() {
	fix.changed = false
	fix.finals = fix.finals[:0]
	if !fix.settled {
		clear(fix.done)
		clear(fix.children)
	}
	fix.settled = false
}
