// Package elementform gives the symbol facts the exporter's element form ids,
// for the frontends that link the exporter.
package elementform

import (
	"github.com/Open-MBEE/OpenSysML/internal/frontend/symbolfacts"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/translate/export"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
)

// New is the exporter's element form read from reader, for a symbol context of
// a frontend that links the exporter.
func New(reader *export.ElementIDReader) symbolfacts.ElementForm {
	return elementForm{reader}
}

type elementForm struct{ reader *export.ElementIDReader }

// ID is read for a declaration, and composed from its owner's for an element
// the semantic side tables synthesize.
func (f elementForm) ID(sc *symbolfacts.Context, sym *symbols.Symbol) string {
	if f.reader == nil {
		return ""
	}
	switch {
	case sym.Expr != nil:
		id, _ := export.SynthesizedElementID(sym, sc.FormID)
		return id
	case sym.Chain != nil:
		id, _, _ := f.reader.ReadExpressionPart(sym.DocName, sym.Decl, "chain")
		return id
	case sym.Implicit != nil:
		if sym.Implicit.Node == nil {
			return ""
		}
		id, _ := f.reader.ReadRelationship(sym.DocName, sym.Implicit.Node)
		return id
	}
	id, _ := f.reader.Read(sym.DocName, sym.Decl, sc.OwnIdentity(sym))
	return id
}

// Owner decodes an `@id` into the qualified name it is written under and how many elements deep
// below that the id can lie: one per expression position, one for a suffix the conversion mints.
func (elementForm) Owner(id string) (owner string, depth int, ok bool) {
	if owner, positions, ok := rdf.DecodeExpressionNodeID(id); ok {
		return owner, 2 * len(positions), true
	}
	if owner, ok = rdf.DecodeElementID(id); ok {
		return owner, 0, true
	}
	owner, ok = export.SynthesizedElementOwner(id)
	return owner, 1, ok
}
