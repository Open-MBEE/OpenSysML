package queryexec

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
)

// derivedBody is the derived-value repro: masses computed from sibling features,
// redefined at type and usage level, chained into owned parts, and summed.
const derivedBody = `
private import SI::*;
private import ISQ::*;

part def Stage {
	attribute dryMass :> ISQ::mass;
	attribute propellantMass :> ISQ::mass;
	attribute mass :> ISQ::mass = dryMass + propellantMass;
	attribute engines : Integer default 1;
	attribute nozzles : Integer = engines * 2;
	attribute heavy : Boolean = mass > 1000000 [kg];
	attribute class : String = if heavy ? "heavy" else "light";
}
part def FirstStage :> Stage {
	attribute :>> dryMass default = 130000 [kg];
	attribute :>> propellantMass = 2160000 [kg];
	attribute :>> engines default = 5;
}
part def UpperStage :> Stage {
	attribute :>> dryMass = 15000 [kg];
	attribute :>> propellantMass = 104000 [kg];
}
part def Vehicle {
	part s1 : FirstStage;
	part s2 : FirstStage {
		attribute :>> dryMass = 120000 [kg];
		attribute :>> engines = 3;
	}
	part s3 : UpperStage;
	attribute liftoffMass :> ISQ::mass = s1.mass + s2.mass + s3.mass;
	attribute engineCount : Integer = s1.engines + s2.engines + s3.engines;
	attribute topMass :> ISQ::mass = s3.mass;
}
part rocket : Vehicle;
part fleet {
	part rocket : Vehicle;
	part spare : UpperStage;
}
`

const derivedMassesQuery = `
calc def Masses :> Query {
	in root : Element;
	Project(
		source = WhereType(source = Descendants(source = root, maxDepth = 1), type = "PartUsage"),
		properties = ("name", "dryMass", "mass", "nozzles", "class")
	)
}
calc def Vehicles :> Query {
	in root : Element;
	Project(
		source = WhereType(source = Descendants(source = root, maxDepth = 1), type = "PartUsage"),
		properties = ("name", "liftoffMass", "engineCount", "topMass")
	)
}
`

func derivedFixture(t *testing.T, query string) executionFixture {
	t.Helper()
	return loadExecutionFixture(t, derivedBody+query)
}

// integerTexts renders one column of integer cells.
func integerTexts(t *testing.T, result *RowSet, column int) []int64 {
	t.Helper()
	var out []int64
	for _, row := range result.Rows() {
		values := row.Cells()[column].Values()
		if len(values) != 1 {
			t.Fatalf("cell %d has %d values, want 1", column, len(values))
		}
		n, ok := values[0].Integer()
		if !ok {
			t.Fatalf("cell %d holds %s, want an integer", column, values[0].Kind())
		}
		out = append(out, n)
	}
	return out
}

// TestExecuteProjectsDerivedValuesFromTheCarrier: `mass = dryMass + propellantMass`
// reads each leaf through the row's own redefinitions — the type-level `:>>` of
// FirstStage for s1, the usage-level `:>>` of s2 for s2 — keeping unit and
// Integer-ness, and conditionals and comparisons evaluate over the result.
func TestExecuteProjectsDerivedValuesFromTheCarrier(t *testing.T) {
	fixture := derivedFixture(t, derivedMassesQuery)
	result := quantityRows(t, fixture, "Masses", "Vehicle")
	if got := cellTexts(t, result, 0); !slices.Equal(got, []string{"s1", "s2", "s3"}) {
		t.Fatalf("names = %v", got)
	}
	if got := cellTexts(t, result, 1); !slices.Equal(got, []string{"130000 [kg]", "120000 [kg]", "15000 [kg]"}) {
		t.Fatalf("dry masses = %v", got)
	}
	if got := cellTexts(t, result, 2); !slices.Equal(got, []string{"2290000 [kg]", "2280000 [kg]", "119000 [kg]"}) {
		t.Fatalf("masses = %v", got)
	}
	if got := integerTexts(t, result, 3); !slices.Equal(got, []int64{10, 6, 2}) {
		t.Fatalf("nozzles = %v", got)
	}
	if got := cellTexts(t, result, 4); !slices.Equal(got, []string{"heavy", "heavy", "light"}) {
		t.Fatalf("classes = %v", got)
	}
	mass := result.Rows()[0].Cells()[2].Values()[0]
	magnitude, ok := mass.Magnitude()
	if n, isInt := magnitude.Integer(); !ok || !isInt || n != 2290000 {
		t.Fatalf("magnitude = %+v, want Integer 2290000", magnitude)
	}
	if !mass.Origin().Located() {
		t.Fatal("derived cells must retain provenance")
	}
}

