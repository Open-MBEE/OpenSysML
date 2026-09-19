// Package rdf provides the minimal RDF graph model, Turtle writer and Turtle
// reader that the SysML v2 ↔ RDF conversion in internal/translate/export is built
// on.
//
// The model is deliberately small: an ordered list of triples over IRIs and
// literals. Blank nodes, RDF collections and named graphs are not represented
// at all, so a Turtle document that uses them is rejected with a diagnostic
// rather than silently losing data — a converted model that quietly dropped
// statements would be worse than one that refused to convert.
package rdf

import (
	"fmt"
	"strings"
)

// TermKind discriminates the two node kinds this model represents.
type TermKind int

const (
	// TermIRI is an absolute IRI reference.
	TermIRI TermKind = iota
	// TermLiteral is a lexical value with an optional datatype IRI or
	// language tag (never both, per RDF 1.1).
	TermLiteral
)

// Term is one RDF node: an IRI or a literal.
type Term struct {
	Kind TermKind
	// Value is the IRI for TermIRI, or the lexical form for TermLiteral.
	Value string
	// Datatype is the datatype IRI of a literal. Empty means xsd:string for a
	// plain literal, and is always empty when Lang is set.
	Datatype string
	// Lang is the BCP 47 language tag of a literal, empty when untagged.
	Lang string
}

// IRI returns an IRI term.
func IRI(iri string) Term { return Term{Kind: TermIRI, Value: iri} }

// String returns a plain string literal.
func String(value string) Term { return Term{Kind: TermLiteral, Value: value} }

// TypedLiteral returns a literal with an explicit datatype IRI.
func TypedLiteral(value, datatype string) Term {
	return Term{Kind: TermLiteral, Value: value, Datatype: datatype}
}

// Bool returns an xsd:boolean literal.
func Bool(value bool) Term {
	return TypedLiteral(fmt.Sprintf("%t", value), XSD+"boolean")
}

// Int returns an xsd:integer literal.
func Int(value int) Term {
	return TypedLiteral(fmt.Sprintf("%d", value), XSD+"integer")
}

// IsIRI reports whether t is an IRI term.
func (t Term) IsIRI() bool { return t.Kind == TermIRI }

// IsLiteral reports whether t is a literal term.
func (t Term) IsLiteral() bool { return t.Kind == TermLiteral }

// Equal reports whether two terms are identical, including datatype and tag.
func (t Term) Equal(other Term) bool { return t == other }

// String renders the term in Turtle-ish form for diagnostics and tests.
func (t Term) String() string {
	if t.IsIRI() {
		return "<" + t.Value + ">"
	}
	out := quoteLiteral(t.Value)
	switch {
	case t.Lang != "":
		return out + "@" + t.Lang
	case t.Datatype != "":
		return out + "^^<" + t.Datatype + ">"
	}
	return out
}

// Triple is one subject-predicate-object statement. Subjects and predicates
// are always IRIs in this model.
type Triple struct {
	Subject   Term
	Predicate Term
	Object    Term
}

// Graph is an ordered set of triples. Insertion order is preserved so that a
// serialized document is stable, and duplicates are dropped so that repeated
// conversion of the same model is idempotent.
type Graph struct {
	triples []Triple
	seen    map[Triple]bool
	// Prefixes maps prefix label to namespace IRI for serialization. It never
	// affects the meaning of the graph.
	Prefixes map[string]string

	// index groups statements by subject. It is built on the first lookup and
	// kept current as triples are added.
	index map[Term]*subjectIndex
}

// subjectIndex holds one subject's statements, keeping predicates in insertion
// order so serialization stays stable.
type subjectIndex struct {
	predicates []string
	objects    map[string][]Term
}

// NewGraph returns an empty graph carrying the SysML prefix bindings.
func NewGraph() *Graph {
	g := &Graph{seen: make(map[Triple]bool), Prefixes: make(map[string]string)}
	for prefix, ns := range DefaultPrefixes {
		g.Prefixes[prefix] = ns
	}
	return g
}

// Add appends a triple unless the graph already contains it.
func (g *Graph) Add(subject, predicate, object Term) {
	g.AddTriple(Triple{Subject: subject, Predicate: predicate, Object: object})
}

// AddTriple appends t unless the graph already contains it.
func (g *Graph) AddTriple(t Triple) {
	if g.seen[t] {
		return
	}
	g.seen[t] = true
	g.triples = append(g.triples, t)
	if g.index != nil {
		g.index[t.Subject] = indexTriple(g.index[t.Subject], t)
	}
}

