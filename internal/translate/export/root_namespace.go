package export

import (
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/identity"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
)

// RootNamespaceSuffix ends the id of the unnamed Namespace that owns a
// document's top-level elements in the API element form. An encoded id never
// ends in a lone '_', so the id collides with no element's or membership's.
const RootNamespaceSuffix = "_ns"

const mNamespace = "Namespace"

// documentGroup is one document's unowned elements: the roots a model of
// several documents marks as its own, wrapped under one root Namespace.
type documentGroup struct {
	document    string
	roots       []rdf.Term
	namespace   rdf.Term
	memberships []rdf.Term
}

// withRootNamespace returns graph wrapped the way the pilot serializes a
// document (KerML 1.0 § 8.3.2.4.5 Namespace): an unnamed, unowned Namespace whose
// OwningMemberships own every element the graph leaves unowned. A model of
// several documents marks each document's elements with sysx:sourceDocument and
// gets one root Namespace per document, in the order the documents appear. A
// graph that already carries such a root is returned as it is.
func withRootNamespace(graph *rdf.Graph) (*rdf.Graph, error) {
	roots := unownedElements(graph)
	if len(roots) == 0 {
		return graph, nil
	}
	for _, root := range roots {
		if transparentRootSubject(graph, root) {
			return graph, nil
		}
	}
	groups := documentGroups(graph, roots)
	// One minter hands out every group's ids: a namespace one group mints is
	// not in the graph yet when the next group mints, so the taken set is what
	// keeps two groups from picking one id.
	mint := &subjectMinter{graph: graph, taken: map[string]bool{}}
	for i, group := range groups {
		namespace, memberships := rootNamespaceIDs(graph, mint, group.roots)
		groups[i].namespace, groups[i].memberships = namespace, memberships
	}
	// The wrapped graph is the source's triples with the wrapper's inserted, so
	// it is assembled as a list and never rebuilds a set of the source's triples.
	// Each triple keeps the place it first takes, as adding them one by one to a
	// graph would: an inserted triple the source states later is skipped there,
	// and one the source stated earlier is not inserted again. Every inserted
	// triple is about a namespace, a membership or a root, so only the source
	// triples about those are remembered.
	triples := make([]rdf.Triple, 0, graph.Len()+8*len(roots)+8*len(groups))
	placed := map[rdf.Triple]bool{}
	wrapperSubjects := map[rdf.Term]bool{}
	for _, group := range groups {
		wrapperSubjects[group.namespace] = true
		for i, root := range group.roots {
			wrapperSubjects[root] = true
			wrapperSubjects[group.memberships[i]] = true
		}
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
	for _, group := range groups {
		namespace := group.namespace
		add(namespace, rdf.IRI(rdf.RDFType), sysml(mNamespace))
		add(namespace, sysml(pElementID), rdf.String(rdf.LocalName(namespace.Value)))
		for i, root := range group.roots {
			membership := group.memberships[i]
			add(namespace, sysml(pOwnedRelationship), membership)
			add(namespace, sysml(pOwnedMembership), membership)
			add(namespace, sysml(pOwnedMember), root)
		}
		for i, root := range group.roots {
			membership := group.memberships[i]
			add(membership, rdf.IRI(rdf.RDFType), sysml(mOwningMembership))
			add(membership, sysml(pElementID), rdf.String(rdf.LocalName(membership.Value)))
			add(membership, sysml(pOwner), namespace)
			add(membership, sysml(pMemberElement), root)
			add(membership, sysml(pOwnedMemberElement), root)
			add(membership, sysml(pOwnedRelatedElement), root)
			add(membership, sysml(pOwningRelatedElement), namespace)
			add(membership, sysml(pMembershipOwningNamespace), namespace)
		}
	}
	owned := map[string]*documentGroup{}
	for _, group := range groups {
		for _, root := range group.roots {
			owned[root.Value] = group
		}
	}
	for _, triple := range graph.Triples() {
		if wrapperSubjects[triple.Subject] {
			if placed[triple] {
				continue
			}
			placed[triple] = true
		}
		triples = append(triples, triple)
		if group, ok := owned[triple.Subject.Value]; ok {
			delete(owned, triple.Subject.Value)
			namespace := group.namespace
			for i, root := range group.roots {
				if root.Value == triple.Subject.Value {
					add(triple.Subject, sysml(pOwner), namespace)
					add(triple.Subject, sysml(pOwningNamespace), namespace)
					add(triple.Subject, sysml(pOwningRelationship), group.memberships[i])
					add(triple.Subject, sysml(pOwningMembership), group.memberships[i])
				}
			}
		}
	}
	for _, group := range groups {
		if len(group.roots) < 2 {
			continue
		}
		namespace := group.namespace
		for _, c := range []struct {
			key     string
			members []rdf.Term
		}{{pOwnedRelationship, group.memberships}, {pOwnedMembership, group.memberships}, {pOwnedMember, group.roots}} {
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

// documentGroups groups the roots by the sysx:sourceDocument they carry, in
// the order the documents first appear: a model of several documents wraps
// each document's roots under its own root Namespace, the pilot's
// one-Namespace-per-resource form. Roots with no document recorded group
// together, so a single-document graph wraps exactly as it always has.
func documentGroups(graph *rdf.Graph, roots []rdf.Term) []*documentGroup {
	var groups []*documentGroup
	byDocument := map[string]*documentGroup{}
	for _, root := range roots {
		document, _ := graph.Lexical(root, rdf.OpenSysML+xSourceDocument)
		group, ok := byDocument[document]
		if !ok {
			group = &documentGroup{document: document}
			byDocument[document] = group
			groups = append(groups, group)
		}
		group.roots = append(group.roots, root)
	}
	return groups
}

// withoutRootNamespace strips the wrapper withRootNamespace adds, so a graph
// read from the element form matches one the encoder builds from notation.
func withoutRootNamespace(graph *rdf.Graph) *rdf.Graph {
	dropped := map[string]bool{}
	for _, subject := range graph.Subjects() {
		if !transparentRootSubject(graph, subject) {
			continue
		}
		dropped[subject.Value] = true
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
		if dropped[triple.Subject.Value] || dropped[triple.Object.Value] && triple.Object.IsIRI() {
			continue
		}
		out.AddTriple(triple)
	}
	return out
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
		if graph.HasProperty(subject, rdf.SysML+pOwner) ||
			graph.HasProperty(subject, rdf.SysML+pOwningRelationship) ||
			graph.HasProperty(subject, rdf.SysML+pOwningRelatedElement) {
			continue
		}
		roots = append(roots, subject)
	}
	return roots
}

// rootNamespaceIDs mints the root Namespace and its memberships the way the
// graph's own ids are spelled: suffixes on the qualified-name ids, or uuid5
// under the first root's namespace when the ids are uuids (see IDUUID).
// A suffix repeats until it names a subject the graph does not already hold,
// since a top-level name may itself end in `_ns` or `_om`.
func rootNamespaceIDs(graph *rdf.Graph, mint *subjectMinter, roots []rdf.Term) (rdf.Term, []rdf.Term) {
	memberships := make([]rdf.Term, len(roots))
	first := roots[0]
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
