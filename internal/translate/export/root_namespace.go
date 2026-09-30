package export

import (
	"sort"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/identity"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
)

// RootNamespaceSuffix ends the id of the unnamed Namespace that owns a
// document's top-level elements in the API element form. An encoded id never
// ends in a lone '_', so the id collides with no element's or membership's.
const RootNamespaceSuffix = "_ns"

const mNamespace = "Namespace"

// withRootNamespace returns graph wrapped the way the pilot serializes a
// document (KerML 1.0 § 8.3.2.4.5 Namespace): an unnamed, unowned Namespace whose
// OwningMemberships own every element the graph leaves unowned. A graph that
// already carries such a root is returned as it is.
func withRootNamespace(graph *rdf.Graph) (*rdf.Graph, error) {
	roots := unownedElements(graph)
	sort.SliceStable(roots, func(i, j int) bool {
		return intOf(graph, roots[i], rdf.OpenSysML+xMemberIndex) < intOf(graph, roots[j], rdf.OpenSysML+xMemberIndex)
	})
	if len(roots) == 0 {
		return graph, nil
	}
	for _, root := range roots {
		if transparentRootSubject(graph, root) {
			return graph, nil
		}
	}
	namespace, memberships := rootNamespaceIDs(graph, roots)
	// The wrapped graph is the source's triples with the wrapper's inserted, so
	// it is assembled as a list and never rebuilds a set of the source's triples.
	// Each triple keeps the place it first takes, as adding them one by one to a
	// graph would: an inserted triple the source states later is skipped there,
	// and one the source stated earlier is not inserted again. Every inserted
	// triple is about the namespace, a membership or a root, so only the source
	// triples about those are remembered.
	triples := make([]rdf.Triple, 0, graph.Len()+8*len(roots)+8)
	placed := map[rdf.Triple]bool{}
	wrapperSubjects := map[rdf.Term]bool{namespace: true}
	for i, root := range roots {
		wrapperSubjects[root] = true
		wrapperSubjects[memberships[i]] = true
	}
	add := func(subject, predicate, object rdf.Term) {
		t := rdf.Triple{Subject: subject, Predicate: predicate, Object: object}
		if placed[t] {
			return
		}
		placed[t] = true
		triples = append(triples, t)
	}
	sysml := rdf.SysMLTerm
	add(namespace, rdf.IRI(rdf.RDFType), sysml(mNamespace))
	add(namespace, sysml(pElementID), rdf.String(rdf.LocalName(namespace.Value)))
	for i, root := range roots {
		membership := memberships[i]
		add(namespace, sysml(pOwnedRelationship), membership)
		add(namespace, sysml(pOwnedMembership), membership)
		add(namespace, sysml(pOwnedMember), root)
	}
	for i, root := range roots {
		membership := memberships[i]
		add(membership, rdf.IRI(rdf.RDFType), sysml(mOwningMembership))
		add(membership, sysml(pElementID), rdf.String(rdf.LocalName(membership.Value)))
		add(membership, sysml(pOwner), namespace)
		add(membership, sysml(pMemberElement), root)
		add(membership, sysml(pOwnedMemberElement), root)
		add(membership, sysml(pOwnedRelatedElement), root)
		add(membership, sysml(pOwningRelatedElement), namespace)
		add(membership, sysml(pMembershipOwningNamespace), namespace)
	}
	owned := map[string]int{}
	for i, root := range roots {
		owned[root.Value] = i
	}
	for _, triple := range graph.Triples() {
		if wrapperSubjects[triple.Subject] {
			if placed[triple] {
				continue
			}
			placed[triple] = true
		}
		triples = append(triples, triple)
		if i, ok := owned[triple.Subject.Value]; ok {
			delete(owned, triple.Subject.Value)
			add(triple.Subject, sysml(pOwner), namespace)
			add(triple.Subject, sysml(pOwningNamespace), namespace)
			add(triple.Subject, sysml(pOwningRelationship), memberships[i])
			add(triple.Subject, sysml(pOwningMembership), memberships[i])
		}
	}
	if len(roots) > 1 {
		for _, c := range []struct {
			key     string
			members []rdf.Term
		}{{pOwnedRelationship, memberships}, {pOwnedMembership, memberships}, {pOwnedMember, roots}} {
			text, err := rdf.CollectionJSON(namespace, c.members)
			if err != nil {
				return nil, err
			}
			add(namespace, rdf.AnnotationJSONTerm(c.key), rdf.String(text))
		}
	}
	out := rdf.NewGraphOf(triples, graph.Prefixes)
	return out, nil
}

