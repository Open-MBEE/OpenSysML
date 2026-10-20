package export

import (
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
)

func apiJSONOrderedFeatureChain(
	graph *rdf.Graph,
	nameIndex *apiJSONNameIndex,
	subject rdf.Term,
) []rdf.Term {
	var links []rdf.Term
	var relationships []rdf.Term
	seen := make(map[string]bool)
	appendRelationship := func(relationship rdf.Term) {
		if rdf.LocalName(graph.Type(relationship)) != mFeatureChaining || seen[relationship.Value] {
			return
		}
		seen[relationship.Value] = true
		relationships = append(relationships, relationship)
	}
	for _, relationship := range graph.ObjectsView(subject, rdf.SysML+pOwnedRelationship) {
		appendRelationship(relationship)
	}
	for _, relationship := range nameIndex.chainRelationships(subject) {
		appendRelationship(relationship)
	}
	for _, relationship := range relationships {
		targets := graph.ObjectsView(relationship, rdf.SysML+pChainingFeature)
		if len(targets) != 1 {
			links = append(links, rdf.String(""))
			continue
		}
		target := targets[0]
		if target.IsLiteral() {
			if resolved, ok := nameIndex.chainTarget(relationship); ok {
				target = resolved.target
			}
		}
		links = append(links, target)
	}
	return links
}
