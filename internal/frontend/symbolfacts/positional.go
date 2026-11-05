package symbolfacts

import (
	"sync"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/query"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/translate/export"
)

// Positional names the symbols no qualified name identifies by the positional
// identity the RDF and API JSON export gives them (`Owner::@3` for the fourth
// member of Owner, `Owner::@3::wheel` for a named member of it), which is what
// the query API reports for them. The names are derived once per model, from
// the documents the host hands over on first use.
type Positional struct {
	once   sync.Once
	index  *symbols.Index
	docs   func() []export.PositionalDocument
	bySym  map[*symbols.Symbol]string
	byName map[string]*symbols.Symbol
}

// NewPositional prepares the positional identities of the documents docs
// returns, derived when first asked for.
func NewPositional(idx *symbols.Index, docs func() []export.PositionalDocument) *Positional {
	return &Positional{index: idx, docs: docs}
}

func (p *Positional) derive() {
	p.once.Do(func() {
		if p.index == nil || p.docs == nil {
			p.bySym, p.byName = export.PositionalIdentities(nil, nil, nil)
			return
		}
		p.bySym, p.byName = export.PositionalIdentities(p.index, p.docs(), func(sym *symbols.Symbol) bool {
			return query.QualifiedIdentity(p.index, sym) != ""
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

// Identity is the id a client addresses sym by: its qualified name when that
// identifies it, else its positional identity, else "" for a symbol no id
// reaches (a body-local declaration).
func (sc *Context) Identity(sym *symbols.Symbol) string {
	if identity := query.QualifiedIdentity(sc.Index, sym); identity != "" {
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
