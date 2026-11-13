package symbolfacts

import (
	"slices"
	"strings"
	"sync"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/naming"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// PositionalDocument pairs a document's source and tree with its root scope,
// for the positional identities of its unnamed declarations.
type PositionalDocument struct {
	File  *source.SourceFile
	Root  *ast.RootNamespace
	Scope *symbols.Scope
}

// Positional lazily derives the positional identities (`Owner::@N`) of the
// declarations no qualified identity names, the way the export names them.
type Positional struct {
	mu      sync.Mutex
	derived bool
	index   *symbols.Index
	docs    func() []PositionalDocument
	read    func(name string) ([]byte, error)
	library map[string]bool
	bySym   map[*symbols.Symbol]string
	byName  map[string]*symbols.Symbol
}

// NewPositional prepares the positional identities of the documents docs
// lists; nothing is derived until an identity is asked for.
func NewPositional(idx *symbols.Index, docs func() []PositionalDocument) *Positional {
	return &Positional{index: idx, docs: docs}
}

// WithLibrary also names the unnamed declarations of the index's library
// documents, each document derived when first asked about, its text from read.
func (p *Positional) WithLibrary(read func(name string) ([]byte, error)) *Positional {
	p.read = read
	return p
}

// derive names the model's documents. Caller holds the lock.
func (p *Positional) derive() {
	if p.derived {
		return
	}
	p.derived = true
	p.bySym, p.byName = map[*symbols.Symbol]string{}, map[string]*symbols.Symbol{}
	if p.index == nil || p.docs == nil {
		return
	}
	docs := p.docs()
	out := make([]naming.Document, 0, len(docs))
	for _, doc := range docs {
		out = append(out, naming.Document{File: doc.File, Root: doc.Root, Scope: doc.Scope})
	}
	p.name(out)
}

func (p *Positional) name(docs []naming.Document) {
	bySym, _ := naming.PositionalIdentities(p.index, docs, func(sym *symbols.Symbol) bool {
		return p.index.QualifiedIdentity(sym) != ""
	})
	for sym, name := range bySym {
		// A name the model's own documents already gave stays theirs.
		if _, taken := p.byName[name]; taken {
			continue
		}
		p.bySym[sym] = name
		p.byName[name] = sym
	}
}

// deriveLibrary names the library document doc, once. Caller holds the lock.
func (p *Positional) deriveLibrary(doc string) {
	if p.read == nil || p.index == nil || p.library[doc] || !p.index.IsLibraryDocument(doc) {
		return
	}
	if p.library == nil {
		p.library = map[string]bool{}
	}
	p.library[doc] = true
	scope := p.index.DocumentRoot(doc)
	if scope == nil {
		return
	}
	// A document restored from its interface record has no tree to take positions from.
	root, ok := scope.Node().(*ast.RootNamespace)
	if !ok || root == nil {
		return
	}
	content, err := p.read(doc)
	if err != nil {
		return
	}
	p.name([]naming.Document{{File: source.New(doc, content), Root: root, Scope: scope}})
}

// Of is the positional identity of sym, empty when it has none.
func (p *Positional) Of(sym *symbols.Symbol) string {
	if p == nil || sym == nil {
		return ""
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.derive()
	if id, ok := p.bySym[sym]; ok {
		return id
	}
	p.deriveLibrary(sym.DocName)
	return p.bySym[sym]
}

// Symbol is the symbol a positional identity names, nil when none does.
func (p *Positional) Symbol(id string) *symbols.Symbol {
	if p == nil {
		return nil
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.derive()
	if sym := p.byName[id]; sym != nil {
		return sym
	}
	if p.read == nil || !naming.IsPositionalIdentity(id) {
		return nil
	}
	// The named prefix of the id says which library document declares it.
	segments := naming.Segments(id)
	for n := len(segments) - 1; n > 0; n-- {
		owners := p.index.LookupQualified(strings.Join(segments[:n], "::"))
		if len(owners) == 0 {
			continue
		}
		for _, owner := range owners {
			p.deriveLibrary(owner.DocName)
		}
		break
	}
	return p.byName[id]
}

// Identity is the id SymbolInfo carries for sym: its qualified identity, else
// its positional one.
func (sc *Context) Identity(sym *symbols.Symbol) string {
	if identity := sc.Index.QualifiedIdentity(sym); identity != "" {
		return identity
	}
	// A parameter named by the one its position redefines is qualified under
	// that name (KerML 8.2.4), which the index binds nothing for.
	if sc.Semantics.PositionallyNamed(sym) {
		if identity := sc.Semantics.QualifiedNameOf(sym); identity != "" {
			return identity
		}
	}
	return sc.Positional.Of(sym)
}

// Lookup finds the element an id the service reported denotes, whatever its
// visibility; an id no element carries is resolved as a qualified name (see Resolve).
func (sc *Context) Lookup(id string) []*symbols.Symbol {
	if syms := LookupNamed(sc.Index, id); len(syms) > 0 {
		return syms
	}
	if syms := sc.unnamed(id); len(syms) > 0 {
		return syms
	}
	return sc.Resolver.ResolveGlobal(nameSegments(id))
}

// LookupMember is Lookup, with a name no declaration answers to walked as a
// chain of members (see LookupNamedMember).
func (sc *Context) LookupMember(id string) []*symbols.Symbol {
	if syms := LookupNamedMember(sc.Index, sc.Semantics, id); len(syms) > 0 {
		return syms
	}
	return sc.Resolver.ResolveGlobal(nameSegments(id))
}

// Resolve resolves id as a qualified name from the root (resolve.Resolver.ResolveGlobal); a
// positional id, or the declared name of a public member of a non-visible namespace, still denotes its element.
func (sc *Context) Resolve(id string) []*symbols.Symbol {
	if syms := sc.Resolver.ResolveGlobal(nameSegments(id)); len(syms) > 0 {
		return syms
	}
	var elements []*symbols.Symbol
	for _, sym := range sc.declaredPublic(id) {
		if element := sc.Resolver.AliasedElement(sym); !sc.Resolver.AliasNamesNothing(element) &&
			!slices.Contains(elements, element) {
			elements = append(elements, element)
		}
	}
	if len(elements) > 0 {
		return elements
	}
	return sc.unnamed(id)
}

// ResolveMember is Resolve with an alias answered as the alias membership its
// namespace holds rather than as the element it names.
func (sc *Context) ResolveMember(id string) []*symbols.Symbol {
	if syms := sc.Resolver.ResolveGlobalMembers(nameSegments(id)); len(syms) > 0 {
		return syms
	}
	if declared := sc.declaredPublic(id); len(declared) > 0 {
		return declared
	}
	return sc.unnamed(id)
}

// declaredPublic finds the members id is the declared qualified name of whose own membership is
// public: an id the service reports, never a name an import (filtered or not) only re-exports.
func (sc *Context) declaredPublic(id string) []*symbols.Symbol {
	name := id
	if plain, ok := UnquotedName(id); ok {
		name = plain
	}
	var declared []*symbols.Symbol
	for _, sym := range LookupNamed(sc.Index, id) {
		if symbols.HasFQN(sym, name) && symbols.VisibleOutside(sym.Visibility) {
			declared = append(declared, sym)
		}
	}
	return declared
}

// unnamed finds the element a positional id denotes.
func (sc *Context) unnamed(id string) []*symbols.Symbol {
	// The chain is walked in the unquoted spelling the index records, which is
	// also the one QualifiedNameOf answers, whatever spelling the id uses.
	name := id
	if plain, ok := UnquotedName(id); ok {
		name = plain
	}
	if sym, _ := MemberChain(sc.Index, sc.Semantics, name); sym != nil && sc.Semantics.PositionallyNamed(sym) && sc.Semantics.QualifiedNameOf(sym) == name {
		return []*symbols.Symbol{sym}
	}
	if sym := sc.Positional.Symbol(id); sym != nil {
		return []*symbols.Symbol{sym}
	}
	return nil
}
