package export

import (
	"sync"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
)

// ElementIDs reports the elementId a conversion of a model writes for each of
// its elements, in the qualified id form. It is read from that conversion
// itself — ToRDF for one document, modelToRDF (ModelToRDFWith) for several,
// the paths Convert takes — so a library file's copy, a declared id and scope
// qualification come out as they are written, and a model the conversion
// refuses has no ids at all.
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

// NewElementIDs converts the documents of one model and records the elementId
// written for each element by its qualified (or positional) name. The error is
// the conversion's: a model Convert refuses has no elementIds to report.
func NewElementIDs(documents []ModelDocument) (*ElementIDs, error) {
	graph, res, encoders, err := modelToRDF(documents, IDQualifiedName)
	if err != nil {
		return nil, err
	}
	return elementIDsOf(graph, res, encoders), nil
}

// elementIDsOf records the elementId a converted graph writes for each element,
// by the declaration each encoder wrote it for and by its qualified (or
// positional) name.
func elementIDsOf(graph *rdf.Graph, res *resolve.Resolver, encoders []*encoder) *ElementIDs {
	ids := &ElementIDs{byNode: map[ast.Node]string{}, bySpan: map[spanKey]string{}, byName: map[string]string{}, res: res, encoders: encoders}
	for _, subject := range graph.Subjects() {
		name, named := graph.Lexical(subject, rdf.SysML+pQualifiedName)
		id, identified := graph.Lexical(subject, rdf.SysML+pElementID)
		if named && identified {
			ids.byName[name] = id
		}
	}
	for _, e := range encoders {
		for node, fqn := range e.fqn {
			if id, ok := graph.Lexical(e.ids.subjectForNode(node, fqn), rdf.SysML+pElementID); ok {
				ids.byNode[node] = id
				ids.bySpan[spanKey{e.file.Name(), node.Span()}] = id
			}
		}
	}
	return ids
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

// convert makes the ids as Convert writes them: one document is parsed afresh
// from its text, as ToRDF does; several are written over the trees read.
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
		e, err := encodeDocument(file, root, "", IDQualifiedName)
		if err != nil {
			r.err = err
			return
		}
		r.ids = elementIDsOf(e.graph, e.res, []*encoder{e})
		return
	}
	r.ids, r.err = NewElementIDs(documents)
}
