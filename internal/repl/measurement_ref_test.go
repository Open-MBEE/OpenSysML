package repl

import (
	"regexp"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
)

// measurementRefSession declares a part measured in kilometres whose unit
// feature holds a measurement reference, with the SI units in scope.
func measurementRefSession(t *testing.T) *Session {
	t.Helper()
	s := NewSession()
	res := s.Submit(`package Survey {
		private import ISQ::*;
		private import SI::*;
		private import MeasurementReferences::*;
		part def Field {
			attribute width : LengthValue = 3 [km];
			attribute unit : LengthUnit = m;
			attribute area : AreaUnit = m * m;
		}
		part field : Field;
	}`)
	if len(res.Diagnostics) > 0 {
		t.Fatalf("fixture has diagnostics: %v", res.Diagnostics)
	}
	return s
}

// A unit named at the prompt is the measurement reference it declares, as is
// one composed from several; the reference of a quantity reads and compares.
func TestEvalMeasurementReferences(t *testing.T) {
	s := measurementRefSession(t)
	wants(t, run(t, s, "%eval m"), "✓ m", "= m")
	wants(t, run(t, s, "%eval SI::'m/s'"), "= 'm/s'")
	wants(t, run(t, s, "%eval km * s / m"), "= km*s/m")
	wants(t, run(t, s, "%eval field.unit"), "= m")
	wants(t, run(t, s, "%eval field.area"), "= m**2")
	wants(t, run(t, s, "%eval field.width.mRef"), "= km")
	wants(t, run(t, s, "%eval field.width.num"), "= 3")
	wants(t, run(t, s, "%eval field.width.mRef == km"), "= true")
	wants(t, run(t, s, "%eval field.width.mRef == field.unit"), "= false")
	wants(t, run(t, s, "%eval SI::'m/s' == m / s"), "= true")
}

// The quantity calculations compute over a reference: a number takes a unit,
// a quantity converts to one, and a reference renders as its unit text.
func TestEvalQuantityCalculationsOverReferences(t *testing.T) {
	s := measurementRefSession(t)
	wants(t, run(t, s, "%eval QuantityCalculations::ConvertQuantity(field.width, field.unit)"), "= 3000.0 [m]")
	wants(t, run(t, s, "%eval QuantityCalculations::ConvertQuantity(2 [m/s], SI::'km/h')"), "= 7.2 ['km/h']")
	wants(t, run(t, s, "%eval QuantityCalculations::'['(2.5, km)"), "= 2.5 [km]")
	wants(t, run(t, s, `%eval MeasurementRefCalculations::ToString(m ** 2)`), `= "m**2"`)
	wants(t, run(t, s, "%eval QuantityCalculations::ConvertQuantity(field.width, s)"),
		"error:", "incommensurable units")
}

// What the runtime does not hold about a unit is a typed failure, never a made-up
// value: members of a unit composed at the prompt, and the operators the library
// defines over no reference.
func TestEvalMeasurementReferenceLimitsAreTyped(t *testing.T) {
	s := measurementRefSession(t)
	wants(t, run(t, s, "%eval (m / s).unitConversion"),
		"error:", "library function is not evaluable",
		"MeasurementReferences::DerivedUnit::unitConversion: m/s is a MeasurementReferences::DerivedUnit reducing to metre·second^-1, which names no declaration")
	wants(t, run(t, s, "%eval field.area.quantityDimension"),
		"error:", "library function is not evaluable",
		"MeasurementReferences::DerivedUnit::quantityDimension: m**2 is a MeasurementReferences::DerivedUnit reducing to metre^2")
	wants(t, run(t, s, "%eval m + m"), "error:", "operator '+' is not defined for a measurement reference")
	wants(t, run(t, s, "%eval m * 3"), "error:", "operator '*' is not defined for a measurement reference and an Integer")
}

