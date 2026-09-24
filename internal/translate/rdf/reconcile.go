package rdf

import (
	"fmt"
	"sort"
	"strings"
)

// CollectionConflictError is a collection stated as typed triples and as a JSON
// annotation with different members; the reader refuses rather than pick one.
type CollectionConflictError struct {
	Subject    string   // subject IRI
	Key        string   // property key: the local name of the sysml: predicate
	Triples    []string // members as the typed triples state them, as JSON
	Annotation []string // members as the annotation states them, as JSON
}

func (e *CollectionConflictError) Error() string {
	return fmt.Sprintf("cannot convert <%s>: sysml:%s is stated twice and the two disagree: the typed triples hold [%s], the json:%s annotation holds [%s]",
		e.Subject, e.Key, strings.Join(e.Triples, ","), e.Key, strings.Join(e.Annotation, ","))
}

// AnnotationError is a collection annotation the reader cannot take as a
// collection: not exactly one literal, not the JSON array the service stores,
// or an array that repeats a member, which no set of triples can hold.
type AnnotationError struct {
	Subject string // subject IRI
	Key     string // property key: the annotation predicate's local name
	Note    string
}

func (e *AnnotationError) Error() string {
	return fmt.Sprintf("cannot convert the annotation json:%s of <%s>: %s", e.Key, e.Subject, e.Note)
}

// collection is one annotated property of one subject, settled in annotation order.
type collection struct {
	subject Term
	key     string
	members []Term
	bare    bool // the graph also states the members as typed triples
}

// ReconcileCollections returns a graph whose sysml: triples state every annotated
// collection in annotation order: materialized if annotation-only, checked if both.
// A graph without annotations is returned as it is.
func ReconcileCollections(graph *Graph) (*Graph, error) {
	var annotations []Triple
	for _, triple := range graph.Triples() {
		if IsAnnotationJSON(triple.Predicate.Value) {
			annotations = append(annotations, triple)
		}
	}
	if len(annotations) == 0 {
		return graph, nil
	}
	type pair struct {
		subject   Term
		predicate string
	}
	ids := subjectsByID(graph)
	settled := map[pair]collection{}
	for _, triple := range annotations {
		c, err := reconcileCollection(graph, ids, triple.Subject, strings.TrimPrefix(triple.Predicate.Value, AnnotationJSON))
		if err != nil {
			return nil, err
		}
		settled[pair{c.subject, SysML + c.key}] = c
	}
	out := NewGraph()
	for label, ns := range graph.Prefixes {
		out.Prefixes[label] = ns
	}
	emitted := map[pair]bool{}
	for _, triple := range graph.Triples() {
		p := pair{triple.Subject, triple.Predicate.Value}
		if c, ok := settled[p]; ok && c.bare {
			if !emitted[p] {
				emitted[p] = true
				for _, member := range c.members {
					out.Add(c.subject, triple.Predicate, member)
				}
			}
			continue
		}
		out.AddTriple(triple)
		if IsAnnotationJSON(triple.Predicate.Value) {
			c := settled[pair{triple.Subject, SysML + strings.TrimPrefix(triple.Predicate.Value, AnnotationJSON)}]
			if !c.bare {
				for _, member := range c.members {
					out.Add(c.subject, SysMLTerm(c.key), member)
				}
			}
		}
	}
	return out, nil
}

