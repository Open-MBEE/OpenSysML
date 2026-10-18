package ontology

import "github.com/Open-MBEE/OpenSysML/internal/translate/rdf"

// MetaclassGraph is the metaclass hierarchy as an RDF vocabulary graph: one rdfs:Class per
// metaclass, one rdfs:subClassOf per declared general, sysx:abstract on the abstract ones.
func MetaclassGraph() *rdf.Graph {
	g := rdf.NewGraph()
	g.Prefixes = map[string]string{
		"rdf":   rdf.RDFNS,
		"rdfs":  rdf.RDFS,
		"xsd":   rdf.XSD,
		"sysml": rdf.SysML,
		"sysx":  rdf.OpenSysML,
	}
	rdfType := rdf.IRI(rdf.RDFType)
	class := rdf.IRI(rdf.RDFS + "Class")
	label := rdf.IRI(rdf.RDFS + "label")
	subClassOf := rdf.IRI(rdf.RDFS + "subClassOf")
	abstract := rdf.OpenSysMLTerm("abstract")
	for _, metaclass := range Classes() {
		subject := rdf.SysMLTerm(metaclass.Name)
		g.Add(subject, rdfType, class)
		g.Add(subject, label, rdf.String(metaclass.Name))
		for _, parent := range metaclass.Parents {
			g.Add(subject, subClassOf, rdf.SysMLTerm(parent))
		}
		if metaclass.Abstract {
			g.Add(subject, abstract, rdf.Bool(true))
		}
	}
	return g
}
