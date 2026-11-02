package queryexec

import (
	"strconv"
	"strings"
	"testing"
)

// A relationship or expression row answers the queryable properties the reader
// knows only for features and types from its metaclass.
func TestQueryablePropertiesOfRelationshipRows(t *testing.T) {
	fixture := loadExecutionFixture(t, `
part def Subsystem { attribute mass : Real; }
part def Mirror :> Subsystem { attribute diameter : Real = 2.4; }
part telescope : Mirror { attribute :>> mass = 1.5; }
calc def Ends :> Query {
	in rels : Element[0..*];
	Project(source = rels, properties = ("type", "general", "isAbstract"))
}
`)
	m := fixture.model
	var rows []Value
	for _, owner := range []string{"Mirror", "telescope", "telescope::mass"} {
		for _, rel := range m.OwnedRelationships(fixture.symbol(t, owner)) {
			rows = append(rows, ElementValue(rel))
		}
	}
	for _, rel := range m.OwnedRelationships(fixture.symbol(t, "Mirror::diameter")) {
		if literals, ok := m.ReflectiveElements(rel, "ownedRelatedElement"); ok {
			for _, literal := range literals {
				rows = append(rows, ElementValue(literal))
			}
		}
	}
	result, err := fixture.execute(t, "Ends", Bindings{"rels": rows}, Options{})
	if err != nil {
		t.Fatalf("execute Ends: %v", err)
	}
	got := map[string]int{}
	for i, name := range []string{"type", "general"} {
		for _, text := range cellTexts(t, result, i) {
			for _, cell := range strings.Fields(text) {
				got[name+"="+cell]++
			}
		}
	}
	for _, row := range result.Rows() {
		for _, value := range row.Cells()[2].Values() {
			if abstract, ok := value.Boolean(); ok {
				got["isAbstract="+strconv.FormatBool(abstract)]++
			}
		}
	}
	for _, want := range []string{
		"type=Observatory::Mirror",             // telescope : Mirror
		"general=Observatory::Subsystem",       // Mirror :> Subsystem
		"general=Observatory::Subsystem::mass", // mass :>> mass
		"isAbstract=false",
	} {
		if got[want] == 0 {
			t.Errorf("rows lack %s; got %v", want, got)
		}
	}
}
