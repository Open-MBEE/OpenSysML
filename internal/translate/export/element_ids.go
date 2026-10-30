package export

import (
	"fmt"
	"strings"
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
	// membershipByNode, membershipBySpan and membershipByName are the ids written
	// for the owning membership of each declared element, keyed as the element's.
	membershipByNode map[ast.Node]string
	membershipBySpan map[spanKey]string
	membershipByName map[string]string
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
	referenced := make([]bool, len(documents))
	for i, doc := range documents {
		referenced[i] = doc.Referenced
	}
	return identityIDsOf(res, encoders, referenced)
}

// identityIDsOf records each written declaration's id (its subject's local name)
// once the claims Convert makes for the documents (as modelToRDF does) succeed.
func identityIDsOf(res *resolve.Resolver, encoders []*encoder, referenced []bool) (*ElementIDs, error) {
	declaredIn := map[string]mintedSubject{}
	for i, e := range encoders {
		minted := e.declaredSubjects()
		if !referenced[i] {
			if err := e.mintDeclarations(); err != nil {
				return nil, fmt.Errorf("%s: %w", e.file.Name(), err)
			}
			minted = e.minted
		}
		if err := declaredOnce(declaredIn, minted); err != nil {
			return nil, err
		}
	}
	ids := &ElementIDs{byNode: map[ast.Node]string{}, bySpan: map[spanKey]string{}, byName: map[string]string{},
		membershipByNode: map[ast.Node]string{}, membershipBySpan: map[spanKey]string{}, membershipByName: map[string]string{},
		res: res, encoders: encoders}
	for _, e := range encoders {
		parents := e.parentDeclarations()
		for _, node := range e.writtenDeclarations() {
			fqn := e.fqn[node]
			subject := e.ids.subjectForNode(node, fqn)
			id := rdf.LocalName(subject.Value)
			ids.byNode[node] = id
			ids.bySpan[spanKey{e.file.Name(), node.Span()}] = id
			if fqn != "" {
				ids.byName[fqn] = id
			}
			if membership, ok := e.writtenOwningMembership(node, parents[node], fqn); ok {
				local := rdf.LocalName(membership.Value)
				ids.membershipByNode[node] = local
				ids.membershipBySpan[spanKey{e.file.Name(), node.Span()}] = local
				ids.membershipByName[fqn] = local
			}
			for i, end := range writtenConnectorEnds(node) {
				if end == nil {
					continue
				}
				endID := rdf.LocalName(e.ids.mintedNode(rdf.ExpressionIRI(subject, fmt.Sprintf("end%d", i)), subject, fmt.Sprintf("end%d", i)).Value)
				ids.byNode[end] = endID
				ids.bySpan[spanKey{e.file.Name(), end.Span()}] = endID
			}
		}
	}
	return ids, nil
}

// writtenConnectorEnds are the ends of a head that binds them (`connect a to b`),
// by position, as bindingEnds writes them: nil where no end feature is written.
func writtenConnectorEnds(node ast.Node) []*ast.ConnectorEnd {
	usage, ok := node.(*ast.Usage)
	if !ok || !verbatimUsage(usage) || len(usage.ConnectorEnds) == 0 {
		return nil
	}
	ends := make([]*ast.ConnectorEnd, len(usage.ConnectorEnds))
	for i, end := range usage.ConnectorEnds {
		if end != nil && end.AttachedTarget() != nil {
			ends[i] = end
		}
	}
	return ends
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

// OwningMembershipOfDeclaration is the elementId written for the owning membership
// of the element declared at decl in doc, named fqn; false for a root or an unknown element.
func (ids *ElementIDs) OwningMembershipOfDeclaration(doc string, decl ast.Node, fqn string) (string, bool) {
	if ids == nil {
		return "", false
	}
	if decl != nil {
		if id, ok := ids.membershipByNode[decl]; ok {
			return id, true
		}
		if id, ok := ids.membershipBySpan[spanKey{doc, decl.Span()}]; ok {
			return id, true
		}
	}
	if id, ok := ids.membershipByName[fqn]; ok {
		return id, true
	}
	// A standard library element the model does not declare: its normative membership,
	// or the one minted beside it, as owningMembershipOf writes a reference to it.
	if !strings.Contains(fqn, "::") || len(ids.encoders) == 0 || ids.res == nil || ids.res.Index() == nil {
		return "", false
	}
	facts := ids.encoders[0].ids
	for _, sym := range ids.res.Index().LookupQualified(fqn) {
		if libraryFQN, ok := facts.libraryElement(sym); ok {
			member := facts.subjectForNode(sym.Decl, libraryFQN)
			return rdf.LocalName(facts.owningMembershipOf(sym.Decl, member).Value), true
		}
	}
	return "", false
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

// ReadOwningMembership is the elementId of the owning membership of the element
// declared at decl in doc, named fqn; empty where the conversion writes none.
func (r *ElementIDReader) ReadOwningMembership(doc string, decl ast.Node, fqn string) (string, error) {
	if r == nil {
		return "", nil
	}
	r.once.Do(r.convert)
	if r.err != nil {
		return "", r.err
	}
	id, _ := r.ids.OwningMembershipOfDeclaration(doc, decl, fqn)
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
		r.ids, r.err = identityIDsOf(e.res, []*encoder{e}, []bool{false})
		return
	}
	r.ids, r.err = NewElementIDs(documents)
}