// TestExecuteProjectsFeatureChainsIntoOwnedParts: `s1.mass + s2.mass + s3.mass`
// reads through the carrier's owned parts, each with its own redefinitions.
func TestExecuteProjectsFeatureChainsIntoOwnedParts(t *testing.T) {
	fixture := derivedFixture(t, derivedMassesQuery)
	result := quantityRows(t, fixture, "Vehicles", "fleet")
	if got := cellTexts(t, result, 0); !slices.Equal(got, []string{"rocket", "spare"}) {
		t.Fatalf("names = %v", got)
	}
	rocket := result.Rows()[0].Cells()
	if quantity, ok := rocket[1].Values()[0].Quantity(); !ok || quantity.String() != "4689000 [kg]" {
		t.Fatalf("liftoff mass = %+v, want 4689000 [kg]", rocket[1].Values())
	}
	if n, ok := rocket[2].Values()[0].Integer(); !ok || n != 9 {
		t.Fatalf("engine count = %+v, want 9", rocket[2].Values())
	}
	if quantity, ok := rocket[3].Values()[0].Quantity(); !ok || quantity.String() != "119000 [kg]" {
		t.Fatalf("top mass = %+v, want 119000 [kg]", rocket[3].Values())
	}
	// spare is an UpperStage: Vehicle's features are absent on it, not errors.
	for column := 1; column < 4; column++ {
		if values := result.Rows()[1].Cells()[column].Values(); len(values) != 0 {
			t.Fatalf("spare column %d = %v, want absent", column, values)
		}
	}
}

// TestExecuteAgreesWithTheRuntimeOnDerivedValues: the query path and a full run
// of the model read the same 2290000 [kg] and 4689000 [kg].
func TestExecuteAgreesWithTheRuntimeOnDerivedValues(t *testing.T) {
	fixture := derivedFixture(t, derivedMassesQuery)
	ctx := runtime.NewContext(runtime.NewModel(fixture.model, fixture.resolver), runtime.DefaultMaxSteps)
	rocket, err := ctx.Instantiate(fixture.symbol(t, "rocket"))
	if err != nil {
		t.Fatalf("Instantiate rocket: %v", err)
	}
	run := func(inst *runtime.Instance, feature string) string {
		fv, err := inst.GetFeatureValue(ctx, feature)
		if err != nil {
			t.Fatalf("runtime %s: %v", feature, err)
		}
		value, err := fv.ReadValue(feature)
		if err != nil {
			t.Fatalf("runtime read %s: %v", feature, err)
		}
		return runtime.FormatValue(value)
	}
	s1, err := rocket.GetFeatureValue(ctx, "s1")
	if err != nil {
		t.Fatalf("runtime s1: %v", err)
	}
	s1Value, err := s1.ReadValue("s1")
	if err != nil {
		t.Fatalf("runtime read s1: %v", err)
	}
	s1ID, ok := s1Value.Object()
	if !ok {
		t.Fatalf("runtime s1 = %s, want an object", runtime.FormatValue(s1Value))
	}
	s1Instance, ok := ctx.Instance(s1ID)
	if !ok {
		t.Fatalf("runtime holds no instance %d", s1ID)
	}

	masses := quantityRows(t, fixture, "Masses", "Vehicle")
	if got, want := cellTexts(t, masses, 2)[0], run(s1Instance, "mass"); got != want || want != "2290000 [kg]" {
		t.Fatalf("query s1.mass = %s, runtime = %s, want 2290000 [kg]", got, want)
	}
	vehicles := quantityRows(t, fixture, "Vehicles", "fleet")
	liftoff, ok := vehicles.Rows()[0].Cells()[1].Values()[0].Quantity()
	if want := run(rocket, "liftoffMass"); !ok || liftoff.String() != want || want != "4689000 [kg]" {
		t.Fatalf("query liftoffMass = %v, runtime = %s, want 4689000 [kg]", liftoff, want)
	}
}

