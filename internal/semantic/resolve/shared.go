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
	// children are the captures settled or replayed inside this one, each
	// recorded once however deep the computation nested; folded into the lists
	// above the first time c is replayed (flatten).
	children []*readCapture
	flat     sync.Once
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

// record replays c into the current frame, its children folded in.
func (c *readCapture) record(r *Resolver) {
	c.flat.Do(c.flatten)
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

// flatten folds the reads of c's children, each once, into c's own lists.
func (c *readCapture) flatten() {
	if len(c.children) == 0 {
		return
	}
	seen := map[*readCapture]bool{c: true}
	var fold func(*readCapture)
	fold = func(d *readCapture) {
		for _, child := range d.children {
			if seen[child] {
				continue
			}
			seen[child] = true
			c.names = append(c.names, child.names...)
			c.namespaces = append(c.namespaces, child.namespaces...)
			c.docs = append(c.docs, child.docs...)
			c.segments = append(c.segments, child.segments...)
			c.deps = append(c.deps, child.deps...)
			c.all = c.all || child.all
			c.spellings = c.spellings || child.spellings
			fold(child)
		}
	}
	fold(c)
	c.children = nil
	c.settle()
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
// settled; an enclosing capture takes it as a child, its computation made the
// same reads.
func (r *Resolver) capturing(compute func()) *readCapture {
	c := &readCapture{}
	r.captures = append(r.captures, c)
	defer func() {
		r.captures = r.captures[:len(r.captures)-1]
		if top := r.topCapture(); top != nil {
			top.adopt(c.settle())
		}
	}()
	compute()
	return c
}