// withoutRootNamespace strips the wrapper withRootNamespace adds, so a graph
// read from the element form matches one the encoder builds from notation.
func withoutRootNamespace(graph *rdf.Graph) *rdf.Graph {
	dropped := map[string]bool{}
	droppedMemberIndexes := map[string]bool{}
	var indexes []rootIndex
	for _, subject := range graph.Subjects() {
		if !transparentRootSubject(graph, subject) {
			continue
		}
		dropped[subject.Value] = true
		restated, dropIndexes := rootMemberIndexes(graph, rootNamespaceMembers(graph, subject))
		indexes = append(indexes, restated...)
		if dropIndexes {
			for _, index := range restated {
				droppedMemberIndexes[index.member.Value] = true
			}
		}
		for _, membership := range graph.Objects(subject, rdf.SysML+pOwnedRelationship) {
			if graph.Type(membership) == rdf.SysML+mOwningMembership {
				dropped[membership.Value] = true
			}
		}
	}
	if len(dropped) == 0 {
		return graph
	}
	out := rdf.NewGraph()
	for prefix, iri := range graph.Prefixes {
		out.Prefixes[prefix] = iri
	}
	for _, triple := range graph.Triples() {
		if dropped[triple.Subject.Value] ||
			dropped[triple.Object.Value] && triple.Object.IsIRI() ||
			droppedMemberIndexes[triple.Subject.Value] && triple.Predicate.Value == rdf.OpenSysML+xMemberIndex {
			continue
		}
		out.AddTriple(triple)
	}
	for _, index := range indexes {
		out.Add(index.member, rdf.OpenSysMLTerm(xMemberIndex), rdf.Int(index.index))
	}
	return out
}

type rootIndex struct {
	member rdf.Term
	index  int
}

// rootMemberIndexes is the member index each root member takes once the
// wrapper's order is dropped, and whether the indexes the members state are
// replaced: none when the members all state one, or state none and appear in
// the wrapper's order already.
func rootMemberIndexes(graph *rdf.Graph, members []rdf.Term) ([]rootIndex, bool) {
	indexed := 0
	listed := make(map[string]bool, len(members))
	for _, member := range members {
		listed[member.Value] = true
		if graph.HasProperty(member, rdf.OpenSysML+xMemberIndex) {
			indexed++
		}
	}
	switch {
	case indexed == len(members):
		return nil, false
	case indexed > 0:
		return rootIndexesOf(members), true
	case inSubjectOrder(graph, members, listed):
		return nil, false
	}
	return rootIndexesOf(members), false
}

func rootIndexesOf(members []rdf.Term) []rootIndex {
	indexes := make([]rootIndex, 0, len(members))
	for i, member := range members {
		indexes = append(indexes, rootIndex{member: member, index: i})
	}
	return indexes
}

// inSubjectOrder reports whether the graph lists the members as subjects in
// the given order.
func inSubjectOrder(graph *rdf.Graph, members []rdf.Term, listed map[string]bool) bool {
	var subjectOrder []rdf.Term
	for _, candidate := range graph.Subjects() {
		if listed[candidate.Value] {
			subjectOrder = append(subjectOrder, candidate)
		}
	}
	if len(subjectOrder) != len(members) {
		return false
	}
	for i := range subjectOrder {
		if subjectOrder[i] != members[i] {
			return false
		}
	}
	return true
}

func rootNamespaceMembers(graph *rdf.Graph, namespace rdf.Term) []rdf.Term {
	var members []rdf.Term
	seen := map[string]bool{}
	appendMember := func(related rdf.Term) {
		member := related
		if graph.Type(related) == rdf.SysML+mOwningMembership {
			member = firstIRI(graph, related, pMemberElement, pOwnedMemberElement)
		}
		if !member.IsIRI() || member.Value == "" || seen[member.Value] {
			return
		}
		seen[member.Value] = true
		members = append(members, member)
	}
	for _, property := range []string{pOwnedRelationship, pOwnedMembership, pOwnedMember} {
		for _, related := range graph.Objects(namespace, rdf.SysML+property) {
			appendMember(related)
		}
	}
	return members
}

// transparentRootSubject reports whether subject is a document wrapper no
// notation prints: an unnamed, unowned Namespace with no qualified name or body,
// unlike a top-level `namespace { … }` the encoder names `@0` and gives a body.
func transparentRootSubject(graph *rdf.Graph, subject rdf.Term) bool {
	return graph.Type(subject) == rdf.SysML+mNamespace &&
		!graph.HasProperty(subject, rdf.SysML+pOwner) &&
		!graph.HasProperty(subject, rdf.SysML+pOwningRelationship) &&
		!graph.HasProperty(subject, rdf.SysML+pDeclaredName) &&
		!graph.HasProperty(subject, rdf.SysML+pDeclaredShortName) &&
		!graph.HasProperty(subject, rdf.SysML+pQualifiedName) &&
		!graph.HasProperty(subject, rdf.OpenSysML+xHasBody)
}