// TestExecuteFiltersAndOrdersDerivedQuantities: WhereFeature and OrderBy read
// derived masses with the commensurability rules of literal ones.
func TestExecuteFiltersAndOrdersDerivedQuantities(t *testing.T) {
	fixture := derivedFixture(t, `
calc def Heavy :> Query {
	in root : Element;
	Project(
		source = WhereFeature(
			source = WhereType(source = Descendants(source = root, maxDepth = 1), type = "PartUsage"),
			'feature' = "mass",
			operator = ">",
			value = "2285000"
		),
		properties = ("name", "mass")
	)
}
calc def Ordered :> Query {
	in root : Element;
	OrderBy(
		source = Project(
			source = WhereType(source = Descendants(source = root, maxDepth = 1), type = "PartUsage"),
			properties = ("name", "mass")
		),
		property = "mass",
		direction = "ascending",
		missing = "last",
		multiple = "error"
	)
}
calc def OrderedGrams :> Query {
	in root : Element;
	OrderBy(
		source = WhereType(source = Descendants(source = root, maxDepth = 1), type = "PartUsage"),
		property = "mass",
		direction = "descending",
		missing = "last",
		multiple = "error"
	)
}
part mixed {
	part s1 : FirstStage;
	part light : Stage {
		attribute :>> dryMass = 500 [g];
		attribute :>> propellantMass = 250000 [g];
	}
}
part def Rod {
	attribute dryMass :> ISQ::mass = 1 [kg];
	attribute length :> ISQ::length = 1 [m];
	attribute mass = dryMass + length;
}
part broken {
	part s1 : FirstStage;
	part rod : Rod;
}
`)
	heavy := quantityRows(t, fixture, "Heavy", "Vehicle")
	if got := cellTexts(t, heavy, 0); !slices.Equal(got, []string{"s1"}) {
		t.Fatalf("heavy names = %v", got)
	}
	ordered := quantityRows(t, fixture, "Ordered", "Vehicle")
	if got := cellTexts(t, ordered, 0); !slices.Equal(got, []string{"s3", "s2", "s1"}) {
		t.Fatalf("ordered names = %v", got)
	}
	grams := quantityRows(t, fixture, "OrderedGrams", "mixed")
	var names []string
	for _, row := range grams.Rows() {
		sym, _ := row.Element().Element()
		names = append(names, sym.Name)
	}
	if !slices.Equal(names, []string{"s1", "light"}) {
		t.Fatalf("grams order = %v", names)
	}
	_, err := fixture.execute(t, "Ordered", Bindings{
		"root": {ElementValue(fixture.symbol(t, "broken"))},
	}, Options{})
	executionErr := executionError(t, err, ErrorUnevaluableFeature)
	if executionErr.Target != "Observatory::broken::rod" || !errors.Is(err, semantics.ErrIncommensurableUnits) {
		t.Fatalf("error = %v, want incommensurable units for Observatory::broken::rod", err)
	}
}

