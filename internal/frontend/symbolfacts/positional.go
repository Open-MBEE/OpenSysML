package symbolfacts

import (
	"sync"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// PositionalDocument is one document of a model with its root scope, the unit
// positional identities are derived over.
type PositionalDocument struct {
	File *source.SourceFile
	Root *symbols.Scope
}

// PositionalNames derives the positional identities of the symbols identifies
// refuses, with the inverse map; the export package implements it.
type PositionalNames func(index *symbols.Index, docs []PositionalDocument, identifies func(*symbols.Symbol) bool) (map[*symbols.Symbol]string, map[string]*symbols.Symbol)

var positionalNames PositionalNames

// RegisterPositional installs the exporter's derivation of positional ids, so
// this package links no exporter; unregistered, unnamed members have no id.
func RegisterPositional(names PositionalNames) {
	if positionalNames != nil {
		panic("symbolfacts: positional names registered twice")
	}
	positionalNames = names
}

// Positional names unnamed symbols by the export's positional identity
// (`Owner::@3`, `Owner::@3::wheel`), derived once per model on first use.
type Positional struct {
	once   sync.Once
	index  *symbols.Index
	docs   func() []PositionalDocument
	bySym  map[*symbols.Symbol]string
	byName map[string]*symbols.Symbol
}

// NewPositional prepares the positional identities of the documents docs
// returns, derived when first asked for.
func NewPositional(idx *symbols.Index, docs func() []PositionalDocument) *Positional {
	return &Positional{index: idx, docs: docs}
}

func (p *Positional) derive() {
	p.once.Do(func() {
		if positionalNames == nil || p.index == nil || p.docs == nil {
			return
		}
		p.bySym, p.byName = positionalNames(p.index, p.docs(), func(sym *symbols.Symbol) bool {
			return p.index.QualifiedIdentity(sym) != ""
		})
	})
}

// Of is the positional identity of sym, "" when it has none.
func (p *Positional) Of(sym *symbols.Symbol) string {
	if p == nil {
		return ""
	}
	p.derive()
	return p.bySym[sym]
}

// Symbol is the symbol the positional identity id names, nil when none does.
func (p *Positional) Symbol(id string) *symbols.Symbol {
	if p == nil {
		return nil
	}
	p.derive()
	return p.byName[id]
}

// Identity is the id a client addresses sym by: its qualified name if that
// identifies it, else its positional identity, else "" (a body-local symbol).
func (sc *Context) Identity(sym *symbols.Symbol) string {
	if identity := sc.Index.QualifiedIdentity(sym); identity != "" {
		return identity
	}
	return sc.Positional.Of(sym)
}

// Lookup resolves a symbol id in either form Identity reports: a qualified
// name, as LookupNamed reads it, or a positional identity.
func (sc *Context) Lookup(id string) []*symbols.Symbol {
	if syms := LookupNamed(sc.Index, id); len(syms) > 0 {
		return syms
	}
	if sym := sc.Positional.Symbol(id); sym != nil {
		return []*symbols.Symbol{sym}
	}
	return nil
}
