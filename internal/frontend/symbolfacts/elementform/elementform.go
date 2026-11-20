// Package elementform gives the symbol facts the exporter's element form ids,
// for the frontends that link the exporter.
package elementform

import (
	"fmt"
	"github.com/Open-MBEE/OpenSysML/internal/frontend/symbolfacts"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
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
	case constraintResultMember(sym) != nil:
		// A constraint body's closing bare condition is written as the member stating it (encodeMember).
		id, _ := f.reader.Read(sym.DocName, constraintResultMember(sym), "")
		return id
	case sym.Expr != nil:
		if id, ok := export.SynthesizedElementID(sym, sc.FormID); ok {
			return id
		}
		// A `filter` condition is minted under its membership, which has no symbol: read it by node.
		id, _ := f.reader.Read(sym.DocName, sym.Decl, "")
		return id
	case sym.Chain != nil:
		id, _, _ := f.reader.ReadExpressionPart(sym.DocName, sym.Decl, "chain")
		return id
	case sym.Implicit != nil:
		if sym.Implicit.Node == nil {
			if node, slot, ok := flowEndReferenceSlot(sym); ok {
				id, _, _ := f.reader.ReadExpressionPart(sym.DocName, node, slot)
				return id
			}
			return ""
		}
		id, _ := f.reader.ReadRelationship(sym.DocName, sym.Implicit.Node)
		return id
	}
	if node, slot, ok := impliedEndSlot(sym); ok {
		id, _, _ := f.reader.ReadExpressionPart(sym.DocName, node, slot)
		return id
	}
	id, _ := f.reader.Read(sym.DocName, sym.Decl, sc.OwnIdentity(sym))
	return id
}

// impliedEndSlot is the flow or `then` succession and the slot (end0, end1; end0.ff, end1.ff) the
// conversion mints the end feature sym implies under, or the ReferenceUsage a flow's end owns.
func impliedEndSlot(sym *symbols.Symbol) (ast.Node, string, bool) {
	decl, ok := symbols.ImpliedDeclOf(sym)
	if !ok || symbols.ImpliedTransitionSuccessionEnd(sym) {
		return nil, "", false
	}
	if i, ok := symbols.ImpliedConnectorEndIndex(sym); ok {
		return decl.Of, fmt.Sprintf("end%d", i), true
	}
	if i, ok := symbols.ImpliedFlowEndFeatureIndex(sym); ok {
		return decl.Of, fmt.Sprintf("end%d.ff", i), true
	}
	return nil, "", false
}

// flowEndReferenceSlot is the flow and the slot (end0.rs, end1.rs) the conversion mints the
// ReferenceSubsetting a flow's end owns under (recordFlowEnds), for the relationship sym reflects.
func flowEndReferenceSlot(sym *symbols.Symbol) (ast.Node, string, bool) {
	if sym.Implicit == nil || sym.Implicit.Form != symbols.ImplicitSuccessionEndReference {
		return nil, "", false
	}
	i, ok := symbols.ImpliedFlowEndIndex(sym.Implicit.Owner)
	if !ok {
		return nil, "", false
	}
	decl, _ := symbols.ImpliedDeclOf(sym.Implicit.Owner)
	return decl.Of, fmt.Sprintf("end%d.rs", i), true
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

// constraintResultMember is the ConstraintMember stating the closing bare condition
// sym reflects as a constraint's result expression (semantics.ConstraintResultMember), or nil.
func constraintResultMember(sym *symbols.Symbol) ast.Node {
	if sym == nil || sym.Expr == nil || sym.Expr.Role != symbols.ExpressionBodyResult || sym.Expr.Owner == nil || sym.Expr.Owner.Expr != nil {
		return nil
	}
	if c := semantics.ConstraintResultCondition(sym.Expr.Owner); c != nil && c.Expression == sym.Decl {
		return c
	}
	return nil
}

// LibraryName is the qualified name of the library element whose normative id is id.
func (elementForm) LibraryName(id string) (string, bool) {
	return export.LibraryElementName(id)
}
