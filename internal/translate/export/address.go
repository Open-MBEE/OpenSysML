package export

import (
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/naming"
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
		segment, rest, more := naming.CutSegment(name)
		if strings.HasPrefix(segment, "@") {
			return true
		}
		if !more {
			return false
		}
		name = rest
	}
}

// xMappingQualifiedName states a named element's address where it is not the
// spelling of sysml:qualifiedName: `P::two words` beside `P::'two words'`.
const xMappingQualifiedName = "mappingQualifiedName"

// stateAddress states a position as sysx:positionalName and a name as the derived
// sysml:qualifiedName (KerML §8.3.2.1.2), the address beside it where the two differ.
func stateAddress(add func(subject, predicate, object rdf.Term), subject rdf.Term, address string) {
	if isPositionalName(address) {
		add(subject, rdf.OpenSysMLTerm(xPositionalName), rdf.String(address))
		return
	}
	qualified := naming.Escaped(address)
	add(subject, rdf.SysMLTerm(pQualifiedName), rdf.String(qualified))
	if qualified != address || mappingQualifiedNameFromSpec(qualified) != address {
		add(subject, rdf.OpenSysMLTerm(xMappingQualifiedName), rdf.String(address))
	}
}

// addressPredicates are the predicates an address is read from, in order: the
// mapping's own statement first, the qualified name it otherwise equals next.
var addressPredicates = [...]string{
	rdf.OpenSysML + xMappingQualifiedName,
	rdf.SysML + pQualifiedName,
	rdf.OpenSysML + xPositionalName,
}

// isAddressPredicate reports whether a predicate states an element's address.
func isAddressPredicate(predicate string) bool {
	return slices.Contains(addressPredicates[:], predicate)
}

// addressOf is the address subject states, or the positional one its element IRI
// encodes when the graph states none (one stripped of sysx: keeps its IRIs).
func addressOf(graph *rdf.Graph, subject rdf.Term) (string, bool) {
	if name, ok := statedAddress(graph, subject); ok {
		return name, true
	}
	return encodedPositionalAddress(subject)
}

// statedAddress is the address subject states: its mapping or positional name,
// or the one its qualifiedName spells when the graph states that alone.
func statedAddress(graph *rdf.Graph, subject rdf.Term) (string, bool) {
	for _, predicate := range addressPredicates {
		name, ok := graph.Lexical(subject, predicate)
		if !ok {
			continue
		}
		if predicate == rdf.SysML+pQualifiedName && strings.ContainsAny(name, `'\`) {
			return mappingQualifiedNameFromSpec(name), true
		}
		return name, true
	}
	return "", false
}

// NamedAddress is the address of a subject that states a qualified name: what
// its element IRI encodes, where sysml:qualifiedName is the spelling to show.
func NamedAddress(graph *rdf.Graph, subject rdf.Term) (string, bool) {
	if !graph.HasProperty(subject, rdf.SysML+pQualifiedName) {
		return "", false
	}
	return statedAddress(graph, subject)
}

// statesAddress reports whether subject states an address by any predicate.
func statesAddress(graph *rdf.Graph, subject rdf.Term) bool {
	for _, predicate := range addressPredicates {
		if graph.HasProperty(subject, predicate) {
			return true
		}
	}
	return false
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
