package queryexec

import (
	"fmt"

	"slices"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/query"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// ElementIDDocument is a document of the model a conversion writes, with its
// parse's errors, which refuse a model of several documents.
type ElementIDDocument struct {
	File       *source.SourceFile
	Root       *ast.RootNamespace
	ParseDiags []parser.Diagnostic
}

// ElementIDReader reads the elementIds a conversion of the model writes: Read, for the
// element declared at decl in document doc, named fqn; ReadOwningMembership, for the
// membership through which its namespace owns it; ReadRelationship, for the relationship
// element the notation implies at a node (a head clause, a bound end's attachment, a
// dependency's prefix metadata). Each reports why the model has none.
type ElementIDReader interface {
	Read(doc string, decl ast.Node, fqn string) (string, error)
	ReadOwningMembership(doc string, decl ast.Node, fqn string) (string, error)
	ReadRelationship(doc string, node ast.Node) (string, error)
	ReadExpressionPart(doc string, node ast.Node, slot string) (element, membership string, err error)
}

// expressionPartSlot is the slot the conversion names a synthesized
// expression parameter by: `in%d` for an operand's or type argument's
// parameter, `out` for the result.
func expressionPartSlot(sym *symbols.Symbol) (string, bool) {
	if sym == nil || sym.Expr == nil || sym.Expr.Owner == nil {
		return "", false
	}
	switch sym.Expr.Role {
	case symbols.ExpressionParameter:
		return fmt.Sprintf("in%d", sym.Expr.Index), true
	case symbols.ExpressionResult:
		return "out", true
	}
	return "", false
}

// ElementIDOf is sym's elementId through read. A relationship the notation writes as
// part of another element reads as the conversion writes it: an implied OwningMembership
// as its member's owning membership, an import as its declaration, a written clause
// or an end's attachment as the element materialized for it, a prefix metadata's
// Annotation as the metadata's; one the conversion never writes reads empty.
func ElementIDOf(read ElementIDReader, idx *symbols.Index, sym *symbols.Symbol) (string, error) {
	if rel := sym.Implicit; rel != nil {
		switch rel.Form {
		case symbols.ImplicitMembership:
			if slot, ok := expressionPartSlot(rel.Member); ok {
				_, membership, err := read.ReadExpressionPart(rel.Member.DocName, rel.Member.Expr.Owner.Decl, slot)
				return membership, err
			}
			if rel.Member != nil {
				return read.ReadOwningMembership(rel.Member.DocName, rel.Member.Decl, query.QualifiedIdentity(idx, rel.Member))
			}
		case symbols.ImplicitImport:
			if rel.Decl != nil {
				return read.Read(sym.DocName, rel.Decl, "")
			}
		case symbols.ImplicitReferenceMembership:
			if rel.Owner != nil && rel.Owner.Decl != nil {
				_, membership, err := read.ReadExpressionPart(rel.Owner.DocName, rel.Owner.Decl, rel.Slot)
				return membership, err
			}
			return "", nil
		case symbols.ImplicitWritten:
			if node := writtenRelationshipNode(rel); node != nil {
				return read.ReadRelationship(sym.DocName, node)
			}
			return "", nil
		case symbols.ImplicitOwnedAnnotation:
			if rel.Member != nil && rel.Member.Decl != nil {
				return read.ReadRelationship(rel.Member.DocName, rel.Member.Decl)
			}
			return "", nil
		case symbols.ImplicitPortConjugation:
			if rel.Owner != nil && rel.Owner.Decl != nil {
				return read.ReadRelationship(rel.Owner.DocName, rel.Owner.Decl)
			}
			return "", nil
		}
	}
	if slot, ok := expressionPartSlot(sym); ok {
		element, _, err := read.ReadExpressionPart(sym.DocName, sym.Expr.Owner.Decl, slot)
		return element, err
	}
	return read.Read(sym.DocName, sym.Decl, query.QualifiedIdentity(idx, sym))
}

// writtenRelationshipNode is the node the conversion writes rel's relationship element
// for: the end whose attachment it is (`connect a to b`), the type of a typing the head
// writes without a clause (`subject s : T`), the multiplicity a `multiplicity m subsets n`
// names, else the clause; nil for a recorded owner.
func writtenRelationshipNode(rel *symbols.ImplicitRelationship) ast.Node {
	if rel.Owner == nil {
		return nil
	}
	if rel.Node == nil {
		// A multiplicity's `subsets` is held on its declaration, not as a clause.
		if decl, ok := rel.Owner.Decl.(*ast.MultiplicityDecl); ok && rel.Kind == ast.RelSubsets && decl.Subsets != nil {
			return decl.Subsets
		}
		return nil
	}
	if end, ok := rel.Owner.Decl.(*ast.ConnectorEnd); ok && rel.Kind == ast.RelReferences &&
		rel.Node.Target == end.AttachedTarget() && !slices.Contains(end.Relationships, rel.Node) {
		return end
	}
	if rel.Node.Span().Len == 0 && rel.Node.Target != nil {
		return rel.Node.Target
	}
	return rel.Node
}

// ElementIDSource opens a reader over a model's documents, converting them
// once, on the first read: how a query's elementId column is filled. The
// converter registers one (queryexec/elementids), so a binary that links no
// translator reports elementId absent.
type ElementIDSource func(docs []ElementIDDocument) ElementIDReader

var elementIDSource ElementIDSource

// RegisterElementIDSource installs the source elementId is read from.
func RegisterElementIDSource(s ElementIDSource) {
	if elementIDSource != nil {
		panic("queryexec: element-id source registered twice")
	}
	elementIDSource = s
}

// OpenElementIDs opens the registered source over docs; nil when none is
// linked, and elementId is then absent.
func OpenElementIDs(docs []ElementIDDocument) ElementIDReader {
	if elementIDSource == nil {
		return nil
	}
	return elementIDSource(docs)
}
