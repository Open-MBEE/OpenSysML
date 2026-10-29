package export

import (
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
)

// apiJSONGraphIndex is built once per write and shared by the evaluator's graph
// structure, the mapping-name pass and the writer's name index.
type apiJSONGraphIndex struct {
	graph    *rdf.Graph
	subjects []rdf.Term
	slots    map[string]int
	// settled counts the leading subjects that belong to the written graph; the
	// rest are library references the evaluation graph adds.
	settled             int
	metaclasses         map[string]apiJSONMetaclass
	incomingMemberships map[string][]rdf.Term
	ownerPositions      map[string]*mappingOwnerPositions
	conformance         map[apiJSONConformance]bool
}

type apiJSONMetaclass struct {
	class string
	ok    bool
}

type apiJSONConformance struct {
	typ, ancestor string
}

// mappingOwnerPositions is the member position each owned relationship, and each
// member it relates, first takes among an owner's position-contributing relationships.
type mappingOwnerPositions struct {
	relationships map[rdf.Term]int
	members       map[rdf.Term]int
}

// newAPIJSONWriteIndex indexes a settled graph extended by the library elements it references.
func newAPIJSONWriteIndex(settled *rdf.Graph) *apiJSONGraphIndex {
	if settled == nil {
		return newAPIJSONGraphIndex(nil, nil)
	}
	return newAPIJSONGraphIndex(apiJSONEvaluationGraph(settled), settled.SubjectsView())
}

// newAPIJSONGraphIndex numbers the settled subjects first, in their order, then
// the graph's other subjects.
func newAPIJSONGraphIndex(graph *rdf.Graph, settled []rdf.Term) *apiJSONGraphIndex {
	index := &apiJSONGraphIndex{
		graph:          graph,
		settled:        len(settled),
		metaclasses:    make(map[string]apiJSONMetaclass),
		ownerPositions: make(map[string]*mappingOwnerPositions),
		conformance:    make(map[apiJSONConformance]bool),
	}
	if graph == nil {
		index.slots = map[string]int{}
		index.incomingMemberships = map[string][]rdf.Term{}
		return index
	}
	all := graph.SubjectsView()
	index.slots = make(map[string]int, len(all))
	index.subjects = make([]rdf.Term, 0, len(all))
	for _, subject := range settled {
		index.slots[subject.Value] = len(index.subjects)
		index.subjects = append(index.subjects, subject)
	}
	for _, subject := range all {
		if _, ok := index.slots[subject.Value]; !ok {
			index.slots[subject.Value] = len(index.subjects)
			index.subjects = append(index.subjects, subject)
		}
	}
	index.incomingMemberships = index.memberships(graph)
	return index
}

func (index *apiJSONGraphIndex) memberships(graph *rdf.Graph) map[string][]rdf.Term {
	incoming := make(map[string][]rdf.Term)
	seenMembers := make(map[string]bool)
	for _, subject := range graph.SubjectsView() {
		if !index.conforms(graph, subject, "Membership") {
			continue
		}
		clear(seenMembers)
		for _, property := range mappingIncomingMembershipPredicates {
			for _, member := range graph.ObjectsView(subject, property) {
				if member.IsIRI() && !seenMembers[member.Value] {
					seenMembers[member.Value] = true
					incoming[member.Value] = append(incoming[member.Value], subject)
				}
			}
		}
	}
	return incoming
}

// conforms is mappingTypeConforms, answered once per stated type and ancestor.
func (index *apiJSONGraphIndex) conforms(graph *rdf.Graph, subject rdf.Term, ancestor string) bool {
	if graph == nil {
		return false
	}
	for _, typ := range graph.ObjectsView(subject, rdf.RDFType) {
		key := apiJSONConformance{typ: typ.Value, ancestor: ancestor}
		conforms, ok := index.conformance[key]
		if !ok {
			conforms = mappingTypeIRIConforms(graph, typ.Value, ancestor)
			index.conformance[key] = conforms
		}
		if conforms {
			return true
		}
	}
	return false
}