// TestExecuteComputesColumnsOverDerivedValues: a computed column reads the
// derived mass of each row and applies quantity arithmetic to it.
func TestExecuteComputesColumnsOverDerivedValues(t *testing.T) {
	fixture := derivedFixture(t, `
calc def Derived :> Query {
	in root : Element;
	Project(
		source = WhereType(source = Descendants(source = root, maxDepth = 1), type = "PartUsage"),
		properties = ("name"),
		columns = (
			Column(name = "twice", expression = Stage::mass * 2),
			Column(name = "dry", expression = Stage::mass - Stage::propellantMass),
			Column(name = "perEngine", expression = Stage::mass / Stage::engines)
		)
	)
}
`)
	result := quantityRows(t, fixture, "Derived", "Vehicle")
	cases := []struct {
		column int
		want   []string
	}{
		{1, []string{"4580000 [kg]", "4560000 [kg]", "238000 [kg]"}},
		{2, []string{"130000 [kg]", "120000 [kg]", "15000 [kg]"}},
		{3, []string{"458000.0 [kg]", "760000.0 [kg]", "119000.0 [kg]"}},
	}
	for _, tc := range cases {
		if got := cellTexts(t, result, tc.column); !slices.Equal(got, tc.want) {
			t.Errorf("column %s = %v, want %v", result.Columns()[tc.column].Name(), got, tc.want)
		}
	}
}

// TestExecuteReadsAnUnboundDerivedLeafAsAbsent: a derived mass over features
// nothing binds is an empty cell, as a value-less feature is.
func TestExecuteReadsAnUnboundDerivedLeafAsAbsent(t *testing.T) {
	fixture := derivedFixture(t, derivedMassesQuery+`
part abstractOnly {
	part bare : Stage;
	part s1 : FirstStage;
}
`)
	result := quantityRows(t, fixture, "Masses", "abstractOnly")
	if got := cellTexts(t, result, 0); !slices.Equal(got, []string{"bare", "s1"}) {
		t.Fatalf("names = %v", got)
	}
	bare := result.Rows()[0].Cells()
	for _, column := range []int{1, 2, 4} {
		if values := bare[column].Values(); len(values) != 0 {
			t.Fatalf("bare %s = %v, want absent", result.Columns()[column].Name(), values)
		}
	}
	if got := integerTexts(t, result, 3); !slices.Equal(got, []int64{2, 10}) {
		t.Fatalf("nozzles = %v, want the default-derived 2 and 10", got)
	}
}

// TestExecuteProjectsCollectionsAndReferences: library functions over a bound
// collection evaluate through the runtime (`sum`, `size`, indexing, `->collect`),
// a collection-valued feature fills the cell, a reference to a sibling attribute
// reads its value while a reference to a package-level element (a unit) stays
// the element it names, and an object is no cell value.
func TestExecuteProjectsCollectionsAndReferences(t *testing.T) {
	fixture := derivedFixture(t, `
private import SequenceFunctions::*;
private import RealFunctions::sum;
private import ControlFunctions::collect;

part def Crate { attribute mass :> ISQ::mass; }
part def Pallet {
	part crates : Crate [*];
	attribute itemMasses :> ISQ::mass [*] = crates->collect { in c : Crate; c.mass };
	attribute total :> ISQ::mass = sum(itemMasses);
	attribute items : Natural = size(itemMasses);
	attribute last :> ISQ::mass = itemMasses#(3);
	attribute alias :> ISQ::mass = total;
	attribute massUnit = SI::kg;
	part engine { part core; }
	attribute heart = engine.core;
}
part yard {
	part pallet : Pallet {
		part :>> crates = (a, b, c);
		part a : Crate { attribute :>> mass = 1 [kg]; }
		part b : Crate { attribute :>> mass = 2500 [g]; }
		part c : Crate { attribute :>> mass = 3 [kg]; }
	}
}
calc def Pallets :> Query {
	in root : Element;
	Project(
		source = WhereType(source = Descendants(source = root, maxDepth = 1), type = "PartUsage"),
		properties = ("name", "total", "items", "last", "alias", "massUnit", "itemMasses")
	)
}
calc def Hearts :> Query {
	in root : Element;
	Project(
		source = WhereType(source = Descendants(source = root, maxDepth = 1), type = "PartUsage"),
		properties = ("name", "heart")
	)
}
`)
	result := quantityRows(t, fixture, "Pallets", "yard")
	cells := result.Rows()[0].Cells()
	for column, want := range map[int]string{1: "6.5 [kg]", 3: "3 [kg]", 4: "6.5 [kg]", 5: "SI::kilogram"} {
		if got := cellTexts(t, result, column); !slices.Equal(got, []string{want}) {
			t.Errorf("%s = %v, want %q", result.Columns()[column].Name(), got, want)
		}
	}
	if got := integerTexts(t, result, 2); !slices.Equal(got, []int64{3}) {
		t.Errorf("items = %v, want 3", got)
	}
	var masses []string
	for _, value := range cells[6].Values() {
		quantity, ok := value.Quantity()
		if !ok {
			t.Fatalf("itemMasses holds %s, want quantities", value.Kind())
		}
		masses = append(masses, quantity.String())
	}
	if !slices.Equal(masses, []string{"1 [kg]", "2500 [g]", "3 [kg]"}) {
		t.Errorf("itemMasses = %v", masses)
	}
	_, err := fixture.execute(t, "Hearts", Bindings{
		"root": {ElementValue(fixture.symbol(t, "yard"))},
	}, Options{})
	executionErr := executionError(t, err, ErrorUnevaluableFeature)
	if executionErr.Property != "heart" || executionErr.Target != "Observatory::yard::pallet" {
		t.Errorf("error names %s of %s, want heart of Observatory::yard::pallet", executionErr.Property, executionErr.Target)
	}
	if !strings.Contains(executionErr.Error(), "not a value") {
		t.Errorf("message = %q, want the object named as no value", executionErr.Error())
	}
}

