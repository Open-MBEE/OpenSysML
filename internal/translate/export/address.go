package export

import (
	"strings"
	"unicode/utf8"

	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
)

// xPositionalName is the address this mapping spells for an element the
// metamodel derives no qualifiedName for: an unnamed element, or one owned
// under such an element, named by its position among its owner's members
// (`P::@2`, `P::@2::x`). KerML §8.3.2.1 derives Element::qualifiedName only
// from a name and an owning namespace that has one, so the address is stated
// as sysx:positionalName and sysml:qualifiedName is reserved for names.
const xPositionalName = "positionalName"

// isPositionalName reports whether an address has an unquoted @ segment,
// the writer's address for an unnamed or minted member, rather than a name.
func isPositionalName(name string) bool {
	for {
		segment, rest, more := cutIdentitySegment(name)
		if strings.HasPrefix(segment, "@") {
			return true
		}
		if !more {
			return false
		}
		name = rest
	}
}

// addressTerm is the predicate an element's address is stated by:
// sysml:qualifiedName for a name, sysx:positionalName for a position.
func addressTerm(name string) rdf.Term {
	if isPositionalName(name) {
		return rdf.OpenSysMLTerm(xPositionalName)
	}
	return rdf.SysMLTerm(pQualifiedName)
}

// addressOf is the address subject is stated under, by either predicate, or
// the positional address its element IRI encodes when a graph states neither
// (one stripped of the sysx: vocabulary keeps its IRIs).
func addressOf(graph *rdf.Graph, subject rdf.Term) (string, bool) {
	if name, ok := graph.Lexical(subject, rdf.SysML+pQualifiedName); ok {
		return name, true
	}
	if name, ok := graph.Lexical(subject, rdf.OpenSysML+xPositionalName); ok {
		return name, true
	}
	return encodedPositionalAddress(subject)
}

// statesAddress reports whether subject states an address by either predicate.
func statesAddress(graph *rdf.Graph, subject rdf.Term) bool {
	return graph.HasProperty(subject, rdf.SysML+pQualifiedName) ||
		graph.HasProperty(subject, rdf.OpenSysML+xPositionalName)
}

// hasAddress reports whether subject has an address, stated or IRI-encoded.
func hasAddress(graph *rdf.Graph, subject rdf.Term) bool {
	_, ok := addressOf(graph, subject)
	return ok
}

// encodedPositionalAddress is the positional address an element IRI of this
// mapping encodes (rdf.EncodeElementID). A minted relationship's id carries a
// suffix no address encodes, so it decodes to no address, or to no text.
func encodedPositionalAddress(subject rdf.Term) (string, bool) {
	if !subject.IsIRI() {
		return "", false
	}
	id, ok := strings.CutPrefix(subject.Value, rdf.Element)
	if !ok {
		return "", false
	}
	name, ok := rdf.DecodeElementID(id)
	if !ok || !utf8.ValidString(name) || !isPositionalName(name) {
		return "", false
	}
	return name, true
}