// A named unit answers its declaration's members, redefinitions and defaults followed, from
// the object %features shows, and keeps that object across an unrelated declaration.
func TestEvalMeasurementReferenceDeclarationMembers(t *testing.T) {
	s := measurementRefSession(t)
	wants(t, run(t, s, "%eval SI::km.unitConversion.conversionFactor"), "= 1000.0")
	wants(t, run(t, s, "%eval km.unitConversion.referenceUnit"), "= m")
	wants(t, run(t, s, "%eval km.unitConversion.isExact"), "= true")
	wants(t, run(t, s, "%eval QuantityCalculations::ConvertQuantity(field.width, km.unitConversion.referenceUnit)"), "= 3000.0 [m]")
	wants(t, run(t, s, "%eval m.quantityDimension.quantityPowerFactors#(1).exponent"), "= 1")
	wants(t, run(t, s, "%eval m.unitPowerFactors#(1).unit"), "= m")
	wants(t, run(t, s, "%eval K.definitionalQuantityValues#(1).num"), "= [273.16]")
	wants(t, run(t, s, "%eval m.isBound"), "= false")
	wants(t, run(t, s, "%eval m.unitConversion"), "= []")

	// The %instantiate is a second object of the declaration, which the name now
	// denotes; a member read at the prompt reaches that object, the one %features shows.
	created := run(t, s, "%instantiate SI::km")
	wants(t, created, "✓ Created instance of SI::kilometre")
	km := "SI::kilometre (ID: " + idIn(t, created, `ID: (\d+)`) + ")"
	listing := run(t, s, "%features SI::km")
	wantsInOrder(t, listing,
		"unitConversion = Instance(ID: ", "prefix = Instance(ID: ", `longName = "kilo"`,
		"referenceUnit = m", "conversionFactor = 1000.0", "isExact = true",
		"quantityDimension = Instance(ID: ", "exponent = 1",
		"unitPowerFactors = Instance(ID: ", "unit = km", "exponent = 1",
		"isBound = false", "mRefs = [km]", "definitionalQuantityValues = []")
	conversionID := idIn(t, listing, `unitConversion = Instance\(ID: (\d+)\)`)
	conversion := "Instance(ID: " + conversionID + ")"
	wants(t, run(t, s, "%eval SI::km.unitConversion"), "= "+conversion)
	wants(t, run(t, s, "%features SI::km.unitConversion"), "Instance: SI::kilometre.unitConversion (ID: "+conversionID+")", "conversionFactor = 1000.0")

	// An unrelated declaration leaves the object, and the records it holds, as they were.
	if res := s.Submit("part def Widget;"); len(res.Notices) != 0 {
		t.Fatalf("unrelated declaration reported %v", res.Notices)
	}
	wants(t, run(t, s, "%instances"), km)
	wants(t, run(t, s, "%eval SI::km.unitConversion"), "= "+conversion)
	wants(t, run(t, s, "%eval SI::km.unitConversion.conversionFactor"), "= 1000.0")
	wants(t, run(t, s, "%features SI::km"), "Instance: "+km, "unitConversion = "+conversion)
}

// A model's own object typed by a library record carries the record's members, redefinitions
// and defaults followed, an optional record whose body binds only at depth included; a
// model's own reference answers the inherited `isBound default false`.
func TestFeaturesOfModelOwnedLibraryRecords(t *testing.T) {
	s := NewSession()
	res := s.Submit(`package Lab {
		private import ISQ::*;
		private import SI::*;
		private import MeasurementReferences::*;
		attribute myConv : ConversionByPrefix { :>> prefix = kilo; :>> referenceUnit = m; }
		attribute furlong : LengthUnit { :>> unitConversion : ConversionByConvention { :>> referenceUnit = m; :>> conversionFactor = 201.168; } }
		attribute stressRef : TensorMeasurementReference { :>> mRefs = (Pa, Pa); :>> dimensions = (2); }
		attribute def Box { attribute conv : ConversionByPrefix[0..1]; }
		attribute deep : Box { :>> conv { :>> prefix { :>> conversionFactor = 1000.0; } } }
		attribute bare : Box;
		attribute hollow : Box { :>> conv { abstract attribute extra : UnitPrefix { :>> conversionFactor = 1000.0; } } }
	}`)
	if len(res.Diagnostics) > 0 {
		t.Fatalf("fixture has diagnostics: %v", res.Diagnostics)
	}
	run(t, s, "%instantiate Lab::myConv")
	wantsInOrder(t, run(t, s, "%features Lab::myConv"),
		"prefix = Instance(ID: ", `longName = "kilo"`, "referenceUnit = m", "conversionFactor = 1000.0", "isExact = true")
	wants(t, run(t, s, "%eval Lab::myConv.conversionFactor"), "= 1000.0")
	wants(t, run(t, s, "%eval Lab::furlong.unitConversion.conversionFactor"), "= 201.168")
	wants(t, run(t, s, "%eval QuantityCalculations::ConvertQuantity(2 [Lab::furlong], m)"), "= 402.336 [m]")
	wants(t, run(t, s, "%eval Lab::deep.conv.prefix.conversionFactor"), "= 1000.0")
	wants(t, run(t, s, "%eval Lab::deep.conv.conversionFactor"), "= 1000.0")
	wants(t, run(t, s, "%eval Lab::bare.conv"), "= "+runtime.UndeterminedText)
	wants(t, run(t, s, "%eval Lab::hollow.conv"), "= "+runtime.UndeterminedText)
	wants(t, run(t, s, "%eval Lab::stressRef.isBound"), "= false")
	run(t, s, "%instantiate Lab::stressRef")
	wantsInOrder(t, run(t, s, "%features Lab::stressRef"), "mRefs = [Pa, Pa]", "dimensions = [2]", "isBound = false")
}

// idIn is the identity the first match of pattern in out captures.
func idIn(t *testing.T, out, pattern string) string {
	t.Helper()
	m := regexp.MustCompile(pattern).FindStringSubmatch(out)
	if m == nil {
		t.Fatalf("no %s in output:\n%s", pattern, out)
	}
	return m[1]
}

// %features lists the reference an attribute holds beside the quantities.
func TestFeaturesListMeasurementReferences(t *testing.T) {
	s := measurementRefSession(t)
	run(t, s, "%instantiate Survey::field")
	wants(t, run(t, s, "%features Survey::field"), "\n  width = 3 [km]", "\n  unit = m", "\n  area = m**2")
}
