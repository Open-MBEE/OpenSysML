package rdf

import (
	"sort"
	"strings"
)

// WriteTurtle serializes g as a Turtle document: the prefix bindings the graph
// carries, then one block per subject in insertion order, with the subject's
// predicates grouped and separated by ';'.
func WriteTurtle(g *Graph) []byte {
	var b strings.Builder
	writePrefixes(&b, g)
	for i, subject := range g.Subjects() {
		if i > 0 {
			b.WriteString("\n")
		}
		writeSubject(&b, g, subject)
	}
	return []byte(b.String())
}

func writePrefixes(b *strings.Builder, g *Graph) {
	labels := make([]string, 0, len(g.Prefixes))
	for label := range g.Prefixes {
		labels = append(labels, label)
	}
	sort.Strings(labels)
	for _, label := range labels {
		b.WriteString("@prefix " + label + ": <" + g.Prefixes[label] + "> .\n")
	}
	if len(labels) > 0 {
		b.WriteString("\n")
	}
}

// writeSubject emits one subject block. rdf:type is written first, as `a`, so
// the metaclass of an element is the first thing a reader sees.
func writeSubject(b *strings.Builder, g *Graph, subject Term) {
	b.WriteString(g.term(subject) + "\n")
	si := g.subjects()[subject]
	if si == nil {
		return
	}
	byPredicate := si.objects
	predicates := make([]string, len(si.predicates))
	copy(predicates, si.predicates)
	sort.SliceStable(predicates, func(i, j int) bool {
		return predicates[i] == RDFType && predicates[j] != RDFType
	})
	for i, predicate := range predicates {
		objects := make([]string, 0, len(byPredicate[predicate]))
		for _, object := range byPredicate[predicate] {
			objects = append(objects, g.term(object))
		}
		name := g.term(IRI(predicate))
		if predicate == RDFType {
			name = "a"
		}
		separator := " ;"
		if i == len(predicates)-1 {
			separator = " ."
		}
		b.WriteString("    " + name + " " + strings.Join(objects, ", ") + separator + "\n")
	}
}

// term renders a term, abbreviating an IRI with a prefix binding when the
// result is a legal Turtle prefixed name.
func (g *Graph) term(t Term) string {
	if t.IsLiteral() {
		out := quoteLiteral(t.Value)
		switch {
		case t.Lang != "":
			return out + "@" + t.Lang
		case t.Datatype != "":
			return out + "^^" + g.term(IRI(t.Datatype))
		}
		return out
	}
	// Overlapping bindings can both match and map order is randomised, so the
	// best candidate is chosen outright: the longest namespace gives the most
	// specific abbreviation, and the smallest label breaks a tie.
	bestLabel, bestNS := "", ""
	for label, ns := range g.Prefixes {
		if !strings.HasPrefix(t.Value, ns) {
			continue
		}
		if !isPrefixedLocalName(strings.TrimPrefix(t.Value, ns)) {
			continue
		}
		if bestLabel == "" || len(ns) > len(bestNS) || (len(ns) == len(bestNS) && label < bestLabel) {
			bestLabel, bestNS = label, ns
		}
	}
	if bestLabel != "" {
		return bestLabel + ":" + strings.TrimPrefix(t.Value, bestNS)
	}
	return "<" + t.Value + ">"
}

// isPrefixedLocalName reports whether local can follow a prefix label without
// escaping. Turtle's PN_LOCAL production is wider than this, but staying
// conservative only costs a written-out IRI.
func isPrefixedLocalName(local string) bool {
	if local == "" {
		return false
	}
	for i, r := range local {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z':
		case r >= '0' && r <= '9', r == '_':
			if i == 0 && r != '_' {
				return false
			}
		case r == ':', r == '-', r == '.':
			if i == 0 || i == len(local)-1 {
				return false
			}
		default:
			return false
		}
	}
	return true
}
