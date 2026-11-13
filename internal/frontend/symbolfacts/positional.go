package symbolfacts

import (
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
	once   sync.Once
	index  *symbols.Index
	docs   func() []PositionalDocument
	bySym  map[*symbols.Symbol]string
	byName map[string]*symbols.Symbol
}

// NewPositional prepares the positional identities of the documents docs
// lists; nothing is derived until an identity is asked for.
func NewPositional(idx *symbols.Index, docs func() []PositionalDocument) *Positional {
	return &Positional{index: idx, docs: docs}
}

func (p *Positional) derive() {
	p.once.Do(func() {
		if p.index == nil || p.docs == nil {
			return
		}
		docs := p.docs()
		out := make([]naming.Document, 0, len(docs))
		for _, doc := range docs {
			out = append(out, naming.Document{File: doc.File, Root: doc.Root, Scope: doc.Scope})
		}
		p.bySym, p.byName = naming.PositionalIdentities(p.index, out, func(sym *symbols.Symbol) bool {
			return p.index.QualifiedIdentity(sym) != ""
		})
	})
}

// Of is the positional identity of sym, empty when it has none.
func (p *Positional) Of(sym *symbols.Symbol) string {
	if p == nil {
		return ""
	}
	p.derive()
	return p.bySym[sym]
}

// Symbol is the symbol a positional identity names, nil when none does.
func (p *Positional) Symbol(id string) *symbols.Symbol {
	if p == nil {
		return nil
	}
	p.derive()
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

// Resolve resolves id as a qualified name from the root (resolve.Resolver.ResolveGlobal);
// a positional id, or a public member under a non-visible namespace, still denotes its element.
func (sc *Context) Resolve(id string) []*symbols.Symbol {
	return sc.resolved(id, sc.Resolver.ResolveGlobal)
}

// ResolveMember is Resolve with an alias answered as the alias membership its
// namespace holds rather than as the element it names.
func (sc *Context) ResolveMember(id string) []*symbols.Symbol {
	return sc.resolved(id, sc.Resolver.ResolveGlobalMembers)
}

func (sc *Context) resolved(id string, resolve func([]string) []*symbols.Symbol) []*symbols.Symbol {
	if syms := resolve(nameSegments(id)); len(syms) > 0 {
		return syms
	}
	if named := LookupNamed(sc.Index, id); len(named) > 0 {
		return ownPublic(named)
	}
	return sc.unnamed(id)
}

// ownPublic keeps the elements whose own membership is public: an id the
// service reports stays fetchable where only a namespace on the way is not.
func ownPublic(syms []*symbols.Symbol) []*symbols.Symbol {
	var public []*symbols.Symbol
	for _, sym := range syms {
		if symbols.VisibleOutside(sym.Visibility) {
			public = append(public, sym)
		}
	}
	return public
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