// reconcileCollection settles one annotated property: one JSON-array literal,
// agreeing with the typed triples where the graph states those too.
func reconcileCollection(graph *Graph, ids map[string][]Term, subject Term, key string) (collection, error) {
	c := collection{subject: subject, key: key}
	refuse := func(note string) (collection, error) {
		return c, &AnnotationError{Subject: subject.Value, Key: key, Note: note}
	}
	objects := graph.Objects(subject, AnnotationJSON+key)
	if len(objects) != 1 {
		return refuse(fmt.Sprintf("it has %d objects, and a collection is stated by exactly one JSON literal", len(objects)))
	}
	if !objects[0].IsLiteral() {
		return refuse(fmt.Sprintf("its object %s is not a literal, and a collection is stated by one JSON literal", objects[0]))
	}
	members, err := ParseCollectionJSON(objects[0].Value)
	if err != nil {
		return refuse(err.Error())
	}
	annotation := make([]string, len(members))
	seen := map[string]int{}
	for i, member := range members {
		if annotation[i], err = member.JSON(); err != nil {
			return refuse(err.Error())
		}
		if first, dup := seen[annotation[i]]; dup {
			return refuse(fmt.Sprintf("the member %s appears at index %d and again at %d; a graph holds each triple once, so a repeated member cannot be kept", annotation[i], first, i))
		}
		seen[annotation[i]] = i
	}
	bare := graph.Objects(subject, SysML+key)
	if len(bare) == 0 {
		for _, member := range members {
			term, err := materialize(member, subject, ids)
			if err != nil {
				return refuse(err.Error())
			}
			c.members = append(c.members, term)
		}
		return c, nil
	}
	c.bare = true
	spelled := make([]string, len(bare))
	for i, term := range bare {
		if spelled[i], err = ValueJSON(subject, term); err != nil {
			return refuse(fmt.Sprintf("the typed sysml:%s value %s has no JSON spelling: %v", key, term, err))
		}
	}
	if !sameMembers(spelled, annotation) {
		return c, &CollectionConflictError{Subject: subject.Value, Key: key, Triples: spelled, Annotation: annotation}
	}
	// Both agree; the annotation carries the order.
	used := make([]bool, len(bare))
	for _, want := range annotation {
		for i, have := range spelled {
			if !used[i] && have == want {
				used[i] = true
				c.members = append(c.members, bare[i])
				break
			}
		}
	}
	return c, nil
}

// sameMembers compares two spellings as multisets; typed triples carry no order.
func sameMembers(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	as, bs := append([]string(nil), a...), append([]string(nil), b...)
	sort.Strings(as)
	sort.Strings(bs)
	for i := range as {
		if as[i] != bs[i] {
			return false
		}
	}
	return true
}

// subjectsByID indexes the subjects minted in an identity namespace by
// scope-qualified id; a subject in any other namespace is no reference target.
func subjectsByID(graph *Graph) map[string][]Term {
	ids := map[string][]Term{}
	for _, subject := range graph.Subjects() {
		if subject.IsIRI() && namespaceOf(subject.Value) != "" {
			id := ownerID(subject.Value)
			ids[id] = append(ids[id], subject)
		}
	}
	return ids
}

// materialize turns an annotation member into the term a typed triple would hold:
// the element or expression node carrying the id in the scope the @id names; an
// unknown id stays the element IRI, dangling as usual.
func materialize(member CollectionMember, referrer Term, ids map[string][]Term) (Term, error) {
	if member.ID == "" {
		return member.Literal, nil
	}
	ref := ReferenceIRI(referrer, member.ID)
	candidates := ids[ownerID(ref.Value)]
	switch len(candidates) {
	case 0:
		return ref, nil
	case 1:
		return candidates[0], nil
	}
	// An element's collection holds elements, an expression node's its operand nodes.
	var own []Term
	for _, candidate := range candidates {
		if namespaceOf(candidate.Value) == namespaceOf(referrer.Value) {
			own = append(own, candidate)
		}
	}
	if len(own) == 1 {
		return own[0], nil
	}
	names := make([]string, len(candidates))
	for i, candidate := range candidates {
		names[i] = candidate.String()
	}
	return Term{}, fmt.Errorf("the member {\"@id\":%q} names %d subjects, %s, and the annotation cannot tell which", member.ID, len(candidates), strings.Join(names, " and "))
}

// namespaceOf is the identity namespace an IRI is minted in, Element or Expression; "" for any other.
func namespaceOf(iri string) string {
	for _, ns := range []string{Element, Expression} {
		if strings.HasPrefix(iri, ns) {
			return ns
		}
	}
	return ""
}
