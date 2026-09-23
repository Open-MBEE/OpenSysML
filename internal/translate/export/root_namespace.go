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

// withRootNamespace returns graph wrapped the way the pilot serializes a
// document (KerML 1.0 § 8.3.2.4.5 Namespace): an unnamed, unowned Namespace whose
// OwningMemberships own every element the graph leaves unowned. A graph that
// already carries such a root is returned as it is.
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
	namespace, memberships := rootNamespaceIDs(graph, roots)
	out := rdf.NewGraph()
	for prefix, iri := range graph.Prefixes {
		out.Prefixes[prefix] = iri
	}
	sysml := rdf.SysMLTerm
	out.Add(namespace, rdf.IRI(rdf.RDFType), sysml(mNamespace))
	out.Add(namespace, sysml(pElementID), rdf.String(rdf.LocalName(namespace.Value)))
	for i, root := range roots {
		membership := memberships[i]
		out.Add(namespace, sysml(pOwnedRelationship), membership)
		out.Add(namespace, sysml(pOwnedMembership), membership)
		out.Add(namespace, sysml(pOwnedMember), root)
	}
	for i, root := range roots {
		membership := memberships[i]
		out.Add(membership, rdf.IRI(rdf.RDFType), sysml(mOwningMembership))
		out.Add(membership, sysml(pElementID), rdf.String(rdf.LocalName(membership.Value)))
		out.Add(membership, sysml(pOwner), namespace)
		out.Add(membership, sysml(pMemberElement), root)
		out.Add(membership, sysml(pOwnedMemberElement), root)
		out.Add(membership, sysml(pOwnedRelatedElement), root)
		out.Add(membership, sysml(pOwningRelatedElement), namespace)
		out.Add(membership, sysml(pMembershipOwningNamespace), namespace)
	}
	owned := map[string]int{}
	for i, root := range roots {
		owned[root.Value] = i
	}
	for _, triple := range graph.Triples() {
		if i, ok := owned[triple.Subject.Value]; ok {
			delete(owned, triple.Subject.Value)
			out.AddTriple(triple)
			out.Add(triple.Subject, sysml(pOwner), namespace)
			out.Add(triple.Subject, sysml(pOwningNamespace), namespace)
			out.Add(triple.Subject, sysml(pOwningRelationship), memberships[i])
			out.Add(triple.Subject, sysml(pOwningMembership), memberships[i])
			continue
		}
		out.AddTriple(triple)
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
			out.Add(namespace, rdf.AnnotationJSONTerm(c.key), rdf.String(text))
		}
	}
	return out, nil
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
// notation prints: an unnamed, unowned Namespace.
func transparentRootSubject(graph *rdf.Graph, subject rdf.Term) bool {
	return graph.Type(subject) == rdf.SysML+mNamespace &&
		!graph.HasProperty(subject, rdf.SysML+pOwner) &&
		!graph.HasProperty(subject, rdf.SysML+pOwningRelationship) &&
		!graph.HasProperty(subject, rdf.SysML+pDeclaredName) &&
		!graph.HasProperty(subject, rdf.SysML+pDeclaredShortName)
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
func rootNamespaceIDs(graph *rdf.Graph, roots []rdf.Term) (rdf.Term, []rdf.Term) {
	memberships := make([]rdf.Term, len(roots))
	first := roots[0]
	if !uuidForm(graph, first) {
		for i, root := range roots {
			memberships[i] = rdf.OwningMembershipIRIOf(root)
		}
		return rdf.IRI(first.Value + RootNamespaceSuffix), memberships
	}
	namespace := rdf.ReferenceIRI(first, identity.DerivedID(rootPackageNamespace(graph, first), rootLocalID(graph, first)+RootNamespaceSuffix))
	for i, root := range roots {
		pkg := rootPackageNamespace(graph, root)
		memberships[i] = rdf.ReferenceIRI(root, identity.DerivedID(pkg, rootLocalID(graph, root)+rdf.OwningMembershipSuffix))
	}
	return namespace, memberships
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