// TestExecuteReadsPackageAttributesAndKeepsElementReferences: a value naming a
// package-level attribute reads that attribute's value; one naming a unit, an
// enumeration literal or an object feature stays the element it names.
func TestExecuteReadsPackageAttributesAndKeepsElementReferences(t *testing.T) {
	fixture := derivedFixture(t, `
attribute base : Integer = 2;
attribute unitMass :> ISQ::mass = 5 [kg];
enum def Color { red; blue; }
part def Thing {
	attribute engines : Integer = base;
	attribute m :> ISQ::mass = unitMass;
	attribute doubled :> ISQ::mass = unitMass * 2;
	part engine;
	attribute heart = engine;
	attribute color : Color = Color::red;
	attribute unit = SI::kg;
}
part yard { part thing : Thing; }
calc def Things :> Query {
	in root : Element;
	Project(
		source = WhereType(source = Descendants(source = root, maxDepth = 1), type = "PartUsage"),
		properties = ("name", "engines", "m", "doubled", "heart", "color", "unit")
	)
}
`)
	result := quantityRows(t, fixture, "Things", "yard")
	if got := integerTexts(t, result, 1); !slices.Equal(got, []int64{2}) {
		t.Errorf("engines = %v, want 2", got)
	}
	for column, want := range map[int]string{2: "5 [kg]", 3: "10 [kg]", 4: "Observatory::Thing::engine", 5: "Observatory::Color::red", 6: "SI::kilogram"} {
		if got := cellTexts(t, result, column); !slices.Equal(got, []string{want}) {
			t.Errorf("%s = %v, want %q", result.Columns()[column].Name(), got, want)
		}
	}
}