// indexTriple records t under its subject's index entry, creating the entry
// when si is nil.
func indexTriple(si *subjectIndex, t Triple) *subjectIndex {
	if si == nil {
		si = &subjectIndex{objects: make(map[string][]Term)}
	}
	if _, seen := si.objects[t.Predicate.Value]; !seen {
		si.predicates = append(si.predicates, t.Predicate.Value)
	}
	si.objects[t.Predicate.Value] = append(si.objects[t.Predicate.Value], t.Object)
	return si
}

// subjects returns the per-subject index, building it if needed. Without it a
// property read scans every triple, making a decode quadratic in model size;
// rebuilding it after every addition would make an encode quadratic too.
func (g *Graph) subjects() map[Term]*subjectIndex {
	if g.index != nil {
		return g.index
	}
	g.index = make(map[Term]*subjectIndex)
	for _, t := range g.triples {
		g.index[t.Subject] = indexTriple(g.index[t.Subject], t)
	}
	return g.index
}

// Triples returns the triples in insertion order. The result aliases the
// graph's storage and must not be mutated.
func (g *Graph) Triples() []Triple { return g.triples }

// Has reports whether the graph contains t.
func (g *Graph) Has(t Triple) bool { return g.seen[t] }

// Len returns the number of triples.
func (g *Graph) Len() int { return len(g.triples) }

// Subjects returns every distinct subject IRI in insertion order.
func (g *Graph) Subjects() []Term {
	var out []Term
	seen := make(map[string]bool)
	for _, t := range g.triples {
		if seen[t.Subject.Value] {
			continue
		}
		seen[t.Subject.Value] = true
		out = append(out, t.Subject)
	}
	return out
}

// Predicates returns every predicate subject states, in insertion order.
func (g *Graph) Predicates(subject Term) []string {
	si := g.subjects()[subject]
	if si == nil {
		return nil
	}
	out := make([]string, len(si.predicates))
	copy(out, si.predicates)
	return out
}

// Objects returns the objects of every (subject, predicate) statement, in
// insertion order.
func (g *Graph) Objects(subject Term, predicate string) []Term {
	si := g.subjects()[subject]
	if si == nil {
		return nil
	}
	found := si.objects[predicate]
	if len(found) == 0 {
		return nil
	}
	// Copied so a caller appending to the result cannot reach into the index.
	out := make([]Term, len(found))
	copy(out, found)
	return out
}

// Object returns the first object of (subject, predicate) and whether one
// exists. A property the mapping treats as single-valued is read with this.
func (g *Graph) Object(subject Term, predicate string) (Term, bool) {
	si := g.subjects()[subject]
	if si == nil {
		return Term{}, false
	}
	if found := si.objects[predicate]; len(found) > 0 {
		return found[0], true
	}
	return Term{}, false
}

// HasProperty reports whether the subject states the predicate at all.
func (g *Graph) HasProperty(subject Term, predicate string) bool {
	_, ok := g.Object(subject, predicate)
	return ok
}

// Lexical returns the lexical form of the first object of (subject,
// predicate), for a property whose value is read as text regardless of whether
// it was written as an IRI or a literal.
func (g *Graph) Lexical(subject Term, predicate string) (string, bool) {
	obj, ok := g.Object(subject, predicate)
	if !ok {
		return "", false
	}
	return obj.Value, true
}

// BoolValue returns the boolean value of (subject, predicate), defaulting to
// false when absent. Any lexical form other than "true" or "1" is false, which
// matches the way the exporter only ever writes true.
func (g *Graph) BoolValue(subject Term, predicate string) bool {
	value, ok := g.Lexical(subject, predicate)
	if !ok {
		return false
	}
	return value == "true" || value == "1"
}

// Type returns the first rdf:type of subject, or "" when it has none.
func (g *Graph) Type(subject Term) string {
	obj, ok := g.Object(subject, RDFType)
	if !ok {
		return ""
	}
	return obj.Value
}

// quoteLiteral renders a lexical form as a Turtle quoted string. Newlines are
// escaped rather than written in a long literal, so every triple stays on one
// line and a line-oriented tool can drop a property whole.
func quoteLiteral(value string) string {
	return `"` + escapeShort(value) + `"`
}

// escapeShort uses Turtle's ECHAR forms and writes any other control as \uXXXX.
func escapeShort(value string) string {
	var b strings.Builder
	for _, r := range value {
		switch r {
		case '\\':
			b.WriteString(`\\`)
		case '"':
			b.WriteString(`\"`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		case '\b':
			b.WriteString(`\b`)
		case '\f':
			b.WriteString(`\f`)
		default:
			if r < 0x20 || r == 0x7f {
				fmt.Fprintf(&b, `\u%04X`, r)
				continue
			}
			b.WriteRune(r)
		}
	}
	return b.String()
}