// unownedElements lists the elements of the element namespace no element owns,
// in subject order: a document's top-level packages and members.
func unownedElements(graph *rdf.Graph) []rdf.Term {
	var roots []rdf.Term
	for _, subject := range graph.Subjects() {
		if !strings.HasPrefix(subject.Value, rdf.Element) || !strings.HasPrefix(graph.Type(subject), rdf.SysML) {
			continue
		}
		// A library element the graph names is owned in the library, not
		// here: the document's namespace does not take it.
		if hasOwner(graph, subject) || LibraryReference(graph, subject) {
			continue
		}
		roots = append(roots, subject)
	}
	return roots
}

// LibraryReference reports whether subject names a standard library element
// (or its owning membership) the graph references rather than declares: marked
// sysml:isLibraryElement, with no owner in the graph.
func LibraryReference(graph *rdf.Graph, subject rdf.Term) bool {
	lexical, ok := graph.Lexical(subject, rdf.SysML+pIsLibraryElement)
	return ok && lexical == "true" && !hasOwner(graph, subject) &&
		!graph.HasProperty(subject, rdf.SysML+pMembershipOwningNamespace)
}

func hasOwner(graph *rdf.Graph, subject rdf.Term) bool {
	return graph.HasProperty(subject, rdf.SysML+pOwner) ||
		graph.HasProperty(subject, rdf.SysML+pOwningRelationship) ||
		graph.HasProperty(subject, rdf.SysML+pOwningRelatedElement)
}

// rootNamespaceIDs mints the root Namespace and its memberships the way the
// graph's own ids are spelled: suffixes on the qualified-name ids, or uuid5
// under the first root's namespace when the ids are uuids (see IDUUID).
// A suffix repeats until it names a subject the graph does not already hold,
// since a top-level name may itself end in `_ns` or `_om`.
func rootNamespaceIDs(graph *rdf.Graph, roots []rdf.Term) (rdf.Term, []rdf.Term) {
	memberships := make([]rdf.Term, len(roots))
	first := roots[0]
	mint := &subjectMinter{graph: graph, taken: map[string]bool{}}
	if !uuidForm(graph, first) {
		namespace := mint.free(func(suffix string) rdf.Term { return rdf.IRI(first.Value + suffix) }, RootNamespaceSuffix)
		for i, root := range roots {
			memberships[i] = mint.free(func(suffix string) rdf.Term { return rdf.IRI(root.Value + suffix) }, rdf.OwningMembershipSuffix)
		}
		return namespace, memberships
	}
	namespace := mint.free(func(suffix string) rdf.Term {
		return rdf.ReferenceIRI(first, identity.DerivedID(rootPackageNamespace(graph, first), rootLocalID(graph, first)+suffix))
	}, RootNamespaceSuffix)
	for i, root := range roots {
		pkg := rootPackageNamespace(graph, root)
		memberships[i] = mint.free(func(suffix string) rdf.Term {
			return rdf.ReferenceIRI(root, identity.DerivedID(pkg, rootLocalID(graph, root)+suffix))
		}, rdf.OwningMembershipSuffix)
	}
	return namespace, memberships
}

// subjectMinter hands out ids that are neither subjects of the graph nor
// already minted.
type subjectMinter struct {
	graph *rdf.Graph
	taken map[string]bool
}

// free returns the candidate for the fewest repetitions of suffix that is still unused.
func (m *subjectMinter) free(candidate func(suffix string) rdf.Term, suffix string) rdf.Term {
	for n := 1; ; n++ {
		term := candidate(strings.Repeat(suffix, n))
		if !m.taken[term.Value] && len(m.graph.Predicates(term)) == 0 {
			m.taken[term.Value] = true
			return term
		}
	}
}

// uuidForm reports whether a root's id is a uuid rather than its encoded name.
func uuidForm(graph *rdf.Graph, root rdf.Term) bool {
	id := rdf.LocalName(root.Value)
	if len(id) != 36 {
		return false
	}
	for i, c := range id {
		switch {
		case i == 8 || i == 13 || i == 18 || i == 23:
			if c != '-' {
				return false
			}
		case c >= '0' && c <= '9', c >= 'a' && c <= 'f':
		default:
			return false
		}
	}
	return true
}

// rootLocalID is the qualified-name-derived id a root carries in the
// qualified form, the name the uuid form hashes.
func rootLocalID(graph *rdf.Graph, root rdf.Term) string {
	if name, ok := graph.Lexical(root, rdf.SysML+pQualifiedName); ok {
		return rdf.EncodeElementID(name)
	}
	if name, ok := graph.Lexical(root, rdf.SysML+pDeclaredName); ok {
		return rdf.EncodeElementID(name)
	}
	return rdf.LocalName(root.Value)
}

// rootPackageNamespace is the uuid namespace a root's derived ids hash under:
// uuid5 of the root's qualified-form IRI, scope qualifier included.
func rootPackageNamespace(graph *rdf.Graph, root rdf.Term) string {
	return identity.NamespaceOf(rdf.ReferenceIRI(root, rootLocalID(graph, root)).Value)
}
