package ontology

import "github.com/Open-MBEE/OpenSysML/internal/translate/rdf"

// MetaclassGraph returns the rdf.SysML metaclass hierarchy as an RDF vocabulary
// graph: one rdfs:Class per metaclass of the pinned pilot metamodel, named in
// the rdf.SysML namespace exactly as rdf:type names it on a converted element, one
// rdfs:subClassOf per declared general metaclass, and sysx:abstract true on the
// metaclasses no element is an instance of. A store that holds one rdf:type per
// element finds the specializations of a metaclass through
// rdf:type/rdfs:subClassOf* once this graph is loaded beside the model.
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
