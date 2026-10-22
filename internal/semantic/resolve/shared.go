package resolve

import (
	"slices"
	"sync"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// SharedCollisions is the import-collision memo (importedCollisions) the
// documents of one batch share: each is analyzed in a resolver of its own,
// and without it each walked the chain of namespaces its imports re-export
// from, so a batch did quadratic work in the chain's depth. An entry is what
// one resolver settled, with what it read of the index to settle it, which a
// resolver taking the entry reads again so its document's record still
// depends on all of it. The index is fixed for a batch, so what one namespace's
// imports bring is the same from every document.
type SharedCollisions struct {
	mu      sync.RWMutex
	entries map[*symbols.Scope]*sharedCollision
	below   map[*symbols.Scope]*sharedBelow
}

type sharedCollision struct {
	c     *importCollisions
	reads *readCapture
}

// sharedBelow is one settled hidesBelow answer with what settling it read.
type sharedBelow struct {
	hides bool
	reads *readCapture
}

// NewSharedCollisions returns an empty memo for one batch to share.
func NewSharedCollisions() *SharedCollisions {
	return &SharedCollisions{
		entries: map[*symbols.Scope]*sharedCollision{},
		below:   map[*symbols.Scope]*sharedBelow{},
	}
}

func (s *SharedCollisions) getBelow(scope *symbols.Scope) (*sharedBelow, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	e, ok := s.below[scope]
	return e, ok
}

func (s *SharedCollisions) putBelow(scope *symbols.Scope, hides bool, reads *readCapture) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, had := s.below[scope]; !had {
		s.below[scope] = &sharedBelow{hides: hides, reads: reads}
	}
}

func (s *SharedCollisions) get(scope *symbols.Scope) (*sharedCollision, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	e, ok := s.entries[scope]
	return e, ok
}

func (s *SharedCollisions) put(scope *symbols.Scope, c *importCollisions, reads *readCapture) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, had := s.entries[scope]; !had {
		s.entries[scope] = &sharedCollision{c: c, reads: reads}
	}
}

// ShareCollisions has r read and settle import collisions through shared.
func (r *Resolver) ShareCollisions(shared *SharedCollisions) {
	if r != nil {
		r.shared = shared
	}
}

// readCapture gathers what the index answered while one computation ran, and
// the documents it came to depend on (Resolver.Depend, EnterDoc: a memoized
// answer it took depends on them in turn), so a shared result can be read
// again by whoever takes it, with the provenance a frame taking the same
// memoized answers would have.
type readCapture struct {
	names, namespaces, docs, segments, deps []string
	all, spellings                          bool
	// children are the captures settled or replayed inside this one; flat is
	// their reads folded in with c's own, made on c's first replay (flatten).
	children []*readCapture
	flat     *readCapture
	once     sync.Once
}

// settle sorts and dedupes the capture once its computation is over.
func (c *readCapture) settle() *readCapture {
	for _, list := range []*[]string{&c.names, &c.namespaces, &c.docs, &c.segments, &c.deps} {
		slices.Sort(*list)
		*list = slices.Compact(*list)
	}
	return c
}

// replay records c's reads into the current frame and hands c to the capture
// open, if any, as a child: the capture's own lists stay the size of its reads.
func (c *readCapture) replay(r *Resolver) {
	if c == nil {
		return
	}
	if top := r.topCapture(); top != nil {
		top.adopt(c)
	}
	if r.cur == nil {
		return
	}
	open := r.captures
	r.captures = nil
	c.record(r)
	r.captures = open
}

// record replays c and its children into the current frame.
func (c *readCapture) record(r *Resolver) {
	c.once.Do(c.flatten)
	if c.flat != nil {
		c = c.flat
	}
	for _, doc := range c.deps {
		r.Depend(doc)
	}
	for _, fqn := range c.names {
		r.ReadName(fqn)
	}
	for _, fqn := range c.namespaces {
		r.ReadNamespace(fqn)
	}
	for _, name := range c.docs {
		r.ReadDocument(name)
	}
	for _, name := range c.segments {
		r.ReadSegment(name)
	}
	if c.all {
		r.ReadAllNames()
	}
	if c.spellings {
		r.ReadSpellings()
	}
}

// flatten makes c.flat: c's reads and those of its children, each once. A
// settled capture is never written again (replays run on parallel batch
// workers), so flatten reads the children's own lists and writes only c.flat.
func (c *readCapture) flatten() {
	if len(c.children) == 0 {
		return
	}
	flat := &readCapture{
		names: slices.Clip(c.names), namespaces: slices.Clip(c.namespaces),
		docs: slices.Clip(c.docs), segments: slices.Clip(c.segments),
		deps: slices.Clip(c.deps), all: c.all, spellings: c.spellings,
	}
	seen := map[*readCapture]bool{c: true}
	var fold func(*readCapture)
	fold = func(d *readCapture) {
		for _, child := range d.children {
			if seen[child] {
				continue
			}
			seen[child] = true
			flat.names = append(flat.names, child.names...)
			flat.namespaces = append(flat.namespaces, child.namespaces...)
			flat.docs = append(flat.docs, child.docs...)
			flat.segments = append(flat.segments, child.segments...)
			flat.deps = append(flat.deps, child.deps...)
			flat.all = flat.all || child.all
			flat.spellings = flat.spellings || child.spellings
			fold(child)
		}
	}
	fold(c)
	c.flat = flat.settle()
}

func (c *readCapture) adopt(child *readCapture) {
	if n := len(c.children); n == 0 || c.children[n-1] != child {
		c.children = append(c.children, child)
	}
}

// topCapture is the innermost read capture open, nil outside one.
func (r *Resolver) topCapture() *readCapture {
	if n := len(r.captures); n > 0 {
		return r.captures[n-1]
	}
	return nil
}

// capturing runs compute with a fresh capture on the stack and returns it
// settled; an enclosing capture, whose computation made the same reads, adopts it.
func (r *Resolver) capturing(compute func()) *readCapture {
	c := &readCapture{}
	r.captures = append(r.captures, c)
	defer func() {
		r.captures = r.captures[:len(r.captures)-1]
		c.settle()
		if top := r.topCapture(); top != nil {
			top.adopt(c)
		}
	}()
	compute()
	return c
}
