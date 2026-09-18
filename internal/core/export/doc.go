// Package export saves a SysML v2 model to a file and converts between the
// two representations OpenSysML can write: SysML textual notation and RDF
// Turtle. It also reads a third, SysML v1 in XMI, which is migrated to v2
// notation on the way in.
//
// # SysML output
//
// Saving a model that came from source writes that source, re-indented by
// internal/core/format. Printing the AST instead would drop comments, notes and
// anything the parser recorded as an ErrorNode, so a save has to keep the token
// stream (see the format package doc).
//
// # RDF output
//
// The graph uses the SysML vocabulary and element IRIs of the Flexo MMS SysML
// v2 service (https://www.omg.org/spec/SysML# and urn:sysmlv2:element:), so a
// converted model loads into that service's triplestore. Elements are addressed
// by qualified name (`urn:sysmlv2:element:Demo::Vehicle`), which makes the IRIs
// stable across conversions of the same model rather than newly generated each
// time.
//
// Each element carries its metaclass as rdf:type and its declaration as SysML
// metamodel properties: declaredName, declaredShortName, owningNamespace,
// visibility, direction, the feature flags, the typing and specialization
// clauses, multiplicity bounds and its value; expression-valued positions are
// expression trees. Properties the metamodel does not define live in a separate
// urn:opensysml:sysml: namespace so a consumer can tell them from the standard
// vocabulary: memberIndex (declaration order, which the notation is sensitive to
// and RDF is not), hasBody, the end forms of heads that bind ends, and each
// element's lines as written, comments included, as sourceText and sourceTail.
//
// # XMI input
//
// SysML v1 as UML XMI (OMG XMI 2.5.1, the Eclipse UML2 .uml serialization
// Papyrus writes, or a .mdzip archive holding it) is migrated to v2 notation by
// internal/core/migrate, then takes the notation path. XMI is never written.
//
// # RDF back to notation
//
// The structural triples are authoritative. An element is written from its
// source text while that text still states what the graph states — so comments
// and layout survive a round trip of an unedited model — and canonically where
// the two disagree or the graph carries no text, as one from another tool does
// not. docs/reference/rdf-mapping.md documents the mapping and its limitations.
package export