// TestExecuteRollsUpDerivedValuesOverSubsettedParts: `mass + sum(subcomponents.totalMass)`
// recurses through parts subsetting a multiplicity-many part, each level reading
// its own carrier, and `->collect`/`size` see the same collection.
func TestExecuteRollsUpDerivedValuesOverSubsettedParts(t *testing.T) {
	fixture := derivedFixture(t, `
private import SequenceFunctions::size;
private import RealFunctions::sum;
private import ControlFunctions::collect;

part def MassedComponent {
	part subcomponents : MassedComponent [*];
	attribute mass :> ISQ::mass;
	attribute totalMass :> ISQ::mass default = mass + sum(subcomponents.totalMass);
	attribute childMasses :> ISQ::mass [*] = subcomponents->collect { in c : MassedComponent; c.mass };
	attribute children : Natural = size(subcomponents);
}
part def Bolt :> MassedComponent {
	attribute :>> mass = 1 [kg];
	attribute :>> totalMass = mass;
}
part def Assembly :> MassedComponent {
	attribute :>> mass = 10 [kg];
	part b1 : Bolt :> subcomponents;
	part b2 : Bolt :> subcomponents;
}
part def Stack :> MassedComponent {
	attribute :>> mass = 100 [kg];
	part a1 : Assembly :> subcomponents;
	part a2 : Assembly :> subcomponents;
}
part depot { part stack : Stack; }
calc def Totals :> Query {
	in root : Element;
	Project(
		source = WhereType(source = Descendants(source = root, maxDepth = 1), type = "PartUsage"),
		properties = ("name", "totalMass", "childMasses", "children")
	)
}
`)
	for root, want := range map[string]struct {
		total    string
		children []string
		count    int64
	}{
		"depot":    {"124 [kg]", []string{"10 [kg]", "10 [kg]"}, 2},
		"Stack":    {"12 [kg]", []string{"1 [kg]", "1 [kg]"}, 2},
		"Assembly": {"1 [kg]", nil, 0},
	} {
		result := quantityRows(t, fixture, "Totals", root)
		if got := cellTexts(t, result, 1); !slices.Equal(got, slices.Repeat([]string{want.total}, len(result.Rows()))) {
			t.Errorf("%s: totalMass = %v, want every row %s", root, got, want.total)
		}
		var masses []string
		for _, value := range result.Rows()[0].Cells()[2].Values() {
			quantity, ok := value.Quantity()
			if !ok {
				t.Fatalf("%s: childMasses holds %s, want quantities", root, value.Kind())
			}
			masses = append(masses, quantity.String())
		}
		if !slices.Equal(masses, want.children) {
			t.Errorf("%s: childMasses = %v, want %v", root, masses, want.children)
		}
		if got := integerTexts(t, result, 3); got[0] != want.count {
			t.Errorf("%s: children = %v, want %d", root, got, want.count)
		}
	}
}

// TestExecuteProjectsEmptyQuantitySumsAndDefaultedSubsettedParts: the rollup as
// a library writes it — `subcomponents [*] default null`, and no leaf redefining
// `totalMass` — projects a leaf's mass plus the zero mass of its empty
// subcomponents, and a stack's mass plus the masses of the parts subsetting
// `subcomponents` despite the default.
func TestExecuteProjectsEmptyQuantitySumsAndDefaultedSubsettedParts(t *testing.T) {
	fixture := derivedFixture(t, `
private import NumericalFunctions::sum;

part def MassedComponent {
	part subcomponents : MassedComponent [*] default null;
	attribute mass :> ISQ::mass;
	attribute totalMass :> ISQ::mass = mass + sum(subcomponents.totalMass);
	attribute childMasses :> ISQ::mass [*] = subcomponents.totalMass;
}
part def Leaf :> MassedComponent {
	attribute :>> mass = 100 [kg];
}
part def Stack :> MassedComponent {
	attribute :>> mass = 10 [kg];
	part a : Leaf :> subcomponents;
	part b : Leaf subsets subcomponents;
}
part hangar {
	part leaf : Leaf;
	part stack : Stack;
}
calc def Totals :> Query {
	in root : Element;
	Project(
		source = WhereType(source = Descendants(source = root, maxDepth = 1), type = "PartUsage"),
		properties = ("name", "totalMass", "childMasses")
	)
}
`)
	result := quantityRows(t, fixture, "Totals", "hangar")
	if got := cellTexts(t, result, 0); !slices.Equal(got, []string{"leaf", "stack"}) {
		t.Fatalf("names = %v", got)
	}
	if got := cellTexts(t, result, 1); !slices.Equal(got, []string{"100 [kg]", "210 [kg]"}) {
		t.Errorf("totalMass = %v, want [100 [kg] 210 [kg]]", got)
	}
	for row, want := range [][]string{nil, {"100 [kg]", "100 [kg]"}} {
		var masses []string
		for _, value := range result.Rows()[row].Cells()[2].Values() {
			quantity, ok := value.Quantity()
			if !ok {
				t.Fatalf("row %d: childMasses holds %s, want quantities", row, value.Kind())
			}
			masses = append(masses, quantity.String())
		}
		if !slices.Equal(masses, want) {
			t.Errorf("row %d: childMasses = %v, want %v", row, masses, want)
		}
	}
	if got := cellTexts(t, quantityRows(t, fixture, "Totals", "Stack"), 1); !slices.Equal(got, []string{"100 [kg]", "100 [kg]"}) {
		t.Errorf("Stack's subsetting members' totalMass = %v, want [100 [kg] 100 [kg]]", got)
	}
}

