package passes

import (
	"sort"
	"sync"

	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// Gathers holds what the workspace-wide audits gathered from each document, and the
// unions they judge over; kept current by Regather and readable concurrently once built.
type Gathers struct {
	mu sync.Mutex
	// docs is the set of workspace documents gathered; nil until first use.
	docs     map[string]bool
	oosem    *oosemUnion
	mosa     *mosaUnion
	identity *identityUnion
}

// NewGathers returns gathers with nothing gathered yet.
func NewGathers() *Gathers { return &Gathers{} }

// Reset forgets every gather, for a change that moves them all.
func (g *Gathers) Reset() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.docs, g.oosem, g.mosa, g.identity = nil, nil, nil, nil
}

// documents lists the workspace documents gathered, sorted, learning them on
// first use; the read is untracked since Regather keeps the set current.
func (g *Gathers) documents(ctx *Context) []string {
	if g.docs == nil {
		g.docs = map[string]bool{}
		ctx.Resolver().Untracked(func() {
			for _, doc := range ctx.Index.WorkspaceDocuments() {
				g.docs[doc] = true
			}
		})
	}
	return sortedKeys(g.docs)
}

// workspaceRoot is the root scope of a workspace document — one the index
// holds that is not bundled library content — or nil.
func (g *Gathers) workspaceRoot(ctx *Context, doc string) *symbols.Scope {
	var root *symbols.Scope
	ctx.Resolver().Untracked(func() {
		if !ctx.Index.IsLibraryDocument(doc) {
			root = ctx.Index.DocumentRoot(doc)
		}
	})
	return root
}

// gather runs f over doc's root in doc's own frame, quietly and as no
// dependency of the analysis under way (see resolve.Resolver.Gather).
func (g *Gathers) gather(ctx *Context, doc string, f func(root *symbols.Scope)) {
	ctx.Resolver().Gather(doc, func() {
		if root := ctx.Index.DocumentRoot(doc); root != nil {
			f(root)
		}
	})
}

// Regather gathers anew the documents in docs — replaced, removed, or whose
// gather the resolver dropped (see resolve.GatheredDoc) — and names what the
// unions now answer differently, for the resolver to drop the readers of.
// Nothing is gathered for a union no analysis has asked for yet.
func (g *Gathers) Regather(ctx *Context, docs map[string]bool) []string {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.docs == nil {
		return nil
	}
	todo := docs
	changed := map[string]bool{}
	about := todo[aboutGather]
	for _, doc := range sortedKeys(todo) {
		if doc == aboutGather {
			continue
		}
		switch {
		case g.workspaceRoot(ctx, doc) != nil:
			about = about || !g.docs[doc]
			g.docs[doc] = true
		case g.docs[doc]:
			about = true
			delete(g.docs, doc)
		default:
			continue
		}
		if g.oosem != nil {
			if a := newOOSEMAudit(ctx); a != nil {
				g.oosem.regather(ctx, g, a, doc, changed)
			}
		}
		if g.mosa != nil {
			if a := newMOSAAudit(ctx); a != nil {
				g.mosa.regather(ctx, g, a, doc, changed)
			}
		}
		if g.identity != nil {
			g.identity.regather(ctx, g, doc, changed)
		}
	}
	if about && g.identity != nil {
		g.identity.regatherAbout(ctx, g, changed)
	}
	return sortedKeys(changed)
}

// has reports whether doc is among the workspace documents gathered.
func (g *Gathers) has(doc string) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.docs[doc]
}

// oosemOf is the OOSEM facts of every workspace document, gathered on first use.
func (g *Gathers) oosemOf(ctx *Context, a *oosemAudit) *oosemUnion {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.oosem == nil {
		u := newOOSEMUnion()
		for _, doc := range g.documents(ctx) {
			u.regather(ctx, g, a, doc, nil)
		}
		g.oosem = u
	}
	return g.oosem
}

// mosaOf is the MOSA facts of every workspace document, gathered on first use.
func (g *Gathers) mosaOf(ctx *Context, a *mosaAudit) *mosaUnion {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.mosa == nil {
		u := newMOSAUnion()
		for _, doc := range g.documents(ctx) {
			u.regather(ctx, g, a, doc, nil)
		}
		g.mosa = u
	}
	return g.mosa
}

// identitiesOf is the identity tables of every workspace document, gathered on
// first use.
func (g *Gathers) identitiesOf(ctx *Context) *identityUnion {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.identity == nil {
		u := newIdentityUnion()
		for _, doc := range g.documents(ctx) {
			u.regather(ctx, g, doc, nil)
		}
		u.regatherAbout(ctx, g, nil)
		g.identity = u
	}
	return g.identity
}

// countSet is the union of per-document sets: a key is in it while any
// document states it.
type countSet[K comparable] map[K]int

func (s countSet[K]) has(k K) bool { return s[k] > 0 }

// move replaces one document's contribution, old by cur, naming the keys whose
// membership flipped in changed when it is not nil.
func move[K comparable](s countSet[K], old, cur map[K]bool, name func(K) string, changed map[string]bool) {
	before := map[K]bool{}
	for k := range old {
		before[k] = s.has(k)
	}
	for k := range cur {
		before[k] = s.has(k)
	}
	for k := range old {
		if s[k] <= 1 {
			delete(s, k)
		} else {
			s[k]--
		}
	}
	for k := range cur {
		s[k]++
	}
	if changed == nil {
		return
	}
	for k, was := range before {
		if s.has(k) != was {
			changed[name(k)] = true
		}
	}
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
