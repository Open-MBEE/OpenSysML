package queryexec

import (
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
// membership through which its namespace owns it. Both report why the model has none.
type ElementIDReader interface {
	Read(doc string, decl ast.Node, fqn string) (string, error)
	ReadOwningMembership(doc string, decl ast.Node, fqn string) (string, error)
}

// ElementIDOf is sym's elementId through read: an implied OwningMembership (which the
// notation writes as part of its member) reads as its member's owning membership.
func ElementIDOf(read ElementIDReader, idx *symbols.Index, sym *symbols.Symbol) (string, error) {
	if sym.Implicit != nil && sym.Implicit.Form == symbols.ImplicitMembership && sym.Implicit.Member != nil {
		member := sym.Implicit.Member
		return read.ReadOwningMembership(member.DocName, member.Decl, query.QualifiedIdentity(idx, member))
	}
	return read.Read(sym.DocName, sym.Decl, query.QualifiedIdentity(idx, sym))
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
