package queryexec

import (
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
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

// An untyped feature's type stays the reader's answer — none — rather than its implied types.
func TestUntypedFeatureTypeStaysEmpty(t *testing.T) {
	fixture := loadExecutionFixture(t, `
part untyped;
calc def Types :> Query {
	in rows : Element[0..*];
	Project(source = rows, properties = ("type"))
}
`)
	result, err := fixture.execute(t, "Types", Bindings{"rows": {ElementValue(fixture.symbol(t, "untyped"))}}, Options{})
	if err != nil {
		t.Fatalf("execute Types: %v", err)
	}
	if texts := cellTexts(t, result, 0); len(texts) != 1 || texts[0] != "" {
		t.Fatalf("type of an untyped part = %q, want one empty cell", texts)
	}
}

// A keyword-first relationship is a declared element whose ends the metaclass answers.
func TestKeywordFirstRelationshipAnswersGeneral(t *testing.T) {
	fixture := loadExecutionSources(t, "package Observatory {"+executionImports+`
calc def Generals :> Query {
	in rows : Element[0..*];
	Project(source = rows, properties = ("general"))
}
}`, map[string]string{"relationships.kerml": `package K {
	classifier A;
	classifier B;
	specialization Gen subtype A specializes B;
}`})
	gen := symbols.PreferDeclared(fixture.index.LookupQualified("K::Gen"))
	if len(gen) != 1 {
		t.Fatalf("lookup K::Gen: got %d symbols", len(gen))
	}
	result, err := fixture.execute(t, "Generals", Bindings{"rows": {ElementValue(gen[0])}}, Options{})
	if err != nil {
		t.Fatalf("execute Generals: %v", err)
	}
	if texts := cellTexts(t, result, 0); len(texts) != 1 || texts[0] != "K::B" {
		t.Fatalf("general of Gen = %q, want K::B", texts)
	}
}
