package ontology

import (
	"encoding/json"
	"testing"
)

// TestTableRecordSpellsTheProperties pins the JSON spelling tools outside this
// module read: object ranges by metaclass name, datatype ranges by IRI, upper -1 for many.
func TestTableRecordSpellsTheProperties(t *testing.T) {
	out, err := MarshalTable()
	if err != nil {
		t.Fatal(err)
	}
	var table Table
	if err := json.Unmarshal(out, &table); err != nil {
		t.Fatal(err)
	}
	byName := map[string]TableProperty{}
	for _, p := range table.Properties {
		byName[p.DefiningClass+"::"+p.Name] = p
	}
	owned := byName["Element::ownedRelationship"]
	if owned.Kind != "object" || owned.Range != "Relationship" || owned.Upper != -1 || !owned.Ordered {
		t.Errorf("Element::ownedRelationship = %+v, want an ordered many object property ranging over Relationship", owned)
	}
	name := byName["Element::declaredName"]
	if name.Kind != "datatype" || name.Range != "http://www.w3.org/2001/XMLSchema#string" || name.Upper != 1 {
		t.Errorf("Element::declaredName = %+v, want a single xsd:string datatype property", name)
	}
	direction := byName["Feature::direction"]
	if direction.Range != "FeatureDirectionKind" {
		t.Errorf("Feature::direction ranges over %q, want the enumeration by name", direction.Range)
	}
	abstract := byName["Type::isAbstract"]
	if abstract.Default == nil || *abstract.Default != "false" {
		t.Errorf("Type::isAbstract default = %v, want \"false\"", abstract.Default)
	}
	if table.Metamodel != Version || table.PilotTag != SourceTag || table.PilotCommit != SourceCommit {
		t.Errorf("header = %+v, want the table's", table)
	}
}
