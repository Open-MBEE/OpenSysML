package queryexec

import (
	"errors"
	"testing"
)

// TestQueryRobustnessValuation exercises what the valuation property does with
// a value the model cannot give a cell: a cycle, an unbound parameter, an
// object, and a metadata path whose def nothing or several things name. Each
// is a typed error or an empty cell, never a panic or a hang.
func TestQueryRobustnessValuation(t *testing.T) {
	t.Run("cyclic_feature_values", testValuationCycle)
	t.Run("value_reading_an_unbound_parameter", testValuationUnboundParameter)
	t.Run("value_naming_a_part_holds_the_element", testValuationNamesElement)
	t.Run("metadata_path_naming_no_def", testMetadataPathNamingNoDef)
	t.Run("metadata_path_naming_a_non_metadata_type", testMetadataPathNamingPartDef)
	t.Run("metadata_path_to_an_undeclared_feature", testMetadataPathUndeclaredFeature)
}

func valuationQuery(source string) string {
	return `
	calc def Probe :> Query {
		Project(source = ` + source + `, properties = ("name", "valuation"))
	}
`
}

func probeError(t *testing.T, body string) error {
	t.Helper()
	fixture := loadExecutionFixture(t, body)
	_, err := fixture.execute(t, "Probe", nil, Options{})
	return err
}

// Two features valued by each other have no constant and no evaluable value:
// the column fails with the runtime's cycle, as a property naming them would.
func testValuationCycle(t *testing.T) {
	err := probeError(t, `
	part def Loop { attribute a : Real = b; attribute b : Real = a; }
`+valuationQuery(`WhereType(source = OwnedElements(source = Named(qualifiedName = "Observatory::Loop")), type = "AttributeUsage")`))
	var typed *Error
	if !errors.As(err, &typed) || typed.Kind != ErrorUnevaluableFeature {
		t.Fatalf("cycle error = %v, want an unevaluable feature", err)
	}
}

// A value computed from a parameter nothing binds cannot be evaluated: the
// column fails with a typed error carrying the runtime's reason, as the
// property form reading the same feature from its owner does.
func testValuationUnboundParameter(t *testing.T) {
	err := probeError(t, `
	calc def Scale { in factor : Real; attribute scaled : Real = factor * 2.0; return : Real = scaled; }
`+valuationQuery(`WhereType(source = OwnedElements(source = Named(qualifiedName = "Observatory::Scale")), type = "AttributeUsage")`))
	var typed *Error
	if !errors.As(err, &typed) || typed.Kind != ErrorUnevaluableFeature {
		t.Fatalf("unbound parameter error = %v, want an unevaluable feature", err)
	}
}

// A part bound to another part names an element of the model: the cell holds
// that element, as a declared value naming a literal or a unit does.
func testValuationNamesElement(t *testing.T) {
	fixture := loadExecutionFixture(t, `
	part def Engine;
	part def Car { part spare : Engine; part fitted : Engine = spare; }
`+valuationQuery(`WhereType(source = OwnedElements(source = Named(qualifiedName = "Observatory::Car")), type = "PartUsage")`))
	result, err := fixture.execute(t, "Probe", nil, Options{})
	if err != nil {
		t.Fatalf("Probe: %v", err)
	}
	rows := result.Rows()
	if len(rows) != 2 || cellText(rows[0].Cells()[1]) != "" || cellText(rows[1].Cells()[1]) != "spare" {
		t.Fatalf("rows = %v, want spare unbound and fitted holding spare", rows)
	}
}

func metadataProbe(property string) string {
	return `
	metadata def Tag { attribute key : String; }
	part def Thing { @Tag { key = "k"; } }
	calc def Probe :> Query {
		Project(source = Named(qualifiedName = "Observatory::Thing"), properties = ("name", "` + property + `"))
	}
`
}

func wantUnknownProperty(t *testing.T, err error, property string) {
	t.Helper()
	var typed *Error
	if !errors.As(err, &typed) || typed.Kind != ErrorUnknownProperty || typed.Property != property {
		t.Fatalf("error = %v, want unknown property %s", err, property)
	}
}

func testMetadataPathNamingNoDef(t *testing.T) {
	wantUnknownProperty(t, probeError(t, metadataProbe("Nowhere::key")), "Nowhere::key")
}

func testMetadataPathNamingPartDef(t *testing.T) {
	wantUnknownProperty(t, probeError(t, metadataProbe("Thing::key")), "Thing::key")
}

func testMetadataPathUndeclaredFeature(t *testing.T) {
	wantUnknownProperty(t, probeError(t, metadataProbe("Tag::missing")), "Tag::missing")
}