// TestExecuteNamesTheSubexpressionOfAnUnevaluableDerivedValue: a value that is
// not declaratively evaluable — an unbound `in` parameter, a cycle, an object
// where a value is needed, a quantity summed with the dimensionless count that
// `sum` makes of an empty collection of Integers — is a typed error naming the
// row, the feature and the runtime's reason.
func TestExecuteNamesTheSubexpressionOfAnUnevaluableDerivedValue(t *testing.T) {
	fixture := derivedFixture(t, derivedMassesQuery+`
private import RealFunctions::sum;

part def Sensor :> Stage {
	action measure { in reading : Real; }
	attribute :>> dryMass = 1 [kg];
	attribute :>> propellantMass = measure.reading [kg];
}
part def Cyclic :> Stage {
	attribute :>> dryMass = 1 [kg];
	attribute :>> propellantMass = mass;
}
part def Objectified :> Stage {
	part engine { part core; }
	attribute :>> dryMass = 1 [kg];
	attribute :>> propellantMass = engine.core;
}
part def Leaf :> Stage {
	part tanks : Stage [*] default null;
	attribute :>> dryMass = 1 [kg];
	attribute :>> propellantMass = 1 [kg] + sum(tanks.nozzles);
}
part parametric { part sensor : Sensor; }
part cyclic { part loop : Cyclic; }
part objectified { part thing : Objectified; }
part childless { part leaf : Leaf; }
`)
	cases := []struct {
		root   string
		target string
		reason string
		cause  error
	}{
		{"parametric", "Observatory::parametric::sensor", "reading", runtime.ErrNoValue},
		{"cyclic", "Observatory::cyclic::loop", "cyclic", runtime.ErrCyclicFeatureValue},
		{"objectified", "Observatory::objectified::thing", "type mismatch", runtime.ErrTypeMismatch},
		{"childless", "Observatory::childless::leaf", "incommensurable units", nil},
	}
	for _, tc := range cases {
		_, err := fixture.execute(t, "Masses", Bindings{
			"root": {ElementValue(fixture.symbol(t, tc.root))},
		}, Options{})
		executionErr := executionError(t, err, ErrorUnevaluableFeature)
		if executionErr.Property != "mass" || executionErr.Target != tc.target {
			t.Errorf("%s: error names %s of %s, want mass of %s", tc.root, executionErr.Property, executionErr.Target, tc.target)
		}
		prefix := "query Observatory::Masses cannot evaluate feature mass of " + tc.target + ": "
		if !strings.HasPrefix(executionErr.Error(), prefix) || !strings.Contains(executionErr.Error(), tc.reason) {
			t.Errorf("%s: message = %q, want prefix %q mentioning %q", tc.root, executionErr.Error(), prefix, tc.reason)
		}
		if tc.cause != nil && !errors.Is(err, tc.cause) {
			t.Errorf("%s: error = %v, want cause %v", tc.root, err, tc.cause)
		}
		if !executionErr.Origin.Located() {
			t.Errorf("%s: error must retain query provenance", tc.root)
		}
	}
}
