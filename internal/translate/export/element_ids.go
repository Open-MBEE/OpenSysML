package export

import (
	"fmt"
	"sync"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
)

// ElementIDs reports the elementId a conversion of a model writes for each of
// its elements, in the qualified id form, from the identity tables Convert mints
// them from (ToRDF's for one document, ModelToRDFWith's for several), so a
// library file's copy, a declared id and scope qualification come out as written.
type ElementIDs struct {
	// byNode keys each id by the declaration the conversion wrote it for: two
	// identity scopes may declare one qualified name, and only the node tells
	// their elements apart.
	byNode   map[ast.Node]string
	bySpan   map[spanKey]string // the same declaration in another parse of the document
	byName   map[string]string
	res      *resolve.Resolver
	encoders []*encoder
}

// NewElementIDs records the elementId Convert writes for each element of the
// documents' model from the identity tables the ids are minted from — nothing is
// written — refusing what the declarations' identity claims refuse in Convert:
// ids colliding within a document (mintDeclarations) or across documents.
func NewElementIDs(documents []ModelDocument) (*ElementIDs, error) {
	res, encoders, err := modelEncodersOf(documents, IDQualifiedName)
	if err != nil {
		return nil, err
	}
	return identityIDsOf(res, encoders)
}

// identityIDsOf is the id each encoder writes for each declaration it writes:
// the local name of the subject its identity tables mint for the declaration.
// The declarations' identity claims are made first (mintDeclarations, declaredOnce).
func identityIDsOf(res *resolve.Resolver, encoders []*encoder) (*ElementIDs, error) {
	declaredIn := map[string]mintedSubject{}
	for _, e := range encoders {
		if err := e.mintDeclarations(); err != nil {
			return nil, fmt.Errorf("%s: %w", e.file.Name(), err)
		}
		if err := declaredOnce(declaredIn, e.minted); err != nil {
			return nil, err
		}
	}
	ids := &ElementIDs{byNode: map[ast.Node]string{}, bySpan: map[spanKey]string{}, byName: map[string]string{}, res: res, encoders: encoders}
	for _, e := range encoders {
		for _, node := range e.writtenDeclarations() {
			fqn := e.fqn[node]
			id := rdf.LocalName(e.ids.subjectForNode(node, fqn).Value)
			ids.byNode[node] = id
			ids.bySpan[spanKey{e.file.Name(), node.Span()}] = id
			if fqn != "" {
				ids.byName[fqn] = id
			}
		}
	}
	return ids, nil
}

// spanKey names a declaration by its document and span, across parses of it.
type spanKey struct {
	doc  string
	span source.Span
}

// OfDeclaration is the elementId written for the element declared at decl in
// document doc, named fqn: the node's own, its span's in a reparse, else Of(fqn).
func (ids *ElementIDs) OfDeclaration(doc string, decl ast.Node, fqn string) (string, bool) {
	if ids == nil {
		return "", false
	}
	if decl != nil {
		if id, ok := ids.byNode[decl]; ok {
			return id, true
		}
		if id, ok := ids.bySpan[spanKey{doc, decl.Span()}]; ok {
			return id, true
		}
	}
	return ids.Of(fqn)
}

// Of is the elementId written for the element named fqn: the model's own as
// the conversion wrote it, or, for a standard library element the model does
// not declare, the normative id every reference to it carries. False where the
// conversion writes none, as for a library element a model annotates with an
// id of its own, whose references are written by name.
func (ids *ElementIDs) Of(fqn string) (string, bool) {
	if ids == nil || fqn == "" {
		return "", false
	}
	if id, ok := ids.byName[fqn]; ok {
		return id, true
	}
	if len(ids.encoders) == 0 || ids.res == nil || ids.res.Index() == nil {
		return "", false
	}
	facts := ids.encoders[0].ids
	for _, sym := range ids.res.Index().LookupQualified(fqn) {
		if libraryFQN, ok := facts.libraryElement(sym); ok {
			return rdf.LocalName(facts.subjectForNode(sym.Decl, libraryFQN).Value), true
		}
	}
	return "", false
}

// ElementIDDocument is a document whose elementIds are read: its parse and the
// parse's errors; one given by its text alone is parsed for the conversion.
type ElementIDDocument struct {
	File       *source.SourceFile
	Root       *ast.RootNamespace
	ParseDiags []parser.Diagnostic
}

// ElementIDReader reads the elementId Convert writes for each element of a
// model, from one conversion of the model made on first use. A model Convert
// refuses has no ids: the refusal is kept and every read reports it.
type ElementIDReader struct {
	documents []ElementIDDocument
	once      sync.Once
	ids       *ElementIDs
	err       error
}

// NewElementIDReader reads the elementIds of the model documents make; a
// document without text is not written. Nothing is converted until Read.
func NewElementIDReader(documents []ElementIDDocument) *ElementIDReader {
	return &ElementIDReader{documents: documents}
}

// Read is the elementId written for the element declared at decl in document
// doc, named fqn (ElementIDs.OfDeclaration), or why the model has none.
func (r *ElementIDReader) Read(doc string, decl ast.Node, fqn string) (string, error) {
	if r == nil {
		return "", nil
	}
	r.once.Do(r.convert)
	if r.err != nil {
		return "", r.err
	}
	id, _ := r.ids.OfDeclaration(doc, decl, fqn)
	return id, nil
}

// convert mints the ids as Convert writes them: one document is parsed afresh
// from its text, as ToRDF does; several are taken over the trees read.
func (r *ElementIDReader) convert() {
	documents := make([]ModelDocument, 0, len(r.documents))
	for _, doc := range r.documents {
		if doc.File == nil {
			continue
		}
		root, diags := doc.Root, doc.ParseDiags
		if root == nil {
			p := parser.New(doc.File)
			root, diags = p.ParseFile(), p.Diagnostics
		}
		if syntax := parser.SyntaxErrorOf(doc.File.Name(), doc.File, diags); syntax != nil {
			r.err = syntax
			return
		}
		documents = append(documents, ModelDocument{File: doc.File, Root: root})
	}
	if len(documents) == 1 {
		given := documents[0].File
		// ToRDF reads a document in the language its name gives it; a name
		// giving none keeps the language the document was held under.
		kind := source.KindOf(given.Name())
		if kind == source.KindUnknown {
			kind = given.Kind()
		}
		file := source.NewWithKind(given.Name(), given.Bytes(), kind)
		p := parser.New(file)
		root := p.ParseFile()
		if syntax := parser.SyntaxErrorOf(file.Name(), file, p.Diagnostics); syntax != nil {
			r.err = syntax
			return
		}
		e, err := newEncoder(file, root, "", IDQualifiedName)
		if err != nil {
			r.err = err
			return
		}
		r.ids, r.err = identityIDsOf(e.res, []*encoder{e})
		return
	}
	r.ids, r.err = NewElementIDs(documents)
}
