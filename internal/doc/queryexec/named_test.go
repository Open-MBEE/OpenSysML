package queryexec

import (
	"strings"
	"testing"
)

const namedBody = `
package Vehicle {
	package Config {
		part def Wheel;
		individual part def FrontLeft :> Wheel;
		individual part def Spare :> Wheel;
	}
	package 'Spare Parts' {
		part def Pad;
	}
	alias Wheels for Config;
}
package Other {
	package Config;
}
package Aliases {
	alias Broken for Missing;
	alias Loop1 for Loop2;
	alias Loop2 for Loop1;
}
calc def ByName :> Query {
	in qualifiedName : String[1..*] ordered;
	Named(qualifiedName = qualifiedName)
}
calc def ConfigWheels :> Query {
	WhereFeature(
		source = WhereType(source = Descendants(source = Named(qualifiedName = "Vehicle::Config")), type = "Vehicle::Config::Wheel"),
		'feature' = "isIndividual", operator = "=", value = "true"
	)
}
`

func TestExecuteNamedResolvesQualifiedNamesInOrder(t *testing.T) {
	fixture := loadExecutionFixture(t, namedBody)
	run := func(names ...string) *RowSet {
		t.Helper()
		values := make([]Value, len(names))
		for i, name := range names {
			values[i] = StringValue(name)
		}
		result, err := fixture.execute(t, "ByName", Bindings{"qualifiedName": values}, Options{})
		if err != nil {
			t.Fatalf("ByName(%v): %v", names, err)
		}
		return result
	}
	// Argument order is row order, a quoted-in-notation name is spelled raw, a
	// full or unique partial qualified name both resolve, and a name given twice
	// yields its element once per mention.
	got := rowNames(run("Observatory::Vehicle::Spare Parts::Pad", "Vehicle::Config::Wheel", "Vehicle::Config::Wheel"))
	want := "Observatory::Vehicle::Spare Parts::Pad,Observatory::Vehicle::Config::Wheel,Observatory::Vehicle::Config::Wheel"
	if strings.Join(got, ",") != want {
		t.Fatalf("Named rows = %v, want %v", got, want)
	}
	// An alias names its target.
	if got := rowNames(run("Vehicle::Wheels")); strings.Join(got, ",") != "Observatory::Vehicle::Config" {
		t.Fatalf("Named alias = %v", got)
	}
	// A unique simple name resolves as it does for WhereType.
	if got := rowNames(run("Pad")); strings.Join(got, ",") != "Observatory::Vehicle::Spare Parts::Pad" {
		t.Fatalf("Named simple = %v", got)
	}
	for _, row := range run("Vehicle::Config").Rows() {
		if !row.Origin().Located() {
			t.Fatalf("row %v without provenance", row.Element())
		}
	}
	// The named element roots a traversal.
	wheels, err := fixture.execute(t, "ConfigWheels", nil, Options{})
	if err != nil {
		t.Fatalf("ConfigWheels: %v", err)
	}
	if got := rowNames(wheels); strings.Join(got, ",") != "Observatory::Vehicle::Config::FrontLeft,Observatory::Vehicle::Config::Spare" {
		t.Fatalf("Descendants(Named) = %v", got)
	}
}

// A name that resolves to nothing, to several elements, or to an alias that
// denotes no element (dangling or cyclic) is an unknown-element error, never a row.
func TestExecuteNamedRejectsUnknownAndAmbiguousNames(t *testing.T) {
	fixture := loadExecutionFixture(t, namedBody)
	for _, name := range []string{"Vehicle::Missing", "Config", "Aliases::Broken", "Aliases::Loop1", "Aliases::Loop2"} {
		_, err := fixture.execute(t, "ByName", Bindings{"qualifiedName": {StringValue(name)}}, Options{})
		unknown := executionError(t, err, ErrorUnknownElement)
		if unknown.Actual != name || unknown.Operation != "named" {
			t.Fatalf("Named(%q) error = %v", name, unknown)
		}
		if !strings.Contains(err.Error(), "names no single element "+name) {
			t.Fatalf("Named(%q) message = %v", name, err)
		}
	}
}
