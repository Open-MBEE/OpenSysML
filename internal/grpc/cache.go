package grpc

import (
	"container/list"
	"fmt"
	goruntime "runtime"
	"sync"

	"connectrpc.com/connect"

	"github.com/Open-MBEE/OpenSysML/internal/core/analysis"
	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/conformance"
	"github.com/Open-MBEE/OpenSysML/internal/core/diag"
	"github.com/Open-MBEE/OpenSysML/internal/core/libs"
	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// CachedDocument is one parsed document of a model.
type CachedDocument struct {
	Root        *ast.RootNamespace
	Source      *source.SourceFile  // For diagnostic line/col mapping
	ParseDiags  []parser.Diagnostic // Parser diagnostics
	PassesDiags []diag.Diagnostic   // Semantic pass diagnostics (name-resolution, type, constraint)
}

// CachedModel holds parsed model data with semantic analysis results
type CachedModel struct {
	// Documents are the model's documents, at least one, in the order the parse
	// request named them. They share Index, so a name one declares resolves in
	// another and an import between them is satisfied.
	Documents []*CachedDocument
	Index     *symbols.Index // For symbol lookups by FQN
	Library   libs.Source    // the files the library in Index was built from, for their spans' text
	// Mode is the conformance strictness the parse request asked for; an edit's
	// notation is judged at the same strictness.
	Mode conformance.Mode

	symCtxOnce sync.Once
	symCtx     *SymbolContext

	// idle are the workers requests have given back, warm with what they resolved.
	idleMu sync.Mutex
	idle   []*analysis.Worker

	// objects are the objects Instantiate created for the model, on a runtime of
	// their own that outlives the requests; nil until the first Instantiate.
	objectsMu sync.Mutex
	objects   *heldObjects
}

// maxIdleWorkers bounds the warm workers a model keeps: as many as can run at once, so a
// burst of requests does not leave the model holding a worker per request in the burst.
func maxIdleWorkers() int { return goruntime.GOMAXPROCS(0) }

// worker takes a model-derived runtime part for one request: an idle one, warm from the requests
// it served, else a new one. It memoizes into plain maps, so a request holds it alone until release.
func (m *CachedModel) worker() (*analysis.Worker, func()) {
	m.idleMu.Lock()
	var w *analysis.Worker
	if n := len(m.idle); n > 0 {
		w, m.idle = m.idle[n-1], m.idle[:n-1]
	}
	m.idleMu.Unlock()
	if w == nil {
		model, _ := m.Semantics()
		w = &analysis.Worker{Model: model}
	}
	// A name a request fails to resolve is the request's error, not the model's: drop it on release.
	resolver := w.Model.Resolver()
	diags := len(resolver.Diagnostics)
	return w, func() {
		if len(resolver.Diagnostics) > diags {
			resolver.Diagnostics = resolver.Diagnostics[:diags]
		}
		m.idleMu.Lock()
		m.idle = append(m.idle, w)
		if bound := maxIdleWorkers(); len(m.idle) > bound {
			clear(m.idle[bound:])
			m.idle = m.idle[:bound]
		}
		m.idleMu.Unlock()
	}
}

// Semantics is the model-derived runtime part as an analysis.Model builds one.
func (m *CachedModel) Semantics() (*runtime.Model, error) {
	resolver := resolve.New(m.Index)
	sem := passes.NewTypedModel(resolver)
	sem.SetSourceText(cachedSourceText(m))
	model := runtime.NewModel(sem, resolver)
	for _, doc := range m.Documents {
		model.RegisterSource(doc.Source)
	}
	return model, nil
}

// Primary is the document a model is named by: the only one of a single-document
// model, and the first named of a multi-document one, whose root namespace is
// where a name with nothing else to resolve against is looked up.
func (m *CachedModel) Primary() *CachedDocument {
	return m.Documents[0]
}

// DocumentRoots are the root scopes of the model's documents, in order, skipping
// any the index does not hold.
func (m *CachedModel) DocumentRoots() []*symbols.Scope {
	roots := make([]*symbols.Scope, 0, len(m.Documents))
	for _, doc := range m.Documents {
		if root := m.Index.DocumentRoot(doc.Source.Name()); root != nil {
			roots = append(roots, root)
		}
	}
	return roots
}

// document is the source of the model's document named name, nil when none is.
func (m *CachedModel) document(name string) *source.SourceFile {
	if name == "" {
		return nil
	}
	for _, doc := range m.Documents {
		if doc.Source.Name() == name {
			return doc.Source
		}
	}
	return nil
}

// PrimaryRoot is the root scope of the document the model is named by.
func (m *CachedModel) PrimaryRoot() *symbols.Scope {
	return m.Index.DocumentRoot(m.Primary().Source.Name())
}

// SoleDocument is the model's one document, for an operation defined on a single
// document's own source, such as writing it back out. A model of several
// documents is refused rather than answered about one of them.
func (m *CachedModel) SoleDocument() (*CachedDocument, error) {
	if len(m.Documents) > 1 {
		return nil, statusErrorf(connect.CodeFailedPrecondition,
			"this operation is defined on one document, and the model has %d: "+
				"name the document to operate on by parsing it on its own", len(m.Documents))
	}
	return m.Primary(), nil
}

// SymbolContext returns the conversion context for this model, building it on
// first use. Name resolution and the semantic relations derived from it are
// memoized in it, so every symbol converted from one cached model shares one.
func (m *CachedModel) SymbolContext() *SymbolContext {
	m.symCtxOnce.Do(func() {
		m.symCtx = NewSymbolContext(m.Index)
		m.symCtx.Semantics.SetSourceText(cachedSourceText(m))
	})
	return m.symCtx
}

// Cache is an LRU cache for parsed models keyed by content hash
type Cache struct {
	mu      sync.RWMutex
	maxSize int
	items   map[string]*list.Element
	lruList *list.List
}

type cacheEntry struct {
	key   string
	value *CachedModel
}

// NewCache creates a cache with the specified max size. It returns an error if
// maxSize is not positive.
func NewCache(maxSize int) (*Cache, error) {
	if maxSize <= 0 {
		return nil, fmt.Errorf("cache maxSize must be positive, got %d", maxSize)
	}
	return &Cache{
		maxSize: maxSize,
		items:   make(map[string]*list.Element),
		lruList: list.New(),
	}, nil
}

// Get retrieves a model from cache, returns (model, true) on hit
func (c *Cache) Get(hash string) (*CachedModel, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	elem, ok := c.items[hash]
	if !ok {
		return nil, false
	}

	c.lruList.MoveToFront(elem)
	entry := elem.Value.(*cacheEntry)
	return entry.value, true
}

// Put adds a model to cache, evicting LRU if at capacity
func (c *Cache) Put(hash string, model *CachedModel) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Check if already exists
	if elem, ok := c.items[hash]; ok {
		c.lruList.MoveToFront(elem)
		elem.Value.(*cacheEntry).value = model
		return
	}
	c.insert(hash, model)
}

// Add caches model under hash unless the hash is already cached, and returns
// the model cached under it: two parses of one model racing to the cache end
// up sharing the one entry, and the objects held on it, rather than replacing it.
func (c *Cache) Add(hash string, model *CachedModel) *CachedModel {
	c.mu.Lock()
	defer c.mu.Unlock()

	if elem, ok := c.items[hash]; ok {
		c.lruList.MoveToFront(elem)
		return elem.Value.(*cacheEntry).value
	}
	c.insert(hash, model)
	return model
}

// insert adds a new entry under the write lock, evicting the LRU at capacity.
func (c *Cache) insert(hash string, model *CachedModel) {
	// Evict if at capacity
	if c.lruList.Len() >= c.maxSize {
		oldest := c.lruList.Back()
		if oldest != nil {
			c.lruList.Remove(oldest)
			delete(c.items, oldest.Value.(*cacheEntry).key)
		}
	}

	// Add new entry
	entry := &cacheEntry{key: hash, value: model}
	elem := c.lruList.PushFront(entry)
	c.items[hash] = elem
}
