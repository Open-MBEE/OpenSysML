package export

import (
	"maps"
	"slices"
	"strings"
	"sync"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/identity"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
)

// libraryIndex reads the bundled standard library once for every reader and
// decoder: the catalog of normative ids, the semantic model that gives each
// element its metaclass, and the index of the unnamed members by minted id.
type libraryIndex struct {
	// mu serializes the model's queries, which memoize: the catalog and the anonymous index are frozen.
	mu        sync.Mutex
	model     *semantics.Model
	catalog   *identity.Catalog
	anonymous *identity.AnonymousIndex
	// source serves exactly the text the index was built from, and forms holds
	// the element form of each of its documents a reader asked for.
	source libs.Source
	forms  sync.Map // document name -> *libraryDocumentForm
}

// stubMetaclass is libraryStubMetaclass for sym under the index's lock.
func (index *libraryIndex) stubMetaclass(sym *symbols.Symbol) string {
	index.mu.Lock()
	defer index.mu.Unlock()
	return libraryStubMetaclass(index.model, sym)
}

// owningMembershipMetaclass is identity.OwningMembershipMetaclass for sym under the index's lock.
func (index *libraryIndex) owningMembershipMetaclass(sym *symbols.Symbol) string {
	index.mu.Lock()
	defer index.mu.Unlock()
	return identity.OwningMembershipMetaclass(index.model, sym)
}

var (
	sharedLibraryIndexOnce sync.Once
	sharedLibraryIndexV    *libraryIndex
)

