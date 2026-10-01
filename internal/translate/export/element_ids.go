package export

import (
	"fmt"

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

// NewElementIDsOfDocument is NewElementIDs for a model of one document, which
// is converted from its text as a conversion of one document is: parsed
// afresh, in the language its name gives it, then written alone (ToRDF). A
// document the parser cannot read whole converts to nothing, so it is refused.
func NewElementIDsOfDocument(name string, data []byte) (*ElementIDs, error) {
	file := source.New(name, data)
	p := parser.New(file)
	root := p.ParseFile()
	if len(p.Diagnostics) > 0 {
		return nil, fmt.Errorf("%s: %d syntax error(s)", name, len(p.Diagnostics))
	}
	e, err := encodeDocument(file, root, "", IDQualifiedName)
	if err != nil {
		return nil, err
	}
	return elementIDsOf(e.graph, e.res, []*encoder{e}), nil
}

// elementIDsOf records the elementId a converted graph writes for each element,
// by the declaration each encoder wrote it for and by its qualified (or
// positional) name.
func elementIDsOf(graph *rdf.Graph, res *resolve.Resolver, encoders []*encoder) *ElementIDs {
	ids := &ElementIDs{byNode: map[ast.Node]string{}, byName: map[string]string{}, res: res, encoders: encoders}
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
			}
		}
	}
	return ids
}

// OfDeclaration is the elementId written for the element declared at decl,
// named fqn: the one its own declaration was written with where the conversion
// encoded that node, else Of(fqn). Two identity scopes may declare one
// qualified name, and the declaration tells their ids apart where the name
// cannot.
func (ids *ElementIDs) OfDeclaration(decl ast.Node, fqn string) (string, bool) {
	if ids == nil {
		return "", false
	}
	if decl != nil {
		if id, ok := ids.byNode[decl]; ok {
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
