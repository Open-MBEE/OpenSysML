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

// Identity is the id SymbolInfo carries for sym: its own identity, or the element form's `@id`
// when another element's generated id is spelled like it, so no two elements share an id.
func (sc *Context) Identity(sym *symbols.Symbol) string {
	identity := sc.OwnIdentity(sym)
	if identity != "" && sc.generatedSpells(identity, sym) {
		if formID := sc.FormID(sym); formID != "" {
			return formID
		}
	}
	return identity
}

// OwnIdentity is the identity sym is declared under: its qualified identity, a declared
// `@N` quoted (naming.Declared), else its positional one.
func (sc *Context) OwnIdentity(sym *symbols.Symbol) string {
	if identity := sc.Index.QualifiedIdentity(sym); identity != "" {
		return naming.Declared(identity)
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

// Lookup reads id the way a request without a lookup kind states it: a qualified name
// first, whatever its visibility, then the element form's `@id` of an element.
func (sc *Context) Lookup(id string) []*symbols.Symbol {
	if syms := sc.lookupDeclared(id); len(syms) > 0 {
		return syms
	}
	return sc.formElements(id)
}

// lookupDeclared finds what id denotes as a declared name whatever its visibility, as a
// positional id, or as a qualified name resolved from the root.
func (sc *Context) lookupDeclared(id string) []*symbols.Symbol {
	if syms := LookupNamed(sc.Index, id); len(syms) > 0 {
		return syms
	}
	if syms := sc.unnamed(id); len(syms) > 0 {
		return syms
	}
	return sc.Resolver.ResolveGlobal(nameSegments(id))
}

// LookupID finds the element reported under id — its ElementIdentity, else the element
// form's `@id` — whatever its visibility; id is never resolved as a name.
func (sc *Context) LookupID(id string) []*symbols.Symbol {
	var found []*symbols.Symbol
	for _, sym := range append(sc.unnamed(id), modelFirst(sc.Index, sc.Index.LookupQualified(naming.Recorded(id)))...) {
		if !slices.Contains(found, sym) && sc.ElementIdentity(sym) == id {
			found = append(found, sym)
		}
	}
	if len(found) > 0 {
		return found
	}
	return sc.formElements(id)
}

// LookupMember is Lookup, with a name no declaration answers to walked as a
// chain of members (see LookupNamedMember).
func (sc *Context) LookupMember(id string) []*symbols.Symbol {
	if syms := LookupNamedMember(sc.Index, sc.Semantics, id); len(syms) > 0 {
		return syms
	}
	return sc.Resolver.ResolveGlobal(nameSegments(id))
}

// Resolve reads id the way a request without a lookup kind asks for name resolution: as a
// qualified name (ResolveName) first, then as the id of the element reported under it.
func (sc *Context) Resolve(id string) []*symbols.Symbol {
	if syms := sc.ResolveName(id); len(syms) > 0 {
		return syms
	}
	return sc.reported(id)
}

// ResolveName resolves name as a qualified name from the root (resolve.Resolver.ResolveGlobal), or as
// the declared name of a public member of a non-visible namespace; an id is never read as one.
func (sc *Context) ResolveName(name string) []*symbols.Symbol {
	if syms := sc.Resolver.ResolveGlobal(nameSegments(name)); len(syms) > 0 {
		return syms
	}
	var elements []*symbols.Symbol
	for _, sym := range sc.declaredPublic(name) {
		if element := sc.Resolver.AliasedElement(sym); !sc.Resolver.AliasNamesNothing(element) &&
			!slices.Contains(elements, element) {
			elements = append(elements, element)
		}
	}
	if len(elements) > 0 {
		return elements
	}
	return sc.positionallyNamed(name)
}

// ResolveMember is Resolve with an alias answered as the alias membership its
// namespace holds rather than as the element it names.
func (sc *Context) ResolveMember(id string) []*symbols.Symbol {
	if syms := sc.ResolveMemberName(id); len(syms) > 0 {
		return syms
	}
	return sc.reported(id)
}

// ResolveMemberName is ResolveName with an alias answered as the alias membership its
// namespace holds rather than as the element it names.
func (sc *Context) ResolveMemberName(name string) []*symbols.Symbol {
	if syms := sc.Resolver.ResolveGlobalMembers(nameSegments(name)); len(syms) > 0 {
		return syms
	}
	if declared := sc.declaredPublic(name); len(declared) > 0 {
		return declared
	}
	return sc.positionallyNamed(name)
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

// reported finds the element an id names once no name answers to it: the positional one,
// else the one the element form writes under it.
func (sc *Context) reported(id string) []*symbols.Symbol {
	if syms := sc.unnamed(id); len(syms) > 0 {
		return syms
	}
	return sc.formElements(id)
}

// unnamed finds the element a positional id denotes.
func (sc *Context) unnamed(id string) []*symbols.Symbol {
	if syms := sc.positionallyNamed(id); len(syms) > 0 {
		return syms
	}
	if sym := sc.Positional.Symbol(id); sym != nil {
		return []*symbols.Symbol{sym}
	}
	return nil
}

// positionallyNamed finds the parameter name is the qualified name of through the one its
// position redefines (KerML 8.2.4).
func (sc *Context) positionallyNamed(name string) []*symbols.Symbol {
	// The chain is walked in the unquoted spelling the index records, which is
	// also the one QualifiedNameOf answers, whatever spelling the name uses.
	if plain, ok := UnquotedName(name); ok {
		name = plain
	}
	if sym, _ := MemberChain(sc.Index, sc.Semantics, name); sym != nil && sc.Semantics.PositionallyNamed(sym) && sc.Semantics.QualifiedNameOf(sym) == name {
		return []*symbols.Symbol{sym}
	}
	return nil
}