func sharedLibraryIndex() *libraryIndex {
	sharedLibraryIndexOnce.Do(func() {
		base, source := libs.SharedLibrary()
		model := semantics.NewModel(resolve.New(base))
		catalog := identity.LibraryCatalogWithOpenSysML(base)
		sharedLibraryIndexV = &libraryIndex{model: model, catalog: catalog, anonymous: identity.AnonymousMemberIndex(model, catalog), source: source}
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
		addLibraryStub(graph, index, subject, id)
	}
}

// apiJSONMembershipReferenceIDs are the ids the completed properties of the graph's own
// elements reference, derived from the evaluator alone, without writing the element form.
func apiJSONMembershipReferenceIDs(graph *rdf.Graph) (map[string]bool, error) {
	wrapped, err := withRootNamespace(graph)
	if err != nil {
		return nil, err
	}
	index := newAPIJSONWriteIndex(wrapped)
	writer := &apiJSONElementWriter{
		graph:           wrapped,
		evaluation:      newAPIJSONEvaluationWithIndex(index),
		comparisonCache: &apiJSONTermCache{},
		scratch:         &apiJSONWriteScratch{},
	}
	ids := map[string]bool{}
	if writer.evaluation == nil || writer.evaluation.evaluator == nil || writer.evaluation.structure == nil || apiJSONExternalForm(wrapped, writer.scratch) {
		return ids, nil
	}
	var collect func(value any)
	collect = func(value any) {
		switch value := value.(type) {
		case APIJSONReference:
			if value.ID != "" {
				ids[value.ID] = true
			}
		case []any:
			for _, item := range value {
				collect(item)
			}
		}
	}
	for _, subject := range wrapped.SubjectsView() {
		if LibraryReference(wrapped, subject) {
			continue
		}
		metaclass := strings.TrimPrefix(wrapped.Type(subject), rdf.SysML)
		semanticElement, ok := writer.evaluation.structure.element(subject)
		if metaclass == "" || !ok {
			continue
		}
		for _, property := range apiJSONCompletedPropertiesOf(metaclass) {
			if property.defaultValue != nil || wrapped.HasProperty(subject, property.predicate) {
				continue
			}
			value, supported := writer.evaluation.evaluator.PropertyReadOnlyUncached(semanticElement, property.class, property.name)
			if !supported {
				continue
			}
			if spelled, ok := writer.derivedJSONValue(value); ok {
				collect(spelled)
			}
		}
	}
	return ids, nil
}

// addLibraryStub states what the Turtle form states about the library element or
// owning membership id names, and reports whether the index knows the id.
func addLibraryStub(graph *rdf.Graph, index *libraryIndex, subject rdf.Term, id string) bool {
	if library, ok := index.catalog.Element(id); ok && library.Symbol != nil {
		addNamedLibraryStub(graph, subject, library, index.stubMetaclass(library.Symbol))
		return true
	}
	if library, ok := index.catalog.OwningMembership(id); ok && library.Symbol != nil {
		member := rdf.ElementIRIForID(library.ID)
		addNamedLibraryStub(graph, member, library, index.stubMetaclass(library.Symbol))
		addLibraryMembershipStub(graph, subject, id, member, index.owningMembershipMetaclass(library.Symbol))
		return true
	}
	if member, ok := index.anonymous.Element(id); ok {
		addAnonymousLibraryStub(graph, subject, member)
		return true
	}
	if member, ok := index.anonymous.OwningMembership(id); ok {
		element := rdf.ElementIRIForID(member.ID)
		addAnonymousLibraryStub(graph, element, member)
		addLibraryMembershipStub(graph, subject, id, element, member.MembershipMetaclass)
		return true
	}
	return false
}

func addNamedLibraryStub(graph *rdf.Graph, subject rdf.Term, library *identity.LibraryElement, metaclass string) {
	if graph.HasProperty(subject, rdf.SysML+pElementID) {
		return
	}
	if metaclass == "" {
		return
	}
	if !graph.HasProperty(subject, rdf.RDFType) {
		graph.Add(subject, rdf.IRI(rdf.RDFType), rdf.SysMLTerm(metaclass))
	}
	graph.Add(subject, rdf.SysMLTerm(pElementID), rdf.String(library.ID))
	graph.Add(subject, rdf.SysMLTerm(pQualifiedName), rdf.String(library.FQN))
	if declared, _ := declaredNameAndMembers(library.Symbol.Decl); declared != "" {
		graph.Add(subject, rdf.SysMLTerm(pDeclaredName), rdf.String(declared))
	}
	graph.Add(subject, rdf.SysMLTerm(pIsLibraryElement), rdf.Bool(true))
}

func addAnonymousLibraryStub(graph *rdf.Graph, subject rdf.Term, member identity.AnonymousMember) {
	if graph.HasProperty(subject, rdf.SysML+pElementID) {
		return
	}
	if !graph.HasProperty(subject, rdf.RDFType) {
		graph.Add(subject, rdf.IRI(rdf.RDFType), rdf.SysMLTerm(member.Metaclass))
	}
	graph.Add(subject, rdf.SysMLTerm(pElementID), rdf.String(member.ID))
	graph.Add(subject, rdf.OpenSysMLTerm(xLibraryPath), rdf.String(member.Path))
	graph.Add(subject, rdf.SysMLTerm(pIsLibraryElement), rdf.Bool(true))
}

func addLibraryMembershipStub(graph *rdf.Graph, subject rdf.Term, id string, member rdf.Term, metaclass string) {
	if graph.HasProperty(subject, rdf.SysML+pElementID) || metaclass == "" {
		return
	}
	if !graph.HasProperty(subject, rdf.RDFType) {
		graph.Add(subject, rdf.IRI(rdf.RDFType), rdf.SysMLTerm(metaclass))
	}
	graph.Add(subject, rdf.SysMLTerm(pElementID), rdf.String(id))
	graph.Add(subject, rdf.SysMLTerm(pMemberElement), member)
	graph.Add(subject, rdf.SysMLTerm(pIsLibraryElement), rdf.Bool(true))
}

// libraryStubMetaclass is the metamodel class a library stub states for sym: the
// semantic model's, which knows a `bool` feature as a BooleanExpression and a
// `subject` as a ReferenceUsage, or the declaration's where the model has none.
func libraryStubMetaclass(model *semantics.Model, sym *symbols.Symbol) string {
	if metaclass := model.MetaclassOf(sym); metaclass != nil && metaclass.Name != "" {
		return metaclass.Name
	}
	return declaredMetaclass(sym.Decl)
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
