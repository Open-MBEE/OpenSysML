package queryexec

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

const relationshipEndQueries = `
calc def Ends :> Query {
	in rows : Element[0..*];
	Project(source = rows, properties = ("source", "target", "relatedElement"))
}
calc def DisjoiningEnds :> Query {
	in rows : Element[0..*];
	Project(source = rows, properties = ("typeDisjoined", "disjoiningType"))
}
calc def InvertingEnds :> Query {
	in rows : Element[0..*];
	Project(source = rows, properties = ("featureInverted", "invertingFeature"))
}
calc def FeaturingEnds :> Query {
	in rows : Element[0..*];
	Project(source = rows, properties = ("featureOfType", "featuringType"))
}
calc def Operators :> Query {
	in rows : Element[0..*];
	Project(source = rows, properties = ("operator"))
}
calc def Typings :> Query {
	in root : Element;
	Project(
		source = WhereType(source = Descendants(source = root, relationships = true), type = "FeatureTyping", exact = true),
		properties = ("source", "target")
	)
}
part def A;
part def B;
part def C;
part a1 : A;
part a2 : A;
binding bn bind a1 = a2;
action act { action s1; action s2; succession sn first s1 then s2; }
dependency dn from A, B to C;
package Machine {
	item def Sig;
	state def S { state s1; state s2; transition t first s1 accept Sig then s2; }
}
`

const relationshipEndKernel = `package K {
	class A { feature b : A; }
	class B;
	feature f : A;
	feature g : A;
	binding bn of f = g;
	succession sn first f then g;
	disjoining dn disjoint A from B;
	inverting vn inverse f of g;
	featuring fn of f by A;
	feature chain = f.b;
	feature collect = f.{in p; p};
	feature select = f.?{in p; true};
}`

func loadRelationshipEndFixture(t *testing.T) executionFixture {
	t.Helper()
	return loadExecutionSources(t, "package Observatory {"+executionImports+relationshipEndQueries+"}",
		map[string]string{"relationship-ends.kerml": relationshipEndKernel})
}

func (f executionFixture) qualified(t *testing.T, name string) *symbols.Symbol {
	t.Helper()
	matches := symbols.PreferDeclared(f.index.LookupQualified(name))
	if len(matches) != 1 {
		t.Fatalf("lookup %s: got %d symbols", name, len(matches))
	}
	return matches[0]
}

// rowCells renders each row as its cells, a cell's elements by qualified name.
func rowCells(t *testing.T, result *RowSet) []string {
	t.Helper()
	var rows []string
	for _, row := range result.Rows() {
		var cells []string
		for _, cell := range row.Cells() {
			var texts []string
			for _, value := range cell.Values() {
				if sym, ok := value.Element(); ok {
					texts = append(texts, symbols.FQNOf(sym))
					continue
				}
				texts = append(texts, differValueText(t, value))
			}
			cells = append(cells, strings.Join(texts, ","))
		}
		rows = append(rows, strings.Join(cells, " | "))
	}
	return rows
}

func assertProjectedRow(t *testing.T, fixture executionFixture, query string, row *symbols.Symbol, want string) {
	t.Helper()
	result, err := fixture.execute(t, query, Bindings{"rows": {ElementValue(row)}}, Options{})
	if err != nil {
		t.Fatalf("%s of %s: %v", query, symbols.FQNOf(row), err)
	}
	if got := rowCells(t, result); len(got) != 1 || got[0] != want {
		t.Fatalf("%s of %s = %q, want %q", query, symbols.FQNOf(row), got, want)
	}
}

// Relationship::source, target and relatedElement of a connector are the features its ends
// reference, in both notations.
func TestProjectReadsConnectorSourceAndTarget(t *testing.T) {
	fixture := loadRelationshipEndFixture(t)
	for name, want := range map[string]string{
		"K::bn":                "K::f | K::g | K::f,K::g",
		"K::sn":                "K::f | K::g | K::f,K::g",
		"Observatory::bn":      "Observatory::a1 | Observatory::a2 | Observatory::a1,Observatory::a2",
		"Observatory::act::sn": "Observatory::act::s1 | Observatory::act::s2 | Observatory::act::s1,Observatory::act::s2",
	} {
		assertProjectedRow(t, fixture, "Ends", fixture.qualified(t, name), want)
	}
}

// Relationship::source and target of a dependency are its clients and its suppliers.
func TestProjectReadsDependencySourceAndTarget(t *testing.T) {
	fixture := loadRelationshipEndFixture(t)
	assertProjectedRow(t, fixture, "Ends", fixture.symbol(t, "dn"),
		"Observatory::A,Observatory::B | Observatory::C | Observatory::A,Observatory::B,Observatory::C")
}

// A declared Disjoining answers typeDisjoined and disjoiningType, and the ends they redefine.
func TestProjectReadsDisjoiningEnds(t *testing.T) {
	fixture := loadRelationshipEndFixture(t)
	dn := fixture.qualified(t, "K::dn")
	assertProjectedRow(t, fixture, "DisjoiningEnds", dn, "K::A | K::B")
	assertProjectedRow(t, fixture, "Ends", dn, "K::A | K::B | K::A,K::B")
}

// A declared FeatureInverting answers featureInverted and invertingFeature, and the ends they redefine.
func TestProjectReadsFeatureInvertingEnds(t *testing.T) {
	fixture := loadRelationshipEndFixture(t)
	vn := fixture.qualified(t, "K::vn")
	assertProjectedRow(t, fixture, "InvertingEnds", vn, "K::f | K::g")
	assertProjectedRow(t, fixture, "Ends", vn, "K::f | K::g | K::f,K::g")
}

// A declared TypeFeaturing answers featureOfType and featuringType, and the ends they redefine.
func TestProjectReadsTypeFeaturingEnds(t *testing.T) {
	fixture := loadRelationshipEndFixture(t)
	fn := fixture.qualified(t, "K::fn")
	assertProjectedRow(t, fixture, "FeaturingEnds", fn, "K::f | K::A")
	assertProjectedRow(t, fixture, "Ends", fn, "K::f | K::A | K::f,K::A")
}

// Expression::operator of a feature chain, a collect and a select expression is the operator
// the specification fixes for the metaclass.
func TestProjectReadsFixedExpressionOperators(t *testing.T) {
	fixture := loadRelationshipEndFixture(t)
	for name, want := range map[string]string{"chain": ".", "collect": "collect", "select": "select"} {
		value := fixture.model.ValueExpression(fixture.qualified(t, "K::"+name))
		if value == nil {
			t.Fatalf("K::%s has no value expression", name)
		}
		assertProjectedRow(t, fixture, "Operators", value, want)
	}
}

// Every FeatureTyping a transition trigger writes answers its target.
func TestProjectReadsTriggerPayloadTypingTarget(t *testing.T) {
	fixture := loadRelationshipEndFixture(t)
	result, err := fixture.execute(t, "Typings", Bindings{"root": {ElementValue(fixture.symbol(t, "Machine"))}}, Options{})
	if err != nil {
		t.Fatalf("Typings: %v", err)
	}
	rows := rowCells(t, result)
	if len(rows) == 0 {
		t.Fatalf("Typings found no FeatureTyping under the transition")
	}
	for _, row := range rows {
		if !strings.HasSuffix(row, " | Observatory::Machine::Sig") {
			t.Errorf("a trigger typing reads %q, want target Observatory::Machine::Sig", row)
		}
	}
}
