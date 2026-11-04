package export

import (
	"maps"
	"slices"
	"sync"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/identity"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
)

// libraryIndex reads the bundled standard library once for every reader and
// decoder: the catalog of normative ids, the semantic model that gives each
// element its metaclass, and the index of the unnamed members by minted id.
type libraryIndex struct {
	model     *semantics.Model
	catalog   *identity.Catalog
	anonymous *identity.AnonymousIndex
}

var (
	sharedLibraryIndexOnce sync.Once
	sharedLibraryIndexV    *libraryIndex
)

func sharedLibraryIndex() *libraryIndex {
	sharedLibraryIndexOnce.Do(func() {
		base := libs.SharedBase()
		model := semantics.NewModel(resolve.New(base))
		catalog := identity.LibraryCatalogWithOpenSysML(base)
		sharedLibraryIndexV = &libraryIndex{model: model, catalog: catalog, anonymous: identity.AnonymousMemberIndex(model, catalog)}
	})
	return sharedLibraryIndexV
}

// addReferencedLibraryStubs states, for every library element the graph or a
// completed derived property (completed, by id) references without holding, what
// the Turtle form states about it: the element by metaclass, id and qualified name
// (or positional path when it has none), and its owning membership.
func addReferencedLibraryStubs(graph *rdf.Graph, completed map[string]bool) {
	index := sharedLibraryIndex()
	subjects := make(map[string]bool, len(graph.SubjectsView()))
	for _, subject := range graph.SubjectsView() {
		subjects[subject.Value] = true
	}
	var unheld []rdf.Term
	seen := map[string]bool{}
	for _, id := range slices.Sorted(maps.Keys(completed)) {
		term := rdf.ElementIRIForID(id)
		if !subjects[term.Value] {
			seen[term.Value] = true
			unheld = append(unheld, term)
		}
	}
	for _, triple := range graph.Triples() {
		if !triple.Object.IsIRI() || subjects[triple.Object.Value] || seen[triple.Object.Value] {
			continue
		}
		if _, ok := rdf.SubjectID(triple.Object); !ok {
			continue
		}
		seen[triple.Object.Value] = true
		unheld = append(unheld, triple.Object)
	}
	for _, subject := range unheld {
		id, _ := rdf.SubjectID(subject)
		if library, ok := index.catalog.Element(id); ok && library.Symbol != nil {
			addNamedLibraryStub(graph, subject, library, index.model)
			continue
		}
		if library, ok := index.catalog.OwningMembership(id); ok && library.Symbol != nil {
			member := rdf.ElementIRIForID(library.ID)
			addNamedLibraryStub(graph, member, library, index.model)
			addLibraryMembershipStub(graph, subject, id, member, identity.OwningMembershipMetaclass(index.model, library.Symbol), "")
			continue
		}
		if member, ok := index.anonymous.Element(id); ok {
			addAnonymousLibraryStub(graph, subject, member)
			continue
		}
		if member, ok := index.anonymous.OwningMembership(id); ok {
			element := rdf.ElementIRIForID(member.ID)
			addAnonymousLibraryStub(graph, element, member)
			addLibraryMembershipStub(graph, subject, id, element, member.MembershipMetaclass, member.MembershipPath)
		}
	}
}

func addNamedLibraryStub(graph *rdf.Graph, subject rdf.Term, library *identity.LibraryElement, model *semantics.Model) {
	if graph.HasProperty(subject, rdf.RDFType) {
		return
	}
	metaclass := declaredMetaclass(library.Symbol.Decl)
	if metaclass == "" {
		return
	}
	graph.Add(subject, rdf.IRI(rdf.RDFType), rdf.SysMLTerm(metaclass))
	graph.Add(subject, rdf.SysMLTerm(pElementID), rdf.String(library.ID))
	graph.Add(subject, rdf.SysMLTerm(pQualifiedName), rdf.String(library.FQN))
	if library.Symbol.Name != "" {
		graph.Add(subject, rdf.SysMLTerm(pDeclaredName), rdf.String(library.Symbol.Name))
	}
	graph.Add(subject, rdf.SysMLTerm(pIsLibraryElement), rdf.Bool(true))
}

func addAnonymousLibraryStub(graph *rdf.Graph, subject rdf.Term, member identity.AnonymousMember) {
	if graph.HasProperty(subject, rdf.RDFType) {
		return
	}
	graph.Add(subject, rdf.IRI(rdf.RDFType), rdf.SysMLTerm(member.Metaclass))
	graph.Add(subject, rdf.SysMLTerm(pElementID), rdf.String(member.ID))
	graph.Add(subject, rdf.OpenSysMLTerm(xLibraryPath), rdf.String(member.Path))
	graph.Add(subject, rdf.SysMLTerm(pIsLibraryElement), rdf.Bool(true))
}

func addLibraryMembershipStub(graph *rdf.Graph, subject rdf.Term, id string, member rdf.Term, metaclass, path string) {
	if graph.HasProperty(subject, rdf.RDFType) || metaclass == "" {
		return
	}
	graph.Add(subject, rdf.IRI(rdf.RDFType), rdf.SysMLTerm(metaclass))
	graph.Add(subject, rdf.SysMLTerm(pElementID), rdf.String(id))
	graph.Add(subject, rdf.SysMLTerm(pMemberElement), member)
	if path != "" {
		graph.Add(subject, rdf.OpenSysMLTerm(xLibraryPath), rdf.String(path))
	}
	graph.Add(subject, rdf.SysMLTerm(pIsLibraryElement), rdf.Bool(true))
}

// apiJSONCompletedReferenceIDs are the ids the completed derived properties of
// objects reference, which the graph states no triple for.
func apiJSONCompletedReferenceIDs(objects []apiJSONElementData) map[string]bool {
	ids := map[string]bool{}
	var walk func(value any)
	walk = func(value any) {
		switch value := value.(type) {
		case map[string]any:
			if id, ok := value["@id"].(string); ok && id != "" && len(value) == 1 {
				ids[id] = true
			}
		case []any:
			for _, item := range value {
				walk(item)
			}
		}
	}
	for _, object := range objects {
		for _, member := range object.members {
			if object.completedProperties[member.key] {
				walk(member.value)
			}
		}
	}
	return ids
}
