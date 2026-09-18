package kit

import (
	"sort"
	"sync"

	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// Union is one workspace-wide gather, kept per document by Gathers.
type Union interface {
	Regather(ctx *Context, g *Gathers, doc string, changed map[string]bool)
}

// AboutUnion also gathers what lies outside the workspace documents.
type AboutUnion interface {
	Union
	RegatherAbout(ctx *Context, g *Gathers, changed map[string]bool)
}

// Gathers holds workspace-wide audit state and the unions built from documents.
type Gathers struct {
	mu     sync.Mutex
	docs   map[string]bool
	unions map[string]Union
}

// NewGathers returns gathers with nothing gathered yet.
func NewGathers() *Gathers { return &Gathers{} }

// Reset forgets every gathered document and union.
func (g *Gathers) Reset() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.docs, g.unions = nil, nil
}

// documents lists the workspace documents gathered, sorted, learning them on
// first use.
func (g *Gathers) documents(ctx *Context) []string {
	if g.docs == nil {
		g.docs = map[string]bool{}
		ctx.Resolver().Untracked(func() {
			for _, doc := range ctx.Index.WorkspaceDocuments() {
				g.docs[doc] = true
			}
		})
	}
	return SortedKeys(g.docs)
}

// workspaceRoot returns the root of a non-library workspace document, or nil.
func (g *Gathers) workspaceRoot(ctx *Context, doc string) *symbols.Scope {
	var root *symbols.Scope
	ctx.Resolver().Untracked(func() {
		if !ctx.Index.IsLibraryDocument(doc) {
			root = ctx.Index.DocumentRoot(doc)
		}
	})
	return root
}

// Gather runs f over doc's root in doc's own resolver frame.
func (g *Gathers) Gather(ctx *Context, doc string, f func(root *symbols.Scope)) {
	ctx.Resolver().Gather(doc, func() {
		if root := ctx.Index.DocumentRoot(doc); root != nil {
			f(root)
		}
	})
}

// Gathered reports whether doc is among the gathered workspace documents.
// It is unlocked for unions inside Regather/RegatherAbout, which run under the lock.
func (g *Gathers) Gathered(doc string) bool { return g.docs[doc] }

// Has reports whether doc is among the gathered workspace documents, with locking.
func (g *Gathers) Has(doc string) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.docs[doc]
}

// Regather updates the requested documents and reports changed union keys.
// Nothing is gathered for a union no analysis has asked for yet.
func (g *Gathers) Regather(ctx *Context, docs map[string]bool) []string {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.docs == nil {
		return nil
	}
	changed := map[string]bool{}
	about := docs[AboutGather]
	for _, doc := range SortedKeys(docs) {
		if doc == AboutGather {
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
		for _, key := range unionKeys(g.unions) {
			g.unions[key].Regather(ctx, g, doc, changed)
		}
	}
	if about {
		for _, key := range unionKeys(g.unions) {
			if u, ok := g.unions[key].(AboutUnion); ok {
				u.RegatherAbout(ctx, g, changed)
			}
		}
	}
	return SortedKeys(changed)
}

// UnionOf returns the union under key, built on first use over every gathered
// document.
func (g *Gathers) UnionOf(ctx *Context, key string, build func() Union) Union {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.unions == nil {
		g.unions = map[string]Union{}
	}
	if u := g.unions[key]; u != nil {
		return u
	}
	u := build()
	for _, doc := range g.documents(ctx) {
		u.Regather(ctx, g, doc, nil)
	}
	if a, ok := u.(AboutUnion); ok {
		a.RegatherAbout(ctx, g, nil)
	}
	g.unions[key] = u
	return u
}

// Union returns the union under key if built, else nil.
func (g *Gathers) Union(key string) Union {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.unions[key]
}

// AboutGather names the gather of the `about`-annotated elements no workspace
// document declares.
const AboutGather = "\x00identity"

// CountSet is a union of per-document sets.
type CountSet[K comparable] map[K]int

func (s CountSet[K]) Has(k K) bool { return s[k] > 0 }

// Move replaces one document's contribution and names flipped memberships.
func Move[K comparable](s CountSet[K], old, cur map[K]bool, name func(K) string, changed map[string]bool) {
	before := map[K]bool{}
	for k := range old {
		before[k] = s.Has(k)
	}
	for k := range cur {
		before[k] = s.Has(k)
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
		if s.Has(k) != was {
			changed[name(k)] = true
		}
	}
}

// SortedKeys returns the sorted keys of a boolean set.
func SortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// unionKeys returns sorted keys of a union map.
func unionKeys(m map[string]Union) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
